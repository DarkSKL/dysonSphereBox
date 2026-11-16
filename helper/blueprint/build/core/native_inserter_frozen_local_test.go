package build

// 文件职责：按实际可能碰撞拆分分拣器端点约束，避免无关设备参与指数回溯。
// 使用：原生端点合法域建立后；边界：保持原OBB规则和20000次赋值预算，不提交不完整解。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
	"sort"
)

// nativeInserterConflictGraph 职责：用空间桶和原OBB检查建立稀疏约束图；使用：只把真正可能冲突的臂关联；边界：不以包络重叠代替精确冲突。
func frozenNativeInserterConflictGraph(ctx context.Context, domains []nativeInserterDomain) ([][]*nativeInserterConflictEdge, error) {
	graph := make([][]*nativeInserterConflictEdge, len(domains))
	envelopes := make([]packingShape, len(domains))
	cells := map[[3]int][]int{}
	for i, domain := range domains {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		shape := packingShape{low: placement.Vector{math.Inf(1), math.Inf(1), math.Inf(1)}, high: placement.Vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}}
		present := false
		for _, choice := range domain.choices {
			for _, box := range choice.boxes {
				present = true
				low, high := box.Geometry.Bounds()
				for axis := 0; axis < 3; axis++ {
					shape.low[axis] = math.Min(shape.low[axis], low[axis]-1e-6)
					shape.high[axis] = math.Max(shape.high[axis], high[axis]+1e-6)
				}
			}
		}
		if !present {
			continue
		}
		envelopes[i] = shape
		seen := map[int]bool{}
		neighbors := []int{}
		packingShapeCells(shape, func(cell [3]int) bool {
			for _, j := range cells[cell] {
				if !seen[j] {
					seen[j] = true
					neighbors = append(neighbors, j)
				}
			}
			return true
		})
		sort.Ints(neighbors)
		for _, j := range neighbors {
			separated := false
			for axis := 0; axis < 3; axis++ {
				if shape.high[axis] < envelopes[j].low[axis] || envelopes[j].high[axis] < shape.low[axis] {
					separated = true
					break
				}
			}
			if separated {
				continue
			}
			other := domains[j]
			edge := &nativeInserterConflictEdge{left: i, right: j, width: len(other.choices), bits: make([]uint64, (len(domain.choices)*len(other.choices)+63)/64)}
			anyConflict := false
			for a, left := range domain.choices {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				for c, right := range other.choices {
					conflict := false
					for _, first := range left.boxes {
						for _, second := range right.boxes {
							if first.Geometry.Overlaps(second.Geometry) {
								conflict = true
								break
							}
						}
						if conflict {
							break
						}
					}
					if conflict {
						slot := a*edge.width + c
						edge.bits[slot>>6] |= uint64(1) << uint(slot&63)
						anyConflict = true
					}
				}
			}
			if anyConflict {
				graph[i] = append(graph[i], edge)
				graph[j] = append(graph[j], edge)
			}
		}
		packingShapeCells(shape, func(cell [3]int) bool { cells[cell] = append(cells[cell], i); return true })
	}
	return graph, nil
}

// solveNativeInserterDomains 职责：分组件执行带前向检查的有限回溯；使用：保留最小改动候选优先顺序；边界：所有组件共享原20000次预算，任一无解则整体失败。
func frozenSolveNativeInserterDomains(ctx context.Context, domains []nativeInserterDomain) ([]int, error) {
	// 首选组合已无冲突时不必构造备选约束图；空间索引仍执行同一OBB检查。
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	preferred := []placement.OwnedBox{}
	for index, domain := range domains {
		if len(domain.choices) == 0 {
			return nil, &blueprint.Error{Code: "GEN_E_COLLISION", Message: "原生分拣器缺少合法端点候选"}
		}
		for _, box := range domain.choices[0].boxes {
			box.Owner = index
			preferred = append(preferred, box)
		}
	}
	left, _, err := placement.FirstConflict(ctx, preferred)
	if err != nil {
		return nil, err
	}
	if left < 0 {
		return make([]int, len(domains)), nil
	}
	graph, err := frozenNativeInserterConflictGraph(ctx, domains)
	if err != nil {
		return nil, err
	}
	selected := make([]int, len(domains))
	for i := range selected {
		selected[i] = -1
	}
	visited := make([]bool, len(domains))
	nodes := 0
	otherOf := func(edge *nativeInserterConflictEdge, domain int) int {
		if edge.left == domain {
			return edge.right
		}
		return edge.left
	}
	compatible := func(domain, choice int) bool {
		for _, edge := range graph[domain] {
			other := otherOf(edge, domain)
			if selected[other] >= 0 && edge.clashes(domain, choice, selected[other]) {
				return false
			}
		}
		return true
	}
	for start := range domains {
		if visited[start] {
			continue
		}
		component := []int{start}
		visited[start] = true
		for at := 0; at < len(component); at++ {
			for _, edge := range graph[component[at]] {
				other := otherOf(edge, component[at])
				if !visited[other] {
					visited[other] = true
					component = append(component, other)
				}
			}
		}
		sort.Ints(component)
		if len(component) == 1 && len(domains[start].choices) > 0 {
			selected[start] = 0
			continue
		}
		var search func(int) bool
		search = func(depth int) bool {
			if depth == len(component) {
				return true
			}
			if ctx.Err() != nil || nodes >= 20000 {
				return false
			}
			domain := component[depth]
			for choice := range domains[domain].choices {
				if ctx.Err() != nil || nodes >= 20000 {
					return false
				}
				nodes++
				if !compatible(domain, choice) {
					continue
				}
				selected[domain] = choice
				forward := true
				for _, edge := range graph[domain] {
					other := otherOf(edge, domain)
					if selected[other] >= 0 {
						continue
					}
					possible := false
					for candidate := range domains[other].choices {
						if compatible(other, candidate) {
							possible = true
							break
						}
					}
					if !possible {
						forward = false
						break
					}
				}
				if forward && search(depth+1) {
					return true
				}
				selected[domain] = -1
				if nodes >= 20000 || ctx.Err() != nil {
					return false
				}
			}
			return false
		}
		if !search(0) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			detail := ""
			for position, index := range component {
				if position >= 8 {
					detail += fmt.Sprintf(" 等%d个域", len(component))
					break
				}
				if len(domains[index].choices) > 0 {
					r := domains[index].choices[0].record
					detail += fmt.Sprintf(" %d(%d:%d→%d:%d)", domains[index].index, r.InputObjIdx, r.InputFromSlot, r.OutputObjIdx, r.OutputToSlot)
				}
			}
			return nil, &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("原生分拣器%d个关联端点域在%d次赋值内没有完整无碰撞组合：%s", len(component), nodes, detail)}
		}
	}
	return selected, nil
}
