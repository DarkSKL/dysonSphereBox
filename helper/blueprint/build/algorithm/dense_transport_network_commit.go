package build

// 文件职责：在串供轨与来源合流真实接通后，把聚合物理路径还原为原规范份额。
// 使用：街道附件提交之后；边界：每条route仍到自己的消费者，公共前缀按下游份额累计，本行轨额外保留自己的消耗负载。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// denseMaterialDirectedPath 职责：在最终存活节点中截取指定首尾的真实有向带；使用：输入轨和串供证明；边界：不穿分拣器、机器或库存，不按坐标补链。
func denseMaterialDirectedPath(from, to *placed, alive map[*placed]bool) ([]*placed, error) {
	path, seen := []*placed{}, map[*placed]bool{}
	for node := from; node != nil && !seen[node]; node = node.output {
		if !alive[node] || !surfaceIsBelt(node.record.ItemID) {
			break
		}
		path = append(path, node)
		if node == to {
			return path, nil
		}
		if node.record.OutputToSlot != 1 {
			break
		}
		seen[node] = true
	}
	return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享消费轨缺失存活节点或真实单向连接"}
}

// denseMaterialSupplyPath 职责：保留实际汇流缓冲到首消费者的全部节点；使用：规范展开及终检比对；边界：不把多来源合流放在机器cargo曲线邻域。
func denseMaterialSupplyPath(source *transportRoute, group *denseMaterialNetwork) ([]*placed, error) {
	if len(source.path) == 0 || source.path[len(source.path)-1] != group.supplyHead {
		return nil, transportAllocationError(source.edge, source.rate, "聚合来源未到达实际共享供给头")
	}
	path := append([]*placed{}, source.path...)
	if len(group.intake) > 0 {
		join := group.intakeRoute
		if join == nil || len(join.path) < 2 || join.path[0] != group.intake[len(group.intake)-1] || join.path[len(join.path)-1] != group.entries[0] {
			return nil, transportAllocationError(source.edge, source.rate, "真实汇流缓冲没有接入首消费者")
		}
		path = append(path, group.intake[1:]...)
		path = append(path, join.path[1:]...)
	}
	return path, nil
}

// commitDenseMaterialNetworks 职责：严格展开每条规范份额经过的上游消费轨；使用：完整路由后；边界：实体只生成一次，展开只写证据，不能伪造普通带分叉。
func (b *builder) commitDenseMaterialNetworks() error {
	plan := b.denseMaterialNetworks
	if plan == nil || plan.committed {
		return nil
	}
	alive := map[*placed]bool{}
	for _, entity := range b.all {
		alive[entity] = true
	}
	crossRecipes, tapped, splitInputs, maxConfigured := 0, 0, 0, 0.
	for _, group := range plan.groups {
		if group.splitInput != nil {
			if err := refreshDenseInputSplitterPrefixes(group); err != nil {
				return err
			}
		}
		if len(group.entries) != len(group.rows) || group.splitInput == nil && !group.tapped && len(group.joins)+1 != len(group.rows) || group.splitInput == nil && group.tapped && len(group.taps) != len(group.rows) || group.splitInput != nil && len(group.splitInput.branches) < len(group.rows) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享消费轨缺少实际端口或串接路线"}
		}
		recipes := map[int]bool{}
		full := 0.
		for i, row := range group.rows {
			recipes[row.recipe.ID] = true
			full += group.configured[i]
			path, err := denseMaterialDirectedPath(group.entries[i], group.exits[i], alive)
			if err != nil {
				return err
			}
			members := map[*placed]bool{}
			for _, node := range path {
				members[node] = true
			}
			index := denseMaterialInputIndex(row, group.item)
			for _, node := range row.line.inputs[index] {
				if !members[node] {
					return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "串供轨没有经过本行全部真实机器输入节点"}
				}
			}
			group.tracks = append(group.tracks, path)
			load := 0.
			if group.tapped || group.splitInput != nil {
				load = group.planned[i]
			} else {
				for _, rate := range group.planned[i:] {
					load += rate
				}
			}
			group.entries[i].record.Parameters = []int32{int32(group.item), int32(math.Ceil(load))}
		}
		maxConfigured = math.Max(maxConfigured, full)
		if len(recipes) > 1 {
			crossRecipes++
		}
		if group.splitInput != nil {
			splitInputs++
		} else if group.tapped {
			tapped++
		}
	}
	for _, route := range plan.canonical {
		binding, ok := plan.bindings[route]
		if !ok {
			continue
		}
		group, source := binding.group, binding.source
		if source.from == nil {
			route.boundary = source.boundary
		}
		if group.splitInput != nil {
			branch := group.splitInput.branches[binding.splitBranch]
			route.start, route.end = branch.route.start, group.entries[binding.target]
			route.path = denseInputSplitConsumerPath(group, branch)
			route.filters = nil
			route.sharedPath = denseInputSplitSharedPath(source, group, binding.splitBranch)
			continue
		}
		if group.tapped {
			tap := group.taps[binding.target]
			if len(tap.route.path) == 0 || tap.route.path[len(tap.route.path)-1] != group.entries[binding.target] {
				return transportAllocationError(route.edge, route.rate, "共享原料过滤支路没有接到消费者原喷涂前入口")
			}
			route.start, route.end = tap.route.start, group.entries[binding.target]
			route.path = append([]*placed{}, tap.route.path...)
			route.filters = append(append([]*placed{}, source.filters...), tap.filters...)
			route.sharedPath = denseMaterialTapSharedPath(source, group, binding.target)
			continue
		}
		path, err := denseMaterialSupplyPath(source, group)
		if err != nil {
			return err
		}
		for i := 0; i < binding.target; i++ {
			if len(group.joins[i].path) < 2 || group.joins[i].path[0] != group.exits[i] || group.joins[i].path[len(group.joins[i].path)-1] != group.entries[i+1] {
				return transportAllocationError(route.edge, route.rate, "共享消费轨之间没有完整真实串接")
			}
			path = append(path, group.tracks[i][1:]...)
			path = append(path, group.joins[i].path[1:]...)
		}
		route.start, route.end, route.path = source.start, group.entries[binding.target], path
		route.filters = append([]*placed{}, source.filters...)
		route.sharedPath = append(append([]*placed{}, source.sharedPath...), group.tracks[binding.target][1:]...)
	}
	for row, physical := range plan.physicalOutputs {
		row.outputs = nil
		for _, source := range physical {
			if members := plan.members[source]; len(members) > 0 {
				row.outputs = append(row.outputs, members...)
			} else {
				row.outputs = append(row.outputs, source)
			}
		}
	}
	b.transportRoutes, plan.committed = plan.canonical, true
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-material-consumer-networks", Status: "passed", Message: fmt.Sprintf("%d组同物料共享网络，其中%d组输入轨串供、%d组共享主干过滤支路、%d组外供真实四向分流及合流，%d组跨配方；物理供给%d条对应%d份规范路线，另有%d条真实辅助连接。最大网络总配置毛需求%.3f/min，外供按单入口容量切片；普通带没有T分支，喷涂计量和回流保持原合同；最终仍逐段核验", len(plan.groups), len(plan.groups)-tapped-splitInputs, tapped, splitInputs, crossRecipes, len(plan.physical), len(plan.canonical), len(plan.joins), maxConfigured)})
	return nil
}
