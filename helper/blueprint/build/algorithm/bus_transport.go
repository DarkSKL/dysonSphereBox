package build

// 文件职责：把 mode6 混带总线的实体带、过滤分拣器和外部端点绑定到规范 FlowGraph。
// 使用：finish 收尾通过 recordFacilityTransport 调用；验证完成后写入统一 LogisticsSummary。
// 边界：不模拟游戏内逐帧物流或分拣器持续吞吐；无法区分物品、端点或共享带容量时硬失败。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// busLinkUse 职责：保存一条规范边在一个总线实体路线上的精确计划份额；使用：合流/分流 lane 拆分；边界：份额之和不得超过 route.rate。
type busLinkUse struct {
	link *busTransportLink
	rate float64
}

// validateBusTransport 职责：逐规范边核对 mode6 总线端点、混带层、带容量和外部边界；使用：recordFacilityTransport；边界：共享环路通过数值与指针证据核验，不宣称游戏运行时供料。
func (b *builder) validateBusTransport() error {
	if b.busFabric != nil {
		return b.validateBusFabric()
	}
	if b.flowGraph == nil || b.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "混带总线诊断缺少规范流图"}
	}
	if len(b.flowGraph.Edges) == 0 {
		return nil
	}
	if len(b.busLinks) == 0 {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "模式 6 没有登记任何总线实体路线，无法覆盖规范流边"}
	}
	indices := make(map[*placed]int, len(b.all))
	for i, p := range b.all {
		indices[p] = i
	}
	rings := make(map[int]map[*placed]bool, len(b.busLayerBelts))
	for layer, belts := range b.busLayerBelts {
		set := make(map[*placed]bool, len(belts))
		for _, belt := range belts {
			if belt != nil {
				set[belt] = true
			}
		}
		rings[layer] = set
		if len(belts) < 4 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("混带第 %d 圈实体带不足以形成闭环", layer+1)}
		}
		for i, belt := range belts {
			if belt == nil || belt.output != belts[(i+1)%len(belts)] {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("混带第 %d 圈在带索引 %d 处未形成有向闭环", layer+1, i)}
			}
		}
	}

	productionNodes := map[int]quant.FlowNode{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			productionNodes[node.ProductionIndex] = node
		}
	}
	if len(productionNodes) == 0 {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "混带规范流图没有生产节点"}
	}

	inputByOwnerItem := map[busFlowKey][]*busTransportLink{}
	returnByOwnerItem := map[busFlowKey][]*busTransportLink{}
	boundaryByOwnerItem := map[busBoundaryKey][]*busTransportLink{}
	rawByItem := map[int][]*busTransportLink{}
	remaining := map[*busTransportLink]map[int]float64{}
	layerLoads := map[int]float64{}
	for i := range b.busLinks {
		link := &b.busLinks[i]
		if err := validateBusLink(link, indices, rings); err != nil {
			return err
		}
		remaining[link] = map[int]float64{}
		switch link.kind {
		case "production-input":
			remaining[link][link.item] = link.rate
			inputByOwnerItem[busFlowKey{owner: link.owner, item: link.item}] = append(inputByOwnerItem[busFlowKey{owner: link.owner, item: link.item}], link)
		case "production-return":
			if len(link.itemRates) == 0 {
				return busLinkError(link, "生产回流没有按物品登记计划流量")
			}
			for _, itemRate := range link.itemRates {
				if itemRate.ItemID <= 0 || itemRate.RatePerMinute <= 0 {
					return busLinkError(link, "生产回流包含无效物品流量")
				}
				returnByOwnerItem[busFlowKey{owner: link.owner, item: itemRate.ItemID}] = append(returnByOwnerItem[busFlowKey{owner: link.owner, item: itemRate.ItemID}], link)
				remaining[link][itemRate.ItemID] = itemRate.RatePerMinute
			}
		case "raw-input":
			remaining[link][link.item] = link.rate
			rawByItem[link.item] = append(rawByItem[link.item], link)
		case "target-output", "leftover":
			remaining[link][link.item] = link.rate
			key := busBoundaryKey{owner: link.owner, item: link.item, kind: link.kind}
			boundaryByOwnerItem[key] = append(boundaryByOwnerItem[key], link)
		default:
			return busLinkError(link, "未知总线实体路线类型")
		}
	}

	b.logistics.Lanes = []TransportLane{}
	b.logistics.ExternalPorts = []BoundaryPort{}
	portIndex := map[string]int{}
	for _, edge := range b.flowGraph.Edges {
		if edge.FlowKind == "internal" || edge.FlowKind == "external-input" {
			layer, ok := b.busLayers[edge.ItemID]
			if !ok {
				return busEdgeError(edge, "规范边物品没有总线层映射")
			}
			layerLoads[layer] += edge.RatePerMinute
		}
		var sources, targets []*busTransportLink
		sourceNode := findFlowNode(b.flowGraph.Nodes, edge.SourceNodeID)
		targetNode := findFlowNode(b.flowGraph.Nodes, edge.TargetNodeID)
		if sourceNode == nil || targetNode == nil {
			return busEdgeError(edge, "规范边引用不存在的节点")
		}
		if sourceNode.Kind == "production" {
			if targetNode.Kind == "production" {
				sources = returnByOwnerItem[busFlowKey{owner: sourceNode.ProductionIndex, item: edge.ItemID}]
			} else {
				sources = boundaryByOwnerItem[busBoundaryKey{owner: sourceNode.ProductionIndex, item: edge.ItemID, kind: edge.FlowKind}]
			}
		} else {
			sources = rawByItem[edge.ItemID]
		}
		// 一条混带回流路线可承载多个物品，按当前规范边逐项取其声明份额。
		if targetNode.Kind == "production" {
			targets = inputByOwnerItem[busFlowKey{owner: targetNode.ProductionIndex, item: edge.ItemID}]
		}
		if sourceNode.Kind == "production" && targetNode.Kind == "production" && len(sources) == 0 {
			return busEdgeError(edge, "生产回流没有对应的物品总线路线")
		}
		if sourceNode.Kind != "production" && len(sources) == 0 {
			return busEdgeError(edge, "原料边没有明确外部总线入口")
		}
		if targetNode.Kind == "production" && len(targets) == 0 {
			return busEdgeError(edge, "生产输入边没有对应的过滤取料路线")
		}
		usedSources, err := consumeBusLinks(edge, sources, remaining)
		if err != nil {
			return err
		}
		usedTargets := []busLinkUse{}
		if targetNode.Kind == "production" {
			usedTargets, err = consumeBusLinks(edge, targets, remaining)
			if err != nil {
				return err
			}
		}
		if err := appendBusFlowLanes(b.logistics, edge, usedSources, usedTargets, indices, portIndex); err != nil {
			return err
		}
	}
	for layer, load := range layerLoads {
		if load > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("总线第 %d 圈规范流量 %.6f/min 超过单带容量 %.6f/min", layer+1, load, b.logistics.BeltCapacityPerMinute)}
		}
	}
	for link, itemRemaining := range remaining {
		for item, left := range itemRemaining {
			if left > math.Max(1e-6, link.rate*1e-8) {
				return busLinkError(link, fmt.Sprintf("物品 %d 的实体路线没有规范边消费，剩余 %.6f/min", item, left))
			}
		}
	}
	// 每条共用门控臂只按所连实体一次累计所有物品计划流，防止逐 lane 看似合格但实体过载。
	sharedArmLoad := map[*placed]float64{}
	for i := range b.busLinks {
		link := &b.busLinks[i]
		switch link.kind {
		case "production-input", "production-return":
			for _, arm := range link.parts {
				sharedArmLoad[arm] += link.rate
			}
		}
	}
	for arm, load := range sharedArmLoad {
		if err := validateBusInserter(arm, load); err != nil {
			return err
		}
	}
	for i := range b.busLinks {
		link := &b.busLinks[i]
		if len(link.limiterParts) == 0 {
			continue
		}
		capacity := 0.
		for _, arm := range link.limiterParts {
			value, err := busInserterCapacity(arm)
			if err != nil {
				return err
			}
			capacity += value
		}
		if capacity+1e-6 < link.rate {
			return busLinkError(link, fmt.Sprintf("回流门控臂额定能力 %.6f/min 小于计划 %.6f/min", capacity, link.rate))
		}
	}
	for i := range b.busLinks {
		link := &b.busLinks[i]
		if link.kind != "raw-input" {
			continue
		}
		feedCapacity, returnCapacity := 0., 0.
		for _, arm := range link.parts {
			if err := validateBusInserterShape(arm, 0); err != nil {
				return err
			}
			if arm.input == nil || arm.output == nil {
				return busLinkError(link, "原料分拣供给臂缺少端点")
			}
			value, err := busInserterCapacity(arm)
			if err != nil {
				return err
			}
			if rings[link.layer][arm.output] && !rings[link.layer][arm.input] {
				if arm.record.FilterID != 0 {
					return busLinkError(link, "原料外部 feed 臂不应带过滤物品")
				}
				feedCapacity += value
			} else if rings[link.layer][arm.input] && !rings[link.layer][arm.output] {
				if int(arm.record.FilterID) != link.item {
					return busLinkError(link, "原料回收臂过滤物品与入口不匹配")
				}
				returnCapacity += value
			} else {
				return busLinkError(link, "原料分拣臂的方向或目标环路不匹配")
			}
		}
		if feedCapacity+1e-6 < link.rate || returnCapacity+1e-6 < link.rate {
			return busLinkError(link, fmt.Sprintf("原料过滤供给/回收臂额定能力 %.6f/%.6f/min 小于计划 %.6f/min", feedCapacity, returnCapacity, link.rate))
		}
		if !hasPortItem(link.endpoint, link.item) {
			return busLinkError(link, "外部原料端口未标记对应物品")
		}
	}
	b.logistics.Metrics.LaneCount = len(b.logistics.Lanes)
	b.logistics.Metrics.ExternalInputCount = 0
	for _, port := range b.logistics.ExternalPorts {
		if port.Kind == "external-input" {
			b.logistics.Metrics.ExternalInputCount++
		}
	}
	checks := b.logistics.Checks[:0]
	for _, check := range b.logistics.Checks {
		if check.RuleID != "bus-flow-coverage" && check.RuleID != "bus-fleet" {
			checks = append(checks, check)
		}
	}
	b.logistics.Checks = checks
	b.logistics.Checks = append(b.logistics.Checks,
		LogisticsCheck{RuleID: "bus-flow-coverage", Status: "passed", Message: "混带总线规范流边已逐边绑定生产端点、共享环路和过滤取料路线"},
		LogisticsCheck{RuleID: "bus-fleet", Status: "unverified", Message: "混带总线检查不模拟分拣器逐帧吞吐、堵带时序或游戏内持续运行"},
	)
	return nil
}

