package build

// 文件职责：验证配送端点的完整本地有向链，并按真实分拣器实体披露机械吞吐缺口。
// 使用：mode2/5 绑定规范流边之前；边界：不假造远程机队连通，不把已知欠能力当成已达产；几何仍由最终规则严格核验。

import (
	"fmt"
	"math"
	"strings"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// flyLocalGraph 职责：缓存本次装配的真实有向引用与成员集合；使用：端点到机器的局部追踪；边界：不从坐标相同推导连线，也不穿过其它机器或配送设施。
type flyLocalGraph struct {
	// members 职责：限定已提交建筑；使用：拒绝幽灵引用；边界：必须属于当前 builder。
	members map[*placed]bool
	// forward/reverse 职责：分别保存源到目的及反向引用；使用：输入向机器、输出逆向查机器；边界：仅由实体 input/output 建立。
	forward, reverse map[*placed][]*placed
	// sprayInputs 职责：登记可抵达同产线喷涂附加通道的供剂入口；使用：识别没有机器Input引用的库存通道；边界：必须来自coaterLinks并沿真实带链抵达，普通同坐标带不算入口。
	sprayInputs map[int]map[*placed]bool
	// filteredMachines 职责：标记已使用逐物品出货过滤的机器；使用：防止其它产物或移除过滤的机器臂被借作当前端点来源；边界：旧单产物全部无过滤臂仍兼容。
	filteredMachines map[*placed]bool
}

// newFlyLocalGraph 职责：一次性收集局部图，避免每端点扫描整座工厂；使用：完整性入口；边界：取消、重复实体和蓝图外引用必须失败。
func (b *builder) newFlyLocalGraph() (*flyLocalGraph, error) {
	graph := &flyLocalGraph{members: map[*placed]bool{}, forward: map[*placed][]*placed{}, reverse: map[*placed][]*placed{}, sprayInputs: map[int]map[*placed]bool{}, filteredMachines: map[*placed]bool{}}
	coaters := map[int][]*placed{}
	for _, p := range b.all {
		if err := b.checkGeneration("配送本地图登记"); err != nil {
			return nil, err
		}
		if p == nil || graph.members[p] {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "配送本地图存在空建筑或重复实体"}
		}
		graph.members[p] = true
		if p.record.ItemID == 2313 {
			coaters[p.owner] = append(coaters[p.owner], p)
		}
	}
	add := func(from, to *placed) error {
		if from == nil || to == nil {
			return nil
		}
		if !graph.members[from] || !graph.members[to] || from == to {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "配送本地图引用自身或蓝图外建筑"}
		}
		if !flyLocalEdge(from, to) {
			return nil
		}
		graph.forward[from] = append(graph.forward[from], to)
		graph.reverse[to] = append(graph.reverse[to], from)
		return nil
	}
	for _, p := range b.all {
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 && p.record.FilterID != 0 && p.input != nil {
			if machineKind, _ := quant.MachineKind(int(p.input.record.ItemID)); machineKind > 0 {
				graph.filteredMachines[p.input] = true
			}
		}
		if err := add(p.input, p); err != nil {
			return nil, err
		}
		if err := add(p, p.output); err != nil {
			return nil, err
		}
	}
	// 真实多出口即使全部被错误清空FilterID，也不能退化为旧单产物兼容。
	// 同一条物品带上的并联臂共享同一有向带尾，不会因此被误判为多产物。
	outputEnds, beltEnds := map[*placed]map[*placed]bool{}, map[*placed]*placed{}
	for index, p := range b.all {
		if index%64 == 0 {
			if err := b.checkGeneration("配送机器出货通道核验"); err != nil {
				return nil, err
			}
		}
		if p.record.ItemID < 2011 || p.record.ItemID > 2014 || p.input == nil || p.output == nil || !surfaceIsBelt(p.output.record.ItemID) {
			continue
		}
		if machineKind, _ := quant.MachineKind(int(p.input.record.ItemID)); machineKind <= 0 {
			continue
		}
		if outputEnds[p.input] == nil {
			outputEnds[p.input] = map[*placed]bool{}
		}
		end := flyDirectedBeltEnd(p.output, beltEnds)
		if end != nil {
			outputEnds[p.input][end] = true
		}
		if len(outputEnds[p.input]) > 1 {
			graph.filteredMachines[p.input] = true
		}
	}
	for item := 1141; item <= 1143; item++ {
		entries := b.coaterLinks[item]
		graph.sprayInputs[item] = map[*placed]bool{}
		for _, entry := range entries {
			if !graph.members[entry] || len(entry.record.Parameters) == 0 || entry.record.Parameters[0] != int32(item) {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "配送喷涂供剂入口没有登记真实带或物品标记"}
			}
			seen := map[*placed]bool{}
			for p := entry; p != nil && surfaceIsBelt(p.record.ItemID) && p.owner == entry.owner && !seen[p]; p = p.output {
				if err := b.checkGeneration("配送喷涂通道追踪"); err != nil {
					return nil, err
				}
				seen[p] = true
				for _, coater := range coaters[entry.owner] {
					if flySamePoint(p.a, vector{coater.a.x - 1, coater.a.y, coater.a.z + 1}) {
						graph.sprayInputs[item][entry] = true
					}
				}
				if p.record.OutputToSlot != 1 {
					break
				}
			}
		}
	}
	return graph, nil
}

