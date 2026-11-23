package planet

import (
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/transport"
)

const fleetEpsilon = 1e-9

// DisableVesselAutoReplenishment 让新建星际塔由玩家按逐塔清单手放定量飞船。
// 用于施工蓝图收尾及已核验图纸的参数重编码，返回是否修改。[331] 为原版从角色背包自动补船的开关。
func DisableVesselAutoReplenishment(b *blueprint.Building) bool {
	if b.ItemID != 2104 || len(b.Parameters) != 2048 || b.Parameters[331] == 0 {
		return false
	}
	b.Parameters[331] = 0
	return true
}

type fleetFeed struct {
	building int32
	item     int
}

func (req *Request) addFleetFeed(building int32, item int, rate float64) {
	if req.exportFeeds == nil {
		req.exportFeeds = map[fleetFeed]float64{}
	}
	req.exportFeeds[fleetFeed{building, item}] += rate
}

// fleetHold 保存一座星际塔上各货物的船时。各槽共用一个总船位上限。
type fleetHold struct {
	object int
	used   float64
	load   []float64
}

type fleetSource struct{ object, slot int }

type fleetDispatch struct {
	req      *Request
	cargo    []industryplan.FleetCargo
	left     []float64
	holds    []fleetHold
	sources  map[int][]fleetSource
	rates    map[int]float64
	capacity int
}

// dispatchFleet 在真实货槽上求可行配船。先仅使用现有远供槽，再利用可接货的空槽，均不足才新建转运塔。
func dispatchFleet(ctx context.Context, req *Request, input ProductionRequest, catalog *placement.Catalog) ([]industryplan.FleetStation, error) {
	a := fleetDispatch{req: req, cargo: input.Plan.OutboundFleet, capacity: input.Plan.VesselsPerStation, sources: map[int][]fleetSource{}, rates: map[int]float64{}}
	if a.capacity == 0 {
		a.capacity = 10
	}
	if a.capacity < 1 || a.capacity > 10 {
		return nil, fmt.Errorf("每座星际塔的配船上限须在 1 至 10 艘之间")
	}
	beltFeeds := map[fleetFeed]bool{}
	for _, object := range req.Objects {
		b := object.Building
		if b.ItemID >= 2001 && b.ItemID <= 2003 && b.OutputObjIdx >= 0 && len(b.Parameters) > 0 && b.Parameters[0] > 0 {
			beltFeeds[fleetFeed{b.OutputObjIdx, int(b.Parameters[0])}] = true
		}
	}
	for i := range req.Objects {
		b := &req.Objects[i].Building
		var slots int
		switch b.ItemID {
		case 2104:
			slots = 5
		case 2103:
			slots = 4
		case 2316:
			slots = 1
		default:
			continue
		}
		if len(b.Parameters) < slots*6 {
			continue
		}
		if b.ItemID == 2104 {
			a.addHold(i)
		}
		for n := 0; n < slots; n++ {
			p := b.Parameters[n*6:]
			// 保留制造、采矿及入站中转原有的本地供给；纯远供只有实体带送入才可开放本地供给。
			item := int(p[0])
			key := fleetFeed{b.Index, item}
			if item > 0 && p[3] > 0 && (p[1] == 1 || p[1] == 0 && p[2] == 1 && (beltFeeds[key] || req.exportFeeds[key] > 0)) {
				a.sources[item] = append(a.sources[item], fleetSource{i, n})
			}
		}
	}
	for _, port := range input.Plan.Ports {
		if port.Kind == "export" {
			a.rates[port.ItemID] += port.RatePerMinute
		}
	}
	for _, c := range a.cargo {
		if c.ItemID <= 0 || c.VesselLoad <= 0 || math.IsNaN(c.VesselLoad) || math.IsInf(c.VesselLoad, 0) {
			return nil, fmt.Errorf("星球 %d 的货物船时无效", req.PlanetID)
		}
		if a.rates[c.ItemID] <= 0 {
			return nil, &Error{PlanetID: req.PlanetID, Item: c.ItemID, Code: "PLANET_FLEET_ROUTE", Message: "发货船时缺少对应的实际出口合同"}
		}
		a.left = append(a.left, c.VesselLoad)
	}
	for {
		for _, allowSlot := range []bool{false, true} {
			for i := range a.cargo {
				for a.left[i] > fleetEpsilon {
					if err := search.Checkpoint(ctx); err != nil {
						return nil, err
					}
					if !a.augment(i, allowSlot) {
						break
					}
				}
			}
		}
		i := slices.IndexFunc(a.left, func(v float64) bool { return v > fleetEpsilon })
		if i < 0 {
			break
		}
		item := a.cargo[i].ItemID
		if !a.enableSource(item, -1) {
			return nil, &Error{PlanetID: req.PlanetID, Item: item, Code: "PLANET_FLEET_ROUTE", Message: "现有发货船位不足，新增转运塔没有可实际接货的本地供给槽"}
		}
		pos, ok := stationGridSite(ctx, append(extendOccupancy(*req, catalog), req.minerals...), req.wet, catalog, true)
		if !ok {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, &Error{PlanetID: req.PlanetID, Item: item, Code: "PLANET_FLEET_SITE", Message: fmt.Sprintf("物品 %d 还缺 %.3f 艘的发货船位，找不到满足间距的新塔落点", item, a.left[i])}
		}
		tower := stationObject(nextObjectIndex(req.Objects), pos, item, 2, 1, input.Options.Setting.RequestStationWarpers)
		tower.Module = "船队转运"
		req.Objects = append(req.Objects, tower)
		a.addHold(len(req.Objects) - 1)
	}
	var stations []industryplan.FleetStation
	for _, h := range a.holds {
		if h.used <= fleetEpsilon {
			continue
		}
		object := req.Objects[h.object]
		// 往返次数、翘曲器及供电均按满船货物计算。只将本张蓝图实际配船的塔设为满载起运。
		if len(object.Building.Parameters) <= 327 {
			return nil, &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_ROUTE", Message: "发货塔缺少远程起运量参数"}
		}
		object.Building.Parameters[327] = 100
		ships := int(math.Ceil(h.used))
		if ships > a.capacity && h.used <= float64(a.capacity)+fleetEpsilon {
			ships = a.capacity
		}
		if ships > a.capacity {
			return nil, fmt.Errorf("运输塔 %d 的货物超过共享船位上限", object.Building.Index)
		}
		row := industryplan.FleetStation{BuildingIndex: int32(h.object), Longitude: math.Remainder(object.First.Longitude, 2*math.Pi) * 180 / math.Pi, Latitude: object.First.Latitude * 180 / math.Pi, Vessels: ships}
		for i, load := range h.load {
			if load > 0 {
				cargo := a.cargo[i]
				cargo.VesselLoad = load
				row.Cargo = append(row.Cargo, cargo)
			}
		}
		stations = append(stations, row)
	}
	if limit := input.Plan.FleetStationLimit; limit > 0 && len(stations) > limit {
		return nil, &Error{PlanetID: req.PlanetID, Code: "PLANET_FLEET_ROUTE", Message: fmt.Sprintf("实际配船需 %d 座发货塔，超过设定的 %d 座上限", len(stations), limit)}
	}
	return stations, nil
}