// busFlowKey 职责：索引生产物品路线；使用：稳定匹配 FlowGraph 生产节点和 item；边界：不按数组顺序猜测节点。
type busFlowKey struct {
	owner int
	item  int
}

// busBoundaryKey 职责：按生产节点、物品和出口语义区分边界路由；使用：目标产出与剩余副产物流绑定；边界：不将同物品的不同用途出口互相代用。
type busBoundaryKey struct {
	owner int
	item  int
	kind  string
}

// validateBusLink 职责：核对单条总线路线引用的建筑、层和速率；使用：总线收尾；边界：缺失端点或跨层引用立即失败。
func validateBusLink(link *busTransportLink, indices map[*placed]int, rings map[int]map[*placed]bool) error {
	if (link.item <= 0 && !(link.kind == "production-return" && link.item == -1)) || link.rate <= 0 || math.IsNaN(link.rate) || math.IsInf(link.rate, 0) {
		return busLinkError(link, "物品或计划流量无效")
	}
	if link.endpoint == nil {
		return busLinkError(link, "缺少生产带或外部边界带")
	}
	if _, ok := indices[link.endpoint]; !ok || !surfaceIsBelt(link.endpoint.record.ItemID) {
		return busLinkError(link, "路线端点不属于有效传送带")
	}
	if link.kind != "target-output" && link.kind != "leftover" {
		if link.ring == nil || !rings[link.layer][link.ring] {
			return busLinkError(link, "路线没有属于指定层的共享环路端点")
		}
		if len(link.ringPath) == 0 || len(link.ringPath) != len(rings[link.layer]) {
			return busLinkError(link, "路线未引用指定层完整共享环路")
		}
		for i, belt := range link.ringPath {
			if belt == nil || !rings[link.layer][belt] || belt.output != link.ringPath[(i+1)%len(link.ringPath)] {
				return busLinkError(link, "路线引用的共享环路未形成有向闭合带链")
			}
		}
	}
	if link.kind == "production-return" {
		if len(link.itemRates) == 0 {
			return busLinkError(link, "生产回流没有按物品登记规范流量")
		}
		flow := 0.
		for _, rate := range link.itemRates {
			if rate.ItemID <= 0 || rate.RatePerMinute <= 0 || math.IsNaN(rate.RatePerMinute) || math.IsInf(rate.RatePerMinute, 0) {
				return busLinkError(link, "生产回流物品计划流量无效")
			}
			flow += rate.RatePerMinute
		}
		if math.Abs(flow-link.rate) > math.Max(1e-6, link.rate*1e-8) {
			return busLinkError(link, "生产回流总量与逐物品计划流量不一致")
		}
	}
	if link.kind == "raw-input" && (len(link.endpoint.record.Parameters) < 2 || int(link.endpoint.record.Parameters[0]) != link.item) {
		return busLinkError(link, "外部原料入口没有可核验的物品标记")
	}
	for _, p := range append(append([]*placed{}, link.parts...), link.limiterParts...) {
		if p == nil {
			return busLinkError(link, "路线包含空建筑指针")
		}
		if _, ok := indices[p]; !ok {
			return busLinkError(link, "路线引用蓝图外建筑")
		}
	}
	for i := 1; i < len(link.row); i++ {
		if link.row[i-1] == nil || link.row[i] == nil || link.row[i-1].output != link.row[i] {
			return busLinkError(link, "生产输出带链中断")
		}
	}
	if link.kind == "raw-input" {
		// raw-input 的 parts 同时包含外部 feed 臂和回收臂，两种方向均须
		// 明确连接到同一层环路，不能只验证原料外部入口的一半。
		for _, arm := range link.parts {
			if err := validateBusInserterShape(arm, 0); err != nil {
				return err
			}
			inputOnRing := rings[link.layer][arm.input]
			outputOnRing := rings[link.layer][arm.output]
			switch {
			case outputOnRing && !inputOnRing && arm.record.FilterID != 0:
				return busLinkError(link, "原料外部 feed 臂不应带过滤物品")
			case inputOnRing && !outputOnRing && int(arm.record.FilterID) != link.item:
				return busLinkError(link, "原料回收臂过滤物品与入口不匹配")
			case !inputOnRing && !outputOnRing:
				return busLinkError(link, "原料分拣臂未连接指定共享环路")
			case inputOnRing && outputOnRing:
				return busLinkError(link, "原料分拣臂两端不能同时位于共享环路")
			}
		}
	}
	if link.kind == "production-input" {
		if len(link.parts) == 0 || link.parts[0].record.FilterID != int16(link.item) || link.parts[0].input != link.ring || link.parts[0].output != link.endpoint {
			return busLinkError(link, "生产输入臂过滤物品或供需方向不匹配")
		}
	}
	if link.kind == "production-return" {
		if len(link.parts) == 0 || link.parts[0].record.FilterID != 0 || link.parts[0].input != link.endpoint || link.parts[0].output != link.ring {
			return busLinkError(link, "生产回流臂混带方向不匹配")
		}
		if !hasOpenGatePath(link) {
			return busLinkError(link, "生产回流过滤门控带链未连到回流臂")
		}
	}
	if link.kind == "target-output" || link.kind == "leftover" {
		if !hasPortItem(link.endpoint, link.item) {
			return busLinkError(link, "目标或剩余出口未标记对应物品")
		}
		if len(link.parts) == 0 {
			return busLinkError(link, "目标或剩余出口缺少逐物品过滤分拣器")
		}
		for _, arm := range link.parts {
			if err := validateBusInserterShape(arm, link.item); err != nil {
				return err
			}
			if int(arm.record.FilterID) != link.item {
				return busLinkError(link, "目标或剩余出口过滤物品不匹配")
			}
		}
	}
	return nil
}