// flyDirectedBeltEnd 职责：缓存一条真实有向物料带的末端身份；使用：区分同物品并联臂与机器多产物口；边界：只沿正确带槽引用，循环或错槽不据坐标补链。
func flyDirectedBeltEnd(start *placed, cache map[*placed]*placed) *placed {
	path, seen := []*placed{}, map[*placed]bool{}
	current, end := start, (*placed)(nil)
	for current != nil && surfaceIsBelt(current.record.ItemID) && !seen[current] {
		if cached, ok := cache[current]; ok {
			end = cached
			break
		}
		seen[current] = true
		path = append(path, current)
		if current.output == nil || !surfaceIsBelt(current.output.record.ItemID) {
			end = current
			break
		}
		if current.record.OutputToSlot != 1 {
			break
		}
		current = current.output
	}
	for _, belt := range path {
		cache[belt] = end
	}
	return end
}

// flyLocalEdge 职责：只把带链和分拣器真实方向槽位加入机器侧图；使用：有向引用收集；边界：箱体、配送器和分流器不能成为绕行通道，直接把带引用到机器也不算合法分拣器连接。
func flyLocalEdge(from, to *placed) bool {
	fromBelt, toBelt := surfaceIsBelt(from.record.ItemID), surfaceIsBelt(to.record.ItemID)
	fromArm, toArm := from.record.ItemID >= 2011 && from.record.ItemID <= 2014, to.record.ItemID >= 2011 && to.record.ItemID <= 2014
	fromMachine, _ := quant.MachineKind(int(from.record.ItemID))
	toMachine, _ := quant.MachineKind(int(to.record.ItemID))
	switch {
	case fromBelt && toBelt:
		return from.output == to && from.record.OutputToSlot == 1
	case fromBelt && toArm:
		return to.input == from && to.record.InputFromSlot == -1
	case fromArm && toBelt:
		return from.output == to && from.record.OutputToSlot == -1
	case fromMachine > 0 && toArm:
		return to.input == from && to.record.InputFromSlot >= 0
	case fromArm && toMachine > 0:
		return from.output == to && from.record.OutputToSlot >= 0
	default:
		return false
	}
}

// flyEndpointFailure 职责：为本地链失败补齐产线和物品上下文；使用：完整性校验；边界：始终硬失败，不发布部分路线。
func flyEndpointFailure(endpoint *flyTransportEndpoint, reason string) error {
	return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 配送本地链不完整：%s", endpoint.owner, endpoint.item, reason)}
}

// flyEndpointArms 职责：合并旧首臂引用与并联实体列表；使用：方向和容量校验；边界：首臂兼容重复仅计一次，列表内部的重复或空臂拒绝。
func flyEndpointArms(endpoint *flyTransportEndpoint) ([]*placed, error) {
	arms := []*placed{}
	seen := map[*placed]bool{}
	for _, arm := range endpoint.arms {
		if arm == nil || seen[arm] {
			return nil, flyEndpointFailure(endpoint, "箱体并联分拣器为空或重复登记")
		}
		seen[arm] = true
		arms = append(arms, arm)
	}
	if endpoint.arm != nil && !seen[endpoint.arm] {
		arms = append(arms, endpoint.arm)
	}
	return arms, nil
}

