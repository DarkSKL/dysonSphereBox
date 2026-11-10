package build

// 文件职责：在一次有界搜索中比较消费端所有合法邻格，避免重复展开同一片三维空间。
// 使用：普通邻格及密铺完整接驳选择；边界：显式计入调用方提供的真实后缀，不改变端口权限或固定展开预算。

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

// transportDestination 职责：把整数接驳点与其真实后缀代价绑定；使用：完整端口及合流路线比较；边界：length只计接驳点之后的节点，reused只允许在目标处复用已有带。
type transportDestination struct {
	// point 职责：指定整数接驳点；使用：主路搜索目标；边界：不替代真实端口。
	point vector
	// cost 职责：保存接驳点到真实端口的后缀成本；使用：完整路线比较；边界：必须有限且非负。
	cost float64
	// length 职责：保存接驳点之后的真实节点数；使用：同成本择短；边界：不重复计接驳点。
	length int
	// reused 职责：许可目标处复用指定已有带；使用：合流锚点；边界：不能经此穿行其它目标。
	reused *placed
}

// findTransportDestinationPath 职责：保留普通消费端邻格选择语义；使用：既有未合流入口；边界：不含最终端口末步，目标仍必须空闲，取消不返回部分路线。
func (env *transportEnvironment) findTransportDestinationPath(ctx context.Context, start vector, candidates []vector, source, target *placed) ([]vector, vector, int) {
	goals := make([]transportDestination, len(candidates))
	for i, candidate := range candidates {
		goals[i].point = candidate
	}
	path, selected, steps := env.findTransportDestinationPathWithSuffix(ctx, start, goals, source, target)
	if selected < 0 {
		return nil, vector{}, steps
	}
	return path, candidates[selected], steps
}

// findTransportDestinationPathWithSuffix 职责：在一次有界搜索中比较到真实端口的完整代价和节点数；使用：密铺分数后缀及既有合流尾链；边界：返回原候选索引，重复接驳点按后缀择优，已有带目标不能作为穿行节点。
// 逻辑流：1. 筛选并合并目标 => 2. 将后缀计入一致下界 => 3. 更新完整路线并继续比较更优下界 => 4. 回溯获选接驳路径，取消时丢弃结果。
func (env *transportEnvironment) findTransportDestinationPathWithSuffix(ctx context.Context, start vector, candidates []transportDestination, source, target *placed) ([]vector, int, int) {
	env.searchCalls++
	steps := 0
	defer func() {
		env.totalExpanded += steps
		env.maxExpanded = max(env.maxExpanded, steps)
		recordPerformanceSearch(ctx, steps, false)
	}()
	inside := func(v vector) bool {
		return v.x >= env.minimum.x && v.x <= env.maximum.x && v.y >= env.minimum.y && v.y <= env.maximum.y && v.z >= 0 && v.z <= transportMaxHeight && !env.reservedByOther(v, source, target)
	}
	if ctx.Err() != nil || !inside(start) {
		return nil, -1, 0
	}
	goals := make([]vector, 0, len(candidates))
	goalSet := make(map[vector]int, len(candidates))
	for i, candidate := range candidates {
		if !inside(candidate.point) || candidate.cost < 0 || math.IsNaN(candidate.cost) || math.IsInf(candidate.cost, 0) || candidate.length < 0 || env.blocked[beltCell(candidate.point)] && (candidate.reused == nil || candidate.reused.a != candidate.point) {
			continue
		}
		old, exists := goalSet[candidate.point]
		if !exists {
			goals = append(goals, candidate.point)
		}
		if !exists || candidate.cost < candidates[old].cost || candidate.cost == candidates[old].cost && candidate.length < candidates[old].length {
			goalSet[candidate.point] = i
		}
	}
	if len(goals) == 0 {
		return nil, -1, 0
	}
	allowed := func(v vector) bool {
		if !inside(v) {
			return false
		}
		goal, exists := goalSet[v]
		return !env.blocked[beltCell(v)] || v == start || exists && candidates[goal].reused != nil
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
			suffix := candidates[goalSet[goal]]
			horizontal := math.Abs(point.x-goal.x) + math.Abs(point.y-goal.y)
			vertical := math.Abs(point.z - goal.z)
			length = min(length, int(math.Ceil(horizontal+vertical))+suffix.length)
			if hasPotential && goalHasPotential[i] {
				horizontal = math.Max(horizontal, math.Abs(potential-goalPotentials[i]))
			}
			cost = math.Min(cost, horizontal+env.transportElevationLowerBound(point, goal)+suffix.cost)
		}
		return cost, length
	}
	cost, length := heuristic(start)
	first := &transportPathNode{point: start, score: cost, length: 1, lengthEstimate: 1 + length}
	queue := &transportDestinationQueue{transportPathQueue{first}}
	seen := map[vector]*transportPathNode{start: first}
	serial := 0
	var best *transportPathNode
	bestGoal, bestLength, bestCost := -1, math.MaxInt, math.Inf(1)
	for queue.Len() > 0 && steps < transportSearchLimit {
		if ctx.Err() != nil {
			return nil, -1, steps
		}
		// 几何目标不是虚拟汇点：只有剩余队列的完整下界不能改善时才能结束。
		if best != nil && (queue.transportPathQueue[0].score > bestCost || queue.transportPathQueue[0].score == bestCost && queue.transportPathQueue[0].lengthEstimate >= bestLength) {
			break
		}
		current := heap.Pop(queue).(*transportPathNode)
		current.closed = true
		steps++
		if selected, exists := goalSet[current.point]; exists {
			suffix := candidates[selected]
			cost, length := current.distance+suffix.cost, current.length+suffix.length
			if cost < bestCost || cost == bestCost && length < bestLength {
				best, bestGoal, bestCost, bestLength = current, selected, cost, length
			}
			if env.blocked[beltCell(current.point)] && current.point != start {
				continue
			}
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
	if ctx.Err() != nil || best == nil {
		return nil, -1, steps
	}
	path := make([]vector, best.length)
	for node, i := best, len(path)-1; node != nil; node, i = node.parent, i-1 {
		path[i] = node.point
	}
	return path, bestGoal, steps
}
