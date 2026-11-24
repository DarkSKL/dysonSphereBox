package industryplan

// 文件职责：结清燃料、戴森球和充放电的能量账；使用：每个候选独立求解；边界：储能不创造能量。

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"

	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// energyCovered 检查每种辅助物料的供给下限，用于辨认换电后需要收回的旧燃料。
func energyCovered(supply, needs []quant.ItemRate) bool {
	available := map[int]float64{}
	for _, x := range supply {
		available[x.ItemID] += x.RatePerMinute
	}
	for _, x := range needs {
		if x.RatePerMinute > available[x.ItemID]+quant.FlowTolerance(x.RatePerMinute, available[x.ItemID]) {
			return false
		}
	}
	return true
}

// matchingEnergySupply 比较按物品排序的实际需求与附加量化目标，避免把试算燃料当作正式副产品。
func matchingEnergySupply(supply, needs []quant.ItemRate) bool {
	return slices.EqualFunc(supply, needs, func(a, b quant.ItemRate) bool {
		return a.ItemID == b.ItemID && math.Abs(a.RatePerMinute-b.RatePerMinute) <= quant.FlowTolerance(a.RatePerMinute, b.RatePerMinute)
	})
}

func (r Request) energyChoice(id int64) EnergyChoice {
	for _, entry := range r.Energy.Overrides {
		if entry.PlanetID == id {
			return entry.Choice
		}
	}
	return r.Energy.Default
}

// validateEnergy 职责：检查声明的星球归属、功率和模式；使用：量化前；边界：声明的外部供能保持可见条件。
func validateEnergy(req Request, planets map[int64]Planet) error {
	check := func(choice EnergyChoice) error {
		if choice.Mode == "" {
			return nil
		}
		if choice.Mode != "auto" && choice.Mode != "fixed" && choice.Mode != "grid" {
			return fmt.Errorf("请选择自动、指定电源或已有电网")
		}
		if choice.Mode == "grid" && (!finite(choice.GridCapacityMW) || choice.GridCapacityMW <= 0) {
			return fmt.Errorf("请填写已有电网可用于新产线的余量")
		}
		if choice.Mode != "fixed" {
			return nil
		}
		if choice.Mode == "fixed" && !slices.Contains([]int{2203, 2204, 2205, 2206, 2208, 2209, 2210, 2211, 2213}, choice.FacilityID) {
			return fmt.Errorf("请选择原版供电设施")
		}
		if choice.FacilityID == 2209 && !planets[choice.ChargePlanetID].Buildable {
			return fmt.Errorf("能量枢纽需要一颗可建设的充电星球")
		}
		if choice.FacilityID == 2205 && (!(choice.SolarFactor > 0) || !(choice.SolarDuty > 0 && choice.SolarDuty <= 1) || !(choice.NightSeconds >= 0) || !finite(choice.SolarFactor+choice.SolarDuty+choice.NightSeconds)) {
			return fmt.Errorf("太阳能需要确认日照倍率、可保证的日照比例和最长夜晚")
		}
		return nil
	}
	if req.EnablePower {
		if err := check(req.Energy.Default); err != nil {
			return err
		}
		seen := map[int64]bool{}
		for _, override := range req.Energy.Overrides {
			if !planets[override.PlanetID].Buildable || seen[override.PlanetID] {
				return fmt.Errorf("逐星电源设置包含无效或重复星球")
			}
			seen[override.PlanetID] = true
			if err := check(override.Choice); err != nil {
				return err
			}
		}
	}
	if req.Dyson.Mode != "" && req.Dyson.Mode != "unconfigured" && req.Dyson.Mode != "sphere" && req.Dyson.Mode != "external" {
		return fmt.Errorf("请选择光子的获取方式")
	}
	if req.Dyson.Mode != "sphere" {
		return nil
	}
	stars, receivers := map[int64]bool{}, map[int64]bool{}
	for _, source := range req.Dyson.Supplies {
		if stars[source.StarID] || source.StarID <= 0 || len(source.PlanetIDs) == 0 || !(source.AvailableMW > 0) || !(source.Efficiency > 0 && source.Efficiency <= 1) || !(source.Uptime > 0 && source.Uptime <= 1) || !finite(source.AvailableMW+source.Efficiency+source.Uptime) {
			return fmt.Errorf("戴森球的可用功率、接收效率和连续率须有效，同一恒星只能设置一次")
		}
		stars[source.StarID] = true
		for _, id := range source.PlanetIDs {
			if !planets[id].Buildable || planets[id].StarID != source.StarID || receivers[id] {
				return fmt.Errorf("接收星球必须属于所选恒星，且不能重复")
			}
			receivers[id] = true
		}
	}
	if req.Dyson.Mode == "sphere" && len(req.Dyson.Supplies) == 0 {
		return fmt.Errorf("请添加戴森球所在恒星与光子接收星球")
	}
	return nil
}