// validateFlyLocalIntegrity 职责：逐节点核对带链、设施槽位和机器接入；使用：规范边绑定前；边界：远程配送不参与本地可达性证明，参数容量不足另作明确告警。
func (b *builder) validateFlyLocalIntegrity(mode int) error {
	if mode != 2 && mode != 5 {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: "配送本地链只支持模式2/5"}
	}
	graph, err := b.newFlyLocalGraph()
	if err != nil {
		return err
	}
	claimed := map[*placed]*flyTransportEndpoint{}
	for _, endpoint := range b.flyEndpoints {
		if err := b.checkGeneration("配送本地链核验"); err != nil {
			return err
		}
		if endpoint == nil || endpoint.delivery == nil || endpoint.box == nil || endpoint.link == nil || len(endpoint.belts) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "配送端点缺少箱体、配送器或实体接驳带"}
		}
		for _, entity := range appendFlyEndpointEntities(nil, endpoint) {
			if !graph.members[entity] || entity.owner != endpoint.owner {
				return flyEndpointFailure(endpoint, "登记实体不属于本蓝图或对应产线")
			}
			if claimed[entity] != nil {
				return flyEndpointFailure(endpoint, "不同配送端点重复占用同一接驳实体")
			}
			claimed[entity] = endpoint
		}
		if !surfaceIsBelt(endpoint.link.record.ItemID) {
			return flyEndpointFailure(endpoint, "机器带端不是独立的真实传送带")
		}
		role := int32(1)
		if endpoint.input {
			role = 2
		}
		if endpoint.delivery.record.ItemID != 2107 || endpoint.box.record.ItemID != 2101 || endpoint.delivery.input != endpoint.box || endpoint.delivery.record.InputFromSlot != 13 || endpoint.delivery.record.InputToSlot != 0 || int(endpoint.delivery.record.FilterID) != endpoint.item || len(endpoint.delivery.record.Parameters) < 2 || endpoint.delivery.record.Parameters[0] != 2 || endpoint.delivery.record.Parameters[1] != role {
			return flyEndpointFailure(endpoint, "配送器箱体连接、物品槽或供需角色错误")
		}
		if !flySamePoint(endpoint.delivery.a, vector{endpoint.box.a.x, endpoint.box.a.y, endpoint.box.a.z + 1.875}) {
			return flyEndpointFailure(endpoint, "配送器没有放置在所引用箱体的真实叠放端口")
		}
		belts := map[*placed]bool{}
		for i, belt := range endpoint.belts {
			if belt == nil || belt == endpoint.link || !surfaceIsBelt(belt.record.ItemID) || belts[belt] {
				return flyEndpointFailure(endpoint, "接驳带包含空节点、非带实体或重复节点")
			}
			belts[belt] = true
			previous := endpoint.link
			if i > 0 {
				previous = endpoint.belts[i-1]
			}
			if endpoint.input && (belt.output != previous || belt.record.OutputToSlot != 1) || !endpoint.input && (previous.output != belt || previous.record.OutputToSlot != 1) {
				return flyEndpointFailure(endpoint, fmt.Sprintf("第%d段接驳带断开或方向错误", i+1))
			}
		}
		if err := b.validateFlyFacilityLink(endpoint, mode, belts); err != nil {
			return err
		}
		seenFilters := map[*placed]bool{}
		for _, filter := range endpoint.filters {
			if endpoint.input || filter == nil || seenFilters[filter] || int(filter.record.FilterID) != endpoint.item || filter.input == nil || filter.output == nil || filter.input.owner != endpoint.owner || filter.output.owner != endpoint.owner || !surfaceIsBelt(filter.input.record.ItemID) || !surfaceIsBelt(filter.output.record.ItemID) || !flyBeltReaches(filter.output, endpoint.link) {
				return flyEndpointFailure(endpoint, "输出过滤臂未通过真实有向支路接入机器带端")
			}
			seenFilters[filter] = true
			if filter.record.InputFromSlot != -1 || filter.record.OutputToSlot != -1 || !flySamePoint(filter.a, filter.input.a) || !flySamePoint(filter.b, filter.output.a) {
				return flyEndpointFailure(endpoint, "输出过滤臂两端没有引用所属传送带的真实槽位")
			}
			if err := flyInserterSpanMatchesGeometry(filter); err != nil {
				return flyEndpointFailure(endpoint, err.Error())
			}
			if _, err := flyEntityInserterCapacity(filter); err != nil {
				return flyEndpointFailure(endpoint, err.Error())
			}
			if !b.flyLinkReachesMachine(graph, &flyTransportEndpoint{owner: endpoint.owner, item: endpoint.item, link: filter.input}) {
				if err := b.checkGeneration("配送过滤臂来源核验"); err != nil {
					return err
				}
				return flyEndpointFailure(endpoint, "输出过滤臂的取料带没有来自同产线机器的有向链")
			}
		}
		if !endpoint.input {
			if err := b.validateFlyFilterRegistration(graph, endpoint, seenFilters); err != nil {
				return err
			}
		}
		if !b.flyLinkReachesMachine(graph, endpoint) {
			if err := b.checkGeneration("配送机器方向核验"); err != nil {
				return err
			}
			return flyEndpointFailure(endpoint, "机器侧带端没有到同产线设备或喷涂通道的有向链")
		}
	}
	return nil
}

