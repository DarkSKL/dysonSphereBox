package build

// 文件职责：把单产物机器行的多去向二次过滤拆成独立真实分流附件。
// 使用：私有密铺候选在机器行之后、全局接线之前调用；边界：不处理多产物和任何回流，不猜测分流器型号、端口位置或净空。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseProducerSplittersKey 职责：隔离真实生产出货树候选；使用：候选context显式启用；边界：缺省保留原带间过滤结构。
type denseProducerSplittersKey struct{}

// denseProducerSplitPlan 职责：保存物理源、辅助接线与规范路线对应关系；使用：装配、提交和终检；边界：辅助根线不重复记为规范净流。
type denseProducerSplitPlan struct {
	groups     []*denseProducerSplitGroup
	bySource   map[*transportRoute]*denseProducerSplitLeaf
	wires      []*transportRoute
	committed  bool
	identities map[*transportRoute]denseProducerRouteIdentity
}

// denseProducerRouteIdentity 职责：冻结物理和规范份额的来源、消费者与净流；使用：canonical展开前后复核；边界：不冻结由公共路由生成的空间坐标。
type denseProducerRouteIdentity struct {
	leaf     *denseProducerSplitLeaf
	edge     quant.FlowEdge
	rate     float64
	from, to *transportRow
	role     productionInputRole
}

// denseProducerSplitGroup 职责：保存一个原机器行的真实单产物出货树；使用：根供给与满负荷带宽证明；边界：不改变原机器行、生产设备或原规范边。
type denseProducerSplitGroup struct {
	row                 *transportRow
	item                int
	head                *placed
	root                *denseProducerSplitNode
	feed                *transportRoute
	leaves              []*denseProducerSplitLeaf
	planned, configured float64
	feeds               []*placed
}

// denseProducerSplitNode 职责：保存一个真实分流附件及逐出口后继；使用：有界平衡树和拓扑复核；边界：一个出口只对应一个子节点或原物理叶路线。
type denseProducerSplitNode struct {
	gate        *placed
	assembly    *splitterGateAssembly
	model       int16
	inputSlot   int8
	outputSlots []int8
	entities    []*placed
	intake      []*placed
	outlets     [][]*placed
	children    []*denseProducerSplitNode
	links       []*transportRoute
	leaves      []*denseProducerSplitLeaf
}

// denseProducerSplitLeaf 职责：固定物理叶路线的身份与根到叶证据；使用：canonical展开前后重新绑定；边界：rate、edge、from、to和用途不得因分流而改变。
type denseProducerSplitLeaf struct {
	group    *denseProducerSplitGroup
	source   *transportRoute
	edge     quant.FlowEdge
	rate     float64
	from, to *transportRow
	role     productionInputRole
	node     *denseProducerSplitNode
	outlet   int
	prefix   []*placed
}

