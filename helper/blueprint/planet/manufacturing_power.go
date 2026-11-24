package planet

// 文件职责：在跨星制造的全纬区后备搜索前核算不可省去的用电；使用：判断当前整份供电合同；边界：补电后仍需按实图核验。

import (
	"fmt"

	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
)

// SourceLoadMW 记录已落位矿区和接收器自身的负荷，供同一采集布局复用。制造、发船和后续物流设备另计。
type SourceLoadMW float64

// measuredSourceLoad 只计入此时已经摆出的实体。调用点在制造开始前，不能混入随制造或航线合同变化的功率。
func measuredSourceLoad(req Request) SourceLoadMW {
	catalog, err := facts.Load()
	if err != nil {
		return 0
	}
	return SourceLoadMW(actualElectricalDemand(req, industryplan.PlanetPlan{}, catalog))
}

// shortfall 按本轮制造、分拣器和发船合同复算负荷下限。供给足够或资料不足时交给后续实装验电。
func (load SourceLoadMW) shortfall(plan industryplan.PlanetPlan, inserter int) float64 {
	power := plan.Power
	if power == nil || power.PowerMW <= 0 || len(plan.Plan.Productions) == 0 {
		return 0
	}
	catalog, err := facts.Load()
	if err != nil {
		return 0
	}
	floor := float64(load) + actualElectricalDemand(Request{}, plan, catalog)
	floor += industryplan.MinimumFactoryInserterMW(catalog, plan.Plan.Dataset, plan.Plan.Productions, inserter)
	if floor <= power.PowerMW+power.ExternalGridMW+1e-6 {
		return 0
	}
	return floor
}

// CheckManufacturingPower 用已落位的采集负荷检查当前可迁星的制造合同。欠电交回原星补电，恢复完整搜索时仍允许先实放制造前缀。
func (load SourceLoadMW) CheckManufacturingPower(plan industryplan.PlanetPlan, inserter int) error {
	if plan.CanRelocate && !plan.SinglePlanet {
		if floor := load.shortfall(plan, inserter); floor > 0 {
			return &industryplan.PowerLoadError{RequiredMW: floor, Err: &Error{
				PlanetID: plan.PlanetID, Code: "PLANET_POWER", Message: fmt.Sprintf("已落位采集、制造与发船的负荷下限 %.3f MW 超出供给 %.3f MW", floor, plan.Power.PowerMW+plan.Power.ExternalGridMW),
			}}
		}
	}
	return nil
}
