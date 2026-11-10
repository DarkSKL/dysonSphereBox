package build

// 文件职责：从最终存活实体重新证明生产分流树、真实槽位和配置毛流容量。
// 使用：独立候选提交后的公共物流终检；边界：只接管显式登记的单产物树，不把原直连、多产物过滤或回流校验降级。

import (
	"DysonSphereBox/helper/blueprint"
	"fmt"
	"math"
)

// denseProducerSplitFailure 职责：构造独立树拓扑硬失败；使用：统一反例拒绝；边界：不把未完成结构降级为普通直连成功。
func denseProducerSplitFailure(reason string) error {
	return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流树无效：" + reason}
}

// denseProducerPath 职责：证明登记的纯带路径与最终output图逐节点一致；使用：根供给、树间线和叶带；边界：不接受未登记分支、循环或设备跳跃。
func denseProducerPath(path []*placed, from, to *placed, alive map[*placed]bool) error {
	if len(path) == 0 || path[0] != from || path[len(path)-1] != to {
		return denseProducerSplitFailure("接线首尾身份不一致")
	}
	actual, err := denseMaterialDirectedPath(from, to, alive)
	if err != nil || !sameDenseMaterialNodes(actual, path) {
		return denseProducerSplitFailure("接线不等于真实存活的有向带链")
	}
	return nil
}

// denseProducerIdentityMatches 职责：核对原规范或聚合物理路线的稳定身份；使用：canonical展开之后；边界：允许几何变化，不允许来源、消费者、用途、边或流量变化。
func denseProducerIdentityMatches(route *transportRoute, id denseProducerRouteIdentity) bool {
	return route != nil && route.from == id.from && route.to == id.to && route.rate == id.rate && route.inputRole == id.role && route.edge == id.edge
}

// validateDenseProducerSplitOutput 职责：为已登记多去向路线提供完整源端证明；使用：公共directOutput入口，在原两种唯一去向判定之前调用；边界：未登记路线仍走原校验，缺失canonical映射、前缀或实际来源时拒绝。
func (b *builder) validateDenseProducerSplitOutput(route *transportRoute) (bool, error) {
	leaf := b.denseProducerLeafFor(route)
	if leaf == nil {
		return false, nil
	}
	plan := b.denseProducerSplits
	if !plan.committed {
		return true, denseProducerSplitFailure("尚未完成真实树提交")
	}
	id, ok := plan.identities[route]
	if !ok || id.leaf != leaf || !denseProducerIdentityMatches(route, id) {
		return true, denseProducerSplitFailure("规范份额来源、消费者或流量被改变")
	}
	source := leaf.source
	if len(route.filters) != 0 || len(source.filters) != 0 || route.start != source.start || len(source.path) == 0 || len(route.path) < len(source.path) || !sameDenseMaterialNodes(route.path[:len(source.path)], source.path) {
		return true, denseProducerSplitFailure("无过滤叶路线没有保留同一真实物理出货路径")
	}
	if len(route.sharedPath) < len(source.sharedPath) || !sameDenseMaterialNodes(route.sharedPath[:len(source.sharedPath)], source.sharedPath) {
		return true, denseProducerSplitFailure("规范份额缺少真实根供给和分流前缀")
	}
	return true, nil
}