// consumeBusLinks 职责：按实体路线声明的计划份额分配规范边；使用：守恒核对；边界：不把总线空余带宽当成额外流量。
func consumeBusLinks(edge quant.FlowEdge, links []*busTransportLink, remaining map[*busTransportLink]map[int]float64) ([]busLinkUse, error) {
	if len(links) == 0 {
		return nil, busEdgeError(edge, "没有可消费的总线实体路线")
	}
	left := edge.RatePerMinute
	used := []busLinkUse{}
	tolerance := math.Max(1e-6, edge.RatePerMinute*1e-8)
	for _, link := range links {
		available := remaining[link][edge.ItemID]
		if available <= tolerance {
			continue
		}
		part := math.Min(left, available)
		if part > 0 {
			used = append(used, busLinkUse{link: link, rate: part})
			remaining[link][edge.ItemID] = available - part
			left -= part
		}
		if left <= tolerance {
			return used, nil
		}
	}
	return nil, busEdgeError(edge, fmt.Sprintf("总线实体路线流量不足，尚缺 %.6f/min", left))
}

// appendBusFlowLanes 职责：将一条规范边按输入/输出实体路线拆成结果 lane；使用：前端诊断与外部端口统计；边界：每片流量不超过单带容量。
func appendBusFlowLanes(summary *LogisticsSummary, edge quant.FlowEdge, sources, targets []busLinkUse, indices map[*placed]int, portIndex map[string]int) error {
	if len(sources) == 0 {
		return busEdgeError(edge, "规范边没有供给实体路线")
	}
	if edge.TargetNodeID != "" && len(targets) == 0 && edge.FlowKind == "internal" {
		return busEdgeError(edge, "内部边没有需求实体路线")
	}
	si, ti := 0, 0
	var sourceUsed, targetUsed float64
	left := edge.RatePerMinute
	for left > math.Max(1e-6, edge.RatePerMinute*1e-8) {
		for si < len(sources) && sourceUsed >= sources[si].rate-math.Max(1e-6, edge.RatePerMinute*1e-8) {
			si++
			sourceUsed = 0
		}
		if si >= len(sources) {
			return busEdgeError(edge, "供给实体路线拆分不足")
		}
		var source *busLinkUse = &sources[si]
		part := math.Min(left, source.rate-sourceUsed)
		var target *busLinkUse
		if len(targets) > 0 {
			for ti < len(targets) && targetUsed >= targets[ti].rate-math.Max(1e-6, edge.RatePerMinute*1e-8) {
				ti++
				targetUsed = 0
			}
			if ti >= len(targets) {
				return busEdgeError(edge, "需求实体路线拆分不足")
			}
			target = &targets[ti]
			part = math.Min(part, target.rate-targetUsed)
			if source.link.ring != nil && target.link.ring != nil && source.link.layer != target.link.layer {
				return busEdgeError(edge, "同一规范边的总线供需端点分属不同环路")
			}
		}
		if part <= 0 || part > summary.BeltCapacityPerMinute+1e-6 {
			return busEdgeError(edge, "总线 lane 流量无效或超过单带容量")
		}
		links := []*busTransportLink{source.link}
		if target != nil {
			links = append(links, target.link)
		}
		buildingIndices := busLinkBuildingIndices(links, indices)
		ringBuildings, err := busRingRouteIndices(source.link, targetLink(target), indices)
		if err != nil {
			return busEdgeError(edge, err.Error())
		}
		buildingIndices = appendUniqueIndices(buildingIndices, ringBuildings...)
		if len(buildingIndices) == 0 {
			return busEdgeError(edge, "总线 lane 没有关联实体建筑")
		}
		summary.Lanes = append(summary.Lanes, TransportLane{EdgeID: edge.ID, ItemID: edge.ItemID, SourceNodeID: edge.SourceNodeID, TargetNodeID: edge.TargetNodeID, FlowKind: edge.FlowKind, RatePerMinute: part, CapacityPerMinute: summary.BeltCapacityPerMinute, Utilization: part / summary.BeltCapacityPerMinute, BuildingIndices: buildingIndices, InternalConnected: edge.FlowKind == "internal" && target != nil})
		if edge.FlowKind != "internal" {
			portLink := source.link
			if edge.FlowKind == "external-input" && source.link.kind != "raw-input" {
				return busEdgeError(edge, "外部输入边未绑定 raw-input 入口")
			}
			key := fmt.Sprintf("%s:%d:%d", edge.ID, portLink.endpointIndex(indices), portLink.item)
			if index, ok := portIndex[key]; ok {
				summary.ExternalPorts[index].RatePerMinute += part
			} else {
				portIndex[key] = len(summary.ExternalPorts)
				point := portLink.endpoint.a
				summary.ExternalPorts = append(summary.ExternalPorts, BoundaryPort{EdgeID: edge.ID, ItemID: edge.ItemID, Kind: edge.FlowKind, RatePerMinute: part, BuildingIndex: indices[portLink.endpoint], X: point.x, Y: point.y, Z: point.z})
			}
		}
		if source != nil {
			sourceUsed += part
		}
		if target != nil {
			targetUsed += part
		}
		left = math.Max(0, left-part)
	}
	return nil
}

