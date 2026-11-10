package build

// 文件职责：从最终存活实体重新证明共享消费轨、规范份额展开与整组配置毛需求。
// 使用：公共物流汇总之前；边界：不凭候选开关放行无过滤出货，额外汇入、断链、喷涂变化或超带宽都拒绝。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// sameDenseMaterialNodes 职责：逐指针比较真实路径；使用：禁止用另一条近邻带冒充原串供链；边界：空路径只与空路径相等。
func sameDenseMaterialNodes(a, c []*placed) bool {
	if len(a) != len(c) {
		return false
	}
	for i := range a {
		if a[i] != c[i] {
			return false
		}
	}
	return true
}

// validateDenseMaterialNetworks 职责：独立重算真实链、份额和配置毛流上界；使用：最终物流证明；边界：并不把原料或机器配置满负荷需求降为规范净流。
func (b *builder) validateDenseMaterialNetworks() error {
	plan := b.denseMaterialNetworks
	if plan == nil {
		return nil
	}
	fail := func(reason string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "共享消费带不完整：" + reason}
	}
	if !plan.committed || len(plan.groups) == 0 || len(b.transportRoutes) != len(plan.canonical) {
		return fail("未完整提交规范物流份额")
	}
	alive := map[*placed]bool{}
	for _, entity := range b.all {
		alive[entity] = true
	}
	inputTotals := map[denseMaterialInputKey]float64{}
	for route, binding := range plan.bindings {
		if binding.group == nil || binding.target < 0 || binding.target >= len(binding.group.rows) || route.to != binding.group.rows[binding.target] || !finitePositive(route.rate) {
			return fail("规范份额没有对应有效消费者")
		}
		inputTotals[denseMaterialInputKey{route.to, route.edge.ItemID}] += route.rate
	}
	maxConfigured := 0.
	for _, group := range plan.groups {
		for i, row := range group.rows {
			if math.Abs(inputTotals[denseMaterialInputKey{row, group.item}]-group.planned[i]) > math.Max(1e-6, group.planned[i]*1e-9) {
				return fail("一个消费者的规范份额没有完整保留")
			}
		}
		if group.splitInput != nil {
			configured, err := b.validateDenseInputSplitters(group, alive)
			if err != nil {
				return err
			}
			maxConfigured = math.Max(maxConfigured, configured)
			continue
		}
		if group.tapped {
			configured, err := b.validateDenseMaterialTapNetwork(group, alive)
			if err != nil {
				return err
			}
			maxConfigured = math.Max(maxConfigured, configured)
			continue
		}
		if len(group.rows) < 2 || len(group.rows) != len(group.tracks) || len(group.joins)+1 != len(group.rows) {
			return fail("消费者或串接轨登记不完整")
		}
		configured := 0.
		shared, allowed, allowedArms := map[*placed]bool{}, map[*placed]bool{}, map[*placed]bool{}
		for i, row := range group.rows {
			if productionSprayItem(row.info) != 0 || recipeHasMachineRecycle(row.recipe) || len(row.line.coaters) != 0 {
				return fail("无喷涂共享组混入喷涂或回流用途")
			}
			full, err := configuredRoleInputRate(row.info, row.recipe, group.item, recipeInputRole)
			if err != nil {
				return err
			}
			full *= float64(row.count) / float64(row.info.MachineCount)
			if !finitePositive(full) || math.Abs(full-group.configured[i]) > math.Max(1e-6, full*1e-9) {
				return fail("消费者配置毛需求与已规划份额不一致")
			}
			configured += full
			track, err := denseMaterialDirectedPath(group.entries[i], group.exits[i], alive)
			if err != nil || !sameDenseMaterialNodes(track, group.tracks[i]) {
				return fail("消费者原料轨已断开或改变")
			}
			for _, node := range track {
				shared[node], allowed[node] = true, true
			}
		}
		if configured > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("物料%d共享消费组配置毛输入%.6f/min，超过所选单带%.6f/min；不能只按净流检查", group.item, configured, b.logistics.BeltCapacityPerMinute)}
		}
		maxConfigured = math.Max(maxConfigured, configured)
		for i, join := range group.joins {
			path, err := denseMaterialDirectedPath(group.exits[i], group.entries[i+1], alive)
			if err != nil || !sameDenseMaterialNodes(path, join.path) {
				return fail("轨尾到下一消费者的真实单向连接已中断")
			}
			for _, node := range path {
				shared[node], allowed[node] = true, true
			}
		}
		if len(group.intake) > 0 {
			if group.intakeRoute == nil || len(group.intakeRoute.path) < 2 {
				return fail("真实汇流缓冲缺少到机器的接线")
			}
			path, err := denseMaterialDirectedPath(group.intake[0], group.entries[0], alive)
			expected := append(append([]*placed{}, group.intake...), group.intakeRoute.path[1:]...)
			if err != nil || !sameDenseMaterialNodes(path, expected) {
				return fail("真实汇流缓冲或到机器的单向接线已中断")
			}
			for _, node := range path {
				shared[node], allowed[node] = true, true
			}
		}
		for source, members := range plan.members {
			if len(members) == 0 || plan.bindings[members[0]].group != group {
				continue
			}
			if source == nil || len(source.path) == 0 {
				return fail("聚合来源缺少真实路径")
			}
			path, err := denseMaterialDirectedPath(source.path[0], group.supplyHead, alive)
			if err != nil || !sameDenseMaterialNodes(path, source.path) {
				return fail("聚合来源没有完整接入共享带首端")
			}
			for _, node := range source.path {
				allowed[node] = true
			}
			for _, arm := range source.filters {
				allowedArms[arm] = true
			}
			total := 0.
			for _, route := range members {
				binding := plan.bindings[route]
				if route.from != source.from || route.to != group.rows[binding.target] || route.edge.ItemID != group.item || route.inputRole != recipeInputRole || route.end != group.entries[binding.target] || !finitePositive(route.rate) || !sameDenseMaterialNodes(route.filters, source.filters) {
					return fail("规范份额物料、用途或指定消费者被替换")
				}
				total += route.rate
				expected, err := denseMaterialSupplyPath(source, group)
				if err != nil {
					return err
				}
				for i := 0; i < binding.target; i++ {
					expected = append(expected, group.tracks[i][1:]...)
					expected = append(expected, group.joins[i].path[1:]...)
				}
				own := append(append([]*placed{}, source.sharedPath...), group.tracks[binding.target][1:]...)
				if !sameDenseMaterialNodes(expected, route.path) || !sameDenseMaterialNodes(own, route.sharedPath) {
					return fail("规范路线没有完整累计公共前缀及本行输入负载")
				}
			}
			if math.Abs(total-source.rate) > math.Max(1e-6, source.rate*1e-9) || total > b.logistics.BeltCapacityPerMinute+1e-6 {
				return fail("同一物理供给与原规范份额不守恒或超带宽")
			}
		}
		for _, entity := range b.all {
			if shared[entity.output] && (surfaceIsBelt(entity.record.ItemID) && !allowed[entity] || entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 && !allowedArms[entity]) {
				return fail("共享消费轨出现未登记的物料汇入")
			}
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-material-network-capacity", Status: "passed", Message: fmt.Sprintf("最终逐指针复核共享消费轨、外供切片、分流/合流及原始规范边贡献；逐入口或串供组保守累计配置毛需求，最高%.3f/min，不超过所选单带%.3f/min，实际带段均累计下游毛负载，未省略本行输入或允许未登记汇入", maxConfigured, b.logistics.BeltCapacityPerMinute)})
	return nil
}

