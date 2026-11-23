package industryplan

import (
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
)

// location 职责：累计一颗星球的净余量及逐项设施；使用：每个候选独立构建；边界：库存与实际建设单独验证。
type location struct {
	planet  Planet
	balance map[int]float64
	plan    PlanetPlan
	fleet   float64
}

// candidate 职责：保存单个分工及船运草案；使用：有界搜索；边界：不修改共享量化结果。
type candidate struct {
	ctx              context.Context
	remoteRules      func(int64, int64, int64) (bool, bool)
	bannedRoute      string
	excludedSources  map[resourceSite]bool
	sourceLimits     map[resourceSite]float64
	powerMultipliers map[int64]float64
	req              Request
	center           int64
	storage          int64
	world            map[int64]*location
	ids              []int64
	nearest          map[int64][]int64
	profile          transport.VesselProfile
	result           Result
	failure          string
	missing          bool
	sphereUsed       map[int64]float64
}

// add 职责：归并同角色同对端的净流；使用：采集、运货和交付；边界：流量只累加一次。
func (site *location) add(port Port) {
	for i := range site.plan.Ports {
		old := &site.plan.Ports[i]
		if old.Kind == port.Kind && old.ItemID == port.ItemID && old.PeerPlanetID == port.PeerPlanetID {
			old.RatePerMinute += port.RatePerMinute
			old.StorageItems += port.StorageItems
			return
		}
	}
	site.plan.Ports = append(site.plan.Ports, port)
}

// distance 职责：计算快照两端直线距离；使用：航时估算；边界：星球坐标采用相同游戏单位。
func distance(from, to Planet) float64 {
	return math.Hypot(math.Hypot(from.Position[0]-to.Position[0], from.Position[1]-to.Position[1]), from.Position[2]-to.Position[2])
}

// dispatchRadius 允许旧调用未写半径时沿用原版普通星半径，新存档输入区分实心与气态星。
func (planet Planet) dispatchRadius() float64 {
	if planet.Radius == 0 {
		return 200
	}
	return planet.Radius
}

// orderFor 职责：按距离及 ID 构造稳定供给顺序；使用：星球需求撮合；边界：原始候选顺序不改动。
func (c *candidate) orderFor(id int64) []int64 {
	if order := c.nearest[id]; order != nil {
		return order
	}
	order := slices.Clone(c.ids)
	sort.SliceStable(order, func(i, j int) bool {
		first, second := c.world[order[i]].planet, c.world[order[j]].planet
		to := c.world[id].planet
		return distance(first, to) < distance(second, to)
	})
	if c.nearest == nil {
		c.nearest = map[int64][]int64{}
	}
	c.nearest[id] = order
	return order
}

// transfer 职责：分配同星系或跨恒星物流；使用：余量、采集和仓储；边界：发送端承担全部往返船时。
func (c *candidate) transfer(from, to *location, item int, desired float64) float64 {
	if from == to {
		return desired
	}
	d := distance(from.planet, to.planet)
	if !enabled(c.req.EnableLogistics, true) {
		c.result.External = append(c.result.External, External{PlanetID: from.planet.ID, PeerPlanetID: to.planet.ID, ItemID: item, RatePerMinute: desired, Kind: "transport"})
		c.result.Routes = append(c.result.Routes, Route{FromPlanetID: from.planet.ID, ToPlanetID: to.planet.ID, ItemID: item, RatePerMinute: desired, Distance: d, Interstellar: from.planet.StarID != to.planet.StarID})
		from.add(Port{ItemID: item, Kind: "external-export", RatePerMinute: desired, PeerPlanetID: to.planet.ID})
		to.add(Port{ItemID: item, Kind: "external", RatePerMinute: desired, PeerPlanetID: from.planet.ID})
		return desired
	}
	seconds, warped, energyMJ, err := c.profile.VesselDispatch(d, from.planet.dispatchRadius(), to.planet.dispatchRadius())
	if c.remoteRules != nil {
		if _, banned := c.remoteRules(from.planet.ID, to.planet.ID, int64(item)); banned {
			c.bannedRoute = fmt.Sprintf("存档禁止物品 %d 在星球 %d 与 %d 间运输", item, from.planet.ID, to.planet.ID)
			return 0
		}
	}
	if err != nil {
		c.missing = true
		return 0
	}
	perVessel := c.profile.Cargo * 60 / seconds
	if limit := c.req.Logistics.MaxStationsPerPlanet; limit > 0 {
		fleetLimit := float64(limit) * float64(c.req.Logistics.VesselsPerStation)
		desired = math.Min(desired, math.Max(0, fleetLimit-from.fleet)*perVessel)
	}
	if !significant(desired) {
		return 0
	}
	route := Route{FromPlanetID: from.planet.ID, ToPlanetID: to.planet.ID, ItemID: item, RatePerMinute: desired, Distance: d, RoundTripSeconds: seconds, VesselLoad: desired / perVessel, Interstellar: from.planet.StarID != to.planet.StarID}
	route.VesselEnergyMJ = energyMJ
	route.VesselPowerMW = energyMJ * desired / c.profile.Cargo / 60
	from.plan.OutboundVesselMW += route.VesselPowerMW
	if warped {
		route.WarperRatePerMinute = 2 * desired / c.profile.Cargo
	}
	from.fleet += route.VesselLoad
	from.add(Port{ItemID: item, Kind: "export", RatePerMinute: desired, PeerPlanetID: to.planet.ID})
	to.add(Port{ItemID: item, Kind: "import", RatePerMinute: desired, PeerPlanetID: from.planet.ID})
	c.result.Routes = append(c.result.Routes, route)
	return desired
}

