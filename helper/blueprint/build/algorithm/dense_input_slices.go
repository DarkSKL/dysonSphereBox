package build

// 文件职责：按所选单带的真实容量切分同物料外供毛需求，并保留每条规范边的净流贡献。
// 使用：超密度直连带物理网络规划；边界：允许消费者跨入口取料，但不改机器、配方、喷涂或净产量，不能用计划净流代替配置毛负载。

import (
	"fmt"
	"math"
	"sort"
)

// planDenseInputSlices 职责：将完整消费者毛需求顺序切入容量桶并等比例展开规范份额；使用：机器几何创建之前；边界：只有入口数能减少时才替换原路线，机器回流及混合内外供给仍沿用原拓扑。
// 逻辑流：1. 收集纯外供消费者 => 2. 按毛需求填满入口容量 => 3. 将跨桶消费者分配到多个真实支路 => 4. 原规范边按同一比例拆份，末份消除舍入尾差。
func (b *builder) planDenseInputSlices(plan *denseMaterialNetworkPlan, rows []*transportRow, incoming map[denseMaterialInputKey][]*transportRoute) error {
	groups := map[int]*denseMaterialNetwork{}
	originals := map[denseMaterialInputKey][]*transportRoute{}
	capacity := b.logistics.BeltCapacityPerMinute
	for _, row := range rows {
		if recipeHasMachineRecycle(row.recipe) {
			continue
		}
		for _, item := range row.recipe.Items {
			flows := incoming[denseMaterialInputKey{row, item}]
			valid, planned := len(flows) > 0, 0.
			for _, route := range flows {
				if route.from != nil || route.edge.FlowKind != "external-input" || route.inputRole != recipeInputRole {
					valid = false
					break
				}
				planned += route.rate
			}
			if !valid {
				continue
			}
			full, err := configuredRoleInputRate(row.info, row.recipe, item, recipeInputRole)
			if err != nil {
				return err
			}
			full *= float64(row.count) / float64(row.info.MachineCount)
			if !finitePositive(full) || full+1e-6 < planned || full > capacity+1e-6 {
				continue
			}
			group := groups[item]
			if group == nil {
				group = &denseMaterialNetwork{item: item, splitInput: &denseInputSplitNetwork{bySource: map[*transportRoute]*denseInputSlice{}}}
				groups[item] = group
			}
			group.rows = append(group.rows, row)
			group.planned = append(group.planned, planned)
			group.configured = append(group.configured, full)
			originals[denseMaterialInputKey{row, item}] = flows
		}
	}
	items := make([]int, 0, len(groups))
	for item := range groups {
		items = append(items, item)
	}
	sort.Ints(items)
	replacements := map[*transportRoute][]*transportRoute{}
	for _, item := range items {
		group := groups[item]
		full, count := 0., 0
		for target, row := range group.rows {
			full += group.configured[target]
			count += len(originals[denseMaterialInputKey{row, item}])
		}
		needed := int(math.Ceil((full - 1e-8) / capacity))
		if needed >= count {
			continue
		}
		group.id = len(plan.groups)
		plan.groups = append(plan.groups, group)
		network := group.splitInput
		remaining := append([]float64{}, group.configured...)
		allocated := map[*transportRoute]float64{}
		for _, bucket := range denseInputCapacityBuckets(group.configured, capacity, needed) {
			first := originals[denseMaterialInputKey{group.rows[bucket[0].target], item}][0]
			copy := *first
			copy.edge.ID = fmt.Sprintf("external-slice:%d:%d", group.id, len(network.slices))
			copy.rate = 0
			slice := &denseInputSlice{source: &copy}
			network.slices = append(network.slices, slice)
			network.bySource[slice.source] = slice
			plan.physical = append(plan.physical, slice.source)
			for _, allocation := range bucket {
				target, part := allocation.target, allocation.configured
				flows := originals[denseMaterialInputKey{group.rows[target], item}]
				branch := &denseInputSplitBranch{target: target, configured: part, source: slice.source, contributions: map[*transportRoute]*transportRoute{}}
				branchIndex := len(network.branches)
				for _, original := range flows {
					copy := *original
					copy.rate = original.rate * part / group.configured[target]
					if remaining[target]-part <= 1e-8 {
						copy.rate = original.rate - allocated[original]
					}
					allocated[original] += copy.rate
					fragment := &copy
					branch.planned += fragment.rate
					slice.source.rate += fragment.rate
					branch.contributions[fragment] = original
					plan.bindings[fragment] = denseMaterialNetworkBinding{group: group, source: slice.source, target: target, splitBranch: branchIndex}
					plan.members[slice.source] = append(plan.members[slice.source], fragment)
					replacements[original] = append(replacements[original], fragment)
				}
				slice.configured += part
				slice.branches = append(slice.branches, branch)
				network.branches = append(network.branches, branch)
				remaining[target] -= part
			}
		}
	}
	canonical := make([]*transportRoute, 0, len(plan.canonical))
	for _, route := range plan.canonical {
		if parts, replaced := replacements[route]; replaced {
			canonical = append(canonical, parts...)
		} else {
			canonical = append(canonical, route)
		}
	}
	plan.canonical = canonical
	return nil
}

// denseInputAllocation 职责：保存一个容量桶对消费者的毛需求份额；使用：构造真实分流树；边界：不携带独立规范边，总净流按原始边比例另行拆分。
type denseInputAllocation struct {
	target     int
	configured float64
}

// denseInputCapacityBuckets 职责：优先以完整消费者达到最少容量口数，必要时才切开消费者；使用：外供装箱；边界：不靠增加入口保留完整组，有限贪心未达到下界时直接用守恒容量切片。
func denseInputCapacityBuckets(configured []float64, capacity float64, needed int) [][]denseInputAllocation {
	order := make([]int, len(configured))
	for target := range order {
		order[target] = target
	}
	sort.SliceStable(order, func(i, j int) bool { return configured[order[i]] > configured[order[j]] })
	buckets, loads := [][]denseInputAllocation{}, []float64{}
	for _, target := range order {
		chosen := -1
		for i, load := range loads {
			if load+configured[target] <= capacity+1e-8 {
				chosen = i
				break
			}
		}
		if chosen < 0 {
			chosen = len(buckets)
			buckets = append(buckets, nil)
			loads = append(loads, 0)
		}
		buckets[chosen] = append(buckets[chosen], denseInputAllocation{target, configured[target]})
		loads[chosen] += configured[target]
	}
	if len(buckets) == needed {
		return buckets
	}
	buckets, loads = nil, nil
	for target, full := range configured {
		for full > 1e-8 {
			if len(loads) == 0 || capacity-loads[len(loads)-1] <= 1e-8 {
				buckets = append(buckets, nil)
				loads = append(loads, 0)
			}
			index := len(loads) - 1
			part := math.Min(full, capacity-loads[index])
			buckets[index] = append(buckets[index], denseInputAllocation{target, part})
			loads[index] += part
			full -= part
		}
	}
	return buckets
}