// photonPrecondition 职责：在调查矿源前检查与分工无关的光子下限；使用：避免缺少戴森声明时遍历全部星球；边界：后续燃料与射线仍需共同扣减额度。
func photonPrecondition(req Request, base quant.Result) (string, string) {
	if req.Dyson.Mode == "" || req.Dyson.Mode == "external" || slices.Contains(base.ExternalInputItemIDs, 1208) {
		return "", ""
	}
	need := 0.0
	for _, raw := range base.RawInputs {
		if raw.ItemID == 1208 {
			need += raw.RatePerMinute
		}
	}
	if need <= 0 {
		return "", ""
	}
	if req.Dyson.Mode != "sphere" {
		return "missing_data", "临界光子需要戴森球：请指定接收恒星与星球，或明确选择玩家外供"
	}
	available := 0.0
	perReceiver := 6.0
	if req.Dyson.Lenses {
		perReceiver = 12
	}
	for _, supply := range req.Dyson.Supplies {
		available += math.Floor((supply.AvailableMW+1e-6)/(perReceiver*20/supply.Efficiency)) * perReceiver * supply.Uptime
	}
	if need > available+quant.FlowTolerance(need, available) {
		message := fmt.Sprintf("当前配方需要 %.1f 个临界光子/分钟，所选戴森球可提供 %.1f。可增加可用功率、调整配方或声明外供临界光子。", need, available)
		return "infeasible", message
	}
	return "", ""
}

// evaluateEnergy 职责：让发电的燃料生产与新增采矿也进入负荷；使用：分工候选；边界：不把燃料辅助目标作为用户交付。
func evaluateEnergy(ctx context.Context, req Request, base quant.Result, world World, planets map[int64]Planet, ids, buildable []int64, profile transport.VesselProfile, policy assignment, progress func(Progress)) Result {
	var afterTrim map[string]bool
	for {
		if search.Checkpoint(ctx) != nil {
			return Result{}
		}
		result := evaluate(ctx, req, base, planets, ids, buildable, profile, policy, world.RemoteRules)
		if result.Status == "infeasible" || result.Status == "missing_data" {
			return result
		}
		if req.Energy.Default.Mode == "" || matchingEnergySupply(req.energySupply, result.EnergyNeeds) {
			if len(result.postPowerShipping) == 0 {
				return result
			}
			req.postPowerShipping = result.postPowerShipping
			continue
		}
		if result.settledAutoPower != nil {
			// 供给回退后，旧燃料运输产生的额外充电负荷也须随新合同重算。
			req.settledAutoPower = result.settledAutoPower
			req.postPowerShipping = nil
			if afterTrim == nil {
				afterTrim = map[string]bool{}
			}
		}
		req.energySupply = result.EnergyNeeds
		if afterTrim != nil {
			state := fmt.Sprint(req.energySupply, req.settledAutoPower, req.postPowerShipping)
			if afterTrim[state] {
				result.Status = "infeasible"
				result.Warnings = append(result.Warnings, "调整发电燃料后，这组星球无法稳定配平供电与燃料需求。请指定其他电源或增加可用矿源。")
				return result
			}
			afterTrim[state] = true
		}
		q := req.Quant
		q.Targets = slices.Clone(req.Quant.Targets)
		for _, need := range req.energySupply {
			q.Targets = append(q.Targets, quant.Target{ItemID: need.ItemID, RatePerMinute: need.RatePerMinute})
		}
		var err error
		base, err = quant.Solve(ctx, q)
		if err == nil {
			_, _, err = resolveCandidateResources(ctx, req, world, base, planets, ids, len(ids), progress)
		}
		if err == nil {
			base, err = SplitProductions(ctx, base, 32)
		}
		if err != nil {
			result.Status = "infeasible"
			result.Warnings = append(result.Warnings, err.Error())
			return result
		}
	}
}

