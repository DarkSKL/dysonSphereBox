package build

// 文件职责：使原生矩形窗口高度独立于基础候选的排序/跨度配对。
// 使用：候选枚举的显式surfaceHeightSeeds策略；边界：只追加有限基础种子，原布局仍保留且全部经过共同语义去重和物理规则。

import "context"

// connectedSurfaceHeightSeeds 职责：给每个已有基础策略增加同一真实窗口高度的起始排布。
// 使用：完整派生之后、规范化和实际分行去重之前；边界：不派生街道、分流、微调或折行，不改输入、选区和执行预算。
func connectedSurfaceHeightSeeds(ctx context.Context, input *PreparedInput, planned []ConnectedCandidateSpec) []ConnectedCandidateSpec {
	req, surface := input.request, input.geometry
	if req.Mode != 1 || req.Density != densityDense || req.Plan.MachineCount < denseNativeLargeMachineThreshold || surface.Strategy != "native-window" || surface.LocalRectangleAspect <= 0 || surface.ExtentRows < 6 {
		return planned
	}
	height := denseLatitudeHeightLimit(ctx, float64(surface.ExtentRows))
	seen := make(map[ConnectedCandidateSpec]bool)
	seeds := make([]ConnectedCandidateSpec, 0, len(planned))
	for _, spec := range planned {
		if spec.Street || spec.Refine || spec.CapacityFill || spec.ProducerSplits || spec.BentRows || spec.TranslateCompact || spec.PairedOutputs {
			continue
		}
		spec.Limit = height
		spec = normalizeConnectedCandidate(req, spec)
		if !seen[spec] {
			seen[spec] = true
			seeds = append(seeds, spec)
		}
	}
	// 同工作量优先真实窗口种子；旧候选仍按原相对次序进入同一个去重/排序入口。
	return append(seeds, planned...)
}