// planDenseProducerSplit 职责：登记可替换二次带间过滤的真实物理去向；使用：attachFilteredOutputs的独立分支；边界：不创建半成品设施，多产物、回流、自供剂和单去向均保留原实现。
func (b *builder) planDenseProducerSplit(row *transportRow) (bool, error) {
	if !b.denseSpatial || b.generationCtx == nil || b.adapter != nil && b.adapter.density != densityDense {
		return false, nil
	}
	enabled, _ := b.generationCtx.Value(denseProducerSplittersKey{}).(bool)
	if !enabled || row == nil || len(row.recipe.Results) != 1 || len(row.outputs) < 2 || len(row.outputs) > 81 || recipeHasMachineRecycle(row.recipe) || len(row.recycles) > 0 || row.selfSpray != nil {
		return false, nil
	}
	item := row.recipe.Results[0]
	if productionSprayItem(row.info) == item {
		return false, nil
	}
	for _, route := range row.outputs {
		if route == nil || route.from != row || route.edge.ItemID != item {
			return false, nil
		}
	}
	capacity := row.info.Capacity
	if capacity == nil || capacity.Basis != quant.ProductionCapacityBasisConfiguredFullLoad || row.info.MachineCount <= 0 {
		return true, transportAllocationError(row.outputs[0].edge, 0, "生产分流缺少配置满负荷毛流")
	}
	full := productionItemRate(capacity.GrossOutputs, item) * float64(row.count) / float64(row.info.MachineCount)
	head := row.line.outputForItem(item)
	if len(head) == 0 || !finitePositive(full) || full > b.logistics.BeltCapacityPerMinute+1e-6 {
		return true, transportAllocationError(row.outputs[0].edge, full, "生产分流根带超过所选带宽或缺少真实输出轨")
	}
	if b.denseProducerSplits == nil {
		b.denseProducerSplits = &denseProducerSplitPlan{bySource: map[*transportRoute]*denseProducerSplitLeaf{}}
	}
	group := &denseProducerSplitGroup{row: row, item: item, head: head[0], configured: full}
	band, machines := map[*placed]bool{}, map[*placed]bool{}
	for _, p := range head {
		band[p] = true
	}
	for _, p := range row.line.machines {
		machines[p] = true
	}
	for _, p := range row.line.all {
		if machines[p.input] && band[p.output] {
			group.feeds = append(group.feeds, p)
		}
	}
	for _, route := range row.outputs {
		if !finitePositive(route.rate) || b.denseProducerSplits.bySource[route] != nil {
			return true, transportAllocationError(route.edge, route.rate, "生产分流物理份额无效或被重复登记")
		}
		leaf := &denseProducerSplitLeaf{group: group, source: route, edge: route.edge, rate: route.rate, from: route.from, to: route.to, role: route.inputRole}
		group.leaves = append(group.leaves, leaf)
		group.planned += route.rate
		b.denseProducerSplits.bySource[route] = leaf
	}
	want := productionItemRate(row.info.Outputs, item) * float64(row.count) / float64(row.info.MachineCount)
	if len(group.feeds) == 0 || math.Abs(group.planned-want) > math.Max(1e-6, want*1e-8) || group.planned > full+1e-6 {
		return true, transportAllocationError(row.outputs[0].edge, group.planned, "生产分流缺少机器出货臂或份额不等于原行净出货")
	}
	b.denseProducerSplits.groups = append(b.denseProducerSplits.groups, group)
	return true, nil
}

// prepareDenseProducerSplitters 职责：把已登记物理去向建立为独立附件单元和真实根线；使用：共享消费轨准备后、全场落位之前；边界：gate几何仅由三模式共享helper提供，失败拒绝整个候选。
func (b *builder) prepareDenseProducerSplitters(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	if b.denseProducerSplits == nil {
		return units, nil
	}
	owner := b.owner
	b.owner = -1
	defer func() { b.owner = owner }()
	for i, group := range b.denseProducerSplits.groups {
		if b.denseProducerSplits.identities == nil {
			b.denseProducerSplits.identities = map[*transportRoute]denseProducerRouteIdentity{}
		}
		for _, leaf := range group.leaves {
			// 消费单元已完整装配；先给独立门的空间评分真实目标，后续塔口/共享出口仍可重绑end。
			if leaf.source.to != nil && leaf.source.end == nil {
				leaf.source.end = leaf.source.to.inputEndpoint(leaf.source.edge.ItemID, leaf.source.inputRole)
			}
			leaf.to = leaf.source.to
			b.rememberDenseProducerRoute(leaf.source, leaf)
			if plan := b.denseMaterialNetworks; plan != nil {
				for _, route := range plan.members[leaf.source] {
					b.rememberDenseProducerRoute(route, leaf)
				}
			}
		}
		root, next, err := b.buildDenseProducerSplitNode(ctx, group, group.leaves, units, i, 0)
		if err != nil {
			return nil, err
		}
		units = next
		group.root = root
		group.feed = b.denseProducerWire(group, group.head, root.intake[0], group.planned, fmt.Sprintf("producer-root:%d", i))
		b.denseProducerSplits.wires = append(b.denseProducerSplits.wires, group.feed)
	}
	return units, nil
}