// endpointIndex 职责：为端点生成稳定内部索引；使用：外部端口去重；边界：未知指针返回 -1。
func (link *busTransportLink) endpointIndex(indices map[*placed]int) int {
	if link == nil || link.endpoint == nil {
		return -1
	}
	if index, ok := indices[link.endpoint]; ok {
		return index
	}
	return -1
}

// busLinkBuildingIndices 职责：收集总线实体路线及共享环路节点的最终索引；使用：TransportLane；边界：去重但不改变建筑连接。
func busLinkBuildingIndices(links []*busTransportLink, indices map[*placed]int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, link := range links {
		if link == nil {
			continue
		}
		for _, p := range append(append(append(append(append([]*placed{}, link.row...), link.parts...), link.limiterParts...), link.gateFeed...), link.gateReturn...) {
			if index, ok := indices[p]; ok && !seen[index] {
				seen[index] = true
				out = append(out, index)
			}
		}
		// 分拣器本体的 input/output 端点也是路线证据；特别是逐物品出口
		// 不应因为分支带单独登记而丢掉共享输出干线起点。
		for _, arm := range append(append([]*placed{}, link.parts...), link.limiterParts...) {
			if arm == nil {
				continue
			}
			for _, endpoint := range []*placed{arm.input, arm.output} {
				if index, ok := indices[endpoint]; ok && !seen[index] {
					seen[index] = true
					out = append(out, index)
				}
			}
		}
		for _, p := range []*placed{link.endpoint, link.gateEndpoint, link.ring} {
			if index, ok := indices[p]; ok && !seen[index] {
				seen[index] = true
				out = append(out, index)
			}
		}
	}
	return out
}

