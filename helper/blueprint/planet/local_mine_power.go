package planet

// 文件职责：根据实际塔槽、制造块净边界、矿机及电厂带路核验本地配送。飞船、仓储启动和塔待机另计。

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
)

type localDroneEndpoint struct {
	tower                                   int
	rate, arc, chargeArc, required, nearest float64
}

type localCargoSlots struct {
	sources, sinks               []localDroneEndpoint
	sourceLimit, sinkLimit, rate float64
	active                       bool
	unknown                      bool
}

// localFreightSlots 将带路的每分钟实际流量对应到最后写入的本地槽；零净流的制造边界不会变成无人机货运。
func localFreightSlots(req *Request, plan industryplan.PlanetPlan, actual *industryplan.PlanetActual, cargo int) map[int]*localCargoSlots {
	type portKey struct {
		tower, item int
		supply      bool
	}
	rates := map[portKey]float64{}
	for _, p := range req.localPorts {
		k := portKey{p.BuildingIndex, p.ItemID, p.Kind != "external-input"}
		rates[k] += p.RatePerMinute
	}
	for _, m := range req.mineCoverage {
		rates[portKey{m.BuildingIndex + 1, m.ItemID, true}] += m.RatePerMinute
	}
	limits := localContractLimits(req, plan, actual)
	result := map[int]*localCargoSlots{}
	for i, o := range req.Objects {
		var count int
		switch o.Building.ItemID {
		case 2103:
			count = 4
		case 2104:
			count = 5
		case 2316:
			count = 1
		default:
			continue
		}
		if len(o.Building.Parameters) < count*6 {
			continue
		}
		for s := 0; s < count; s++ {
			p := o.Building.Parameters[6*s:]
			item, role := int(p[0]), p[1]
			if item <= 0 || role != 1 && role != 2 {
				continue
			}
			route := result[item]
			if route == nil {
				route = &localCargoSlots{}
				result[item] = route
			}
			rate, known := rates[portKey{i, item, role == 1}]
			point := localDroneEndpoint{tower: i, nearest: math.Inf(1)}
			if known && role == 2 && p[2] != 2 && (limits[item] == nil || !limits[item].manual) {
				point.required = rate // 有真实出带需求且不能直接远程补库；这部分货必须由无人机送到。
			}
			if limit := limits[item]; limit != nil {
				// 只有本供远需与本需远供可将远程合同转成本地飞行；本需远需的进口直接进需方仓。
				if role == 1 && (!known || p[2] == 2) {
					bound := limit.made + limit.mined + limit.incoming
					if bound > 0 {
						route.sourceLimit = bound
						if known {
							rate += limit.incoming
						} else if p[2] == 2 && o.Module == "物料中转" {
							rate = limit.incoming
						} else {
							rate = bound
						}
						known = true
					}
				} else if role == 2 && (!known || p[2] == 1) {
					bound := limit.used + limit.fuel + limit.outgoing + limit.stored
					if bound > 0 {
						route.sinkLimit = bound
						if known {
							rate += limit.outgoing
						} else if p[2] == 1 && limit.outgoing > 0 {
							rate = limit.outgoing
						} else {
							rate = bound
						}
						known = true
					}
				}
			}
			route.unknown = route.unknown || !known || int(p[3]) < cargo ||
				o.Building.ItemID != 2104 || o.Building.ModelIndex != 50 || len(o.Building.Parameters) != 2048 || o.First.Height != 0
			point.rate = rate
			if role == 1 {
				route.sources = append(route.sources, point)
			} else {
				route.sinks = append(route.sinks, point)
			}
		}
	}
	return result
}

