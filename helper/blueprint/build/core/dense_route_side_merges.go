package build

// 文件职责：为原直接整数合流已失败的真实尾链补充有限半格接驳。
// 使用：原循环及普通半格备用均失败之后；边界：只复用真实尾链锚点，输入容量、原始候选次序和全链物理规则不变。

import (
	"context"
	"math"
)

// denseSideMergeAnchor 职责：绑定真实尾链位置、合法备用入口及不可省略的下游成本。
// 使用：当前路线局部比较；边界：不创建实体、不复制尾链，不把分数坐标四舍五入成锚点身份。
type denseSideMergeAnchor struct {
	index   int
	portals []denseRoutePortal
	cost    float64
	length  int
}

// findDenseTransportSideMerge 职责：在最多四个有剩余输入能力的锚点处侧接，并保持完整尾链成本。
// 使用：尾链唯一整数锚点无直接入边、其它真实节点仍有合法半格入口时；边界：不绕过三输入上限，取消丢弃试探，只返回原tail索引及实际新增路径。
func (env *transportEnvironment) findDenseTransportSideMerge(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	tailIndex = -1
	if len(tail) == 0 || ctx.Err() != nil {
		return
	}
	source := route.transportSource()
	costs := make([]float64, len(tail))
	for i := len(tail) - 2; i >= 0; i-- {
		costs[i] = costs[i+1] + env.transportPathCost([]vector{tail[i].a, tail[i+1].a})
	}
	anchors := []denseSideMergeAnchor{}
	fallback := vector{x: -1}
	if route.to != nil {
		fallback = denseRotateVector(fallback, route.to.quarterTurns)
	}
	for i, node := range tail {
		if node == nil || node == source || incoming[node] >= 3 {
			continue
		}
		fixed := node == route.end && route.to == nil && route.edge.FlowKind == "leftover" && route.boundary == route.end
		side := env.denseSidePortals(node, source, node, fallback, false, fixed)
		if len(side) == 0 {
			continue
		}
		portals := env.densePortals(node, source, node, fallback, false, fixed)
		portals = append(portals, side...)
		anchors = append(anchors, denseSideMergeAnchor{index: i, portals: portals, cost: costs[i], length: len(tail) - i - 1})
		if len(anchors) == 4 {
			break
		}
	}
	if len(anchors) == 0 {
		return
	}
	sides := env.denseSidePortals(source, source, route.end, preferred, true, false)
	tried := map[vector]bool{}
	triedSide := map[int]bool{}
	initial := min(float64(transportMaxHeight), max(0, math.Ceil(source.a.z)))
	for height := initial; height <= transportMaxHeight; height++ {
		prefixes := env.densePortals(source, source, route.end, preferred, true, false, height)
		originalCount := len(prefixes)
		sideIndices := []int{}
		for i, p := range sides {
			if p.height <= height && !triedSide[i] {
				prefixes = append(prefixes, p)
				sideIndices = append(sideIndices, i)
			}
		}
		var best []vector
		bestIndex, bestLength, bestCost := -1, math.MaxInt, math.Inf(1)
		for i, prefix := range prefixes {
			if ctx.Err() != nil {
				return nil, -1, expanded, maximum, searches
			}
			start := prefix.points[len(prefix.points)-1]
			if i < originalCount {
				if tried[start] {
					continue
				}
				tried[start] = true
			} else {
				triedSide[sideIndices[i-originalCount]] = true
			}
			if env.transportPathCost(prefix.points) > bestCost {
				continue
			}
			for _, anchor := range anchors {
				// 只替换本次试探的物理终点；原canonical路线及实际尾链完全不写入。
				probe := *route
				probe.end = tail[anchor.index]
				full, _, cost, length, steps, peak, calls := env.findDenseTransportPrefixPath(ctx, &probe, nil, incoming, prefix, anchor.portals, bestCost-anchor.cost)
				expanded, maximum, searches = expanded+steps, max(maximum, peak), searches+calls
				cost, length = cost+anchor.cost, length+anchor.length
				if len(full) > 0 && (cost < bestCost || cost == bestCost && length < bestLength) {
					best, bestIndex, bestCost, bestLength = full, anchor.index, cost, length
				}
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