// validateFlyFilterRegistration 职责：逆查过滤合流带的每个真实臂入口，拒绝漏登记的并联臂；使用：多过滤支路完整性；边界：只沿所属产线有向带反查，不穿机器或配送设施，也不凭坐标补链。
func (b *builder) validateFlyFilterRegistration(graph *flyLocalGraph, endpoint *flyTransportEndpoint, filters map[*placed]bool) error {
	queue, seen := []*placed{endpoint.link}, map[*placed]bool{}
	for head := 0; head < len(queue); head++ {
		if err := b.checkGeneration("配送过滤支路登记核验"); err != nil {
			return err
		}
		belt := queue[head]
		if seen[belt] {
			continue
		}
		seen[belt] = true
		for _, source := range graph.reverse[belt] {
			if source.owner != endpoint.owner {
				return flyEndpointFailure(endpoint, "输出过滤合流带旁接其它产线的带或臂")
			}
			if surfaceIsBelt(source.record.ItemID) {
				queue = append(queue, source)
				continue
			}
			machineKind := 0
			if source.input != nil {
				machineKind, _ = quant.MachineKind(int(source.input.record.ItemID))
			}
			if machineKind <= 0 && !filters[source] {
				return flyEndpointFailure(endpoint, "输出过滤合流带存在未登记的真实分拣器入口")
			}
		}
	}
	return nil
}

// validateFlyFacilityLink 职责：核验带链设施侧的箱体臂或分流器槽位；使用：逐端点完整性；边界：没有正确引用时禁止用相同坐标代替接线。
func (b *builder) validateFlyFacilityLink(endpoint *flyTransportEndpoint, mode int, belts map[*placed]bool) error {
	end := endpoint.belts[len(endpoint.belts)-1]
	if mode == 5 {
		splitter := endpoint.splitter
		if splitter == nil || endpoint.arm != nil || len(endpoint.arms) != 0 || endpoint.box.input != splitter || endpoint.box.record.InputFromSlot != 15 || splitter.record.ItemID != 2020 || splitter.record.ModelIndex != 39 || splitter.record.Yaw != 90 || !flySamePoint(endpoint.box.a, vector{splitter.a.x, splitter.a.y, 2}) || math.Abs(splitter.a.z) > 1e-6 || math.Abs(end.a.y-splitter.a.y) > 1e-6 || math.Abs(math.Abs(end.a.x-splitter.a.x)-1) > 1e-6 || math.Abs(end.a.z) > 1e-6 {
			return flyEndpointFailure(endpoint, "分流器堆叠或接驳不在已核对的Model39地面东西口")
		}
		want := int8(0)
		if end.a.x < splitter.a.x {
			want = 2
		}
		if endpoint.input && (end.input != splitter || end.record.InputFromSlot != want) || !endpoint.input && (end.output != splitter || end.record.OutputToSlot != want) {
			return flyEndpointFailure(endpoint, "分流器接驳方向或槽位错误")
		}
		return nil
	}
	arms, err := flyEndpointArms(endpoint)
	if err != nil {
		return err
	}
	if endpoint.splitter != nil || len(arms) == 0 {
		return flyEndpointFailure(endpoint, "箱体分拣器缺失或带尾旁接其它实体")
	}
	if !endpoint.input && end.output != nil {
		registered := false
		for _, arm := range arms {
			registered = registered || end.output == arm && arm.input == end
		}
		if !registered {
			return flyEndpointFailure(endpoint, "箱体接驳带尾旁接未登记的其它实体")
		}
	}
	usedSlots := map[int8]bool{}
	for _, arm := range arms {
		belt, slot, boxPoint, beltPoint := arm.input, arm.record.OutputToSlot, arm.b, arm.a
		if endpoint.input {
			belt, slot, boxPoint, beltPoint = arm.output, arm.record.InputFromSlot, arm.a, arm.b
		}
		if !belts[belt] || endpoint.input && arm.input != endpoint.box || !endpoint.input && arm.output != endpoint.box || usedSlots[slot] || !endpoint.input && int(arm.record.FilterID) != endpoint.item || endpoint.input && arm.record.FilterID != 0 && int(arm.record.FilterID) != endpoint.item {
			return flyEndpointFailure(endpoint, "箱体分拣器方向、过滤或独立槽位错误")
		}
		usedSlots[slot] = true
		point, ok := storageSlotPoint(endpoint.box.a, endpoint.box.record.Yaw, int(slot))
		if !ok || !flySamePoint(point, boxPoint) || !flySamePoint(belt.a, beltPoint) {
			return flyEndpointFailure(endpoint, "箱体分拣器端点未对齐真实槽位或所属带节点")
		}
		if endpoint.input && arm.record.OutputToSlot != -1 || !endpoint.input && arm.record.InputFromSlot != -1 {
			return flyEndpointFailure(endpoint, "箱体分拣器的带侧连接不是带槽位")
		}
		if err := flyInserterSpanMatchesGeometry(arm); err != nil {
			return flyEndpointFailure(endpoint, err.Error())
		}
		if _, err := flyEntityInserterCapacity(arm); err != nil {
			return flyEndpointFailure(endpoint, err.Error())
		}
	}
	return nil
}

