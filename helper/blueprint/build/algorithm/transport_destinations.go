package build

// 文件职责：在一次有界搜索中比较消费端所有合法邻格，避免重复展开同一片三维空间。
// 使用：普通邻格及密铺完整接驳选择；边界：显式计入调用方提供的真实后缀，不改变端口权限或固定展开预算。

import (
	"container/heap"
	"context"
	"math"
)

// transportDestinationQueue 职责：按代价估计和节点数下界字典序展开；使用：精确及有界多终点搜索；边界：启发权重由调用方设置，停止条件另行核验。
type transportDestinationQueue struct{ transportPathQueue }

// Less 职责：按代价估计、长度下界、已行代价和插入顺序排序；使用：稳定堆比较；边界：排序本身不证明最优性，代价界由搜索器核验。
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
	weight := 1.
	// 大型原生网格的可达状态已超过单次展开预算；采用有界代价引导并保留重新打开节点，避免反复耗尽预算仍得不到完整路线。
	if env.densePorts != nil && env.densePorts.polarFrame != nil && !env.densePorts.strictBelts && (env.maximum.x-env.minimum.x+1)*(env.maximum.y-env.minimum.y+1) >= 16384 {
		weight = 2
	}
	return env.findTransportDestinationPathWeighted(ctx, start, candidates, source, target, weight)
}

// findTransportDestinationPathWeighted 职责：在完整后缀代价下执行精确或有界搜索；使用：普通网格权重 1，大型原生网格权重 2；边界：引导搜索允许重开节点，只有队列下界证明代价界或队列耗尽时才交付，预算和硬几何不变。
func (env *transportEnvironment) findTransportDestinationPathWeighted(ctx context.Context, start vector, candidates []transportDestination, source, target *placed, weight float64) ([]vector, int, int) {
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
			goals = append(goals, candidate.point)
		}
		if !exists || candidate.cost < candidates[old].cost || candidate.cost == candidates[old].cost && candidate.length < candidates[old].length {
			goalSet[candidate.point] = i
		}
	}
	if len(goals) == 0 {
		return nil, -1, 0
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
		goal, exists := goalSet[v]
		return !env.blocked[beltCell(v)] || v == start || exists && candidates[goal].reused != nil
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
	first := &transportPathNode{point: start, score: weight * cost, length: 1, lengthEstimate: 1 + length}
	queue := &transportDestinationQueue{transportPathQueue{first}}
	seen := newTransportSearchIndex(start, env.minimum, env.maximum)
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
			// 精确分支不会重新打开已关闭节点；先读状态，避免为这些邻格重复查询占位、整段净空和软代价。
			old := seen.get(point)
			if old != nil && old.closed && weight == 1 {
				continue
			}
			if !allowed(point) || !env.denseTransportStepClear(current.point, point) {
				continue
			}
			distance, length := current.distance+env.transportStepCost(current.point, point), current.length+1
			if old != nil && (old.distance < distance || old.distance == distance && old.length <= length) {
				continue
			}
			remainingCost, remainingLength := heuristic(point)
			if old != nil {
				old.parent, old.distance, old.length = current, distance, length
				old.score, old.lengthEstimate = distance+weight*remainingCost, length+remainingLength
				if old.closed {
					old.closed = false
					heap.Push(queue, old)
				} else {
					heap.Fix(queue, old.index)
				}
				continue
			}
			serial++
			node := &transportPathNode{point: point, parent: current, distance: distance, score: distance + weight*remainingCost, order: serial, length: length, lengthEstimate: length + remainingLength}
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
