package industryplan

import (
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/quant"
	"fmt"
	"math"
)

const coalItemID = 1006
const thermalFacilityID = 2204

// MinimumFactoryInserterMW 按原配方的进出货种与可选档位，核算非堆叠制造设备必须安装的分拣器负荷。
// 研究站共用底层分拣器，分馏塔与采集设备使用原生带口，这些设备交给实物计量。
func MinimumFactoryInserterMW(catalog *facts.Catalog, dataset string, rows []quant.Production, level int) float64 {
	if len(rows) == 0 {
		return 0
	}
	data, err := quant.LoadDataset(dataset)
	if err != nil {
		return 0
	}
	armMW := math.Inf(1)
	for tier := max(1, level); tier <= 4; tier++ {
		item, ok := catalog.Lookup(2010 + tier)
		if !ok || item.Power == nil || item.Power.Work <= 0 {
			return 0
		}
		armMW = min(armMW, float64(item.Power.Work)*.00006)
	}
	if math.IsInf(armMW, 1) {
		return 0
	}
	var total float64
	for _, row := range rows {
		index, ok := data.RecipeByID[row.RecipeID]
		if !ok || row.MachineType == 6 {
			continue
		}
		recipe := data.Recipes[index]
		if recipe.Type >= 1 && recipe.Type <= 5 {
			total += float64(row.MachineCount) * float64(len(recipe.Items)+len(recipe.Results)) * armMW
		}
	}
	return total
}

// powerDemand 汇总本星制造、采集与已分配航线的满载派船负荷，物流塔同时充电的上限另列。
func powerDemand(catalog *facts.Catalog, site *location) float64 {
	mw := site.plan.OutboundVesselMW
	for _, production := range site.plan.Plan.Productions {
		if production.Capacity != nil {
			mw += production.Capacity.PowerMW
		} else {
			mw += production.PowerMW
		}
	}
	for _, mine := range site.plan.Mines {
		if item, ok := catalog.Lookup(mine.FacilityID); ok && item.Power != nil {
			mw += float64(mine.Count) * float64(item.Power.Work) * 0.00006
		}
	}
	return mw
}

// planPower 职责：为已分工产线规划外接电网或原版火电与煤入口；使用：必要工业物料分配之后；边界：不把火电燃料算为用户交付。
func (c *candidate) planPower() error {
	if c.req.Energy.Default.Mode != "" {
		return c.planEnergy()
	}
	catalog, err := facts.Load()
	if err != nil {
		return err
	}
	generator, _ := catalog.Lookup(thermalFacilityID)
	coal, _ := catalog.Lookup(coalItemID)
	if generator.Power == nil || generator.Power.Generation <= 0 || generator.Power.FuelUse <= 0 || coal.HeatValue <= 0 {
		return fmt.Errorf("缺少火电或煤热值的版本原型")
	}
	perGeneratorMW := float64(generator.Power.Generation) * 0.00006
	perGeneratorFuel := float64(generator.Power.FuelUse) * 3600 / float64(coal.HeatValue)
	fuelSites := 0
	for _, id := range c.ids {
		site := c.world[id]
		mw := (powerDemand(catalog, site) + c.req.postPowerShipping[id]) * max(1, c.powerMultipliers[id])
		if mw <= 0 {
			continue
		}
		if !c.req.EnablePower || !site.planet.Buildable {
			c.result.External = append(c.result.External, External{PlanetID: id, Kind: "grid-power", PowerMW: mw})
			continue
		}
		count := math.Ceil(mw / perGeneratorMW)
		if !finite(count) || count >= float64(math.MaxInt) {
			return fmt.Errorf("星球 %d 火电规模超出计算范围", id)
		}
		rate := count * perGeneratorFuel
		site.plan.Power = &PowerPlan{FacilityID: thermalFacilityID, FuelItemID: coalItemID, Count: int(count), FuelRatePerMinute: rate, PowerMW: count * perGeneratorMW, RequiredMW: mw}
		site.add(Port{ItemID: coalItemID, Kind: "power-fuel", RatePerMinute: rate})
		if c.req.EnableFuel {
			site.balance[coalItemID] -= rate
		} else {
			site.add(Port{ItemID: coalItemID, Kind: "external", RatePerMinute: rate})
			c.result.External = append(c.result.External, External{PlanetID: id, ItemID: coalItemID, RatePerMinute: rate, Kind: "fuel-input"})
		}
		fuelSites++
	}
	if fuelSites == 0 {
		if c.req.EnableFuel {
			c.result.Warnings = append(c.result.Warnings, "已选择燃料配套，但当前方案没有发电燃料负荷")
		}
		return nil
	}
	if c.req.EnableFuel {
		c.allocate(coalItemID, true, false, true)
	}
	c.result.Warnings = append(c.result.Warnings, "火电按未增产煤和已列机器额定功率设计。塔充电尖峰、启动供电以及独立贴入的电网连接需现场核对")
	return nil
}

// checkGrid 职责：按运力与新增采煤披露额外电力；使用：确定最终路线后；边界：塔充电峰值单列，不计作持续耗电。
func (c *candidate) checkGrid() error {
	catalog, err := facts.Load()
	if err != nil {
		return err
	}
	tower, _ := catalog.Lookup(2104)
	for _, site := range c.result.Planets {
		if site.StationCount > 0 && tower.Power != nil {
			peak := float64(tower.Power.Work) * float64(site.StationCount) * .00006
			c.result.External = append(c.result.External, External{PlanetID: site.PlanetID, Kind: "charging-peak", PowerMW: peak})
		}
		if !c.req.EnablePower || c.world[site.PlanetID].planet.Buildable == false {
			continue
		}
		capacity := 0.0
		if site.Power != nil {
			capacity = site.Power.PowerMW + site.Power.ExternalGridMW
		}
		if deficit := powerDemand(catalog, c.world[site.PlanetID]) - capacity; deficit > 1e-6 {
			c.result.External = append(c.result.External, External{PlanetID: site.PlanetID, Kind: "grid-power", PowerMW: deficit})
		}
	}
	return nil
}
