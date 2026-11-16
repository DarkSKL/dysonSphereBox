package build

// 文件职责：为原接驳算法已失败的端口补充先侧移半格、再接整数主路的有限备用。
// 使用：密铺寻路原循环结束后；边界：不替换、重排或裁减原portal，不放宽任何物理阈值及单次搜索预算。

import (
	"context"
	"math"
	"sort"
)

// denseSidePortals 职责：从真实端点两侧各外伸半格，检查相邻整数接驳。
// 使用：原算法没有完整路线时；边界：最多三个相邻高度、每高度两个不同出口，固定地面直出头不使用此备用；所有新增段仍核对原权限、静态盒、独立带、自距及连续预留。
func (env *transportEnvironment) denseSidePortals(endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool) []denseRoutePortal {
	if env.densePorts == nil || endpoint == nil || fixedGroundStem {
		return nil
	}
	index := env.densePorts
	outward := index.outward(endpoint, preferred, sourceSide)
	linked := index.linkedPortBuildings(endpoint, sourceSide)
	clear := func(path []vector, point vector) bool {
		if point.x < env.minimum.x || point.x > env.maximum.x || point.y < env.minimum.y || point.y > env.maximum.y || point.z < 0 || point.z > transportMaxHeight || !index.nodeClear(point, nil) || env.reservedByOther(point, source, target) || !env.reservedSpanClear(path[len(path)-1], point, source, target) || !index.spanClearWithLinks(path[len(path)-1], point, densePortalLinkPermissions(linked, len(path)-1, len(path))) {
			return false
		}
		for _, previous := range path {
			delta := index.world(previous).Sub(index.world(point))
			if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
				return false
			}
		}
		return true
	}
	byExit := map[vector]denseRoutePortal{}
	for _, sign := range []float64{-1, 1} {
		stem := vector{endpoint.a.x - sign*outward.y*.5, endpoint.a.y + sign*outward.x*.5, endpoint.a.z}
		base := []vector{endpoint.a}
		if !clear(base, stem) {
			continue
		}
		base = append(base, stem)
		for z := math.Floor(stem.z); z <= min(float64(transportMaxHeight), math.Ceil(stem.z)+1); z++ {
			for dx := -1.; dx <= 1; dx++ {
				for dy := -1.; dy <= 1; dy++ {
					point := vector{math.Round(stem.x) + dx, math.Round(stem.y) + dy, z}
					if !clear(base, point) {
						continue
					}
					candidate := denseRoutePortal{points: []vector{endpoint.a, stem, point}, height: max(endpoint.a.z, z)}
					candidate.cost = denseRouteLength(candidate.points)
					if old, ok := byExit[point]; !ok || candidate.cost < old.cost-1e-9 {
						byExit[point] = candidate
					}
				}
			}
		}
	}
	portals := make([]denseRoutePortal, 0, len(byExit))
	for _, p := range byExit {
		portals = append(portals, p)
	}
	sort.Slice(portals, func(i, j int) bool {
		a, b := portals[i], portals[j]
		if a.height != b.height {
			return a.height < b.height
		}
		if math.Abs(a.cost-b.cost) > 1e-9 {
			return a.cost < b.cost
		}
		x, y := a.points[2], b.points[2]
		if x.x != y.x {
			return x.x < y.x
		}
		if x.y != y.y {
			return x.y < y.y
		}
		return x.z < y.z
	})
	out := []denseRoutePortal{}
	levels := map[int]int{}
	for _, p := range portals {
		level := int(math.Ceil(p.height))
		if levels[level] < 2 {
			out = append(out, p)
			levels[level]++
		}
	}
	return out
}

// findDenseTransportPath 职责：保留原完整路线的成功结果，只在确实失败后尝试半格侧移。
// 使用：全部原调用方；边界：原成功路径、合流锚点及搜索计数逐项保持，备用耗时和展开如实累加，取消不进入备用。
func (env *transportEnvironment) findDenseTransportPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	points, tailIndex, expanded, maximum, searches = env.findDenseTransportPathPrimary(ctx, route, tail, incoming, preferred)
	if len(points) > 0 || ctx.Err() != nil {
		return
	}
	var extra, peak, calls int
	points, tailIndex, extra, peak, calls = env.findDenseTransportPathSide(ctx, route, tail, incoming, preferred)
	expanded, maximum, searches = expanded+extra, max(maximum, peak), searches+calls
	if len(points) == 0 && ctx.Err() == nil && len(tail) > 0 {
		points, tailIndex, extra, peak, calls = env.findDenseTransportSideMerge(ctx, route, tail, incoming, preferred)
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