// allocatePhotons 职责：从声明的接收星分配光子；使用：原料结算；边界：同一恒星的功率额度只扣一次。
func (c *candidate) allocatePhotons(explicit bool) {
	if c.sphereUsed == nil {
		c.sphereUsed = map[int64]float64{}
	}
	for _, id := range c.ids {
		to := c.world[id]
		need := -to.balance[1208]
		if !significant(need) {
			continue
		}
		if c.req.Dyson.Mode == "external" || explicit {
			to.add(Port{ItemID: 1208, Kind: "external", RatePerMinute: need})
			c.result.External = append(c.result.External, External{PlanetID: id, ItemID: 1208, Kind: "declared-input", RatePerMinute: need})
			to.balance[1208] = 0
			continue
		}
		if c.req.Dyson.Mode != "sphere" {
			c.failure = "临界光子需要戴森球：请指定接收恒星与星球，或明确选择玩家外供"
			c.missing = true
			return
		}
		for _, supply := range c.req.Dyson.Supplies {
			perReceiver := 6.0
			if c.req.Dyson.Lenses {
				perReceiver = 12
			}
			receivers := make([]int64, 0, len(supply.PlanetIDs))
			for _, receiver := range supply.PlanetIDs {
				if !c.excludedSources[resourceSite{planetID: receiver, itemID: 1208}] {
					receivers = append(receivers, receiver)
				}
			}
			for index, receiver := range receivers {
				from := c.world[receiver]
				remaining := max(0, supply.AvailableMW-c.sphereUsed[supply.StarID])
				unitPower := perReceiver * 20 / supply.Efficiency
				availableCount := int(math.Floor((remaining + 1e-6) / unitPower))
				// 用户选中的接收星共同承担接收区，避免把全部接收站挤在第一颗星球。
				count := min(availableCount, int(math.Ceil(need/(perReceiver*supply.Uptime*float64(len(receivers)-index)))))
				if count <= 0 {
					continue
				}
				rate := c.transfer(from, to, 1208, min(need, float64(count)*perReceiver*supply.Uptime))
				if !significant(rate) {
					continue
				}
				if from.plan.Photons == nil {
					from.plan.Photons = &PhotonPlan{Efficiency: supply.Efficiency, Uptime: supply.Uptime}
				}
				p := from.plan.Photons
				p.Count += count
				p.RatePerMinute += rate
				p.SpherePowerMW += float64(count) * unitPower
				if c.req.Dyson.Lenses {
					p.LensRatePerMinute += float64(count)
				}
				c.sphereUsed[supply.StarID] += float64(count) * unitPower
				from.add(Port{ItemID: 1208, Kind: "mining", RatePerMinute: rate})
				to.balance[1208] += rate
				need -= rate
				if !significant(need) {
					break
				}
			}
			if !significant(need) {
				break
			}
		}
		if significant(need) {
			c.failure = fmt.Sprintf("戴森球可用功率或接收运输能力不足，仍缺 %.4g 个临界光子/分钟", need)
			return
		}
	}
}