// targetLink 职责：将可选实体路由份额转回其声明路线；使用：lane 的共享总线区间；边界：nil 表示目标外部开放端口。
func targetLink(target *busLinkUse) *busTransportLink {
	if target == nil {
		return nil
	}
	return target.link
}

// busRingRouteIndices 职责：返回一条物料边在闭环上的有向最短拓扑段；使用：结果 lane 建筑索引；边界：只列带连接证据，不声称运行时物品时序。
func busRingRouteIndices(source, target *busTransportLink, indices map[*placed]int) ([]int, error) {
	if source == nil || target == nil || source.ring == nil && target.ring == nil {
		return nil, nil
	}
	if source.ring == nil || target.ring == nil || source.layer != target.layer || len(source.ringPath) == 0 || len(source.ringPath) != len(target.ringPath) {
		return nil, fmt.Errorf("规范边的总线端点缺少同层共享环路")
	}
	start, end := -1, -1
	for i, belt := range source.ringPath {
		if belt == source.ring {
			start = i
		}
		if belt == target.ring {
			end = i
		}
	}
	if start < 0 || end < 0 {
		return nil, fmt.Errorf("规范边的总线端点不在已验证闭环中")
	}
	out := []int{}
	seen := map[int]bool{}
	for i, count := start, 0; count <= len(source.ringPath); i, count = (i+1)%len(source.ringPath), count+1 {
		if index, ok := indices[source.ringPath[i]]; ok && !seen[index] {
			out = append(out, index)
			seen[index] = true
		}
		if i == end {
			return out, nil
		}
	}
	return nil, fmt.Errorf("规范边总线段无法抵达需求端")
}

