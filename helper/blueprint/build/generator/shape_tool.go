package generator

// 文件职责：把近方形要求作为显式工作流验证步骤。
// 使用：完整布局及物理规则通过后、窗口候选择优前；边界：不改实体、不添加填充建筑，不把不达标窗口作为成功截断后续搜索。

import (
	"DysonSphereBox/helper/blueprint"
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
	"context"
	"fmt"
)

// CheckShape 职责：验证已选方案的物理形状；使用：各方案最终汇合；边界：塔口精修允许先改进基线再做最终验收。
func (t *toolset) CheckShape(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	selected, err := workflow.Read(s, t.keys.selected)
	if err != nil {
		return err
	}
	return checkLayoutShape(ctx, s, selected.Verified.Metrics())
}

// CheckCandidateShape 职责：在入口组择优前拒绝不合格形状；使用：直连叶规则之后；边界：不提前消费验证产物或创建候选选择副本。
func (t *toolset) CheckCandidateShape(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	verified, err := workflow.Read(s, t.keys.verified)
	if err != nil {
		return err
	}
	return checkLayoutShape(ctx, s, verified.Metrics())
}

// checkLayoutShape 职责：复用唯一物理形状判定；使用：候选和最终选择的只读适配；边界：只读已算指标，不重算实体几何或放宽阈值。
func checkLayoutShape(ctx context.Context, s *workflow.StepContext, metrics core.LayoutMetrics) error {
	if !core.LayoutShapeAccepted(metrics) {
		return domainFailure(ctx, s, &blueprint.Error{Code: "GEN_E_SHAPE", Message: fmt.Sprintf("当前排布的实际长宽比 %.3f 超出近方形上限%.1f", metrics.EnvelopeAspectRatio, core.LayoutAspectLimit(metrics))})
	}
	return nil
}