// useResource 职责：将矿业能力分配给净需求；使用：完成内部共享后补足原料；边界：只对剩余需求配置新产能。
func (c *candidate) useResource(to *location, item int, desired float64, used map[int64]float64, origins []int64) float64 {
	supplied := 0.0
	for _, id := range origins {
		key := resourceSite{planetID: id, itemID: item}
		if c.excludedSources[key] {
			continue
		}
		from := c.world[id]
		if !from.planet.ResourcesKnown && len(from.planet.Resources) == 0 && enabled(c.req.EnableMining, true) {
			c.missing = true
		}
		for _, resource := range from.planet.Resources {
			if resource.ItemID != item {
				continue
			}
			if !from.planet.ResourcesKnown && resource.Kind != "orbital" && enabled(c.req.EnableMining, true) {
				c.missing = true
			}
			capacity, unit, facility := resourceCapacity(c.req, resource, used[id]+desired-supplied)
			if !finite(capacity) || !finite(unit) || unit <= 0 || capacity <= 0 {
				if facility != 0 && resource.Count > 0 && (resource.RemainingItems == nil || *resource.RemainingItems > 0) {
					c.missing = true
				}
				continue
			}
			if limit, limited := c.sourceLimits[key]; limited {
				capacity = math.Min(capacity, limit)
			}
			remaining := math.Min(capacity-used[id], desired-supplied)
			if !significant(remaining) {
				continue
			}
			if (used[id]+remaining)/unit >= float64(math.MaxInt) {
				c.missing = true
				continue
			}
			rate := c.transfer(from, to, item, remaining)
			if !significant(rate) {
				continue
			}
			used[id] += rate
			supplied += rate
			from.add(Port{ItemID: item, Kind: "mining", RatePerMinute: rate})
			var mine *Mine
			for i := range from.plan.Mines {
				if from.plan.Mines[i].ItemID == item {
					mine = &from.plan.Mines[i]
					break
				}
			}
			if mine == nil {
				facilityMultiplier := 1.0
				if resource.Kind == "solid" {
					facilityMultiplier = c.req.Mining.FacilityMultiplier
				}
				from.plan.Mines = append(from.plan.Mines, Mine{ItemID: item, Kind: resource.Kind, Source: resource.Source, FacilityID: facility, SpeedMultiplier: c.req.Mining.SpeedMultiplier, FacilityMultiplier: facilityMultiplier})
				mine = &from.plan.Mines[len(from.plan.Mines)-1]
			}
			mine.RatePerMinute += rate
			mine.Count = int(math.Ceil(mine.RatePerMinute / unit))
		}
		if !significant(desired - supplied) {
			break
		}
	}
	return supplied
}

// miningContribution 按本星本物料的已用矿速和实接上限，估算还能就地承担多少新增需求。
func (c *candidate) miningContribution(site *location, item int, used, demand float64) float64 {
	key := resourceSite{planetID: site.planet.ID, itemID: item}
	if demand <= 0 || c.excludedSources[key] {
		return 0
	}
	for _, resource := range site.planet.Resources {
		if resource.ItemID == item {
			capacity, _, _ := resourceCapacity(c.req, resource, used+demand)
			if limit, ok := c.sourceLimits[key]; ok {
				capacity = min(capacity, limit)
			}
			return max(0, min(demand, capacity-used))
		}
	}
	return 0
}