// appendUniqueIndices 职责：合并 lane 建筑索引并保留首次出现顺序；使用：生产带、分拣器和共享环路；边界：不排序以免丢失路径证据次序。
func appendUniqueIndices(indices []int, additions ...int) []int {
	seen := make(map[int]bool, len(indices)+len(additions))
	for _, index := range indices {
		seen[index] = true
	}
	for _, index := range additions {
		if !seen[index] {
			indices = append(indices, index)
			seen[index] = true
		}
	}
	return indices
}

// validateBusInserterShape 职责：确认实体分拣器记录保留了跨度和端点；使用：混带转运核验；边界：缺少参数或端点时不推测默认等级。
func validateBusInserterShape(arm *placed, filter int) error {
	if arm == nil || int(arm.record.ItemID) < 2011 || int(arm.record.ItemID) > 2014 || len(arm.record.Parameters) == 0 || arm.input == nil || arm.output == nil {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "混带路线包含缺少端点或跨度的分拣器"}
	}
	if filter != 0 && int(arm.record.FilterID) != filter {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("分拣器过滤物品 %d 与计划物品 %d 不一致", arm.record.FilterID, filter)}
	}
	distance := int(arm.record.Parameters[0])
	if distance < 1 || distance > 3 {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("分拣器跨度 %d 未经核验", distance)}
	}
	actual := math.Abs(arm.input.a.x-arm.output.a.x) + math.Abs(arm.input.a.y-arm.output.a.y)
	if math.Abs(actual-float64(distance)) > .01 || arm.input.a.z != arm.output.a.z {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "分拣器跨度参数与实体坐标不一致"}
	}
	return nil
}