// sourceFor 职责：把一种实际电源换算成稳定供电合同；使用：自动比较或指定设备；边界：所有环境声明在结果保留。
func (c *candidate) sourceFor(site *location, choice EnergyChoice, mw float64, catalog *facts.Catalog) (PowerSource, error) {
	id := choice.FacilityID
	source := PowerSource{FacilityID: id, Factor: 1}
	device, ok := catalog.Lookup(id)
	if !ok || device.Power == nil {
		return source, fmt.Errorf("缺少所选发电设备的原版数据")
	}
	unit := float64(device.Power.Generation) * .00006
	switch id {
	case 2203:
		source.Factor = site.planet.WindFactor
		if source.Factor <= 0 {
			return source, fmt.Errorf("这颗星球没有可用风力")
		}
	case 2205:
		if choice.SolarFactor <= 0 || choice.SolarDuty <= 0 || choice.SolarDuty > 1 {
			return source, fmt.Errorf("太阳能需要确认日照与蓄电条件")
		}
		source.Factor = choice.SolarFactor * choice.SolarDuty
	case 2204, 2211, 2210:
		fuel := choice.FuelItemID
		if fuel == 0 {
			fuel = map[int]int{2204: 1006, 2211: 1802, 2210: 1803}[id]
		}
		item, known := catalog.Lookup(fuel)
		if !known || item.HeatValue <= 0 || item.FuelType&device.Power.FuelMask == 0 {
			return source, fmt.Errorf("所选燃料不能用于这类电厂")
		}
		source.FuelItemID = fuel
		if fuel == 1804 {
			source.Factor = 2
		}
		source.Count = int(math.Ceil(mw / (unit * source.Factor)))
		source.PowerMW = float64(source.Count) * unit * source.Factor
		source.FuelRatePerMinute = float64(source.Count) * float64(device.Power.FuelUse) * 3600 * source.Factor / float64(item.HeatValue)
		return source, nil
	case 2208:
		found := false
		for _, supply := range c.req.Dyson.Supplies {
			if c.req.Dyson.Mode != "sphere" || !slices.Contains(supply.PlanetIDs, site.planet.ID) {
				continue
			}
			unit = 15
			if c.req.Dyson.Lenses {
				unit = 30
			}
			source.Factor = supply.Uptime
			source.Count = int(math.Ceil(mw / (unit * supply.Uptime)))
			source.SpherePowerMW = float64(source.Count) * unit / supply.Efficiency
			if source.SpherePowerMW+c.sphereUsed[supply.StarID] > supply.AvailableMW+1e-6 {
				return source, fmt.Errorf("光子与射线发电合计超出同一戴森球的可用功率")
			}
			if c.req.Dyson.Lenses {
				source.FuelItemID = 1209
				source.FuelRatePerMinute = float64(source.Count)
			}
			found = true
			break
		}
		if !found {
			return source, fmt.Errorf("射线发电需要声明该恒星的戴森球与接收星球")
		}
	case 2213:
		if !site.planet.Lava {
			return source, fmt.Errorf("地热需要可核验的熔岩落点；该星球没有已确认的熔岩地表")
		}
		// 熔岩强度公式的下界，落位后仍验证每个采样点和机组间距。
		source.Factor = .1
	case 2206:
		return source, fmt.Errorf("蓄电器只储存能量，请选择充电电源；太阳能会自动配套跨夜蓄电器")
	case 2209:
		if choice.ChargePlanetID == site.planet.ID {
			return source, fmt.Errorf("充电星球与放电星球不能相同")
		}
		unit = float64(device.Power.ExchangeEnergyPerTick) * .00006
		source.Mode, source.PeerPlanetID = "discharge", choice.ChargePlanetID
	default:
		return source, fmt.Errorf("所选设施不是持续电源")
	}
	if unit*source.Factor <= 0 || !finite(mw/(unit*source.Factor)) {
		return source, fmt.Errorf("所选电源缺少有效持续功率")
	}
	if source.Count == 0 {
		source.Count = int(math.Ceil(mw / (unit * source.Factor)))
	}
	source.PowerMW = float64(source.Count) * unit * source.Factor
	return source, nil
}

// fuelEstimate 职责：缓存一件每分钟燃料的上游成本；使用：同一候选的各星电源比较；边界：使用精确机器量，避免把单位需求的取整误差线性放大。
type fuelEstimate struct {
	plan     quant.Result
	machines float64
	powerMW  float64
	err      error
}

