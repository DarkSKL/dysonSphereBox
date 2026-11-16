package build

// 文件职责：把真实容量拆行产生的缓存负担纳入原生球面布局分流。
// 使用：已归一化请求的布局入口；边界：不把物流负担伪装为生产设备，不改变目标、产量或设备等级。

import (
	"DysonSphereBox/helper/quant"
	"context"
)

// denseNativeWorkloadKey 职责：保存制造设备与缓存模块合计的几何负担；使用：候选预算与原生坐标分流；边界：不写入量化计划或蓝图。
type denseNativeWorkloadKey struct{}

// denseNativeLargeRequest 职责：识别需要提前固定球面坐标的请求；使用：入口、目标范围和均衡候选共用；边界：未启用缓存估计时保持原机器数量条件。
func denseNativeLargeRequest(ctx context.Context, req Request) bool {
	workload, _ := ctx.Value(denseNativeWorkloadKey{}).(int)
	return req.Plan.MachineCount >= denseNativeLargeMachineThreshold || workload >= denseNativeLargeMachineThreshold
}

// denseNativeWorkload 职责：按满负荷容量组估算缓存模块数；使用：避免中型制造厂加缓存后仍先跑巨大平面候选；边界：只影响算法分流，最终模块数和几何仍由完整装配决定。
func denseNativeWorkload(ctx context.Context, req Request, dataset *quant.Dataset) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	workload := req.Plan.MachineCount
	if req.Density != densityDense || !req.Setting.EnableIntermediateBuffer || workload >= denseNativeLargeMachineThreshold {
		return workload, nil
	}
	for _, production := range req.Plan.Productions {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return 0, err
		}
		recipe, err = effectiveProductionRecipe(production, recipe)
		if err != nil {
			return 0, err
		}
		counts, err := layoutLineCounts(production, recipe, req.Setting, dataset.ByID[production.MachineID].Speed, req.Density)
		if err != nil {
			return 0, err
		}
		counts, err = denseSpatialRowCounts(production, recipe, req.Setting, counts, 32, false)
		if err != nil {
			return 0, err
		}
		outputs := 0
		for _, output := range production.Capacity.Outputs {
			if output.RatePerMinute > 0 {
				outputs++
			}
		}
		workload += len(counts) * outputs
	}
	return workload, nil
}
