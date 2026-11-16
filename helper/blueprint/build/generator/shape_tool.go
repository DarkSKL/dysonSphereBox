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

func (t *toolset) CheckShape(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	selected, err := workflow.Read(s, t.keys.selected)
	if err != nil {
		return err
	}
	metrics := selected.Verified.Metrics()
	if !core.LayoutShapeAccepted(metrics) {
		return domainFailure(ctx, s, &blueprint.Error{Code: "GEN_E_SHAPE", Message: fmt.Sprintf("当前排布的实际长宽比 %.3f 超出近方形上限1.8", metrics.EnvelopeAspectRatio)})
	}
	return nil
}