// flySamePoint 职责：比较本地装配端点；使用：指针接线已确认后的槽位几何核对；边界：不会依据坐标建立任何连接。
func flySamePoint(a, c vector) bool {
	return math.Abs(a.x-c.x) < 1e-6 && math.Abs(a.y-c.y) < 1e-6 && math.Abs(a.z-c.z) < 1e-6
}

// flyBeltReaches 职责：沿真实output引用验证过滤支路接入；使用：输出过滤臂校验；边界：循环、非带或缺链失败，不穿过其它分拣器或机器。
func flyBeltReaches(start, target *placed) bool {
	seen := map[*placed]bool{}
	for current := start; current != nil && surfaceIsBelt(current.record.ItemID) && !seen[current]; current = current.output {
		if current == target {
			return true
		}
		seen[current] = true
		if current.record.OutputToSlot != 1 {
			return false
		}
	}
	return false
}

// flyLinkReachesMachine 职责：沿正确方向追踪机器侧端点；使用：排除接驳带自成孤岛；边界：遇到任意机器即停止，不借用其它物品设施或远程配送路径绕行。
func (b *builder) flyLinkReachesMachine(graph *flyLocalGraph, endpoint *flyTransportEndpoint) bool {
	adjacency := graph.reverse
	if endpoint.input {
		adjacency = graph.forward
	}
	filters := map[*placed]bool{}
	for _, filter := range endpoint.filters {
		filters[filter] = true
	}
	queue, seen := []*placed{endpoint.link}, map[*placed]bool{}
	for head := 0; head < len(queue); head++ {
		if head%64 == 0 && b.checkGeneration("配送机器方向追踪") != nil {
			return false
		}
		p := queue[head]
		if seen[p] || p.owner != endpoint.owner {
			continue
		}
		seen[p] = true
		machineKind, _ := quant.MachineKind(int(p.record.ItemID))
		if machineKind > 0 && p.record.RecipeID > 0 {
			return true
		}
		if !surfaceIsBelt(p.record.ItemID) && (p.record.ItemID < 2011 || p.record.ItemID > 2014) {
			continue
		}
		if endpoint.input && graph.sprayInputs[endpoint.item][p] {
			return true
		}
		if !endpoint.input && p.record.ItemID >= 2011 && p.record.ItemID <= 2014 && p.input != nil && p.output != nil {
			if surfaceIsBelt(p.input.record.ItemID) && surfaceIsBelt(p.output.record.ItemID) && (!filters[p] || int(p.record.FilterID) != endpoint.item) {
				continue
			}
			if machineKind, _ := quant.MachineKind(int(p.input.record.ItemID)); machineKind > 0 && (p.record.FilterID != 0 || graph.filteredMachines[p.input]) && int(p.record.FilterID) != endpoint.item {
				continue
			}
		}
		queue = append(queue, adjacency[p]...)
	}
	return false
}

