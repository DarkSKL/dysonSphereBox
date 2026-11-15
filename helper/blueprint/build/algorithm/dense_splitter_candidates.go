package build

// 文件职责：在已有容量优先候选上比较三种真实分流器形态及独立产物分流树。
// 使用：超密度直连的候选登记；边界：八个容量布局与四个均衡布局各比较三种模型，不移除原过滤方案或改变设备等级。

// appendDenseSplitterVariants 职责：保留已验证分组参数并分别重建三种真实模型；使用：共同几何与树构造完成后的公开候选入口；边界：幂等且不递归复制新候选，原均衡与容量优先候选继续独立参与终检。
func appendDenseSplitterVariants(req Request, variants []connectedLayoutVariant) []connectedLayoutVariant {
	if req.Mode != 1 || req.Density != densityDense {
		return variants
	}
	for _, variant := range variants {
		if variant.producerSplits {
			return variants
		}
	}
	original := append([]connectedLayoutVariant{}, variants...)
	copied := 0
	balanced := map[int]bool{}
	appendModels := func(variant connectedLayoutVariant) {
		for _, model := range []int16{38, 39, 40} {
			candidate := variant
			candidate.producerSplits, candidate.splitterModel = true, model
			variants = append(variants, candidate)
		}
	}
	for _, variant := range original {
		if !variant.capacityFill || variant.pairedOutputs || variant.splitterModel != 0 {
			continue
		}
		if copied >= 8 {
			break
		}
		copied++
		appendModels(variant)
		if !variant.street && variant.denseOrder >= 0 && variant.denseOrder < 4 && !balanced[variant.denseOrder] {
			// 消除长过滤头后，均衡整数布局可能优于容量微调；两种分组必须独立比较。
			balanced[variant.denseOrder] = true
			variant.capacityFill, variant.refine = false, false
			appendModels(variant)
		}
	}
	return variants
}