// energyEstimates 职责：共享本候选的单位估价与已选燃料额度；使用：自动推荐；边界：只预留本轮相对既有辅助供给的增量。
type energyEstimates struct {
	fuels    map[int]fuelEstimate
	selected map[int]float64
	raw      map[int]float64
	// rawCapacity 缓存本轮不再变化的矿源上界，已选燃料增量仍由 raw 单独扣除。
	rawCapacity map[int][3]float64
}

func newEnergyEstimates() *energyEstimates {
	return &energyEstimates{fuels: map[int]fuelEstimate{}, selected: map[int]float64{}, raw: map[int]float64{}, rawCapacity: map[int][3]float64{}}
}

func (c *candidate) estimateFuel(item int, estimates *energyEstimates) fuelEstimate {
	if cached, found := estimates.fuels[item]; found {
		return cached
	}
	q := c.req.Quant
	q.Targets = []quant.Target{{ItemID: item, RatePerMinute: 1}}
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	plan, err := quant.Solve(ctx, q)
	estimate := fuelEstimate{plan: plan, powerMW: plan.PowerMW, err: err}
	for _, line := range plan.Productions {
		estimate.machines += line.ExactMachineCount
	}
	// 与主负荷核算相同的分拣臂预留，最终仍须按实际设备复核。
	estimate.powerMW += estimate.machines * .054
	estimates.fuels[item] = estimate
	return estimate
}

// availableFuelRaw 职责：汇总当前候选尚未占用的真实矿源与最低采集功耗；使用：排除没有供给条件的燃料；边界：未精算矿种、巨星及已禁用矿源不算已知能力。
func (c *candidate) availableFuelRaw(item int, catalog *facts.Catalog) (available, powerPerRate, machinesPerRate float64) {
	powerPerRate = math.Inf(1)
	for _, id := range c.ids {
		site := c.world[id]
		if !site.planet.Buildable || c.excludedSources[resourceSite{planetID: id, itemID: item}] {
			continue
		}
		used := 0.0
		for _, mine := range site.plan.Mines {
			if mine.ItemID == item {
				used += mine.RatePerMinute
			}
		}
		for _, resource := range site.planet.Resources {
			if resource.ItemID != item {
				continue
			}
			capacity, unit, facility := resourceCapacity(c.req, resource, math.MaxFloat64)
			if limit, known := c.sourceLimits[resourceSite{planetID: id, itemID: item}]; known {
				capacity = min(capacity, limit)
			}
			if capacity <= 0 || unit <= 0 {
				continue
			}
			available += max(0, capacity-used)
			device, ok := catalog.Lookup(facility)
			if !ok || device.Power == nil {
				continue
			}
			power := float64(device.Power.Work) * .00006 / unit
			if facility == 2316 {
				power *= c.req.Mining.FacilityMultiplier * c.req.Mining.FacilityMultiplier
			}
			if power < powerPerRate {
				powerPerRate, machinesPerRate = power, 1/unit
			}
		}
	}
	return available, powerPerRate, machinesPerRate
}

// availableFuelPhotons 职责：按恒星剩余额度计算辅助燃料可用光子；使用：已有生产光子和射线发电扣款后；边界：接收星数量不复制恒星额度。
func (c *candidate) availableFuelPhotons() (available, receiverPerRate, spherePerRate float64) {
	receiverPerRate, spherePerRate = math.Inf(1), math.Inf(1)
	if c.req.Dyson.Mode != "sphere" {
		return
	}
	rate := 6.0
	if c.req.Dyson.Lenses {
		rate = 12
	}
	for _, supply := range c.req.Dyson.Supplies {
		if len(supply.PlanetIDs) == 0 || supply.Efficiency <= 0 || supply.Uptime <= 0 {
			continue
		}
		unitPower := rate * 20 / supply.Efficiency
		count := math.Floor(max(0, supply.AvailableMW-c.sphereUsed[supply.StarID]) / unitPower)
		available += count * rate * supply.Uptime
		receiverPerRate = min(receiverPerRate, 1/(rate*supply.Uptime))
		spherePerRate = min(spherePerRate, 20/(supply.Efficiency*supply.Uptime))
	}
	return
}

