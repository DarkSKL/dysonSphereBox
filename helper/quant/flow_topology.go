package quant

// 文件职责：对规范物料图执行确定性 Tarjan 和 DAG 排序；使用：生产布局与画板共享层级；边界：绝不删除循环边，循环组内部仅规定稳定顺序。

import (
	"context"
	"fmt"
	"sort"
)

// orderFlowComponents 职责：写入 SCC、层级及原产线索引序；使用：BuildFlowGraphContext；边界：只改变图派生字段，不修改边的流量或方向。
func orderFlowComponents(ctx context.Context, graph *FlowGraph) error {
	adj := map[string][]string{}
	for _, edge := range graph.Edges {
		adj[edge.SourceNodeID] = append(adj[edge.SourceNodeID], edge.TargetNodeID)
	}
	for id := range adj {
		sort.Strings(adj[id])
	}
	index, low, onStack := map[string]int{}, map[string]int{}, map[string]bool{}
	stack, groups := []string{}, [][]string{}
	next := 0
	var visit func(string) error
	visit = func(id string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		next++
		index[id], low[id], onStack[id] = next, next, true
		stack = append(stack, id)
		for _, target := range adj[id] {
			if index[target] == 0 {
				if err := visit(target); err != nil {
					return err
				}
				if low[target] < low[id] {
					low[id] = low[target]
				}
			} else if onStack[target] && index[target] < low[id] {
				low[id] = index[target]
			}
		}
		if low[id] == index[id] {
			group := []string{}
			for {
				last := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[last] = false
				group = append(group, last)
				if last == id {
					break
				}
			}
			sort.Strings(group)
			groups = append(groups, group)
		}
		return nil
	}
	for _, node := range graph.Nodes {
		if index[node.ID] == 0 {
			if err := visit(node.ID); err != nil {
				return err
			}
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i][0] < groups[j][0] })
	member := map[string]int{}
	for i, group := range groups {
		for _, id := range group {
			member[id] = i
		}
	}
	children := make([]map[int]bool, len(groups))
	indegree, level, cyclic := make([]int, len(groups)), make([]int, len(groups)), make([]bool, len(groups))
	for i, group := range groups {
		children[i] = map[int]bool{}
		cyclic[i] = len(group) > 1
	}
	for _, edge := range graph.Edges {
		a, b := member[edge.SourceNodeID], member[edge.TargetNodeID]
		if a == b {
			cyclic[a] = true
		} else if !children[a][b] {
			children[a][b] = true
			indegree[b]++
		}
	}
	ready := []int{}
	for i, degree := range indegree {
		if degree == 0 {
			ready = append(ready, i)
		}
	}
	ordered := []int{}
	for len(ready) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		sort.Ints(ready)
		current := ready[0]
		ready = ready[1:]
		ordered = append(ordered, current)
		nextGroups := make([]int, 0, len(children[current]))
		for child := range children[current] {
			nextGroups = append(nextGroups, child)
		}
		sort.Ints(nextGroups)
		for _, child := range nextGroups {
			if level[current]+1 > level[child] {
				level[child] = level[current] + 1
			}
			indegree[child]--
			if indegree[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	byID := map[string]FlowNode{}
	for i := range graph.Nodes {
		node := &graph.Nodes[i]
		group := member[node.ID]
		node.Level, node.ComponentID = level[group], fmt.Sprintf("scc:%d", group)
		byID[node.ID] = *node
	}
	for _, group := range ordered {
		graph.Components = append(graph.Components, FlowComponent{ID: fmt.Sprintf("scc:%d", group), NodeIDs: groups[group], Level: level[group], Cyclic: cyclic[group]})
		for _, id := range groups[group] {
			if node := byID[id]; node.Kind == "production" {
				graph.ProductionOrder = append(graph.ProductionOrder, node.ProductionIndex)
			}
		}
	}
	return nil
}