func (a *fleetDispatch) addHold(object int) {
	a.holds = append(a.holds, fleetHold{object: object, load: make([]float64, len(a.cargo))})
}

// source 只从补塔前已经存在的物料来源取货，不能让新塔彼此充当来源。
func (a *fleetDispatch) source(item, except int) (fleetSource, bool) {
	for _, s := range a.sources[item] {
		if s.object != except {
			return s, true
		}
	}
	return fleetSource{}, false
}

func (a *fleetDispatch) enableSource(item, except int) bool {
	found := false
	for _, s := range a.sources[item] {
		if s.object != except {
			a.req.Objects[s.object].Building.Parameters[s.slot*6+1] = 1
			found = true
		}
	}
	return found
}

// room 限制多源物品从直连生产塔可调度的船时。转运塔可用无人机汇集各来源；
// 生产塔只能发自己的出带量，同货多来源时按每条航线同比例安排直发。
func (a *fleetDispatch) room(h *fleetHold, i, n int) float64 {
	c := a.cargo[i]
	b := &a.req.Objects[h.object].Building
	p := b.Parameters[n*6:]
	limit := c.VesselLoad
	if p[0] != 0 && p[1] != 2 {
		if _, other := a.source(c.ItemID, h.object); other {
			rate := a.req.exportFeeds[fleetFeed{b.Index, c.ItemID}]
			limit *= min(1, rate/a.rates[c.ItemID])
		}
	}
	return max(0, limit-h.load[i])
}