// energyScore 职责：比较完整供能链的设备、运输与能耗负担；使用：自动选择；边界：耗电不小于发电的燃料链不能用自身闭合。
func (c *candidate) energyScore(source PowerSource, catalog *facts.Catalog, estimates *energyEstimates) (float64, map[int]float64, bool) {
	score := float64(source.Count)
	reserved := map[int]float64{}
	if source.FuelItemID == 0 || source.FuelRatePerMinute <= 0 || !c.req.EnableFuel {
		return score, reserved, true
	}
	estimate := c.estimateFuel(source.FuelItemID, estimates)
	if estimate.err != nil {
		return 0, nil, false
	}
	prior := 0.0
	for _, supply := range c.req.energySupply {
		if supply.ItemID == source.FuelItemID {
			prior += supply.RatePerMinute
		}
	}
	// 上一轮的燃料产量已经包含在矿区和光子负荷中，只检查新增部分。
	increment := max(0, estimates.selected[source.FuelItemID]+source.FuelRatePerMinute-prior) - max(0, estimates.selected[source.FuelItemID]-prior)
	power, machines, logistics := estimate.powerMW, estimate.machines, 0.0
	for _, raw := range estimate.plan.RawInputs {
		logistics += raw.RatePerMinute
		if slices.Contains(estimate.plan.ExternalInputItemIDs, raw.ItemID) || raw.ItemID == 1208 && c.req.Dyson.Mode == "external" || raw.ItemID != 1208 && !enabled(c.req.EnableMining, true) {
			continue
		}
		additional := raw.RatePerMinute * increment
		if raw.ItemID == 1208 {
			available, receivers, _ := c.availableFuelPhotons()
			if estimates.raw[raw.ItemID]+additional > available+quant.FlowTolerance(available, additional) || !finite(receivers) {
				return 0, nil, false
			}
			machines += receivers * raw.RatePerMinute
		} else {
			capacity, cached := estimates.rawCapacity[raw.ItemID]
			if !cached {
				capacity[0], capacity[1], capacity[2] = c.availableFuelRaw(raw.ItemID, catalog)
				estimates.rawCapacity[raw.ItemID] = capacity
			}
			available, miningPower, miners := capacity[0], capacity[1], capacity[2]
			if estimates.raw[raw.ItemID]+additional > available+quant.FlowTolerance(available, additional) || !finite(miningPower) {
				return 0, nil, false
			}
			power += miningPower * raw.RatePerMinute
			machines += miners * raw.RatePerMinute
		}
		reserved[raw.ItemID] += additional
	}
	burden := power * source.FuelRatePerMinute / source.PowerMW
	if !finite(burden) || burden >= 1-1e-9 {
		return 0, nil, false
	}
	// 稳态闭合需要额外发电来制造燃料；1/(1-burden) 是该反馈的实际放大系数。
	// 所有燃料使用相同的设备、MW 和带吞吐单位，不再按物品手写权重。
	// 已声明的戴森球余量仅作为容量约束；其 MW 不能直接当作新增设备数量罚分。
	score += source.FuelRatePerMinute * (machines + power + logistics/1800)
	return score / (1 - burden), reserved, true
}

func (c *candidate) chooseEnergy(site *location, mw float64, catalog *facts.Catalog) (PowerSource, error) {
	return c.chooseEnergyEstimated(site, mw, catalog, newEnergyEstimates())
}