// validateDenseProducerSplits 职责：逐实体复算机器来源、树槽位、规范展开及每段配置毛负载；使用：公共物流出口；边界：辅助线不重复进入规范净流，混入未登记物料、删臂、断链或超带宽均硬失败。
func (b *builder) validateDenseProducerSplits() error {
	plan := b.denseProducerSplits
	if plan == nil {
		return nil
	}
	if !plan.committed || len(plan.groups) == 0 {
		return denseProducerSplitFailure("树未完整提交")
	}
	alive := map[*placed]bool{}
	for _, p := range b.all {
		alive[p] = true
	}
	usedWires := map[*transportRoute]bool{}
	protected := map[*placed]bool{}
	expectedIncoming := map[*placed]map[*placed]bool{}
	gates := map[*placed]*denseProducerSplitNode{}
	loads := map[*placed]float64{}
	sources := map[*transportRoute]bool{}
	allow := func(a, c *placed) {
		if expectedIncoming[c] == nil {
			expectedIncoming[c] = map[*placed]bool{}
		}
		expectedIncoming[c][a] = true
		protected[c] = true
	}
	for _, group := range plan.groups {
		if err := b.checkGeneration("生产分流树终检"); err != nil {
			return err
		}
		if group == nil || group.row == nil || group.row.line == nil || group.root == nil || len(group.root.intake) < 2 || len(group.row.recipe.Results) != 1 || group.row.recipe.Results[0] != group.item || recipeHasMachineRecycle(group.row.recipe) || len(group.row.recycles) > 0 || group.row.selfSpray != nil || productionSprayItem(group.row.info) == group.item {
			return denseProducerSplitFailure("机器来源不再是独立无回流单产物")
		}
		row := group.row
		capacity := row.info.Capacity
		if capacity == nil || row.info.MachineCount <= 0 {
			return denseProducerSplitFailure("缺少原配置毛流")
		}
		configured := productionItemRate(capacity.GrossOutputs, group.item) * float64(row.count) / float64(row.info.MachineCount)
		if !finitePositive(configured) || math.Abs(configured-group.configured) > math.Max(1e-6, configured*1e-9) || configured > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "生产分流配置满负荷根流量超过所选带宽或已被改变"}
		}
		band := row.line.outputForItem(group.item)
		if len(band) == 0 || band[0] != group.head {
			return denseProducerSplitFailure("机器真实输出轨头已被替换")
		}
		for i := len(band) - 1; i > 0; i-- {
			if !alive[band[i]] || band[i].output != band[i-1] || band[i].record.OutputToSlot != 1 {
				return denseProducerSplitFailure("机器出货轨已断开")
			}
			allow(band[i], band[i-1])
		}
		machineSet, bandSet := map[*placed]bool{}, map[*placed]bool{}
		for _, p := range row.line.machines {
			machineSet[p] = true
		}
		for _, p := range band {
			bandSet[p] = true
			protected[p] = true
		}
		if len(group.feeds) == 0 {
			return denseProducerSplitFailure("缺少原机器出货臂")
		}
		for _, arm := range group.feeds {
			if !alive[arm] || !machineSet[arm.input] || !bandSet[arm.output] || int(arm.record.ItemID) != 2010+b.setting.InserterLv {
				return denseProducerSplitFailure("原机器到带分拣器被删除、换接或改变等级")
			}
			allow(arm, arm.output)
		}
		if group.feed == nil || usedWires[group.feed] || group.feed.rate != group.planned {
			return denseProducerSplitFailure("根线份额或身份无效")
		}
		usedWires[group.feed] = true
		if err := denseProducerPath(group.feed.path, group.head, group.root.intake[0], alive); err != nil {
			return err
		}
		for i := 1; i < len(group.feed.path); i++ {
			allow(group.feed.path[i-1], group.feed.path[i])
		}
		total, err := b.validateDenseProducerNode(group, group.root, group.feed.path, alive, usedWires, allow, gates)
		if err != nil {
			return err
		}
		if math.Abs(total-group.planned) > math.Max(1e-6, total*1e-9) {
			return denseProducerSplitFailure("所有真实叶份额之和不等于根净流")
		}
		for _, leaf := range group.leaves {
			if leaf == nil || leaf.source == nil {
				return denseProducerSplitFailure("原物理叶缺失")
			}
			source := leaf.source
			if sources[source] || plan.bySource[source] != leaf || leaf.group != group {
				return denseProducerSplitFailure("物理源重复或没有唯一树归属")
			}
			sources[source] = true
			id, ok := plan.identities[source]
			if !ok || !denseProducerIdentityMatches(source, id) || id.leaf != leaf || source.from != group.row || source.edge.ItemID != group.item || source.rate != leaf.rate || len(source.filters) != 0 {
				return denseProducerSplitFailure("物理叶身份或原始份额被改变")
			}
			outlet := leaf.node.outlets[leaf.outlet]
			if source.start != outlet[len(outlet)-1] || len(source.path) < len(outlet) || !sameDenseMaterialNodes(source.path[:len(outlet)], outlet) {
				return denseProducerSplitFailure("物理叶没有从真实门出口连续出货")
			}
			target := source.end
			if target == nil && source.to == nil {
				target = source.boundary
			}
			if target == nil {
				return denseProducerSplitFailure("物理叶缺少最终真实交付端")
			}
			if err := denseProducerPath(source.path, outlet[0], target, alive); err != nil {
				return err
			}
			for i, p := range source.path {
				protected[p] = true
				if i > 0 {
					allow(source.path[i-1], p)
				}
			}
			expected := append(append([]*placed{}, leaf.prefix...), band[1:]...)
			if !sameDenseMaterialNodes(source.sharedPath, expected) {
				return denseProducerSplitFailure("物理叶丢失或重复登记真实公共前缀")
			}
		}
	}
	if len(sources) != len(plan.bySource) {
		return denseProducerSplitFailure("存在树外未登记的物理叶")
	}
	if len(usedWires) != len(plan.wires) {
		return denseProducerSplitFailure("存在未证明或重复的辅助树接线")
	}
	for _, wire := range plan.wires {
		if !usedWires[wire] {
			return denseProducerSplitFailure("树接线引用未登记来源")
		}
	}
	// 共享消费段允许其它已登记规范来源的精确实体入边；这些来源仍由下方毛流累计及公共物料/逐边终检证明。
	// sharedPath只是前缀实体集合，不据此授权入边；共享消费改从其显式binding.source的真实有向path取证。
	registered := map[*transportRoute]bool{}
	routes := []*transportRoute{}
	for _, canonical := range b.transportRoutes {
		if !registered[canonical] {
			registered[canonical] = true
			routes = append(routes, canonical)
		}
		if networks := b.denseMaterialNetworks; networks != nil {
			if binding, ok := networks.bindings[canonical]; ok && binding.source != nil && !registered[binding.source] {
				registered[binding.source] = true
				routes = append(routes, binding.source)
			}
		}
	}
	type ordinaryBandKey struct {
		row  *transportRow
		item int
	}
	type ordinaryBand struct {
		nodes map[*placed]bool
		full  float64
	}
	ordinaryBands := map[ordinaryBandKey]ordinaryBand{}
	closed := map[*transportRoute]bool{}
	// 合流可能落在另一真实来源的原机器轨头；向上游闭合整条已登记来源，防止外来带改从其未保护的上游混入。
	// 每条路线最多处理一次，新增保护节点才使其它路线可达；没有遍历任意sharedPath或放行未登记T分支。
	for changed := true; changed; {
		changed = false
		if err := b.checkGeneration("共享来源入边闭合"); err != nil {
			return err
		}
		for _, route := range routes {
			if closed[route] || route.from == nil || route.from.line == nil || b.denseProducerLeafFor(route) != nil {
				continue
			}
			row := route.from
			band := row.line.outputForItem(route.edge.ItemID)
			intersects := false
			for _, node := range band {
				intersects = intersects || protected[node]
			}
			for _, node := range route.path {
				intersects = intersects || protected[node]
			}
			if !intersects {
				continue
			}
			members := map[*placed]bool{}
			for _, node := range band {
				members[node] = true
			}
			if len(route.filters) == 0 {
				if err := b.validateDenseProducerSharedDirectSource(route); err != nil {
					return err
				}
			} else {
				// 规范消费tap的上游属于binding.source，由物料网络独立证明；此处只闭合原机器带上真实取料的物理过滤来源。
				original := true
				for _, arm := range route.filters {
					onPath := false
					for _, node := range route.path {
						onPath = onPath || node == arm.output
					}
					original = original && alive[arm] && members[arm.input] && onPath && int(arm.record.FilterID) == route.edge.ItemID && int(arm.record.ItemID) == 2010+b.setting.InserterLv
				}
				if !original {
					continue
				}
			}
			target := route.end
			if target == nil && route.to == nil {
				target = route.boundary
			}
			if len(route.path) == 0 || target == nil {
				return denseProducerSplitFailure("共享来源缺少真实首尾端点")
			}
			if err := denseProducerPath(route.path, route.path[0], target, alive); err != nil {
				return err
			}
			key := ordinaryBandKey{row, route.edge.ItemID}
			if _, ok := ordinaryBands[key]; !ok {
				if row.info.Capacity == nil || row.info.MachineCount <= 0 {
					return denseProducerSplitFailure("共享来源缺少配置毛流")
				}
				full := productionItemRate(row.info.Capacity.GrossOutputs, route.edge.ItemID) * float64(row.count) / float64(row.info.MachineCount)
				if !finitePositive(full) || full > b.logistics.BeltCapacityPerMinute+1e-6 {
					return denseProducerSplitFailure("共享来源原机器带配置毛流超过所选带宽")
				}
				for _, node := range band {
					if !alive[node] || int(node.record.ItemID) != 2000+b.setting.BeltLv {
						return denseProducerSplitFailure("共享来源机器轨缺失或带等级改变")
					}
					protected[node] = true
				}
				for i := 1; i < len(band); i++ {
					if band[i].output != band[i-1] || band[i].record.OutputToSlot != 1 {
						return denseProducerSplitFailure("共享来源机器轨实际入边改变")
					}
					allow(band[i], band[i-1])
				}
				machines := map[*placed]bool{}
				for _, machine := range row.line.machines {
					if !alive[machine] {
						return denseProducerSplitFailure("共享来源机器已删除")
					}
					machines[machine] = true
				}
				feeds := 0
				for _, arm := range row.line.all {
					if machines[arm.input] && members[arm.output] {
						if !alive[arm] || int(arm.record.ItemID) != 2010+b.setting.InserterLv || len(row.recipe.Results) > 1 && int(arm.record.FilterID) != route.edge.ItemID {
							return denseProducerSplitFailure("共享来源出货臂缺失、过滤或等级改变")
						}
						allow(arm, arm.output)
						feeds++
					}
				}
				if feeds == 0 {
					return denseProducerSplitFailure("共享来源没有真实机器出货臂")
				}
				ordinaryBands[key] = ordinaryBand{members, full}
			}
			for i, node := range route.path {
				protected[node] = true
				if i > 0 {
					allow(route.path[i-1], node)
				}
			}
			for _, arm := range route.filters {
				allow(arm, arm.output)
			}
			closed[route] = true
			changed = true
		}
	}
	for _, route := range routes {
		for i := 1; i < len(route.path); i++ {
			if alive[route.path[i-1]] && alive[route.path[i]] && route.path[i-1].output == route.path[i] && protected[route.path[i]] {
				allow(route.path[i-1], route.path[i])
			}
		}
		for _, arm := range route.filters {
			if !alive[arm] || arm.input == nil || arm.output == nil || int(arm.record.FilterID) != route.edge.ItemID {
				continue
			}
			for _, node := range route.path {
				if node == arm.output && protected[node] {
					allow(arm, node)
					break
				}
			}
		}
	}
	// 只为真实受保护带建立负载账；普通来源原机器轨按整行毛流记一次，规范展开不能重复或遗漏其上游负载。
	for node := range protected {
		if surfaceIsBelt(node.record.ItemID) {
			loads[node] = 0
		}
	}
	for _, band := range ordinaryBands {
		for node := range band.nodes {
			loads[node] += band.full
		}
	}
	// 只有登记的输入带能进入受保护附件/根线；普通带不能在门外另造T分支或混入其它来源。
	for _, entity := range b.all {
		if protected[entity.output] && !expectedIncoming[entity.output][entity] && !b.mode1DeliveryAllowsEdge(entity, entity.output) {
			return denseProducerSplitFailure(fmt.Sprintf("建筑%d的未登记output汇入生产附件", entity.record.ItemID))
		}
		if node := gates[entity.input]; node != nil {
			found := false
			for _, outlet := range node.outlets {
				found = found || entity == outlet[0]
			}
			if !found {
				return denseProducerSplitFailure("门上出现未登记的额外输出带")
			}
		}
	}
	// 按原规范份额比例恢复源行配置毛流，再逐唯一实体累计；公共树前缀不能被每个叶子重复按整行满流登记。
	seenCanonical := map[*transportRoute]bool{}
	for _, route := range b.transportRoutes {
		leaf := b.denseProducerLeafFor(route)
		if leaf == nil {
			continue
		}
		id, ok := plan.identities[route]
		if !ok || id.leaf != leaf || !denseProducerIdentityMatches(route, id) || seenCanonical[route] {
			return denseProducerSplitFailure("canonical份额缺失、重复或被改变")
		}
		seenCanonical[route] = true
		if len(route.filters) == 0 {
			if _, err := b.validateDenseProducerSplitOutput(route); err != nil {
				return err
			}
		}
		gross := leaf.group.configured * route.rate / leaf.group.planned
		seen := map[*placed]bool{}
		for _, path := range [][]*placed{route.path, route.sharedPath} {
			for _, p := range path {
				if !alive[p] {
					return denseProducerSplitFailure("规范路线引用已裁掉的树节点")
				}
				if surfaceIsBelt(p.record.ItemID) && !seen[p] {
					loads[p] += gross
					seen[p] = true
				}
			}
		}
	}
	// 同一物理叶可能由共享消费网络展开为多份canonical；这些份额必须恰好恢复原叶净流。
	for source, leaf := range plan.bySource {
		total := 0.
		for route := range seenCanonical {
			if b.denseProducerLeafFor(route) == leaf {
				total += route.rate
			}
		}
		if math.Abs(total-source.rate) > math.Max(1e-6, source.rate*1e-9) {
			return denseProducerSplitFailure("canonical展开没有恰好恢复一个物理叶的原始净流")
		}
	}
	for _, route := range b.transportRoutes {
		if b.denseProducerLeafFor(route) != nil {
			continue
		}
		intersects := false
		for _, path := range [][]*placed{route.path, route.sharedPath} {
			for _, p := range path {
				_, tracked := loads[p]
				intersects = intersects || tracked
			}
		}
		if !intersects {
			continue
		}
		if route.from == nil || route.from.info.Capacity == nil || recipeHasMachineRecycle(route.from.recipe) {
			return denseProducerSplitFailure("树后共享带混入不能独立证明配置毛流的其它来源")
		}
		row := route.from
		planned := productionItemRate(row.info.Outputs, route.edge.ItemID) * float64(row.count) / float64(row.info.MachineCount)
		full := productionItemRate(row.info.Capacity.GrossOutputs, route.edge.ItemID) * float64(row.count) / float64(row.info.MachineCount)
		if !finitePositive(planned) || !finitePositive(full) {
			return denseProducerSplitFailure("其它共享来源缺少配置出货基准")
		}
		seen := map[*placed]bool{}
		for _, path := range [][]*placed{route.path, route.sharedPath} {
			for _, p := range path {
				_, tracked := loads[p]
				_, ownBand := ordinaryBands[ordinaryBandKey{row, route.edge.ItemID}].nodes[p]
				if tracked && !ownBand && !seen[p] {
					loads[p] += full * route.rate / planned
					seen[p] = true
				}
			}
		}
	}
	maximum := 0.
	for _, load := range loads {
		maximum = math.Max(maximum, load)
		if !finitePositive(load) || load > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("生产树实际共享带配置毛流%.6f/min超过所选带宽%.6f/min", load, b.logistics.BeltCapacityPerMinute)}
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-producer-splitters", Status: "passed", Message: fmt.Sprintf("%d个单产物机器行经%d座真实门和%d条辅助线分流；原机器出货臂及全部canonical份额保留，逐门槽位、实际前缀和配置毛带负载重新核对，最高%.3f/min", len(plan.groups), len(gates), len(usedWires), maximum)})
	return nil
}