// localCargoBudget 对去回不重复计载；用供端及需端分别可证明的耗电上限中的较小者，覆盖本货种的任意孤立配对。
func localCargoBudget(req *Request, group *localCargoSlots, cargo, speed float64, sending map[int]bool, fleet map[int]float64) (rate, mw float64) {
	var supply, demand float64
	for _, s := range group.sources {
		supply += s.rate
	}
	for _, d := range group.sinks {
		demand += d.rate
	}
	if group.sourceLimit > 0 {
		supply = min(supply, group.sourceLimit)
	}
	if group.sinkLimit > 0 {
		demand = min(demand, group.sinkLimit)
	}
	rate = min(supply, demand)
	if rate <= 1e-8 {
		return 0, 0
	}
	group.active = true
	group.rate = rate
	for i := range group.sources {
		s := &group.sources[i]
		if s.rate > 0 {
			sending[s.tower] = true
		}
		for j := range group.sinks {
			d := &group.sinks[j]
			if s.tower == d.tower {
				continue
			}
			arc := ilsDroneArc(req.Objects[s.tower].First, req.Objects[d.tower].First)
			s.chargeArc, d.chargeArc = max(s.chargeArc, arc), max(d.chargeArc, arc)
			if s.rate <= 0 || d.rate <= 0 {
				continue
			}
			s.arc = max(s.arc, arc)
			d.arc = max(d.arc, arc)
			d.nearest = min(d.nearest, arc)
		}
	}
	endBudget := func(points []localDroneEndpoint) float64 {
		sort.SliceStable(points, func(i, j int) bool { return points[i].arc > points[j].arc })
		var energy float64
		left := rate
		for _, p := range points {
			if p.rate <= 0 {
				continue
			}
			// InternalTickLocal 对一次往返扣除 800000 + 40000×单程弧长 J。
			energy += min(left, p.rate) / cargo * (.8 + .04*p.arc) / 60
			left -= min(left, p.rate)
			// CalcLocalSingleTripTime 每程包含巡航与两次起降。兼有供需的塔，其机器也可能去上游取货。
			fleet[p.tower] += min(rate, p.rate) / cargo * droneTripMinutes(p.arc, speed)
			req.Objects[p.tower].Building.Parameters[330] = 0
		}
		return energy
	}
	sourceMW, sinkMW := endBudget(group.sources), endBudget(group.sinks)
	return rate, min(sourceMW, sinkMW)
}