// validateDenseMaterialDirectOutput 职责：证明多个规范去向确实共用单一物理出货轨；使用：无额外过滤的源端检查；边界：只接受已登记串供聚合源，不以dense开关放行普通多去向。
func (b *builder) validateDenseMaterialDirectOutput(route *transportRoute) (bool, error) {
	plan := b.denseMaterialNetworks
	if plan == nil || !plan.committed {
		return false, nil
	}
	binding, ok := plan.bindings[route]
	if !ok {
		return false, nil
	}
	fail := func(reason string) (bool, error) {
		return true, transportAllocationError(route.edge, route.rate, "共享轨来源直连无效："+reason)
	}
	source, row := binding.source, route.from
	if source.from != row || row == nil || row.line == nil || len(source.filters) != 0 || route.start != source.start || len(source.path) == 0 || len(route.path) < len(source.path) {
		return fail("没有对应真实聚合来源")
	}
	count := 0
	for _, physical := range plan.physicalOutputs[row] {
		if physical.edge.ItemID == route.edge.ItemID {
			count++
			if physical != source {
				return fail("仍有其它独立物理去向")
			}
		}
	}
	belts := row.line.outputForItem(route.edge.ItemID)
	if count != 1 || len(belts) == 0 || belts[0] != source.start || source.path[0] != source.start || !sameDenseMaterialNodes(route.path[:len(source.path)], source.path) {
		return fail("没有从实际独立出货带进入共享消费轨")
	}
	for i := len(belts) - 1; i > 0; i-- {
		if belts[i].output != belts[i-1] || belts[i].record.OutputToSlot != 1 {
			return fail("机器出货带内部断开")
		}
	}
	machines, members := map[*placed]bool{}, map[*placed]bool{}
	for _, machine := range row.line.machines {
		machines[machine] = true
	}
	for _, belt := range belts {
		members[belt] = true
	}
	feeds := 0
	for _, entity := range row.line.all {
		if machines[entity.input] && members[entity.output] {
			feeds++
			if len(row.recipe.Results) > 1 && int(entity.record.FilterID) != route.edge.ItemID {
				return fail("多产物机器端没有明确过滤")
			}
		}
	}
	if feeds == 0 {
		return fail("没有真实机器出货臂")
	}
	return true, nil
}
