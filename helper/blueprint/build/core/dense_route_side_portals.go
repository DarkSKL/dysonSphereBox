package build

// 文件职责：为原接驳算法已失败的端口保留原半格备用并在全部失败后接入共用局部搜索。
// 使用：密铺寻路原循环结束后；边界：不替换、重排或裁减原portal，不放宽任何物理阈值及单次搜索预算。

import (
	"context"
	"math"
)

// denseSidePortals 职责：从真实端点两侧各外伸半格，检查相邻整数接驳。
// 使用：原算法没有完整路线时；边界：最多三个相邻高度、每高度两个不同出口，固定地面直出头不使用此备用；所有新增段仍核对原权限、静态盒、独立带、自距及连续预留。
func (env *transportEnvironment) denseSidePortals(endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool) []denseRoutePortal {
	return env.denseLateralPortals(endpoint, source, target, preferred, sourceSide, fixedGroundStem, []float64{.5}, 2)
}

// findDenseTransportPath 为边界供料和交付先找低层完整路线，再使用全高度和侧向备用接驳。
// 其它路线直接使用完整高度。每次试算都计入搜索开销，返回前恢复候选原有的高度范围。
func (env *transportEnvironment) findDenseTransportPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	fullHeight := env.maximum.z
	limits := []float64{fullHeight}
	if env.densePorts.boundary != nil && (route.from == nil || route.to == nil) && fullHeight > 4 {
		limits = []float64{2, 4, fullHeight}
	}
	required := max(route.transportSource().a.z, route.end.a.z)
	var extra, peak, calls int
	for _, limit := range limits {
		if limit < required {
			continue
		}
		env.maximum.z = limit
		points, tailIndex, extra, peak, calls = env.findDenseTransportPathPrimary(ctx, route, tail, incoming, preferred)
		env.maximum.z = fullHeight
		expanded, maximum, searches = expanded+extra, max(maximum, peak), searches+calls
		if len(points) > 0 || ctx.Err() != nil {
			return
		}
	}
	points, tailIndex, extra, peak, calls = env.findDenseTransportPathSide(ctx, route, tail, incoming, preferred)
	expanded, maximum, searches = expanded+extra, max(maximum, peak), searches+calls
	if len(points) == 0 && ctx.Err() == nil && len(tail) > 0 {
		points, tailIndex, extra, peak, calls = env.findDenseTransportSideMerge(ctx, route, tail, incoming, preferred)
		expanded, maximum, searches = expanded+extra, max(maximum, peak), searches+calls
	}
	if len(points) == 0 && ctx.Err() == nil {
		points, tailIndex, extra, peak, calls = env.findDenseLocalTransportPath(ctx, route, tail, incoming, preferred)
		expanded, maximum, searches = expanded+extra, max(maximum, peak), searches+calls
	}
	return
}

// findDenseTransportPathSide 职责：在旧候选原顺序之后尝试有限侧移，并复用原完整路线终检。
// 使用：原全高度循环未找到任何路线之后；边界：已有目标或源候选不重排，不在没有新增候选时重复搜索，相同出口的不同侧移前缀保留各自身份。
func (env *transportEnvironment) findDenseTransportPathSide(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	tailIndex = -1
	source, target := route.transportSource(), route.end
	sourceSide := env.denseSidePortals(source, source, target, preferred, true, false)
	var targets, extraTargets []denseRoutePortal
	if len(tail) == 0 {
		fallback := vector{x: -1}
		if route.to != nil {
			fallback = denseRotateVector(fallback, route.to.quarterTurns)
		}
		fixed := route.to == nil && route.edge.FlowKind == "leftover" && route.boundary == route.end
		extraTargets = env.denseSidePortals(target, source, target, fallback, false, fixed)
		if len(extraTargets) > 0 || len(sourceSide) > 0 {
			targets = env.densePortals(target, source, target, fallback, false, fixed)
			targets = append(targets, extraTargets...)
		}
	}
	if len(sourceSide) == 0 && len(extraTargets) == 0 {
		return
	}
	tried := map[vector]bool{}
	triedSide := map[int]bool{}
	initial := min(float64(transportMaxHeight), max(0, math.Ceil(source.a.z)))
	for height := initial; height <= transportMaxHeight; height++ {
		prefixes := []denseRoutePortal{}
		if len(extraTargets) > 0 {
			prefixes = env.densePortals(source, source, target, preferred, true, false, height)
		}
		originalCount := len(prefixes)
		sideIndices := []int{}
		for i, p := range sourceSide {
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
			full, selected, cost, length, steps, peak, calls := env.findDenseTransportPrefixPath(ctx, route, tail, incoming, prefix, targets)
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
