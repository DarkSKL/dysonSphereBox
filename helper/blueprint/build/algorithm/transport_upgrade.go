package build

// 文件职责：按真实运输能力自动提高本次蓝图的传送带与分拣器等级。
// 使用：公开生成入口及共享准备阶段；边界：不修改用户请求、量化产量或生产设备，最高档仍须通过原有全部校验。

import (
	"context"
	"fmt"
	"sort"

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

// generateWithTransportUpgrades 职责：在共享任务时限内重建容量不足的布局，优先尝试升档步数较少的组合；使用：全部公开模式和纬带；边界：最多12种等级组合，失败候选不发布最终蓝图，取消立即停止。
func generateWithTransportUpgrades(ctx context.Context, req Request, adapter *modeAdapter) (Result, error) {
	original, err := normalizeSetting(req.Setting)
	if err != nil {
		return Result{}, err
	}
	pending := []Setting{original}
	visited := map[[2]int]bool{}
	var last error
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return Result{}, previewContextError(err)
		}
		setting := pending[0]
		pending = pending[1:]
		key := [2]int{setting.BeltLv, setting.InserterLv}
		if visited[key] {
			continue
		}
		candidate := req
		candidate.Setting = setting
		prepared, err := prepareGeneration(ctx, candidate, adapter)
		setting = prepared.request.Setting
		key = [2]int{setting.BeltLv, setting.InserterLv}
		if visited[key] {
			continue
		}
		visited[key] = true
		if err == nil {
			if setting.BeltLv != original.BeltLv || setting.InserterLv != original.InserterLv {
				reportProgress(ctx, "transport-upgrade", transportUpgradeMessage(original, setting), 0, 0)
			}
			var result Result
			result, err = generateTransportAttempt(ctx, prepared, adapter)
			if err == nil {
				if setting.BeltLv != original.BeltLv || setting.InserterLv != original.InserterLv {
					message := transportUpgradeMessage(original, setting)
					result.Warnings = append(result.Warnings, message)
					if result.LogisticsSummary != nil {
						result.LogisticsSummary.Checks = append(result.LogisticsSummary.Checks, LogisticsCheck{RuleID: "automatic-transport-upgrade", Status: "passed", Message: message + "；已按实际升档设备完成容量、连通和碰撞校验"})
					}
				}
				return publishFinalBlueprintProgress(ctx, prepared.request, prepared.dataset, result, nil)
			}
		}
		last = err
		if ctx.Err() != nil {
			return Result{}, previewContextError(ctx.Err())
		}
		belt, inserter := transportUpgradeNeeds(err)
		if inserter && setting.InserterLv < 4 {
			next := setting
			next.InserterLv++
			pending = append(pending, next)
		}
		if belt && setting.BeltLv < 3 {
			next := setting
			next.BeltLv++
			pending = append(pending, next)
		}
		sort.SliceStable(pending, func(i, j int) bool {
			return pending[i].BeltLv+pending[i].InserterLv < pending[j].BeltLv+pending[j].InserterLv
		})
	}
	return Result{}, last
}

// generateTransportAttempt 职责：执行一个已归一化运输等级组合的完整布局；使用：自动升档循环；边界：原生球面回退保持同一等级，最终发布由外层唯一负责。
func generateTransportAttempt(ctx context.Context, prepared preparedGeneration, adapter *modeAdapter) (Result, error) {
	req := prepared.request
	if req.SurfaceLayout != nil && req.SurfaceLayout.LatitudeZone != "" {
		if selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone); ok && req.Density == densityDense && (selection.tier >= 4 || req.Plan.MachineCount >= denseNativeLargeMachineThreshold) {
			return generateDensePolar(ctx, req, adapter)
		}
	}
	prepared.builder.beginPreview()
	result, err := adapter.generate(ctx, prepared.builder, req, prepared.dataset)
	if err != nil && ctx.Err() == nil && denseNativeLayoutRetryable(req, err) {
		prepared.builder.rollbackPreview()
		reportProgress(ctx, "dense-native-retry", "正在按所选纬带的真实球面重新排布与接线", 0, 0)
		return generateDensePolar(ctx, req, adapter)
	}
	if err != nil {
		prepared.builder.rollbackPreview()
	}
	return result, err
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
	return "为满足输送能力，本次蓝图已自动升档：" + changes
}