// validateDenseProducerSharedDirectSource 职责：按提交前物理输出快照证明聚合源唯一性；使用：上游闭合中的原机器轨合流；边界：canonical展开可恢复多份规范边，不能因此误当成多个物理出口；存活带/臂和毛流由调用方逐节点重证。
func (b *builder) validateDenseProducerSharedDirectSource(route *transportRoute) error {
	plan := b.denseMaterialNetworks
	if plan == nil || !plan.committed || len(plan.members[route]) == 0 {
		return b.validateDenseDirectOutput(route)
	}
	row := route.from
	if row == nil || row.line == nil || len(route.filters) != 0 || len(route.path) == 0 {
		return denseProducerSplitFailure("共享物理来源没有真实直出首端")
	}
	count := 0
	for _, source := range plan.physicalOutputs[row] {
		if source.edge.ItemID == route.edge.ItemID {
			count++
			if source != route {
				return denseProducerSplitFailure("共享来源仍有其它独立物理出口")
			}
		}
	}
	band := row.line.outputForItem(route.edge.ItemID)
	if count != 1 || len(band) == 0 || band[0] != route.start || route.path[0] != route.start {
		return denseProducerSplitFailure("共享物理来源没有接到原机器出货轨")
	}
	for _, recycle := range row.recycles {
		if recycle.item == route.edge.ItemID {
			return denseProducerSplitFailure("共享物理来源必须保留回流专用分支")
		}
	}
	if productionSprayItem(row.info) == route.edge.ItemID {
		if err := validateDenseConnectedSelfBranch(row, route.edge.ItemID); err != nil {
			return err
		}
	}
	return nil
}