func (c *candidate) chooseEnergyEstimated(site *location, mw float64, catalog *facts.Catalog, estimates *energyEstimates) (PowerSource, error) {
	choice := c.req.energyChoice(site.planet.ID)
	if choice.Mode != "auto" {
		return c.sourceFor(site, choice, mw, catalog)
	}
	// 自动策略只推荐持续电源。枢纽和蓄电器必须由显式充电/储能合同安排。
	allowed := c.req.Energy.AllowedFacilityIDs
	if len(allowed) == 0 {
		allowed = []int{2203, 2211, 2210, 2208, 2204}
	}
	best, cost := PowerSource{}, math.Inf(1)
	var bestReserved map[int]float64
	for _, id := range allowed {
		if id == 2206 || id == 2209 {
			continue
		}
		choice.FacilityID, choice.FuelItemID = id, 0
		source, err := c.sourceFor(site, choice, mw, catalog)
		if err != nil {
			continue
		}
		score, reserved, feasible := c.energyScore(source, catalog, estimates)
		if !feasible {
			continue
		}
		preferred := id == c.req.settledAutoPower[site.planet.ID]
		if score < cost || preferred {
			best, cost = source, score
			bestReserved = reserved
		}
		if preferred {
			break
		}
	}
	if best.Count == 0 {
		return best, fmt.Errorf("允许的电源缺少燃料或戴森球供给，或燃料制造耗电不低于发电，请调整电源与供给条件")
	}
	estimates.selected[best.FuelItemID] += best.FuelRatePerMinute
	for item, rate := range bestReserved {
		estimates.raw[item] += rate
	}
	return best, nil
}

