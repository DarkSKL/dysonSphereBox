package build

// 文件职责：编排物流塔模式的布局、装配和候选收尾阶段。
// 使用：mode 3/4 适配器调用；边界：不承担公共请求准备和最终结果发布。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
	"DysonSphereBox/helper/quant"
)

// generateStationMode 职责：执行当前目录组合提供的布局阶段；使用：所有物流塔组合的统一编排；边界：不再决定模式配置或布局算法，旧实体装配逐步迁移。
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
	blocks, err := assembleStationLayout(ctx, builder, request, dataset, input, layout, stages.Config)
	if err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔生成已取消或超时"}
	}
	return builder.finishSurroundStationLayouts(ctx, request, dataset, blocks)
}
