package build

// 文件职责：封闭复制已经完成逐步变换的实体与关系图并建立内容指纹。
// 使用：geometry.freeze节点的最终复制边界；边界：不编排几何步骤、不执行规则或编码，实际变换位于geometry_steps.go。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// freezeFinalGeometry 职责：隔离已定稿记录、请求和全部关系图证据；使用：几何构造完成后；边界：不信任旧规则缓存，不保留可变装配器。
func freezeFinalGeometry(ctx context.Context, b *builder, req Request, dataset *quant.Dataset, surface *surfaceEncoding, document blueprint.Document, bindings []ProductionBinding, shared []int, zones []Zone) (GeometrySnapshot, error) {
	rules, err := captureFinalizationRuleContext(ctx)
	if err != nil {
		return GeometrySnapshot{}, err
	}
	c := newFinalizationCloner()
	data := &finalizedGeometry{
		proofIdentity: new(ruleProofIdentity),
		document:      c.blueprintDocument(document),
		request:       c.buildRequest(req),
		rules:         rules,
		dataset:       copyFinalizationPointer(c, dataset, c.quantDataset),
		evidence:      captureFinalizationEvidence(b, c),
		surface:       copyFinalizationPointer(c, surface, c.buildSurfaceEncoding),
		bindings:      copyFinalizationSlice(c, bindings, c.buildProductionBinding),
		shared:        copyFinalizationSlice(c, shared, func(v int) int { return v }),
		zones:         copyFinalizationSlice(c, zones, c.buildZone),
		warnings:      copyFinalizationSlice(c, b.warnings, c.copyString),
	}
	data.previewScene = c.previewCopy(captureFinalizationPreview(b, data.document))
	data.bytes = c.retainedBytes(4096)
	if data.surface != nil {
		data.surface.positionSnapshot = nil
		data.surface.plan.BeltConnectionsChecked = false
		data.surface.plan.StaticCollidersChecked = false
		data.surface.plan.CollisionCoverage = "none"
	}
	data.fingerprint, err = finalGeometryFingerprint(data.document, data.request, data.rules)
	if err != nil {
		return GeometrySnapshot{}, err
	}
	return GeometrySnapshot{data: data}, nil
}

// surfaceAnchorRetryable 职责：判断球面锚点失败是否可重试；使用：几何定稿及候选搜索；边界：未知原型、超时和未验证错误不被软化。
func surfaceAnchorRetryable(err error) bool {
	failure, ok := err.(*blueprint.Error)
	if !ok {
		return false
	}
	return failure.Code == "GEN_E_COLLISION" || failure.Code == "GEN_E_SURFACE_ROUTING"
}