// miningOrder 优先本地矿、再复用已开工矿区。需要新增矿区时，按可承担的缺口选矿，等量优先同系近星。
func (c *candidate) miningOrder(to *location, item int, desired float64, used map[int64]float64) []int64 {
	order := slices.Clone(c.orderFor(to.planet.ID))
	type rank struct {
		class        int
		contribution float64
		near         bool
	}
	ranks := make(map[int64]rank, len(order))
	for _, id := range order {
		s := c.world[id]
		r := rank{class: 3, contribution: c.miningContribution(s, item, used[id], desired), near: s.planet.StarID == to.planet.StarID}
		if significant(r.contribution) {
			switch {
			case id == to.planet.ID:
				r.class = 0
			case len(s.plan.Mines) > 0 || len(s.plan.Plan.Productions) > 0 || len(s.plan.Ports) > 0:
				r.class = 1
			default:
				r.class = 2
			}
		}
		ranks[id] = r
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := ranks[order[i]], ranks[order[j]]
		if a.class != b.class {
			return a.class < b.class
		}
		if a.class == 1 && a.near != b.near {
			return a.near
		}
		if a.contribution != b.contribution {
			return a.contribution > b.contribution
		}
		return a.near && !b.near
	})
	return order
}

// allocate 职责：先共享产出，再分配新采集与外供；使用：单物品守恒；边界：制造净缺口不可由未声明的来源替换。
func (c *candidate) allocate(item int, raw, explicit, fuel bool) {
	if item == 1208 && c.req.Dyson.Mode != "" {
		c.allocatePhotons(explicit)
		return
	}
	used := map[int64]float64{}
	for _, id := range c.ids {
		for _, mine := range c.world[id].plan.Mines {
			if mine.ItemID == item {
				used[id] += mine.RatePerMinute
			}
		}
	}
	// 无制造余产可调拨时，先把各星本地矿供给本地用矿工序，再用剩余矿量跨星供货。
	if raw && !explicit && !slices.ContainsFunc(c.ids, func(id int64) bool { return significant(c.world[id].balance[item]) }) {
		for _, id := range c.ids {
			site := c.world[id]
			if need := -site.balance[item]; significant(need) {
				site.balance[item] += c.useResource(site, item, need, used, []int64{id})
			}
		}
	}
	for _, id := range c.ids {
		to := c.world[id]
		if !significant(-to.balance[item]) {
			continue
		}
		reason := "缺少制造或运输能力"
		c.bannedRoute = ""
		for _, origin := range c.orderFor(id) {
			from := c.world[origin]
			if !significant(from.balance[item]) {
				continue
			}
			rate := c.transfer(from, to, item, math.Min(-to.balance[item], from.balance[item]))
			from.balance[item] -= rate
			to.balance[item] += rate
			if !significant(-to.balance[item]) {
				break
			}
		}
		if raw && !explicit && significant(-to.balance[item]) {
			need := -to.balance[item]
			to.balance[item] += c.useResource(to, item, need, used, c.miningOrder(to, item, need, used))
			reason = "采集或运输能力不足"
		}
		if significant(-to.balance[item]) {
			rate := -to.balance[item]
			if c.bannedRoute != "" {
				c.result.Warnings = append(c.result.Warnings, c.bannedRoute)
			}
			if !raw {
				c.failure = fmt.Sprintf("星球 %d 的物品 %d 缺少 %.6g/min：%s", id, item, rate, reason)
				return
			}
			kind := "input"
			switch {
			case fuel:
				kind = "fuel-input"
			case explicit:
				kind = "declared-input"
			case c.orbitalOnly(item):
				kind = "collector-disabled"
				if enabled(c.req.EnableOrbitalCollectors, false) {
					kind = "collector-placement"
					c.result.Warnings = append(c.result.Warnings, fmt.Sprintf("物品 %d 需要轨道采集器。当前原版禁止在气态巨星粘贴蓝图，可将该物品设为外供并在游戏内配置采集器。", item))
				}
			case !enabled(c.req.EnableMining, true) && !enabled(c.req.EnableOrbitalCollectors, false):
				kind = "mining-disabled"
			case c.missing:
				kind = "resource-data"
			default:
				kind = "resource-shortfall"
			}
			to.add(Port{ItemID: item, Kind: "external", RatePerMinute: rate})
			c.result.External = append(c.result.External, External{PlanetID: id, ItemID: item, RatePerMinute: rate, Kind: kind})
			to.balance[item] = 0
		}
	}
}

