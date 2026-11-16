package build

// 文件职责：冻结单入边预检时代的完整A*；使用：短通道优化的独立对照；边界：仅临时验证，不供生产调用。

import (
	"container/heap"
	"context"
	"math"
)

// frozenGoalEntryPossible 职责：冻结本轮优化前的单入边预检；使用：旧A*基线；边界：不随生产算法改写。
func frozenGoalEntryPossible(env *transportEnvironment, start, goal vector, source, target *placed, inside func(vector) bool) bool {
	if goal == start {
		return true
	}
	for _, d := range []vector{{-1, 0, 0}, {0, 1, 0}, {1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
		p := vector{goal.x + d.x, goal.y + d.y, goal.z + d.z}
		if inside(p) && (!env.blocked[beltCell(p)] || p == start) && env.denseTransportStepClear(p, goal) && env.reservedSpanClear(p, goal, source, target) {
			return true
		}
	}
	return false
}
func (env *transportEnvironment) frozenGoalEntryPathWeighted(ctx context.Context, start vector, candidates []transportDestination, source, target *placed, weight float64) ([]vector, int, int) {
	env.searchCalls++
	if weight > 1 {
		env.guidedCalls++
	}
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
			// 一个合法入边仍可能只通向封闭死路；仅以同一有向规则证明短链无解时才排除目标。
			if !frozenGoalEntryPossible(env, start, candidate.point, source, target, inside) {
				continue
			}
			goals = append(goals, candidate.point)
		}
		if !exists || candidate.cost < candidates[old].cost || candidate.cost == candidates[old].cost && candidate.length < candidates[old].length {
			goalSet[candidate.point] = i
		}
	}
	if len(goals) == 0 {
		return nil, -1, 0
	}
	goalLow, goalHigh := goals[0], goals[0]
	for _, goal := range goals[1:] {
		goalLow = vector{min(goalLow.x, goal.x), min(goalLow.y, goal.y), min(goalLow.z, goal.z)}
		goalHigh = vector{max(goalHigh.x, goal.x), max(goalHigh.y, goal.y), max(goalHigh.z, goal.z)}
	}
	// 目标通常只占端口周围数格，包络外的展开节点不必反复哈希整组三浮点坐标。
	goalAt := func(point vector) (int, bool) {
		if point.x < goalLow.x || point.x > goalHigh.x || point.y < goalLow.y || point.y > goalHigh.y || point.z < goalLow.z || point.z > goalHigh.z {
			return -1, false
		}
		index, exists := goalSet[point]
		return index, exists
	}
	// 去重后的目标及获选后缀在本次搜索内固定；按原目标顺序预取，避免每个节点重复哈希同一组浮点坐标。
	goalSuffixes := make([]transportDestination, len(goals))
	for i, goal := range goals {
		goalSuffixes[i] = candidates[goalSet[goal]]
	}
	heuristicGoals := transportHeuristicGoals(goalSuffixes)
	allowed := func(v vector) bool {
		if !inside(v) {
			return false
		}
		if !env.blocked[beltCell(v)] || v == start {
			return true
		}
		goal, exists := goalAt(v)
		return exists && candidates[goal].reused != nil
	}
	env.prepareTransportHeuristic(ctx, goals[0])
	// 相邻目标共用势场；预取目标势能，避免每个三维节点重复查询五次目标。
	field := env.corridorPotential
	goalPotentials := make([]float64, len(goals))
	goalHasPotential := make([]bool, len(goals))
	if field != nil && field.version == env.version {
		for _, i := range heuristicGoals {
			goal := goals[i]
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
		for _, i := range heuristicGoals {
			goal := goals[i]
			suffix := goalSuffixes[i]
			horizontal := math.Abs(point.x-goal.x) + math.Abs(point.y-goal.y)
			vertical := math.Abs(point.z - goal.z)
			length = min(length, int(math.Ceil(horizontal+vertical))+suffix.length)
			if hasPotential && goalHasPotential[i] {
				horizontal = max(horizontal, math.Abs(potential-goalPotentials[i]))
			}
			cost = min(cost, horizontal+env.transportElevationLowerBound(point, goal)+suffix.cost)
		}
		return cost, length
	}
	cost, length := heuristic(start)
	arena := &env.searchArena
	arena.used = 0
	first := arena.add(transportPathNode{point: start, score: weight * cost, length: 1, lengthEstimate: 1 + length, heuristicCost: cost, heuristicLength: length})
	queue := &transportDestinationQueue{append(env.searchQueue[:0], first)}
	defer func() { env.searchQueue = queue.transportPathQueue[:0] }()
	seen := &env.searchIndex
	seen.reset(start, env.minimum, env.maximum)
	seen.set(start, first)
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
		if selected, exists := goalAt(current.point); exists {
			suffix := candidates[selected]
			cost, length := current.distance+suffix.cost, current.length+suffix.length
			if cost < bestCost || cost == bestCost && length < bestLength {
				best, bestGoal, bestCost, bestLength = current, selected, cost, length
			}
			if env.blocked[beltCell(current.point)] && current.point != start {
				continue
			}
		}
		for _, delta := range transportNeighborOffsets {
			point := vector{current.point.x + delta.x, current.point.y + delta.y, current.point.z + delta.z}
			// 精确分支不会重新打开已关闭节点；先读状态，避免为这些邻格重复查询占位、整段净空和软代价。
			old := seen.get(point)
			if old != nil && old.closed && weight == 1 {
				continue
			}
			distance, length := 0., current.length+1
			if old != nil {
				distance = current.distance + env.transportStepCost(current.point, point)
				// 已知不能改善的邻边无需再做静态净空；发现新节点或确实松弛时仍执行全部几何规则。
				if old.distance < distance || old.distance == distance && old.length <= length {
					continue
				}
			}
			if !allowed(point) || !env.denseTransportStepClear(current.point, point) || !env.reservedSpanClear(current.point, point, source, target) {
				continue
			}
			if old != nil {
				old.parent, old.distance, old.length = current, distance, length
				old.score, old.lengthEstimate = distance+weight*old.heuristicCost, length+old.heuristicLength
				if old.closed {
					old.closed = false
					heap.Push(queue, old)
				} else {
					heap.Fix(queue, old.index)
				}
				continue
			}
			distance = current.distance + env.transportStepCost(current.point, point)
			remainingCost, remainingLength := heuristic(point)
			serial++
			node := arena.add(transportPathNode{point: point, parent: current, distance: distance, score: distance + weight*remainingCost, order: serial, length: length, lengthEstimate: length + remainingLength, heuristicCost: remainingCost, heuristicLength: remainingLength})
			seen.set(point, node)
			heap.Push(queue, node)
		}
	}
	if ctx.Err() != nil || best == nil {
		return nil, -1, steps
	}
	if weight > 1 && queue.Len() > 0 && bestCost > queue.transportPathQueue[0].score {
		return nil, -1, steps
	}
	path := make([]vector, best.length)
	for node, i := best, len(path)-1; node != nil; node, i = node.parent, i-1 {
		path[i] = node.point
	}
	return path, bestGoal, steps
}