// slot 返回已有远供槽，或在允许调整时返回能安全启用的槽。相同物品的既有远需和已有带口保持原角色。
func (a *fleetDispatch) slot(h *fleetHold, item int, allowSlot bool) (int, bool) {
	b := &a.req.Objects[h.object].Building
	n := stationSlot(b, item)
	if n >= 0 {
		p := b.Parameters[n*6:]
		if p[3] <= 0 || p[2] == 2 {
			return 0, false
		}
		if p[2] == 1 {
			if p[1] == 1 {
				return n, true
			}
			if p[1] == 2 {
				_, ok := a.source(item, h.object)
				return n, ok
			}
			for _, s := range a.sources[item] {
				if s.object == h.object && s.slot == n {
					return n, true
				}
			}
			return 0, false
		}
		if allowSlot && p[1] == 1 {
			return n, true
		}
		// 现有本地需求槽可能要给本塔连接的生产或能源设施供料，不改成发货塔。
		return 0, false
	}
	if allowSlot {
		if _, ok := a.source(item, h.object); ok {
			for n := range 5 {
				if b.Parameters[n*6] == 0 {
					return n, true
				}
			}
		}
	}
	return 0, false
}

// augment 沿货物→可发该货的塔→该塔已承担的其他货物找增广路。满塔上的货可挪到兼容的余量塔，避免贪心重复占用船位。
func (a *fleetDispatch) augment(first int, allowSlot bool) bool {
	parentCargo := make([]int, len(a.cargo))
	parentTower := make([]int, len(a.holds))
	for i := range parentCargo {
		parentCargo[i] = -1
	}
	for j := range parentTower {
		parentTower[j] = -1
	}
	parentCargo[first] = -2
	queue := []int{first}
	last := -1
	for len(queue) > 0 && last < 0 {
		i := queue[0]
		queue = queue[1:]
		for j := range a.holds {
			h := &a.holds[j]
			if parentTower[j] >= 0 {
				continue
			}
			n, ok := a.slot(h, a.cargo[i].ItemID, allowSlot)
			if !ok || a.room(h, i, n) <= fleetEpsilon {
				continue
			}
			parentTower[j] = i
			if float64(a.capacity)-h.used > fleetEpsilon {
				last = j
				break
			}
			for k, v := range h.load {
				if v > fleetEpsilon && parentCargo[k] == -1 {
					parentCargo[k] = j
					queue = append(queue, k)
				}
			}
		}
	}
	if last < 0 {
		return false
	}
	flow := min(a.left[first], float64(a.capacity)-a.holds[last].used)
	for j := last; ; {
		i := parentTower[j]
		n, _ := a.slot(&a.holds[j], a.cargo[i].ItemID, allowSlot)
		flow = min(flow, a.room(&a.holds[j], i, n))
		if i == first {
			break
		}
		j = parentCargo[i]
		flow = min(flow, a.holds[j].load[i])
	}
	for j := last; ; {
		i := parentTower[j]
		h := &a.holds[j]
		item := a.cargo[i].ItemID
		n, _ := a.slot(h, item, allowSlot)
		p := a.req.Objects[h.object].Building.Parameters[n*6:]
		if p[0] == 0 {
			capacity, _ := transport.StationItemCapacity(2104)
			p[0], p[1], p[2], p[3] = int32(item), 2, 1, int32(capacity)
		} else {
			p[2] = 1
		}
		if p[1] == 2 {
			a.enableSource(item, h.object)
		}
		h.load[i] += flow
		h.used += flow
		if i == first {
			break
		}
		j = parentCargo[i]
		a.holds[j].load[i] -= flow
		a.holds[j].used -= flow
	}
	a.left[first] -= flow
	return true
}

// fleetImportSlots 核对新建蓝图中有可存货的真实星际远需槽，不把本地槽或其他货物的空仓算作收货端。
func fleetImportSlots(objects []Object, ports []industryplan.Port, planetID int64) error {
	have := map[int]bool{}
	for _, o := range objects {
		b := o.Building
		if b.ItemID != 2104 || len(b.Parameters) < 30 {
			continue
		}
		for n := range 5 {
			p := b.Parameters[n*6:]
			if p[2] == 2 && p[3] > 0 {
				have[int(p[0])] = true
			}
		}
	}
	for _, p := range ports {
		if p.Kind == "import" && p.RatePerMinute > 0 && !have[p.ItemID] {
			return &Error{PlanetID: planetID, Item: p.ItemID, Code: "PLANET_FLEET_ROUTE", Message: "实际蓝图中没有这种货物的远程需求库存槽"}
		}
	}
	return nil
}
