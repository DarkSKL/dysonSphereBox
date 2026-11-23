package industryplan

import (
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
)

// portKey 职责：标识单一物流合同；使用：需求和回执比较；边界：对端属于合同的一部分。
type portKey struct {
	item int
	kind string
	peer int64
}

// stationKey 职责：标识塔内所需独立物品角色；使用：估算槽数；边界：同向同物品允许共用槽位。
type stationKey struct {
	item int
	kind string
}

// finalize 职责：核实星球槽位和船队上限后构建蓝图切片；使用：逐候选终检；边界：端口实物验证交给生成器。
func (c *candidate) finalize(ctx context.Context, base quant.Result) error {
	manualRoutes := needsStationRoutes(c.result.Routes)
	warpers := map[int64]float64{}
	loads := map[int64]map[int]FleetCargo{}
	const warperID = 1210
	for _, route := range c.result.Routes {
		warpers[route.FromPlanetID] += route.WarperRatePerMinute
		if route.VesselLoad > 0 {
			if loads[route.FromPlanetID] == nil {
				loads[route.FromPlanetID] = map[int]FleetCargo{}
			}
			cargo := loads[route.FromPlanetID][route.ItemID]
			cargo.ItemID = route.ItemID
			cargo.VesselLoad += route.VesselLoad
			if route.RoundTripSeconds > 0 {
				cargo.MaxVesselMW = max(cargo.MaxVesselMW, route.VesselEnergyMJ/route.RoundTripSeconds)
			}
			loads[route.FromPlanetID][route.ItemID] = cargo
		}
	}
	for _, id := range c.ids {
		site := c.world[id]
		site.plan.ManualRoutes = manualRoutes
		site.plan.WarpDistance = c.req.Logistics.WarpDistance
		if len(site.plan.Ports) == 0 && len(site.plan.Plan.Productions) == 0 {
			continue
		}
		if warpers[id] > 0 {
			c.result.External = append(c.result.External, External{PlanetID: id, ItemID: warperID, RatePerMinute: warpers[id], Kind: "warpers"})
		}
		kinds := map[stationKey]bool{}
		for _, port := range site.plan.Ports {
			if port.Kind == "import" || port.Kind == "export" {
				kinds[stationKey{port.ItemID, port.Kind}] = true
			}
		}
		if len(kinds) > 0 {
			slots, err := transport.StationItemKindLimit(2104)
			if err != nil {
				return err
			}
			if warpers[id] > 0 {
				slots--
			}
			if !finite(site.fleet) || site.fleet >= float64(math.MaxInt) {
				return fmt.Errorf("星球 %d 船队规模超出计算范围", id)
			}
			site.plan.VesselCount = int(math.Ceil(site.fleet))
			slotDemand := len(kinds)
			shipsPerStation := c.req.Logistics.VesselsPerStation
			site.plan.VesselsPerStation = shipsPerStation
			site.plan.FleetStationLimit = c.req.Logistics.MaxStationsPerPlanet
			for _, item := range slices.Sorted(maps.Keys(loads[id])) {
				cargo := loads[id][item]
				site.plan.OutboundFleet = append(site.plan.OutboundFleet, cargo)
				// 用多塔为同一种货物发货时，每座发送塔都需实际的该物品供给槽。
				slotsForCargo := int(math.Ceil(cargo.VesselLoad / float64(shipsPerStation)))
				if slotsForCargo > math.MaxInt-slotDemand {
					return fmt.Errorf("星球 %d 物流塔槽位规模超出计算范围", id)
				}
				slotDemand += slotsForCargo - 1
			}
			site.plan.StationCount = max(1+(slotDemand-1)/slots, 1+(max(site.plan.VesselCount, 1)-1)/shipsPerStation)
			if limit := c.req.Logistics.MaxStationsPerPlanet; limit > 0 && site.plan.StationCount > limit {
				return fmt.Errorf("星球 %d 需要至少 %d 座物流塔，设定上限是 %d", id, site.plan.StationCount, limit)
			}
		}
		if err := project(ctx, base, site, fmt.Sprintf("industry-%d", id)); err != nil {
			return fmt.Errorf("星球 %d 物料复核失败：%w", id, err)
		}
		c.result.Planets = append(c.result.Planets, site.plan)
	}
	return nil
}

// needsStationRoutes 识别同一物品有多个发货地及收货地时，共组会放开的额外配对。
// 已分配的运量和船队绑定具体两端，此时在游戏内确认逐站航线。
func needsStationRoutes(routes []Route) bool {
	type endpoints struct {
		from, to                 int64
		multipleFrom, multipleTo bool
	}
	items := map[int]endpoints{}
	for _, route := range routes {
		if route.VesselLoad <= 0 {
			continue
		}
		e, ok := items[route.ItemID]
		if !ok {
			e.from, e.to = route.FromPlanetID, route.ToPlanetID
		}
		e.multipleFrom = e.multipleFrom || route.FromPlanetID != e.from
		e.multipleTo = e.multipleTo || route.ToPlanetID != e.to
		if e.multipleFrom && e.multipleTo {
			return true
		}
		items[route.ItemID] = e
	}
	return false
}

