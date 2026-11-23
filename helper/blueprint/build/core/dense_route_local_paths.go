package build

// 文件职责：把共用局部短链与原主路搜索组合，保留真实尾链、容量和计数。
// 使用：原primary/half/merge全部失败后；边界：不提交占位，不重跑纯旧组合，不改变root或单次A*预算。
import (
	"context"
	"math"
)

// denseLocalPortalIdentity 职责：为完整端口短链建立精确键；使用：只去掉真正重复的前缀/后缀；边界：出口相同但中间点不同仍是不同身份。
func denseLocalPortalIdentity(p denseRoutePortal) denseLocalPortalKey {
	key := denseLocalPortalKey{count: len(p.points)}
	if key.count > denseLocalPortalNodes {
		return key
	}
	copy(key.points[:], p.points)
	return key
}

// denseLocalNovelPortals 职责：排除已经穷尽的完全相同短链；使用：新旧候选组合；边界：不按出口合并，也不把超长旧链错误装入短键。
func denseLocalNovelPortals(local, old []denseRoutePortal) []denseRoutePortal {
	seen := map[denseLocalPortalKey]bool{}
	for _, p := range old {
		if len(p.points) <= denseLocalPortalNodes {
			seen[denseLocalPortalIdentity(p)] = true
		}
	}
	out := []denseRoutePortal{}
	for _, p := range local {
		key := denseLocalPortalIdentity(p)
		if !seen[key] {
			out = append(out, p)
			seen[key] = true
		}
	}
	return out
}

