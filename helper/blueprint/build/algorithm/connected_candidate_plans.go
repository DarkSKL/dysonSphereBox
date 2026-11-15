package build

// 文件职责：在直连候选装配前移除不参与执行的参数并稳定去重。
// 使用：普通直连候选派发；边界：密铺参数、有效排版差异和两种原料入口分别保留，不缓存失败。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/candidates"
)

// deduplicateConnectedVariants 职责：按当前密度实际读取的完整参数合并候选；使用：生成预览及派发之前；边界：密铺只合并完全相同的结构值，不凭面积、机器数或失败记录裁剪。
func deduplicateConnectedVariants(ctx context.Context, req Request, variants []connectedLayoutVariant) []connectedLayoutVariant {
	normalized := make([]connectedLayoutVariant, 0, len(variants))
	inactive := 0
	for _, variant := range variants {
		if req.Density != densityDense {
			// 普通派发只读取嵌入的四个布局字段，所有密铺字段都在 densityDense 分支内。
			variant = connectedLayoutVariant{LayoutVariant: variant.LayoutVariant}
			// prepareConnectedRows 仅在自动纬带且启用轮廓压缩时读取 Horizontal。
			if variant.Horizontal && (req.SurfaceLayout == nil || req.SurfaceLayout.Mode != "autoTropic" || !variant.Compact) {
				variant.Horizontal = false
				inactive++
			}
		}
		normalized = append(normalized, variant)
	}
	unique := candidates.StableUnique(normalized, func(variant connectedLayoutVariant) connectedLayoutVariant { return variant })
	recordCandidatePlans(ctx, "connected", len(variants), len(unique), inactive)
	return unique
}
