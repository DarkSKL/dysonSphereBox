package build

// 文件职责：把密铺源端前缀、整数主路和真实目标后缀放在同一成本下比较。
// 使用：最低可行高度层中的端口选择；边界：不移动既有尾链，不提交占位，所有临时屏蔽在返回或取消时恢复。

import (
	"context"
	"math"
)

// findDenseTransportPrefixPath 职责：按当前搜索权重与预算比较完整合法目标；使用：密铺前缀比较；边界：合流只返回至锚点的新链，代价包含完整旧尾链，每次搜索仍受固定展开上限约束，不宣称整厂全局最优。
func (env *transportEnvironment) findDenseTransportPrefixPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, prefix denseRoutePortal, portals []denseRoutePortal) (full []vector, tailIndex int, cost float64, length, expanded, maximum, searches int) {
	tailIndex, cost = -1, math.Inf(1)
	source, target := route.start, route.end
	start := prefix.points[len(prefix.points)-1]
	added := map[vector]bool{}
	for _, point := range prefix.points[1 : len(prefix.points)-1] {
		cells := map[vector]bool{}
		env.densePorts.blockNode(cells, point)
		for cell := range cells {
			if cell != start && !env.blocked[cell] {
				env.blocked[cell], added[cell] = true, true
			}
		}
	}
	defer func() {
		for point := range added {
			delete(env.blocked, point)
		}
	}()
	goals := []transportDestination{}
	suffixes := [][]vector{}
	indices := []int{}
	if len(tail) > 0 {
		// 从后向前一次累加真实有向旧尾链，避免把分数节点截断或按最近锚点零成本比较。
		costs := make([]float64, len(tail))
		for i := len(tail) - 2; i >= 0; i-- {
			costs[i] = costs[i+1] + env.transportPathCost([]vector{tail[i].a, tail[i+1].a})
		}
		for i, node := range tail {
			if !denseIntegerNode(node.a) || incoming[node] >= 3 {
				continue
			}
			goals = append(goals, transportDestination{point: node.a, cost: costs[i], length: len(tail) - i - 1, reused: node})
			suffixes, indices = append(suffixes, nil), append(indices, i)
		}
	} else {
		for _, portal := range portals {
			suffix := make([]vector, len(portal.points))
			for i, point := range portal.points {
				suffix[len(suffix)-1-i] = point
			}
			goals = append(goals, transportDestination{point: suffix[0], cost: env.transportPathCost(suffix), length: len(suffix) - 1})
			suffixes, indices = append(suffixes, suffix), append(indices, -1)
		}
	}
	// 整链几何可能排除某个接驳；保留原四次有限重试，不把失败候选当作可提交结果。
	for attempts := 0; attempts < 4 && len(goals) > 0; attempts++ {
		path, selected, steps, peak, calls := env.findDenseNativeDestinationPath(ctx, start, goals, source, target)
		expanded, maximum, searches = expanded+steps, max(maximum, peak), searches+calls
		if ctx.Err() != nil || len(path) == 0 {
			return
		}
		candidate := append(append([]vector{}, prefix.points[:len(prefix.points)-1]...), path...)
		endpoint := target
		if indices[selected] >= 0 {
			endpoint = tail[indices[selected]]
		} else {
			candidate = append(candidate, suffixes[selected][1:]...)
		}
		if env.densePathClear(candidate, source, endpoint) {
			full, tailIndex = candidate, indices[selected]
			cost, length = env.transportPathCost(candidate), len(candidate)
			if tailIndex >= 0 {
				cost, length = cost+goals[selected].cost, length+goals[selected].length
			}
			return
		}
		goals = append(goals[:selected], goals[selected+1:]...)
		suffixes = append(suffixes[:selected], suffixes[selected+1:]...)
		indices = append(indices[:selected], indices[selected+1:]...)
	}
	return
}