// busInserterCapacity 职责：读取版本锁定的零科技分拣器额定上限；使用：混带臂端负载验证；边界：配置目录未知时直接失败，不声称游戏机队连续吞吐。
func busInserterCapacity(arm *placed) (float64, error) {
	if err := validateBusInserterShape(arm, 0); err != nil {
		return 0, err
	}
	capacity, err := transport.InserterCapacityPerMinute(int(arm.record.ItemID)-2010, int(arm.record.Parameters[0]))
	if err != nil {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "无法读取混带分拣器额定容量：" + err.Error()}
	}
	return capacity, nil
}

// validateBusInserter 职责：核对单臂计划流量不超过已核对额定上限；使用：生产取料和回流；边界：额定值是上界条件，不等于游戏内实测。
func validateBusInserter(arm *placed, rate float64) error {
	capacity, err := busInserterCapacity(arm)
	if err != nil {
		return err
	}
	if rate > capacity+1e-6 {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("混带分拣器 %d 跨 %d 格额定能力 %.6f/min，小于实体路线计划 %.6f/min", arm.record.ItemID, arm.record.Parameters[0], capacity, rate)}
	}
	return nil
}

// hasPortItem 职责：确认开放带标明对应物品；使用：原料/目标/剩余端口校验；边界：不存在明确 item 参数时返回 false。
func hasPortItem(port *placed, item int) bool {
	return port != nil && len(port.record.Parameters) > 0 && int(port.record.Parameters[0]) == item
}

