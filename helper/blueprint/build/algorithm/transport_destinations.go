package build

// 文件职责：在一次有界搜索中比较消费端所有合法邻格，避免重复展开同一片三维空间。
// 使用：走廊复用策略的未合流入口；边界：优化至接驳邻格的代价与节点数，不含连接最终端口的末步，不宣称完整路线全局最优；端口权限和固定展开预算不变。

import (
	"container/heap"
	"context"
	"math"
)

// transportDestinationQueue 职责：按至候选邻格的代价和节点数下界字典序展开；使用：多终点A*；边界：两个下界都一致，同坐标只保留代价更低或同价更短的前缀。
type transportDestinationQueue struct{ transportPathQueue }

// Less 职责：先优化平面走廊代价、再优化实际带链长度；使用：稳定堆比较；边界：同价同长度下界仍向终点推进，最终按插入顺序确定。
func (q transportDestinationQueue) Less(i, j int) bool {
	a, b := q.transportPathQueue[i], q.transportPathQueue[j]
	if a.score != b.score {
		return a.score < b.score
	}
	if a.lengthEstimate != b.lengthEstimate {
		return a.lengthEstimate < b.lengthEstimate
	}
	if a.distance != b.distance {
		return a.distance > b.distance
	}
	return a.order < b.order
}

// findTransportDestinationPath 职责：一次搜索选出消费端最优合法接驳邻格；使用：独立物料带首次接入；边界：只接受实际空闲邻格，取消不返回部分路线，不复用单终点缓存。
// 逻辑流：1. 筛选合法目标 => 2. 共用平面势场与两个一致下界 => 3. 按代价/长度展开 => 4. 首个完整目标回溯真实逐格路径。
func (env *transportEnvironment) findTransportDestinationPath(ctx context.Context, start vector, candidates []vector, source, target *placed) ([]vector, vector, int) {
	env.searchCalls++
	steps := 0
	defer func() {
		env.totalExpanded += steps
		env.maxExpanded = max(env.maxExpanded, steps)
		recordPerformanceSearch(ctx, steps, false)
	}()
	allowed := func(v vector) bool {
		return v.x >= env.minimum.x && v.x <= env.maximum.x && v.y >= env.minimum.y && v.y <= env.maximum.y && v.z >= 0 && v.z <= transportMaxHeight && (!env.blocked[beltCell(v)] || v == start) && !env.reservedByOther(v, source, target)
	}
	if ctx.Err() != nil || !allowed(start) {
		return nil, vector{}, 0
	}
	goals := make([]vector, 0, len(candidates))
	goalSet := make(map[vector]bool, len(candidates))
	for _, candidate := range candidates {
		if !env.blocked[beltCell(candidate)] && allowed(candidate) && !goalSet[candidate] {
			goals, goalSet[candidate] = append(goals, candidate), true
		}
	}
	if len(goals) == 0 {
		return nil, vector{}, 0
	}
	env.prepareTransportHeuristic(ctx, goals[0])
	// 相邻目标共用势场；预取目标势能，避免每个三维节点重复查询五次目标。
	field := env.corridorPotential
	goalPotentials := make([]float64, len(goals))
	goalHasPotential := make([]bool, len(goals))
	if field != nil && field.version == env.version {
		for i, goal := range goals {
			goalPotentials[i], goalHasPotential[i] = field.distanceAt(goal)
		}
	} else {
		field = nil
	}
	heuristic := func(point vector) (float64, int) {
		cost, length := math.Inf(1), math.MaxInt
		potential, hasPotential := 0., false
		if field != nil {
			potential, hasPotential = field.distanceAt(point)
		}
		for i, goal := range goals {
			horizontal := math.Abs(point.x-goal.x) + math.Abs(point.y-goal.y)
			vertical := math.Abs(point.z - goal.z)
			length = min(length, int(math.Ceil(horizontal+vertical)))
			if hasPotential && goalHasPotential[i] {
				horizontal = math.Max(horizontal, math.Abs(potential-goalPotentials[i]))
			}
			cost = math.Min(cost, horizontal+1.5*vertical)
		}
		return cost, length
	}
	cost, length := heuristic(start)
	first := &transportPathNode{point: start, score: cost, length: 1, lengthEstimate: 1 + length}
	queue := &transportDestinationQueue{transportPathQueue{first}}
	seen := map[vector]*transportPathNode{start: first}
	serial := 0
	for queue.Len() > 0 && steps < transportSearchLimit {
		if ctx.Err() != nil {
			return nil, vector{}, steps
		}
		current := heap.Pop(queue).(*transportPathNode)
		current.closed = true
		steps++
		if goalSet[current.point] {
			path := make([]vector, current.length)
			for node, i := current, len(path)-1; node != nil; node, i = node.parent, i-1 {
				path[i] = node.point
			}
			return path, current.point, steps
		}
		for _, delta := range []vector{{-1, 0, 0}, {0, 1, 0}, {1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
			point := vector{current.point.x + delta.x, current.point.y + delta.y, current.point.z + delta.z}
			if !allowed(point) || !env.denseTransportStepClear(current.point, point) {
				continue
			}
			distance, length := current.distance+env.transportStepCost(current.point, point), current.length+1
			old := seen[point]
			if old != nil && (old.closed || old.distance < distance || old.distance == distance && old.length <= length) {
				continue
			}
			remainingCost, remainingLength := heuristic(point)
			if old != nil {
				old.parent, old.distance, old.length = current, distance, length
				old.score, old.lengthEstimate = distance+remainingCost, length+remainingLength
				heap.Fix(queue, old.index)
				continue
			}
			serial++
			node := &transportPathNode{point: point, parent: current, distance: distance, score: distance + remainingCost, order: serial, length: length, lengthEstimate: length + remainingLength}
			seen[point] = node
			heap.Push(queue, node)
		}
	}
	return nil, vector{}, steps
}
