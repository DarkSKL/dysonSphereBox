package build

// 文件职责：按真实运输能力自动提高本次蓝图的传送带与分拣器等级。
// 使用：公开生成入口及共享准备阶段；边界：不修改用户请求、量化产量或生产设备，最高档仍须通过原有全部校验。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// transportUpgradeNeeds 职责：遍历单错误及候选聚合错误中的容量原因；使用：决定可重试的运输等级；边界：碰撞、陈旧计划与未验证拓扑不单独触发升档。
func transportUpgradeNeeds(err error) (belt, inserter bool) {
	if failure, ok := err.(*blueprint.Error); ok {
		switch failure.Code {
		case "GEN_E_INSERTER_CAPACITY":
			inserter = true
		case "GEN_E_OVERFLOW", "GEN_E_BELT_CAPACITY":
			belt = true
		case "GEN_E_PORT_CAPACITY":
			belt, inserter = true, true
		}
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			b, i := transportUpgradeNeeds(cause)
			belt, inserter = belt || b, inserter || i
		}
	} else if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		b, i := transportUpgradeNeeds(wrapped.Unwrap())
		belt, inserter = belt || b, inserter || i
	}
	return
}

// upgradeProductionTransport 职责：先满足每种机器的单机/堆叠端口及单带毛流下界；使用：可信计划验证之后；边界：只在容量失败时逐档提升，保留并联臂与拆行策略，最高等级无解返回原错误。
func upgradeProductionTransport(ctx context.Context, plan quant.Result, dataset *quant.Dataset, setting Setting) (Setting, error) {
	for _, production := range plan.Productions {
		if err := ctx.Err(); err != nil {
			return setting, err
		}
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return setting, err
		}
		for {
			err = validateMachineInserterCapacity(production, recipe, setting)
			_, needsInserter := transportUpgradeNeeds(err)
			if err == nil {
				break
			}
			if !needsInserter || setting.InserterLv >= 4 {
				return setting, err
			}
			setting.InserterLv++
		}
		for {
			_, err = lineCounts(production, recipe, setting, dataset.ByID[production.MachineID].Speed)
			needsBelt, _ := transportUpgradeNeeds(err)
			if err == nil {
				break
			}
			if !needsBelt || setting.BeltLv >= 3 {
				return setting, err
			}
			setting.BeltLv++
		}
	}
	return normalizeSetting(setting)
}

// transportUpgradeMessage 职责：说明本次蓝图实际运输档位；使用：进度、结果提示与诊断；边界：只描述有变化的设备，不承诺最高档之外的能力。
func transportUpgradeMessage(original, actual Setting) string {
	changes := ""
	if actual.BeltLv != original.BeltLv {
		changes = fmt.Sprintf("传送带 Mk.%d → Mk.%d", original.BeltLv, actual.BeltLv)
	}
	if actual.InserterLv != original.InserterLv {
		if changes != "" {
			changes += "，"
		}
		changes += fmt.Sprintf("分拣器 Mk.%d → Mk.%d", original.InserterLv, actual.InserterLv)
	}
	return "本次蓝图实际使用的运输设备已自动升档：" + changes
}
