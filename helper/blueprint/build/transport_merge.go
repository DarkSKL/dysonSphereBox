package build

// 文件职责：在消费者已有真实带链上选择可达且未用尽输入槽的合流格。
// 使用：直连带多源供给共享消费者入口时反向多源寻路；边界：不借用其它端口预留柱、不增大路由预算、不生成装饰连接。

import (
	"container/heap"
	"context"
)

// findTransportMergePath 职责：同时从全部合法下游锚点搜索新供给端，避免孤立锚点耗尽预算。
// 使用：已接好消费者的下一条供应路线；边界：返回源到锚点的真实逐格路径及原尾链索引，失败返回空路径/-1，所有候选合计至多一次固定展开预算。
func (env *transportEnvironment) findTransportMergePath(ctx context.Context, start vector, tail []*placed, source, target *placed, incoming map[*placed]int) ([]vector, int, int) {
	env.searchCalls++
	steps := 0
	defer func() {
		env.totalExpanded += steps
		env.maxExpanded = max(env.maxExpanded, steps)
		recordPerformanceSearch(ctx, steps, false)
	}()
	if ctx.Err() != nil || env.blocked[beltCell(start)] || env.reservedByOther(start, source, target) {
		return nil, -1, 0
	}
	inside := func(v vector) bool {
		return v.x >= env.minimum.x && v.y >= env.minimum.y && v.x <= env.maximum.x && v.y <= env.maximum.y && v.z >= 0 && v.z <= transportMaxHeight
	}
	env.prepareTransportHeuristic(ctx, start)
	heuristic := func(v vector) float64 {
		return env.transportHeuristic(v, start)
	}
	queue := transportPathQueue{}
	seen := map[vector]*transportPathNode{}
	roots := map[*transportPathNode]int{}
	serial := 0
	for i, candidate := range tail {
		v := candidate.a
		if !inside(v) || incoming[candidate] >= 3 || env.reservedByOther(v, source, target) || seen[v] != nil {
			continue
		}
		node := &transportPathNode{point: v, score: heuristic(v), order: serial, length: 1}
		serial++
		seen[v], roots[node] = node, i
		heap.Push(&queue, node)
	}
	for len(queue) > 0 && steps < transportSearchLimit {
		if ctx.Err() != nil {
			return nil, -1, steps
		}
		current := heap.Pop(&queue).(*transportPathNode)
		current.closed = true
		steps++
		if current.point == start {
			path := []vector{}
			root := current
			for node := current; node != nil; node = node.parent {
				path = append(path, node.point)
				root = node
			}
			return path, roots[root], steps
		}
		for _, delta := range []vector{{-1, 0, 0}, {0, 1, 0}, {1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
			v := vector{current.point.x + delta.x, current.point.y + delta.y, current.point.z + delta.z}
			if !inside(v) || env.blocked[beltCell(v)] || env.reservedByOther(v, source, target) {
				continue
			}
			distance := current.distance + env.transportStepCost(current.point, v)
			old := seen[v]
			if old != nil && (old.closed || distance >= old.distance) {
				continue
			}
			if old != nil {
				old.distance, old.score, old.parent = distance, distance+heuristic(v), current
				heap.Fix(&queue, old.index)
				continue
			}
			node := &transportPathNode{point: v, parent: current, distance: distance, score: distance + heuristic(v), order: serial}
			serial++
			seen[v] = node
			heap.Push(&queue, node)
		}
	}
	return nil, -1, steps
}
