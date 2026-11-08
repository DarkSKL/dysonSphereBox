package build

// 文件职责：在原生高纬坐标中完成端口适配后再固定供电坐标。
// 使用：高纬整厂几何入口；边界：失败候选不提交，前三档沿用已验收策略。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
)

// denseNativeLatitude 职责：识别原生高纬生成；使用：几何和供电共同入口；边界：普通模式及前三档不改变。
func denseNativeLatitude(req Request) bool {
	if req.Density != densityDense || req.SurfaceLayout == nil {
		return false
	}
	selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	return ok && selection.tier >= 4
}

// validateNativeSurfaceCandidates 职责：把端口适配加入原生锚点检验；使用：补电前和最终编码；边界：仅成功候选修改实体，不在供电后迁移整图。
func (b *builder) validateNativeSurfaceCandidates(ctx context.Context, req Request, encoding *surfaceEncoding, records []blueprint.Building) ([]blueprint.Building, error) {
	if err := b.alignFlowMonitorDirections(ctx, encoding, records, true); err != nil {
		return records, err
	}
	if !denseNativeLatitude(req) {
		return records, encoding.validateAnchorCandidates(ctx, records)
	}
	var last error = &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "高纬没有可验证的原生锚点"}
	for _, anchor := range encoding.anchorCandidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		encoding.reanchorSurfaceEncoding(anchor)
		adjusted, err := b.alignNativeInserterPorts(ctx, encoding, records)
		if err == nil {
			return adjusted, nil
		}
		last = fmt.Errorf("锚点%d: %w", anchor, err)
	}
	return records, last
}

// scaleNativeLatitudeLayout 职责：按纬带最窄处的物理格距补偿经向间距；使用：高纬编码前；边界：不倾斜建筑，完整几何仍逐锚点复核。
func (b *builder) scaleNativeLatitudeLayout(layout *SurfaceLayout) {
	if b.nativeLatitudeScaled || b.adapter == nil || b.adapter.density != densityDense || layout == nil {
		return
	}
	selection, ok := densityLatitudeSelectionFor(layout.LatitudeZone)
	if !ok || selection.tier < 4 {
		return
	}
	low, high := bounds(b.all)
	height := int(math.Ceil(high.y-low.y)) + 1
	anchors := surfaceLayoutAnchorCandidates(layout, 1, height)
	if len(anchors) == 0 {
		return
	}
	pole := math.Max(math.Abs(float64(anchors[0])), math.Abs(float64(anchors[0]+height-1)))
	scale := float64(longitudeSegmentCount(anchors[0], 200)) / 200 / math.Cos(pole*math.Pi/500)
	for _, p := range b.all {
		p.a.x *= scale
		p.b.x *= scale
		p.densityCargo.relative.x *= scale
	}
	b.nativeLatitudeScaled = true
	b.nativeLongitudeScale = scale
}