// findDenseLocalPrefixPaths 职责：在有限新前缀中保持完整路径身份并调用原完整搜索。
// 使用：两端共用局部备用的组合；边界：调用预算耗尽只返回未知失败，取消不返回部分获选路线，既有A*、goal预检和整链守卫完全不变。
func (env *transportEnvironment) findDenseLocalPrefixPaths(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, prefixes, targets []denseRoutePortal, budget *int) (best []vector, bestIndex, expanded, maximum, searches int) {
	bestIndex = -1
	bestCost, bestLength := math.Inf(1), math.MaxInt
	seen := map[denseLocalPortalKey]bool{}
	for _, prefix := range prefixes {
		if ctx.Err() != nil {
			return nil, -1, expanded, maximum, searches
		}
		if *budget <= 0 {
			break
		}
		if len(prefix.points) <= denseLocalPortalNodes {
			key := denseLocalPortalIdentity(prefix)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		*budget = *budget - 1
		full, index, cost, length, steps, peak, calls := env.findDenseTransportPrefixPath(ctx, route, tail, incoming, prefix, targets, bestCost)
		expanded += steps
		maximum = max(maximum, peak)
		searches += calls
		if ctx.Err() != nil {
			return nil, -1, expanded, maximum, searches
		}
		if len(full) > 0 && (cost < bestCost || cost == bestCost && length < bestLength) {
			best, bestIndex, bestCost, bestLength = full, index, cost, length
			return
		}
	}
	if ctx.Err() != nil {
		best = nil
		bestIndex = -1
	}
	return
}

// findDenseLocalTransportPath 职责：原primary/half/merge失败后组合新局部前缀与后缀。
// 使用：主路求解最终备用；边界：不重跑纯旧组合，尾链只复用真实未饱和节点及其下游成本/索引，不提交占位。
func (env *transportEnvironment) findDenseLocalTransportPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	tailIndex = -1
	if ctx.Err() != nil || env.densePorts == nil {
		return
	}
	source, target := route.transportSource(), route.end
	fixedTailEnd := len(tail) > 0 && route.to == nil && route.edge.FlowKind == "leftover" && route.boundary == target
	if fixedTailEnd {
		// 既有尾链的裸交付头不能变成新增合流锚点；上游真实节点仍保留原输入额度，原入度表只读。
		copyIncoming := make(map[*placed]int, len(incoming)+1)
		for p, n := range incoming {
			copyIncoming[p] = n
		}
		copyIncoming[target] = 3
		incoming = copyIncoming
	}
	oldSources := env.densePortals(source, source, target, preferred, true, false)
	oldSources = append(oldSources, env.denseSidePortals(source, source, target, preferred, true, false)...)
	localSources, _ := env.denseLocalPortals(ctx, source, source, target, preferred, true, false)
	localSources = denseLocalNovelPortals(localSources, oldSources)
	budget := denseLocalPortalTrials
	try := func(r *transportRoute, t []*placed, ps, ts []denseRoutePortal) ([]vector, int) {
		full, index, steps, peak, calls := env.findDenseLocalPrefixPaths(ctx, r, t, incoming, ps, ts, &budget)
		expanded += steps
		maximum = max(maximum, peak)
		searches += calls
		return full, index
	}
	if len(tail) > 0 {
		share := budget
		budget = min(8, budget)
		full, index := try(route, tail, localSources, nil)
		used := min(8, share) - budget
		budget = share - used
		if len(full) > 0 {
			return full[1:], index, expanded, maximum, searches
		}
	}
	anchors := []int{-1}
	if len(tail) > 0 {
		anchors = nil
		for i, p := range tail {
			if p != nil && p != source && incoming[p] < 3 {
				anchors = append(anchors, i)
				if len(anchors) == 4 {
					break
				}
			}
		}
	}
	bestCost, bestLength := math.Inf(1), math.MaxInt
	var chosen []vector
	chosenIndex := -1
	accept := func(full []vector, index int, cost float64, length int) {
		if len(full) > 0 && (cost < bestCost || cost == bestCost && length < bestLength) {
			chosen, chosenIndex, bestCost, bestLength = full, index, cost, length
		}
	}
	for _, anchor := range anchors {
		if ctx.Err() != nil || budget <= 0 {
			break
		}
		probe := *route
		suffixCost, suffixLength := 0., 0
		if anchor >= 0 {
			probe.end = tail[anchor]
			suffixLength = len(tail) - anchor - 1
			for i := anchor; i+1 < len(tail); i++ {
				suffixCost += env.transportPathCost([]vector{tail[i].a, tail[i+1].a})
			}
		}
		fallback := vector{x: -1}
		if probe.to != nil {
			fallback = denseRotateVector(fallback, probe.to.quarterTurns)
		}
		fixed := probe.to == nil && probe.edge.FlowKind == "leftover" && probe.boundary == probe.end
		oldTargets := env.densePortals(probe.end, source, probe.end, fallback, false, fixed)
		oldTargets = append(oldTargets, env.denseSidePortals(probe.end, source, probe.end, fallback, false, fixed)...)
		localTargets, _ := env.denseLocalPortals(ctx, probe.end, source, probe.end, fallback, false, fixed)
		localTargets = denseLocalNovelPortals(localTargets, oldTargets)
		if len(localTargets) > 0 {
			if full, _ := try(&probe, nil, oldSources, localTargets); len(full) > 0 {
				if len(tail) == 0 {
					return full[1:], anchor, expanded, maximum, searches
				}
				accept(full, anchor, env.transportPathCost(full)+suffixCost, len(full)+suffixLength)
				continue
			}
		}
		allTargets := append(append([]denseRoutePortal(nil), oldTargets...), localTargets...)
		if len(localSources) > 0 && len(allTargets) > 0 {
			if full, _ := try(&probe, nil, localSources, allTargets); len(full) > 0 {
				if len(tail) == 0 {
					return full[1:], anchor, expanded, maximum, searches
				}
				accept(full, anchor, env.transportPathCost(full)+suffixCost, len(full)+suffixLength)
			}
		}
	}
	if ctx.Err() == nil && len(chosen) > 0 {
		points, tailIndex = chosen[1:], chosenIndex
	}
	return
}