// rememberDenseProducerRoute 职责：保存一份原规范或聚合物理路线身份；使用：几何准备完成、全局接线之前；边界：只登记本树拥有的来源，不修改规范图。
func (b *builder) rememberDenseProducerRoute(route *transportRoute, leaf *denseProducerSplitLeaf) {
	b.denseProducerSplits.identities[route] = denseProducerRouteIdentity{leaf: leaf, edge: route.edge, rate: route.rate, from: route.from, to: route.to, role: route.inputRole}
}

// denseProducerWire 职责：建立待公共路由器完成的真实辅助线；使用：机器到根及树间连接；边界：只用代理表示辅助入口，不替换规范叶路线原来源或消费者。
func (b *builder) denseProducerWire(group *denseProducerSplitGroup, start, end *placed, rate float64, id string) *transportRoute {
	proxy := &transportRow{owner: -1, inputs: map[int]*placed{group.item: end}}
	return &transportRoute{edge: quant.FlowEdge{ID: id, ItemID: group.item, FlowKind: "internal"}, rate: rate, from: group.row, to: proxy, start: start, end: end, path: []*placed{start}}
}

// buildDenseProducerSplitNode 职责：以至多三出口递归构造平衡物理树；使用：二三路一门及更多有限去向；边界：深度最多四、每门独立落位，不把长供给线绑回机器单元。
func (b *builder) buildDenseProducerSplitNode(ctx context.Context, group *denseProducerSplitGroup, leaves []*denseProducerSplitLeaf, units []denseSpatialUnit, groupID, depth int) (*denseProducerSplitNode, []denseSpatialUnit, error) {
	if ctx.Err() != nil {
		return nil, nil, previewContextError(ctx.Err())
	}
	if len(leaves) < 2 || depth >= 4 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "生产分流树超出有界分支预算"}
	}
	count := min(3, len(leaves))
	module, err := b.buildDenseProducerGate(ctx, count)
	if err != nil {
		return nil, nil, err
	}
	if module == nil || module.gate == nil || len(module.outputs) != count || len(module.outputSlots) != count || module.inputSlot < 0 || module.inputSlot > 3 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流几何helper未返回真实设施"}
	}
	registered := map[*placed]bool{}
	for _, entity := range module.entities {
		if entity == nil || registered[entity] {
			return nil, nil, denseProducerSplitFailure("共享门实体为空或重复")
		}
		registered[entity] = true
	}
	if !registered[module.gate] {
		return nil, nil, denseProducerSplitFailure("共享门本体未登记为附件实体")
	}
	outlets := make([][]*placed, count)
	usedSlots := map[int8]bool{module.inputSlot: true}
	for i, slot := range module.outputSlots {
		if slot < 0 || slot > 3 || usedSlots[slot] {
			return nil, nil, denseProducerSplitFailure("共同角色picker返回重复或非法槽位")
		}
		usedSlots[slot] = true
		outlets[i] = module.outputs[slot]
	}
	node := &denseProducerSplitNode{gate: module.gate, assembly: module.gate.splitterGate, model: module.gate.record.ModelIndex, inputSlot: module.inputSlot, outputSlots: append([]int8{}, module.outputSlots...), entities: append([]*placed{}, module.entities...), intake: append([]*placed{}, module.input...), outlets: outlets, children: make([]*denseProducerSplitNode, count), links: make([]*transportRoute, count), leaves: make([]*denseProducerSplitLeaf, count)}
	if len(node.intake) < 2 || len(node.outlets) != count {
		return nil, nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流附件缺少约定的真实端口"}
	}
	ports := []denseStreetPort{{item: group.item, input: true, head: node.intake[0], inner: node.intake[1]}}
	for _, outlet := range node.outlets {
		if len(outlet) < 2 {
			return nil, nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流附件出口没有完整带头"}
		}
		ports = append(ports, denseStreetPort{item: group.item, head: outlet[len(outlet)-1], inner: outlet[len(outlet)-2]})
	}
	for _, path := range append([][]*placed{node.intake}, node.outlets...) {
		for _, entity := range path {
			if entity == nil || !registered[entity] {
				return nil, nil, denseProducerSplitFailure("共享门接带未登记为真实附件实体")
			}
		}
	}
	units = append(units, denseSpatialUnit{entities: node.entities, ports: ports})
	sizes := denseProducerSplitSizes(len(leaves))
	at := 0
	for i := 0; i < count; i++ {
		n := sizes[i]
		part := leaves[at : at+n]
		at += n
		outlet := node.outlets[i]
		if n == 1 {
			leaf := part[0]
			node.leaves[i] = leaf
			leaf.node, leaf.outlet = node, i
			source := leaf.source
			source.start = outlet[len(outlet)-1]
			source.path = append([]*placed{}, outlet...)
			source.filters = nil
			if source.to == nil {
				source.boundary = source.start
			}
			continue
		}
		child, next, err := b.buildDenseProducerSplitNode(ctx, group, part, units, groupID, depth+1)
		if err != nil {
			return nil, nil, err
		}
		units = next
		node.children[i] = child
		rate := 0.
		for _, leaf := range part {
			rate += leaf.rate
		}
		wire := b.denseProducerWire(group, outlet[len(outlet)-1], child.intake[0], rate, fmt.Sprintf("producer-join:%d:%d:%d", groupID, len(b.denseProducerSplits.wires), i))
		node.links[i] = wire
		b.denseProducerSplits.wires = append(b.denseProducerSplits.wires, wire)
	}
	return node, units, nil
}