// accountLocalFreight 为有带路和实体库存可对照的矿料、制造中间品及燃料计入持续用电，并说明其余未核验的本地货种。
func accountLocalFreight(req *Request, input ProductionRequest, actual *industryplan.PlanetActual) error {
	cargo, speed := input.Options.DroneCargo, input.Options.DroneSpeed
	if cargo <= 0 || speed <= 0 {
		return nil
	}
	slots := localFreightSlots(req, input.Plan, actual, cargo)
	sending, fleet := map[int]bool{}, map[int]float64{}
	var details, unknown []string
	var power float64
	catalog, _ := facts.Load()
	for _, item := range slices.Sorted(maps.Keys(slots)) {
		group := slots[item]
		if len(group.sources) == 0 || len(group.sinks) == 0 {
			continue
		}
		name := fmt.Sprint(item)
		if catalog != nil {
			if f, ok := catalog.Lookup(item); ok {
				name = f.Name
			}
		}
		if group.unknown {
			unknown = append(unknown, name)
			continue
		}
		rate, mw := localCargoBudget(req, group, float64(cargo), speed, sending, fleet)
		if rate > 0 {
			if need, available := hardLocalFleetFloor(group, float64(cargo), speed); need > float64(available)+1e-7 {
				return &Error{PlanetID: req.PlanetID, Item: item, Code: "PLANET_LOCAL_FLEET", Message: fmt.Sprintf("%s的实际带口每分钟必须接货。即使全部满载并走最近塔对，至少需要 %.1f 架无人机，所有可派机的现有供需塔合计只有 %d 个机位，需分散工序或增设接驳塔。", name, need, available)}
			}
			power += mw
			details = append(details, fmt.Sprintf("%s %.1f 件/分", name, rate))
		}
	}
	req.dronePowerMW = power
	if power > 0 {
		maxFleet := func(active map[int]bool) int {
			maxDrones := 0
			for tower := range active {
				maxDrones = max(maxDrones, int(math.Ceil(fleet[tower]))+1)
			}
			return maxDrones
		}
		maxDrones := maxFleet(sending)
		mixedFleet := false
		direction := "请给上述货种的本地供货塔配机，每座按最长往返计最多准备 %d 架；纯收货塔留空。"
		if maxDrones > 100 {
			picking := map[int]bool{}
			for _, group := range slots {
				if !group.active {
					continue
				}
				for _, d := range group.sinks {
					if d.rate > 0 && fleet[d.tower] > 0 {
						picking[d.tower] = true
					}
				}
			}
			if pickupMax := maxFleet(picking); pickupMax > 0 && pickupMax < maxDrones {
				sending, maxDrones = picking, pickupMax
				direction = "请给上述货种的本地收货塔配机取货，每座按最长往返计最多准备 %d 架；纯供货塔留空。"
			}
			if maxDrones > 100 {
				if mixed := mixedLocalDispatch(slots, fleet); mixed != nil {
					mixedFleet = true
					sending, maxDrones = mixed, maxFleet(mixed)
					direction = "请只给本图最低起运量为 100%% 的本地塔装机，每座最多 %d 架；同星各货种分别由供货塔送或收货塔取，其余参与塔的起运量已设为 10%%，请留空机位。"
					for tower := range fleet {
						req.Objects[tower].Building.Parameters[326] = 10
					}
				}
			}
		}
		for tower := range sending {
			req.Objects[tower].Building.Parameters[326] = 100
		}
		drones := map[int]int{}
		for tower := range sending {
			count := maxDrones
			if mixedFleet {
				count = int(math.Ceil(fleet[tower])) + 1
			}
			drones[tower] = min(count, 100)
		}
		req.droneChargingMW = droneStationCharging(slots, drones, float64(cargo), speed, power)
		if maxDrones > 100 {
			direction = "按不限距离配对，单塔运力预算最高 %d 架，超过 100 个机位。当前目标有运力条件：须缩短相应塔的无人机距离、分流到其他塔，并实测吞吐后再按此产量开工。"
		}
		actual.Warnings = append(actual.Warnings, fmt.Sprintf("本地无人机持续用电已预留 %.3f MW（%s）。每架满载 %d 件。%s配对塔均关闭无人机自动补充。须与原有同货种物流隔离，开工先备货和充电。", power, strings.Join(details, "、"), cargo, fmt.Sprintf(direction, maxDrones)))
		if mixedFleet {
			var locations []string
			for _, i := range slices.Sorted(maps.Keys(sending)) {
				obj := req.Objects[i]
				locations = append(locations, fmt.Sprintf("%d 号塔（经度 %.2f°、纬度 %.2f°）%d 架", obj.Building.Index, math.Remainder(obj.First.Longitude, 2*math.Pi)*180/math.Pi, obj.First.Latitude*180/math.Pi, int(math.Ceil(fleet[i]))+1))
			}
			actual.Warnings = append(actual.Warnings, "本星手动装入无人机的塔和数量为"+strings.Join(locations, "，")+"。其余对应塔留空机位。")
		}
	}
	if len(unknown) > 0 {
		actual.Warnings = append(actual.Warnings, fmt.Sprintf("以下货物还有本地供需塔，尚无法从现有蓝图带路定出配送电耗：%s。开工时须核对实际配对、配机和持续供电。", strings.Join(unknown, "、")))
	}
	return nil
}

// ilsDroneArc 与原版用两停机坪平均半径乘圆心角一致。型号 50 地表原点半径 200.2 m，停机坪沿径向高 26 m。
func ilsDroneArc(a, b Position) float64 {
	cos := math.Sin(a.Latitude)*math.Sin(b.Latitude) + math.Cos(a.Latitude)*math.Cos(b.Latitude)*math.Cos(a.Longitude-b.Longitude)
	return 226.2 * math.Acos(max(-1, min(1, cos)))
}
