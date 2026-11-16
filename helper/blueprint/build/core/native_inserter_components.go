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
	"time"
)

// nativeInserterConflictEdge 职责：缓存两个合法域之间的真实碰撞组合；使用：组件求解；边界：位图只描述已经执行过的OBB检查。
type nativeInserterConflictEdge struct {
	left, right, width int
	bits               []uint64
}

// clashes 职责：按查询方向读取候选碰撞位；使用：赋值和前向检查；边界：候选编号来自同一不可变域。
func (edge *nativeInserterConflictEdge) clashes(domain, choice, otherChoice int) bool {
	slot := choice*edge.width + otherChoice
	if domain != edge.left {
		slot = otherChoice*edge.width + choice
	}
	return edge.bits[slot>>6]&(uint64(1)<<uint(slot&63)) != 0
}

// nativeInserterConflictGraph 职责：用空间桶和原OBB检查建立稀疏约束图；使用：只把真正可能冲突的臂关联；边界：不以包络重叠代替精确冲突。
func nativeInserterConflictGraph(ctx context.Context, domains []nativeInserterDomain) ([][]*nativeInserterConflictEdge, error) {
	graph := make([][]*nativeInserterConflictEdge, len(domains))
	envelopes := make([]packingShape, len(domains))
	cells := map[[3]int][]int{}
	geometries := make([]nativeInserterGeometrySet, len(domains))
	marks := make([]int, len(domains))
	neighbors := make([]int, 0)
	progress := newNativeInserterProgress(ctx, "native-inserter-conflicts", "正在检查分拣器候选冲突", len(domains))
	for i, domain := range domains {
		progress.report(i, false)
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
		neighbors = neighbors[:0]
		packingShapeCells(shape, func(cell [3]int) bool {
			for _, j := range cells[cell] {
				if marks[j] != i+1 {
					marks[j] = i + 1
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
			leftGeometry, rightGeometry := &geometries[i], &geometries[j]
			if leftGeometry.choices == nil {
				*leftGeometry = nativeInserterGeometries(domain)
			}
			if rightGeometry.choices == nil {
				*rightGeometry = nativeInserterGeometries(other)
			}
			// 先按精确盒序列缓存组合，再映射回全部原候选位；域与候选顺序不变。
			var cache []uint64
			if len(leftGeometry.boxes) < len(domain.choices) || len(rightGeometry.boxes) < len(other.choices) {
				cache = make([]uint64, (len(leftGeometry.boxes)*len(rightGeometry.boxes)+31)/32)
			}
			for a := range domain.choices {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				progress.report(i, false)
				leftID := leftGeometry.choices[a]
				for c := range other.choices {
					rightID := rightGeometry.choices[c]
					key := leftID*len(rightGeometry.boxes) + rightID
					state := uint64(0)
					if cache != nil {
						state = (cache[key>>5] >> uint((key&31)*2)) & 3
					}
					if state == 0 {
						state = 1
						if nativeInserterGeometryConflict(leftGeometry.boxes[leftID], rightGeometry.boxes[rightID]) {
							state = 2
						}
						if cache != nil {
							cache[key>>5] |= state << uint((key&31)*2)
						}
					}
					if state == 2 {
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
	progress.report(len(domains), true)
	return graph, nil
}

// solveNativeInserterDomains 职责：分组件执行带前向检查的有限回溯；使用：保留最小改动候选优先顺序；边界：所有组件共享原20000次预算，任一无解则整体失败。
func solveNativeInserterDomains(ctx context.Context, domains []nativeInserterDomain) ([]int, error) {
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
	graph, err := nativeInserterConflictGraph(ctx, domains)
	if err != nil {
		return nil, err
	}
	selected := make([]int, len(domains))
	for i := range selected {
		selected[i] = -1
	}
	visited := make([]bool, len(domains))
	nodes := 0
	searchProgress := newNativeInserterProgress(ctx, "native-inserter-search", "正在选择无碰撞分拣器端点", len(domains))
	completed := 0
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
		searchProgress.report(completed, false)
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
			completed++
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
				searchProgress.report(completed, false)
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
		completed += len(component)
	}
	searchProgress.report(len(domains), true)
	return selected, nil
}

// nativeInserterProgress 职责：节流真实已处理域计数；使用：端点构图与搜索；边界：同步通知，不估算整体百分比、不发布尚未完成的域。
type nativeInserterProgress struct {
	ctx            context.Context
	stage, message string
	total          int
	next           time.Time
}

// newNativeInserterProgress 职责：在实际入口发布零完成计数；使用：局部计算阶段；边界：取消后的通知由 reportProgress 统一抑制。
func newNativeInserterProgress(ctx context.Context, stage, message string, total int) nativeInserterProgress {
	reportProgress(ctx, stage, message, 0, total)
	return nativeInserterProgress{ctx: ctx, stage: stage, message: message, total: total, next: time.Now().Add(250 * time.Millisecond)}
}

// report 职责：至多每250毫秒发布当前已处理域数并允许强制收尾；使用：热点循环；边界：current 必须来自实际循环位置，不把预算当完成量。
func (progress *nativeInserterProgress) report(current int, force bool) {
	if !force && time.Now().Before(progress.next) {
		return
	}
	reportProgress(progress.ctx, progress.stage, progress.message, current, progress.total)
	progress.next = time.Now().Add(250 * time.Millisecond)
}