// validateDenseProducerNode 职责：复算一门的输入/输出槽、全部接带和子树唯一叶；使用：根到叶递归证明；边界：不按坐标或连通分量猜跨门路径，所有普通带跳转仍须有真实output。
func (b *builder) validateDenseProducerNode(group *denseProducerSplitGroup, node *denseProducerSplitNode, incoming []*placed, alive map[*placed]bool, usedWires map[*transportRoute]bool, allow func(*placed, *placed), gates map[*placed]*denseProducerSplitNode) (float64, error) {
	if node == nil || node.gate == nil || gates[node.gate] != nil || !alive[node.gate] || node.gate.record.ItemID != 2020 || node.gate.record.ModelIndex != node.model || node.model < 38 || node.model > 40 || node.gate.input != nil || node.gate.output != nil || node.gate.record.FilterID != 0 || len(node.gate.record.Parameters) != 6 {
		return 0, denseProducerSplitFailure("门模型、身份或单物料角色无效")
	}
	model, err := splitterGateModel(b.generationCtx)
	if err != nil {
		return 0, err
	}
	if model != node.model {
		return 0, denseProducerSplitFailure("门型号不等于当前候选指定模型")
	}
	if node.assembly != nil {
		if err := b.validateSplitterGateAssembly(node.assembly, alive); err != nil {
			return 0, err
		}
		if node.assembly.gate != node.gate || node.assembly.inputSlot != node.inputSlot || len(node.assembly.outputSlots) != len(node.outputSlots) {
			return 0, denseProducerSplitFailure("共同门与树登记的角色不一致")
		}
		for i, slot := range node.outputSlots {
			if node.assembly.outputSlots[i] != slot {
				return 0, denseProducerSplitFailure("共同门输出角色次序改变")
			}
		}
	} else {
		var factory denseProducerGateFactory
		if b.generationCtx != nil {
			factory, _ = b.generationCtx.Value(denseProducerGateFactoryKey{}).(denseProducerGateFactory)
		}
		if factory == nil {
			return 0, denseProducerSplitFailure("真实生产门缺少共同接头证明")
		}
	}
	for _, value := range node.gate.record.Parameters {
		if value != 0 {
			return 0, denseProducerSplitFailure("单物料门被写入额外过滤或优先级")
		}
	}
	gates[node.gate] = node
	if node.inputSlot < 0 || node.inputSlot > 3 || len(node.outputSlots) != len(node.outlets) || len(node.intake) < 2 || len(node.outlets) < 2 || len(node.outlets) > 3 || len(node.children) != len(node.outlets) || len(node.links) != len(node.outlets) || len(node.leaves) != len(node.outlets) || len(incoming) == 0 || incoming[len(incoming)-1] != node.intake[0] {
		return 0, denseProducerSplitFailure("门的端口/子树登记不完整")
	}
	members := map[*placed]bool{node.gate: true}
	addTrack := func(path []*placed) error {
		for _, p := range path {
			if !alive[p] || members[p] || int(p.record.ItemID) != 2000+b.setting.BeltLv {
				return denseProducerSplitFailure("门接带缺失、重用或带等级变化")
			}
			members[p] = true
		}
		for i := 1; i < len(path); i++ {
			if path[i-1].output != path[i] || path[i-1].record.OutputToSlot != 1 {
				return denseProducerSplitFailure("门接带内部output中断")
			}
			allow(path[i-1], path[i])
		}
		return nil
	}
	if err := addTrack(node.intake); err != nil {
		return 0, err
	}
	last := node.intake[len(node.intake)-1]
	if last.output != node.gate || last.record.OutputToSlot != node.inputSlot {
		return 0, denseProducerSplitFailure("根/子门输入未接到共同角色picker指定真实槽")
	}
	allow(last, node.gate)
	prefix := append(append(append([]*placed{}, incoming...), node.intake[1:]...), node.gate)
	slots := map[int8]bool{node.inputSlot: true}
	total := 0.
	for i, outlet := range node.outlets {
		if len(outlet) < 2 {
			return 0, denseProducerSplitFailure("门输出缺少真实带头")
		}
		if err := addTrack(outlet); err != nil {
			return 0, err
		}
		allow(node.gate, outlet[0])
		slot := outlet[0].record.InputFromSlot
		if slot < 0 || slot > 3 || slots[slot] || outlet[0].input != node.gate || slot != node.outputSlots[i] {
			return 0, denseProducerSplitFailure("门出口重复、反向或接到错误槽位")
		}
		slots[slot] = true
		if leaf := node.leaves[i]; leaf != nil {
			if node.children[i] != nil || node.links[i] != nil || leaf.group != group || leaf.node != node || leaf.outlet != i || b.denseProducerSplits.bySource[leaf.source] != leaf || !sameDenseMaterialNodes(leaf.prefix, prefix) {
				return 0, denseProducerSplitFailure("门叶去向或公共前缀被改变")
			}
			total += leaf.rate
			continue
		}
		child, wire := node.children[i], node.links[i]
		if child == nil || len(child.intake) < 2 || wire == nil || usedWires[wire] {
			return 0, denseProducerSplitFailure("树出口没有唯一子树接线")
		}
		usedWires[wire] = true
		if err := denseProducerPath(wire.path, outlet[len(outlet)-1], child.intake[0], alive); err != nil {
			return 0, err
		}
		for j := 1; j < len(wire.path); j++ {
			allow(wire.path[j-1], wire.path[j])
		}
		next := append(append(append([]*placed{}, prefix...), outlet...), wire.path[1:]...)
		part, err := b.validateDenseProducerNode(group, child, next, alive, usedWires, allow, gates)
		if err != nil {
			return 0, err
		}
		if !finitePositive(wire.rate) || math.Abs(wire.rate-part) > math.Max(1e-6, part*1e-9) {
			return 0, denseProducerSplitFailure("子树辅助线份额不等于其所有真实叶")
		}
		total += part
	}
	if len(members) != len(node.entities) {
		return 0, denseProducerSplitFailure("门模块存在未登记或遗漏实体")
	}
	seen := map[*placed]bool{}
	for _, p := range node.entities {
		if !members[p] || seen[p] {
			return 0, denseProducerSplitFailure("门实体集合重复或被替换")
		}
		seen[p] = true
	}
	return total, nil
}
