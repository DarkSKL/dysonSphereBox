package build

// 文件职责：冻结半格备用接驳前的完整密铺寻路循环。
// 使用：原算法可通样本的逐路径、计数和顺序差分；边界：不调用新增备用入口，不随生产实现改写。

import (
	"context"
	"math"
)

// sidePortalLegacyPath 职责：按完整端口路线代价比较最低可行源高度层；使用：密铺直连及配送；边界：返回不含源且包含复用目标的路径，合流索引保持原尾链身份，取消丢弃未提交结果。
func (env *transportEnvironment) sidePortalLegacyPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	tailIndex = -1
	source, target := route.transportSource(), route.end
	var targetPortals []denseRoutePortal
	if len(tail) == 0 {
		fallback := vector{x: -1}
		if route.to != nil {
			fallback = denseRotateVector(fallback, route.to.quarterTurns)
		}
		// 裸带回收边界保持既有东向入口；塔/箱黑盒由真实内侧邻居确定方向。
		targetPortals = env.densePortals(target, source, target, fallback, false, route.to == nil && route.edge.FlowKind == "leftover" && route.boundary == route.end)
	}
	tried := map[vector]bool{}
	initialHeight := math.Min(transportMaxHeight, math.Max(0, math.Ceil(source.a.z)))
	for height := initialHeight; height <= transportMaxHeight; height++ {
		var best []vector
		bestIndex, bestLength, bestCost := -1, math.MaxInt, math.Inf(1)
		for _, prefix := range env.densePortals(source, source, target, preferred, true, false, height) {
			if ctx.Err() != nil {
				return nil, -1, expanded, maximum, searches
			}
			start := prefix.points[len(prefix.points)-1]
			if tried[start] {
				continue
			}
			tried[start] = true
			if env.transportPathCost(prefix.points) > bestCost {
				continue
			}
			full, selected, cost, length, steps, peak, calls := env.findDenseTransportPrefixPath(ctx, route, tail, incoming, prefix, targetPortals)
			expanded, maximum, searches = expanded+steps, max(maximum, peak), searches+calls
			if len(full) > 0 && (cost < bestCost || cost == bestCost && length < bestLength) {
				best, bestIndex, bestCost, bestLength = full, selected, cost, length
			}
		}
		if ctx.Err() != nil {
			return nil, -1, expanded, maximum, searches
		}
		if len(best) > 0 {
			return best[1:], bestIndex, expanded, maximum, searches
		}
	}
	return
}
