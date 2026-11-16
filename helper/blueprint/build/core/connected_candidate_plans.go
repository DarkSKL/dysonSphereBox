package build

// 文件职责：在直连候选装配前统一规范化执行参数及街道容量分组并稳定去重。
// 使用：完整候选生成后、预览和派发之前。
// 边界：真实拓扑、未知容量键及入口策略分别保留，不缓存失败。

import (
	"DysonSphereBox/helper/blueprint/build/candidates"
	"context"
)

// normalizeConnectedCandidate 职责：删除当前装配分支不会读取的参数；使用：全部派生完成后统一调用。
// 边界：不按面积猜测高度、排序、微调、容量或分流等价；密铺Compact在生成移近候选之前不能清除。
func normalizeConnectedCandidate(req Request, spec ConnectedCandidateSpec) ConnectedCandidateSpec {
	if req.Density != densityDense {
		spatial, span, order := spec.SpatialRows, spec.DenseSpan, spec.DenseOrder
		spec = ConnectedCandidateSpec{ObjectiveBound: spec.ObjectiveBound, Limit: spec.Limit, Compact: spec.Compact, Horizontal: spec.Horizontal, GlobalPower: spec.GlobalPower, LegacyInputs: spec.LegacyInputs}
		if spatial {
			spec.SpatialRows, spec.DenseSpan, spec.DenseOrder = true, span, order
			spec.Compact, spec.Horizontal = false, false
		}
		if req.SurfaceLayout == nil || req.SurfaceLayout.Mode != "autoTropic" || !spec.Compact {
			spec.Horizontal = false
		}
		return spec
	}
	// prepareConnectedRows 对每个密铺行直接收集denseUnits，绕过Compact/Horizontal排版。
	spec.Compact, spec.Horizontal = false, false
	spec.GlobalPower = req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar"
	if !spec.Street {
		spec.StreetRotation, spec.StreetGroupLimit = 0, 0
	}
	return spec
}

// deduplicateConnectedCandidates 职责：统一完成参数规范化和实际街道分组稳定去重。
// 使用：预览、分组及派发之前，调用方交出planned暂存区所有权；边界：未知键保留，已知空键删除无适配产线的街道；返回值独立，双侧、移近、朝向和容量策略均属于完整身份。
func deduplicateConnectedCandidates(ctx context.Context, req Request, planned []ConnectedCandidateSpec, streetKeys, capacityKeys map[int]string) []ConnectedCandidateSpec {
	normalized := planned[:0]
	inactive := 0
	identity := func(spec ConnectedCandidateSpec) ConnectedCandidateSpec {
		if spec.Street && spec.DenseSpan == 0 {
			keys := streetKeys
			if spec.CapacityFill {
				keys = capacityKeys
			}
			if plan, known := keys[spec.StreetGroupLimit]; known {
				// 用同一真实键的首个合法分组作为身份，不另复制一份遗漏新字段的结构。
				// 未知分组没有参与此映射，不会与已知组的代表值相撞。
				for _, group := range []int{0, 2, -1} {
					if other, exists := keys[group]; exists && other == plan {
						spec.StreetGroupLimit = group
						break
					}
				}
			}
		}
		return spec
	}
	for _, spec := range planned {
		normal := normalizeConnectedCandidate(req, spec)
		if normal != spec {
			inactive++
		}
		// 探针只证明未附加跨度拆行的街道，不能套用到自行提供的其它跨度。
		if normal.Street && normal.DenseSpan == 0 {
			keys := streetKeys
			if normal.CapacityFill {
				keys = capacityKeys
			}
			if plan, known := keys[normal.StreetGroupLimit]; known && plan == "" {
				continue
			}
		}
		normalized = append(normalized, normal)
	}
	unique := candidates.StableUnique(normalized, identity)
	recordCandidatePlans(ctx, "connected", len(planned), len(unique), inactive)
	return unique
}
