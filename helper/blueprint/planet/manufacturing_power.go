package planet

// 文件职责：在跨星制造的全纬区后备搜索前核算不可省去的用电；使用：判断当前整份供电合同；边界：补电后仍需按实图核验。

import (
	"DysonSphereBox/helper/factory/statistics/facts"
	"DysonSphereBox/helper/industryplan"
)

// insufficientManufacturingPower 返回已经超过本星供给的负荷下限。输入占地为制造前已安装的矿区及接收器。
func insufficientManufacturingPower(req Request, input ProductionRequest) float64 {
	plan := input.Plan
	power := plan.Power
	if power == nil || power.PowerMW <= 0 || len(plan.Plan.Productions) == 0 {
		return 0
	}
	catalog, err := facts.Load()
	if err != nil {
		return 0
	}
	floor := actualElectricalDemand(req, plan, catalog)
	floor += industryplan.MinimumFactoryInserterMW(catalog, plan.Plan.Dataset, plan.Plan.Productions, input.Options.Setting.InserterLv)
	if floor <= power.PowerMW+power.ExternalGridMW+1e-6 {
		return 0
	}
	return floor
}