// allocateSurplus 职责：在必需供给路线完成后分配剩余副产物；使用：全局仓储；边界：仓储不能提前耗尽生产和交付所需船队。
func (c *candidate) allocateSurplus(item int) {
	for _, id := range c.ids {
		from := c.world[id]
		rate := from.balance[item]
		if !significant(rate) {
			continue
		}
		if !enabled(c.req.EnableStorage, true) {
			from.add(Port{ItemID: item, Kind: "external-export", RatePerMinute: rate})
			c.result.External = append(c.result.External, External{PlanetID: id, ItemID: item, RatePerMinute: rate, Kind: "surplus-removal"})
			continue
		}
		to := c.world[c.storage]
		if c.req.StoragePlanetID == 0 && from.planet.Buildable && (c.req.Preference == "energy" || c.req.Preference == "logistics") {
			// 必需产耗和交付航线已经结清。自动仓储在余料所在星建足缓冲，省去只为入库的飞船行程。
			to = from
		}
		c.bannedRoute = ""
		sent := c.transfer(from, to, item, rate)
		if significant(sent) {
			c.store(to, item, sent)
		}
		if left := rate - sent; left > quant.FlowTolerance(rate, rate) {
			if c.req.StoragePlanetID != 0 || !from.planet.Buildable {
				c.failure = fmt.Sprintf("物品 %d 无法按 %.6g/min 运到仓储星球 %d", item, rate, to.planet.ID)
				if c.bannedRoute != "" {
					c.result.Warnings = append(c.result.Warnings, c.bannedRoute)
				}
				return
			}
			c.store(from, item, left)
		}
	}
}

// orbitalOnly 区分巨星独有来源和可在固态行星采集的物料，制造联产仍优先使用。
func (c *candidate) orbitalOnly(item int) bool {
	orbital := false
	for _, site := range c.world {
		for _, res := range site.planet.Resources {
			if res.ItemID != item {
				continue
			}
			if res.Kind != "orbital" {
				return false
			}
			orbital = true
		}
	}
	return orbital
}

// store 职责：按有限时长分配库存；使用：副产物汇总后；边界：溢出保护并保留持续入库速率。
func (c *candidate) store(to *location, item int, rate float64) {
	count := math.Ceil(rate * c.req.StorageMinutes)
	if count >= float64(math.MaxInt) {
		c.failure = fmt.Sprintf("物品 %d 仓储需求超出可表达范围", item)
		return
	}
	for _, old := range to.plan.Ports {
		if old.ItemID == item && old.Kind == "storage" && int(count) > math.MaxInt-old.StorageItems {
			c.failure = fmt.Sprintf("物品 %d 合计仓储需求超出可表达范围", item)
			return
		}
	}
	to.add(Port{ItemID: item, Kind: "storage", RatePerMinute: rate, StorageItems: int(count)})
}

// project 职责：把边界流量投影回逐星球量化契约；使用：蓝图引擎；边界：不会重新求解或更换设备。
func project(ctx context.Context, base quant.Result, site *location, planID string) error {
	plan := site.plan.Plan
	plan.RawInputs, plan.Targets, plan.Leftovers, plan.ExternalInputItemIDs = nil, nil, nil, nil
	plan.PlanID = planID
	plan.Dataset, plan.DataVersion, plan.GameVersion = base.Dataset, base.DataVersion, base.GameVersion
	external := map[int]bool{}
	for _, port := range site.plan.Ports {
		rate := quant.ItemRate{ItemID: port.ItemID, RatePerMinute: port.RatePerMinute}
		switch port.Kind {
		case "mining", "import", "external":
			plan.RawInputs = append(plan.RawInputs, rate)
			external[port.ItemID] = external[port.ItemID] || port.Kind != "mining"
		case "delivery", "export", "external-export", "power-fuel":
			plan.Targets = append(plan.Targets, quant.Target{ItemID: port.ItemID, RatePerMinute: port.RatePerMinute})
		case "storage":
			plan.Leftovers = append(plan.Leftovers, rate)
		}
	}
	for item, required := range external {
		if required {
			plan.ExternalInputItemIDs = append(plan.ExternalInputItemIDs, item)
		}
	}
	sort.Ints(plan.ExternalInputItemIDs)
	graph, err := quant.BuildFlowGraphContext(ctx, plan)
	if err != nil {
		return err
	}
	plan.FlowGraph = &graph
	plan.PlanID += "-" + graph.Digest
	site.plan.Plan = plan
	return nil
}

// finite 职责：检查数值可参与估算；使用：采集、输入与校验；边界：允许零。
func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