// flyEntityInserterCapacity 职责：从实际实体原型和Parameters跨距取得额定能力；使用：本地臂和过滤臂核算；边界：参数未知硬失败，不使用设置等级猜测或隐含科技。
func flyEntityInserterCapacity(arm *placed) (float64, error) {
	if arm == nil || len(arm.record.Parameters) == 0 {
		return 0, fmt.Errorf("分拣器缺少真实跨距参数")
	}
	return transport.InserterCapacityPerMinute(int(arm.record.ItemID)-2010, int(arm.record.Parameters[0]))
}

// flyInserterSpanMatchesGeometry 职责：防止把长臂参数伪写成短跨度来夸大能力；使用：槽位和引用通过后的本地臂核验；边界：仅校验当前轴向1..3格端点，最终球面插槽仍交规则目录修正。
func flyInserterSpanMatchesGeometry(arm *placed) error {
	if arm == nil || len(arm.record.Parameters) == 0 {
		return fmt.Errorf("分拣器缺少真实跨距参数")
	}
	distance := math.Hypot(arm.a.x-arm.b.x, arm.a.y-arm.b.y)
	if math.Abs(arm.a.z-arm.b.z) > 1e-6 || distance < .9 || distance > 3.01 || int32(math.Round(distance)) != arm.record.Parameters[0] {
		return fmt.Errorf("分拣器实际端点跨度 %.3f 与已核对的1..3格跨距参数不一致", distance)
	}
	return nil
}

// flyLocalInserterChecks 职责：逐端点核算已生成臂组并显式披露欠能力；使用：完整拓扑通过后、路线提交前；边界：欠能力允许结构导出但只标unverified，不声称实际产能或机队吞吐已通过。
func (b *builder) flyLocalInserterChecks(mode int) ([]LogisticsCheck, []string, error) {
	checks, warnings := []LogisticsCheck{}, []string{}
	groups, insufficient := 0, false
	for _, endpoint := range b.flyEndpoints {
		if err := b.checkGeneration("配送分拣器能力核算"); err != nil {
			return nil, nil, err
		}
		arms, err := flyEndpointArms(endpoint)
		if err != nil {
			return nil, nil, err
		}
		for i, group := range [][]*placed{arms, endpoint.filters} {
			if mode == 5 && i == 0 || len(group) == 0 {
				continue
			}
			groups++
			capacity := 0.
			seen := map[*placed]bool{}
			counts := [5][4]int{}
			for _, arm := range group {
				if seen[arm] {
					return nil, nil, flyEndpointFailure(endpoint, "分拣器重复计入并行能力")
				}
				seen[arm] = true
				part, err := flyEntityInserterCapacity(arm)
				if err != nil {
					return nil, nil, flyEndpointFailure(endpoint, err.Error())
				}
				capacity += part
				counts[int(arm.record.ItemID)-2010][arm.record.Parameters[0]]++
			}
			if endpoint.rate > capacity+1e-6 {
				insufficient = true
				kind := "箱体接驳"
				if i == 1 {
					kind = "输出过滤"
				}
				details := []string{}
				for level := 1; level <= 4; level++ {
					for reach := 1; reach <= 3; reach++ {
						if counts[level][reach] > 0 {
							details = append(details, fmt.Sprintf("Mk.%d 跨 %d 格 %d 个", level, reach, counts[level][reach]))
						}
					}
				}
				message := fmt.Sprintf("产线 %d 物品 %d 配送%s计划 %.3f/min 超过 %d 个真实分拣器（%s）合计的已验证能力 %.3f/min；保留结构导出，持续吞吐未验证，请增加并行接口、提高分拣器等级或降低工作倍率", endpoint.owner, endpoint.item, kind, endpoint.rate, len(group), strings.Join(details, "，"), capacity)
				checks = append(checks, LogisticsCheck{RuleID: "fly-local-inserter-capacity", Status: "unverified", Message: message})
				warnings = append(warnings, message)
			}
		}
	}
	if groups > 0 && !insufficient {
		checks = append(checks, LogisticsCheck{RuleID: "fly-local-inserter-capacity", Status: "passed", Message: fmt.Sprintf("%d 个配送本地分拣器组按实际原型、跨度和唯一实体计数未超额；不代表机队、库存或实际游戏运行产能", groups)})
	}
	return checks, warnings, nil
}
