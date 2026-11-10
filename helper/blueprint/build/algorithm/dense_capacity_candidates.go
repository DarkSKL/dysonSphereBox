package build

// 文件职责：给mode1密铺增加固定上限的容量优先分组候选。
// 使用：完整候选登记及策略去重前；边界：保留全部原均衡候选，不影响普通模式、配送或塔，不扩大设备能力或递归复制候选。

// appendDenseCapacityFillVariants 职责：最多复制四个空间排序和四个双段街道朝向作为容量优先候选；使用：mode1真实搜索前；边界：仅从既有均衡候选选取，最大跨度及同跨度微调优先，缺少双段时复用自动街道，不叠加双侧过滤组合。
func appendDenseCapacityFillVariants(req Request, variants []connectedLayoutVariant) []connectedLayoutVariant {
	if req.Mode != 1 || req.Density != densityDense {
		return variants
	}
	for _, variant := range variants {
		if variant.capacityFill {
			return variants
		}
	}
	original := append([]connectedLayoutVariant{}, variants...)
	for order := 0; order < 4; order++ {
		chosen := -1
		for i, variant := range original {
			if variant.capacityFill || variant.street || variant.pairedOutputs || variant.denseOrder != order {
				continue
			}
			if chosen < 0 || variant.denseSpan > original[chosen].denseSpan || variant.denseSpan == original[chosen].denseSpan && variant.refine && !original[chosen].refine {
				chosen = i
			}
		}
		if chosen >= 0 {
			candidate := original[chosen]
			candidate.capacityFill = true
			variants = append(variants, candidate)
		}
	}
	for turn := 0; turn < 4; turn++ {
		for _, limit := range []int{2, 0} {
			chosen := -1
			for i, variant := range original {
				if !variant.capacityFill && variant.street && !variant.pairedOutputs && variant.streetGroupLimit == limit && variant.streetRotation == turn {
					chosen = i
					break
				}
			}
			if chosen >= 0 {
				variant := original[chosen]
				variant.capacityFill = true
				variants = append(variants, variant)
				break
			}
		}
	}
	return variants
}