// score 职责：按用户偏好比较可行候选；使用：候选排序；边界：能耗和运行负载为基于规划量的代理估计。
func (c *candidate) score() (float64, string) {
	var vessels, stations, equipment, shipWork, miningWork, traffic, shortage, gridMW float64
	for _, site := range c.result.Planets {
		vessels += float64(site.VesselCount)
		stations += float64(site.StationCount)
		equipment += float64(site.Plan.MachineCount)
		if site.Power != nil {
			equipment += float64(site.Power.Count)
			if site.Power.ExternalGridMW == 0 {
				gridMW += site.Power.RequiredMW
			}
		}
		for _, mine := range site.Mines {
			equipment += float64(mine.Count)
			miningWork += mine.RatePerMinute
		}
	}
	for _, route := range c.result.Routes {
		shipWork += route.VesselLoad
		traffic += route.RatePerMinute
	}
	for _, external := range c.result.External {
		if external.Kind == "grid-power" {
			gridMW += external.PowerMW
		}
		if external.Kind == "resource-shortfall" || external.Kind == "resource-data" {
			shortage += external.RatePerMinute
		}
	}
	var cost float64
	basis := "先减少建设星球，再集中上下游制造，最后比较设施与船队规模"
	switch c.req.Preference {
	case "logistics":
		cost = traffic + shipWork + stations
		basis = "比较跨星球物料速率、往返船时占用与物流塔数量"
	case "energy":
		cost = gridMW + miningWork*.001 + shipWork + stations*.1
		basis = "按制造、采集和满载发船的持续负荷比较用电，兼顾在途船队和物流塔规模"
	case "game-load":
		cost = equipment + vessels + stations + float64(len(c.result.Routes))
		basis = "以设施、运输船与路线数量估计游戏运行负载"
	default:
		cost = float64(len(c.result.Planets))*1000 + equipment + stations*5 + vessels
	}
	if shortage > 0 {
		basis += "，附加未满足的资源速率惩罚"
	}
	return cost + shortage*1000, basis
}

// verifyPorts 职责：把实际端口吞吐和库存与合同对账；使用：选中前；边界：容量不足即尝试剩余候选。
func verifyPorts(site PlanetPlan, actual PlanetActual) error {
	if expected := site.Power; expected != nil {
		got := actual.Power
		if got == nil || got.FacilityID != expected.FacilityID || got.FuelItemID != expected.FuelItemID || got.Count < expected.Count ||
			!finite(got.PowerMW) || !finite(got.ExternalGridMW) || got.ExternalGridMW != expected.ExternalGridMW || !finite(got.FuelRatePerMinute) || got.PowerMW+got.ExternalGridMW+1e-6 < expected.RequiredMW || got.FuelRatePerMinute+1e-6 < expected.FuelRatePerMinute {
			return fmt.Errorf("发电设施或燃料供给不足，需要 %d 台、%.6g MW 和 %.6g 燃料/分钟", expected.Count, expected.RequiredMW, expected.FuelRatePerMinute)
		}
	}
	capacities := map[portKey]Port{}
	for _, port := range actual.Ports {
		if !finite(port.RatePerMinute) || port.RatePerMinute < 0 || port.StorageItems < 0 {
			return fmt.Errorf("物品 %d 的实际端口容量无效", port.ItemID)
		}
		key := portKey{port.ItemID, port.Kind, port.PeerPlanetID}
		merged := capacities[key]
		merged.RatePerMinute += port.RatePerMinute
		if port.StorageItems > math.MaxInt-merged.StorageItems {
			return fmt.Errorf("物品 %d 的实际库存超出计算范围", port.ItemID)
		}
		merged.StorageItems += port.StorageItems
		capacities[key] = merged
	}
	for _, expected := range site.Ports {
		got, ok := capacities[portKey{expected.ItemID, expected.Kind, expected.PeerPlanetID}]
		if !ok || expected.RatePerMinute-got.RatePerMinute > quant.FlowTolerance(expected.RatePerMinute, got.RatePerMinute) || expected.StorageItems > got.StorageItems {
			return fmt.Errorf("物品 %d 端口 %s 对端 %d 需要 %.6g/min 与 %d 库存，实际为 %.6g/min 与 %d 库存", expected.ItemID, expected.Kind, expected.PeerPlanetID, expected.RatePerMinute, expected.StorageItems, got.RatePerMinute, got.StorageItems)
		}
	}
	return nil
}