// denseProducerSplitSizes 职责：按最少门预算平衡连续叶份额；使用：二三叶一门、更多最多三子树；边界：N叶恰用floor(N/2)门且保持原份额顺序，不用平均叶数造成额外二出口门。
func denseProducerSplitSizes(leaves int) []int {
	if leaves <= 3 {
		sizes := make([]int, leaves)
		for i := range sizes {
			sizes[i] = 1
		}
		return sizes
	}
	budget := leaves/2 - 1
	sizes := make([]int, 3)
	for i := range sizes {
		gates := budget / 3
		if i < budget%3 {
			gates++
		}
		sizes[i] = 2*gates + 1
	}
	if leaves%2 == 0 {
		largest := 0
		for i := 1; i < len(sizes); i++ {
			if sizes[i] > sizes[largest] {
				largest = i
			}
		}
		sizes[largest]--
	}
	return sizes
}

// denseProducerSplitRoutes 职责：公开本候选真实辅助接线给空间与公共路由；使用：公共路线汇总；边界：不加入规范净流边。
func (b *builder) denseProducerSplitRoutes() []*transportRoute {
	if b.denseProducerSplits == nil {
		return nil
	}
	routes := []*transportRoute{}
	var appendChildren func(*denseProducerSplitNode)
	appendChildren = func(node *denseProducerSplitNode) {
		if node == nil {
			return
		}
		for i, child := range node.children {
			if child != nil {
				routes = append(routes, node.links[i])
				appendChildren(child)
			}
		}
	}
	for _, group := range b.denseProducerSplits.groups {
		if group.feed != nil {
			routes = append(routes, group.feed)
		}
		appendChildren(group.root)
	}
	return routes
}