// hasOpenGatePath 职责：核对回流门控的实际带链和输入/输出方向；使用：共享回流路线；边界：不模拟门控逐帧吞吐或物品选择。
func hasOpenGatePath(link *busTransportLink) bool {
	if link == nil || len(link.gateFeed) < 1 || len(link.gateReturn) < 1 || len(link.limiterParts) == 0 || len(link.row) == 0 {
		return false
	}
	if link.row[len(link.row)-1] == nil || link.row[len(link.row)-1].output != link.gateFeed[0] {
		return false
	}
	for i := 1; i < len(link.gateFeed); i++ {
		if link.gateFeed[i-1] == nil || link.gateFeed[i-1].output != link.gateFeed[i] {
			return false
		}
	}
	for _, arm := range link.limiterParts {
		if validateBusInserterShape(arm, 0) != nil || arm.record.FilterID != 0 || !containsPlaced(link.gateFeed, arm.input) || !containsPlaced(link.gateReturn, arm.output) {
			return false
		}
	}
	for i := 1; i < len(link.gateReturn); i++ {
		if link.gateReturn[i] == nil || link.gateReturn[i].output != link.gateReturn[i-1] {
			return false
		}
	}
	return link.gateReturn[0] != nil && link.gateReturn[0].output == link.gateEndpoint
}

// containsPlaced 职责：按指针检查一条局部路径是否含指定端点；使用：门控方向验证；边界：不按坐标相等代替引用关系。
func containsPlaced(path []*placed, target *placed) bool {
	for _, p := range path {
		if p == target {
			return true
		}
	}
	return false
}

// findFlowNode 职责：按稳定节点 ID 查询图节点；使用：总线边绑定；边界：未找到返回 nil，由调用方硬失败。
func findFlowNode(nodes []quant.FlowNode, id string) *quant.FlowNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

// busLinkError 职责：生成包含总线物品、产线和流量的诊断；使用：硬失败返回；边界：不泄漏无上下文错误。
func busLinkError(link *busTransportLink, reason string) error {
	if link == nil {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "总线实体路线为空：" + reason}
	}
	return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("总线产线 %d 物品 %d %.6f/min：%s", link.owner, link.item, link.rate, reason)}
}

// busEdgeError 职责：生成包含规范边和流量的诊断；使用：边覆盖失败；边界：保留稳定 edge ID 便于前端定位。
func busEdgeError(edge quant.FlowEdge, reason string) error {
	return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("总线边 %s 物品 %d %.6f/min：%s", edge.ID, edge.ItemID, edge.RatePerMinute, reason)}
}
