package build

// 文件职责：用真实局部建筑范围推导偏矩形候选，并记录可复现的形状目标。
// 使用：直连带和配送器在全局寻路前选择有限的列高；最终摘要按编码后的 Area 计分。
// 边界：试装不运行最终编码、不改变机器或流量；候选仍需通过原有完整规则，不承诺数学全局最优。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// layoutHeightCandidates 职责：从本次真实机器、端口和配送设施占地估计矩形候选；使用：有限布局搜索；边界：平面保留旧列高，球面直连保留低纬宽排候选，取消和试装错误不得被忽略。
func (b *builder) layoutHeightCandidates(ctx context.Context, req Request, dataset *quant.Dataset, connected bool) ([]float64, error) {
	probe, err := newBuilder(b.setting)
	if err != nil {
		return nil, err
	}
	probe.generationCtx, probe.flowGraph = ctx, b.flowGraph
	probe.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	if connected {
		probe.connectedCompactBelts = true
		err = probe.prepareConnectedRows(ctx, req, dataset, math.Inf(1), false)
	} else {
		err = probe.assembleFlyLayout(ctx, req, dataset, flyLayoutVariant{heightLimit: math.Inf(1)})
	}
	if err != nil {
		return nil, err
	}
	// 同一产线的多行在试装中上下相邻；按产线而不是整个长列求面积，
	// 避免最长机器行把所有较短产线的空白都计成必要占地。
	blocks := make([][]*placed, len(req.Plan.Productions))
	for _, p := range probe.all {
		if p.owner >= 0 && p.owner < len(blocks) {
			blocks[p.owner] = append(blocks[p.owner], p)
		}
	}
	area := 0.
	for _, block := range blocks {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "布局形状估计已取消或超时"}
		}
		if len(block) == 0 {
			continue
		}
		low, high := bounds(block)
		area += (math.Ceil(high.x-low.x) + 2) * (math.Ceil(high.y-low.y) + 2)
	}
	fallback := b.setting.BlockWidthLimit
	if connected && req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		// 水平行架还须容纳路由扩展，不能用整个赤道纬带高度估算：
		// 行架取其四分之三，为锚点选择及高架曲率留出上下余量。
		for _, band := range surfaceLatitudeBands(packingSegment(req.SurfaceLayout)) {
			if band.low <= 0 && band.high >= 0 {
				usableHeight := math.Max(1, math.Floor(float64(band.high-band.low+1)*.75))
				fallback = math.Max(math.Sqrt(area)*1.6, math.Ceil(area*1.25/usableHeight))
				break
			}
		}
	}
	return rectangularHeightCandidates(area, fallback), nil
}

// rectangularHeightCandidates 职责：提供固定最多四种列高；使用：真实面积估计之后；边界：只为软形状目标，不将旧平面列高作为安全规则或固定最小高度。
func rectangularHeightCandidates(area, legacy float64) []float64 {
	ideal := math.Max(10, math.Ceil(math.Sqrt(math.Max(1, area))))
	values := []float64{ideal, math.Ceil(ideal * .8), math.Ceil(ideal * 1.25), math.Max(60, legacy)}
	result := make([]float64, 0, len(values))
	for _, value := range values {
		duplicate := false
		for _, previous := range result {
			duplicate = duplicate || previous == value
		}
		if !duplicate {
			result = append(result, value)
		}
	}
	return result
}

// updateLayoutShapeMetrics 职责：从最终 Area 包络计算形状诊断；使用：物流摘要收尾；边界：多区面积加权，不使用绘图坐标或缩小游戏碰撞外形。
func updateLayoutShapeMetrics(summary *LogisticsSummary, areas []blueprint.Area) {
	weightedRatio, total := 0., 0.
	for _, area := range areas {
		width, height := float64(area.Width), float64(area.Height)
		if width <= 0 || height <= 0 {
			continue
		}
		size := width * height
		weightedRatio += size * math.Max(width, height) / math.Min(width, height)
		total += size
	}
	summary.Metrics.EnvelopeAspectRatio = 1
	if total > 0 {
		summary.Metrics.EnvelopeAspectRatio = weightedRatio / total
	}
	summary.Metrics.ShapeObjective = layoutShapeObjective(summary.Metrics)
}

// layoutShapeObjective 职责：让超过二比一的狭长候选承担明确面积代价；使用：完整可行候选比较；边界：缺少形状指标的历史内部调用仍按面积，不因零值变为最优。
func layoutShapeObjective(metrics LayoutMetrics) float64 {
	return metrics.EnvelopeArea * math.Max(1, metrics.EnvelopeAspectRatio/2)
}