// denseProducerLeafFor 职责：在canonical展开前后取得同一物理叶证明；使用：直接出货终检；边界：未登记的多去向不能借此放行。
func (b *builder) denseProducerLeafFor(route *transportRoute) *denseProducerSplitLeaf {
	if b.denseProducerSplits == nil {
		return nil
	}
	if leaf := b.denseProducerSplits.bySource[route]; leaf != nil {
		return leaf
	}
	if plan := b.denseMaterialNetworks; plan != nil {
		if binding, ok := plan.bindings[route]; ok {
			return b.denseProducerSplits.bySource[binding.source]
		}
	}
	return nil
}

// commitDenseProducerSplits 职责：将根线和真实分流树前缀交给原物理份额；使用：公共路由后、消费网络canonical展开前；边界：不复制实体、不重复登记辅助线净流，也不重写原始来源和消费者。
func (b *builder) commitDenseProducerSplits() error {
	plan := b.denseProducerSplits
	if plan == nil || plan.committed {
		return nil
	}
	for _, group := range plan.groups {
		if err := b.checkGeneration("生产分流树提交"); err != nil {
			return err
		}
		if group.root == nil || len(group.root.intake) < 2 || group.feed == nil || len(group.feed.path) < 2 || group.feed.path[0] != group.head || group.feed.path[len(group.feed.path)-1] != group.root.intake[0] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流根线未真实连接"}
		}
		if err := b.commitDenseProducerNode(group, group.root, group.feed.path); err != nil {
			return err
		}
	}
	plan.committed = true
	return nil
}

// commitDenseProducerNode 职责：按真实根线、门输入、出口和子树路线构造唯一公共前缀；使用：递归提交；边界：前缀不包含本叶route.path已有的出口带，避免容量重复计数。
func (b *builder) commitDenseProducerNode(group *denseProducerSplitGroup, node *denseProducerSplitNode, prefix []*placed) error {
	if node == nil || len(node.intake) < 2 || len(node.links) != len(node.outlets) || len(node.children) != len(node.outlets) || len(node.leaves) != len(node.outlets) || len(group.row.line.outputForItem(group.item)) == 0 {
		return denseProducerSplitFailure("提交树节点或原出货轨缺失")
	}
	prefix = append(append(append([]*placed{}, prefix...), node.intake[1:]...), node.gate)
	for i, outlet := range node.outlets {
		if len(outlet) < 2 {
			return denseProducerSplitFailure("提交树出口接带缺失")
		}
		if leaf := node.leaves[i]; leaf != nil {
			leaf.prefix = append([]*placed{}, prefix...)
			leaf.source.sharedPath = append(append([]*placed{}, prefix...), group.row.line.outputForItem(group.item)[1:]...)
			continue
		}
		wire, child := node.links[i], node.children[i]
		if wire == nil || child == nil || len(child.intake) < 2 || len(wire.path) < 2 || wire.path[0] != outlet[len(outlet)-1] || wire.path[len(wire.path)-1] != child.intake[0] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "生产分流子树没有真实逐带连接"}
		}
		next := append(append(append([]*placed{}, prefix...), outlet...), wire.path[1:]...)
		if err := b.commitDenseProducerNode(group, child, next); err != nil {
			return err
		}
	}
	return nil
}

// preserveDenseProducerSplits 职责：保护真实门附件、根线及物理树连接不被普通带尾裁剪；使用：全场排布和终检前裁尾；边界：不保护源机器轨道的原无用长尾。
func (b *builder) preserveDenseProducerSplits(required map[*placed]bool) {
	if b.denseProducerSplits == nil {
		return
	}
	var protect func(*denseProducerSplitNode)
	protect = func(node *denseProducerSplitNode) {
		if node == nil {
			return
		}
		for _, p := range node.entities {
			required[p] = true
		}
		for _, child := range node.children {
			protect(child)
		}
	}
	for _, group := range b.denseProducerSplits.groups {
		required[group.head] = true
		protect(group.root)
	}
	for _, wire := range b.denseProducerSplits.wires {
		required[wire.start], required[wire.end] = true, true
		for _, p := range wire.path {
			required[p] = true
		}
	}
}
