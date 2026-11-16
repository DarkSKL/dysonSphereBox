package build

// 文件职责：省去已完成布线且固定单区域包络严格更差的密铺候选的供电求解。
// 使用：已有完整可用结果后；边界：仅单区域全局补电，等面积仍继续比较真实线路，不减少不同几何的初步搜索。

import (
	"DysonSphereBox/helper/blueprint"
	"math"
)

// denseEnvelopeDominated 职责：以不会因补电减少的格网面积提供保守下界；使用：已存在近方形完整解时；边界：临时格网比例不能与最终物理比例比较，未达标胜者不能剪掉可能达标的更大候选。
func denseEnvelopeDominated(candidate, best LayoutMetrics) bool {
	return LayoutShapeAccepted(best) && candidate.EnvelopeArea > layoutShapeObjective(best)+1e-6
}

// rejectDominatedDenseEnvelope 职责：在构造固定Area前用相同宽高口径剪去严格劣解；使用：实际routes/附件/裁剪均完成后；边界：多区可能重排时不剪枝。
func (b *builder) rejectDominatedDenseEnvelope(req Request) error {
	if !b.denseSpatial || !b.connectedGlobalPower || b.connectedBestObjective == nil || req.SurfaceLayout == nil || req.SurfaceLayout.Mode == "planar" {
		return nil
	}
	// 明确选区会在最终阶段重定位球面几何，原装配坐标的包络不是最终Area的目标下界。
	if req.SurfaceLayout.LatitudeZone != "" {
		return nil
	}
	low, high := bounds(b.all)
	width, height := int(math.Ceil(high.x-low.x))+1, int(math.Ceil(high.y-low.y))+1
	if width <= 0 || height <= 0 || len(surfaceLayoutAnchorCandidates(req.SurfaceLayout, width, height)) == 0 {
		return nil
	}
	objective := LayoutMetrics{EnvelopeArea: float64(width) * float64(height), EnvelopeAspectRatio: math.Max(float64(width), float64(height)) / math.Min(float64(width), float64(height))}
	if denseEnvelopeDominated(objective, *b.connectedBestObjective) {
		return &blueprint.Error{Code: "GEN_I_DENSITY_DOMINATED", Message: "此完整布线候选的固定单区矩形目标已严格劣于已验证结果，跳过区内供电求解"}
	}
	return nil
}