// planEnergy 职责：确定电源、充电端负荷和辅助物料；使用：常规资源分配完成后；边界：同一任务不把循环电池当消耗品制造。
func (c *candidate) planEnergy() error {
	catalog, err := facts.Load()
	if err != nil {
		return err
	}
	if c.sphereUsed == nil {
		c.sphereUsed = map[int64]float64{}
	}
	needs := map[int]float64{}
	estimates := newEnergyEstimates()
	consume := func(site *location, item int, rate float64) {
		if rate <= 0 {
			return
		}
		site.add(Port{ItemID: item, Kind: "power-fuel", RatePerMinute: rate})
		if c.req.EnableFuel {
			needs[item] += rate
			site.balance[item] -= rate
		} else {
			site.add(Port{ItemID: item, Kind: "external", RatePerMinute: rate})
			c.result.External = append(c.result.External, External{PlanetID: site.planet.ID, ItemID: item, Kind: "fuel-input", RatePerMinute: rate})
		}
	}
	loads := map[int64]float64{}
	for _, id := range c.ids {
		site := c.world[id]
		arms := float64(site.plan.Plan.MachineCount) * .054
		if len(c.ids) > 1 && c.req.ConstructionInserterLevel > 0 {
			arms = max(arms, MinimumFactoryInserterMW(catalog, c.req.Quant.Dataset, site.plan.Plan.Productions, c.req.ConstructionInserterLevel))
			arms = estimatedFactoryEquipmentMW(catalog, c.req, site, arms)
		}
		droneMW := estimatedFactoryDronePower(site, c.req.ConstructionDroneCargo)
		loads[id] = (powerDemand(catalog, site) + c.req.postPowerShipping[id] + arms + droneMW) * max(1, c.powerMultipliers[id])
		if p := site.plan.Photons; p != nil {
			consume(site, 1209, p.LensRatePerMinute)
		}
	}
	for _, id := range c.ids {
		if loads[id] <= 0 {
			continue
		}
		choice := c.req.energyChoice(id)
		if !c.req.EnablePower || choice.FacilityID != 2209 || choice.Mode != "fixed" {
			continue
		}
		charge := choice.ChargePlanetID
		chargeChoice := c.req.energyChoice(charge)
		if charge == id || chargeChoice.Mode == "fixed" && chargeChoice.FacilityID == 2209 {
			return fmt.Errorf("能量枢纽的充电端必须使用独立电源，不能形成充放电循环")
		}
		source, err := c.sourceFor(c.world[id], choice, loads[id], catalog)
		if err != nil {
			return err
		}
		loads[charge] += source.PowerMW
		charging := source
		charging.Mode = "charge"
		charging.PeerPlanetID = id
		charging.PowerMW = 0
		if c.world[charge].plan.Power == nil {
			c.world[charge].plan.Power = &PowerPlan{}
		}
		c.world[charge].plan.Power.Sources = append(c.world[charge].plan.Power.Sources, charging)
		// 满、空电池是同一循环货物，两端各有真实枢纽出入口。
		rate := source.PowerMW / 9
		for _, pair := range []struct {
			from, to int64
			item     int
		}{{charge, id, 2207}, {id, charge, 2206}} {
			from, to := c.world[pair.from], c.world[pair.to]
			from.add(Port{ItemID: pair.item, Kind: "mining", RatePerMinute: rate})
			to.add(Port{ItemID: pair.item, Kind: "power-fuel", RatePerMinute: rate})
			if c.transfer(from, to, pair.item, rate)+1e-6 < rate {
				return fmt.Errorf("电池往返运输能力不足")
			}
		}
		c.result.External = append(c.result.External, External{PlanetID: charge, ItemID: 2206, Kind: "battery-startup"})
	}
	for _, id := range c.ids {
		site, mw := c.world[id], loads[id]
		if mw <= 0 {
			continue
		}
		choice := c.req.energyChoice(id)
		if !c.req.EnablePower || choice.Mode == "grid" {
			if c.req.EnablePower && mw > choice.GridCapacityMW+1e-6 {
				return fmt.Errorf("%s的电网余量不足：需要 %.3f MW，可用 %.3f MW", site.planet.Name, mw, choice.GridCapacityMW)
			}
			if c.req.EnablePower {
				if site.plan.Power == nil {
					site.plan.Power = &PowerPlan{}
				}
				// 充电枢纽已经在上一轮登记；外电只补足容量声明，保留真实设备且不冒充新建发电。
				site.plan.Power.RequiredMW = mw
				site.plan.Power.ExternalGridMW = choice.GridCapacityMW
			}
			c.result.External = append(c.result.External, External{PlanetID: id, Kind: "grid-power", PowerMW: mw})
			continue
		}
		source, err := c.chooseEnergyEstimated(site, mw, catalog, estimates)
		if err != nil {
			return fmt.Errorf("%s：%w", site.planet.Name, err)
		}
		if site.plan.Power == nil {
			site.plan.Power = &PowerPlan{}
		}
		power := site.plan.Power
		power.Sources = append(power.Sources, source)
		power.RequiredMW = mw
		power.PowerMW += source.PowerMW
		power.Count += source.Count
		power.FacilityID = source.FacilityID
		power.FuelItemID = source.FuelItemID
		power.FuelRatePerMinute = source.FuelRatePerMinute
		consume(site, source.FuelItemID, source.FuelRatePerMinute)
		c.sphereUsed[site.planet.StarID] += source.SpherePowerMW
		if source.FacilityID == 2205 && choice.SolarDuty < 1 {
			count := int(math.Ceil(max(mw*choice.NightSeconds/540, mw/2.25, mw*(1-choice.SolarDuty)/choice.SolarDuty/1.5)))
			power.Sources = append(power.Sources, PowerSource{FacilityID: 2206, Count: count, Mode: "buffer", StoredMJ: float64(count) * 540})
			c.result.External = append(c.result.External, External{PlanetID: id, Kind: "solar-conditions"})
		}
	}
	for item, rate := range needs {
		c.result.EnergyNeeds = append(c.result.EnergyNeeds, quant.ItemRate{ItemID: item, RatePerMinute: rate})
	}
	sort.Slice(c.result.EnergyNeeds, func(i, j int) bool { return c.result.EnergyNeeds[i].ItemID < c.result.EnergyNeeds[j].ItemID })
	if !matchingEnergySupply(c.req.energySupply, c.result.EnergyNeeds) {
		return nil
	}
	for _, need := range c.result.EnergyNeeds {
		c.allocate(need.ItemID, true, false, false)
		if c.failure != "" {
			return fmt.Errorf("能源配套：%s", c.failure)
		}
	}
	if len(c.req.Dyson.Supplies) > 0 {
		c.result.Warnings = append(c.result.Warnings, "戴森球功率为本次声明的可用余量；接收站需预热并满足所设连续接收率")
		c.result.Status = "conditional"
	}
	return nil
}
