package build

// 文件职责：编排共享物流塔全场布局及候选收尾阶段。
// 使用：mode 3/4 适配器调用；边界：不承担公共请求准备和最终结果发布。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
	"DysonSphereBox/helper/quant"
)

// generateStationMode 职责：校验塔模式布局计划并生成共享设施全场候选；使用：行星塔和星际塔的普通/超密度组合；边界：失败直接返回，不退回旧中央庭院布局。
func generateStationMode(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset) (Result, error) {
	if builder.adapter == nil || builder.adapter.station == nil || builder.adapter.station.Layout == nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔组合缺少布局适配器"}
	}
	stages := builder.adapter.station
	if stages.Config.Mode != request.Mode {
		return Result{}, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔阶段与请求模式不匹配"}
	}
	input, err := prepareStationLayoutInput(ctx, request, dataset, builder.setting, builder.flowGraph)
	if err != nil {
		return Result{}, err
	}
	layout, err := stages.Layout(ctx, input)
	if err != nil {
		return Result{}, err
	}
	if err := stationlayout.Validate(ctx, input, layout, stages.Config); err != nil {
		return Result{}, err
	}
	if builder.adapter.density == densityDense {
		return builder.generateDenseStationField(ctx, request, dataset, input, layout, stages.Config)
	}
	return builder.generateStandardStationField(ctx, request, dataset, stages.Config)
}
