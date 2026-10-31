package build

// 文件职责：把配送器/四向分流实体端点绑定到规范 FlowGraph；使用：mode2/5 收尾；边界：只证明端点、计划流量和蓝图内带链，不伪造机队运行时吞吐。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// flyEndpointUse 职责：记录一条规范边实际占用某配送端点的流量份额；使用：供需两侧稳定配对拆 lane；边界：只分配登记端点的计划流量，不把端点数量当成额外供给。
type flyEndpointUse struct {
	// endpoint 职责：保存本次份额使用的真实配送端点；使用：提取本地建筑链；边界：不能为 nil。
	endpoint *flyTransportEndpoint
	// rate 职责：保存该规范边在端点上的每分钟份额；使用：逐路守恒；边界：不是整条规范边流量。
	rate float64
}

// bindFlyTransportEdges 职责：按规范边稳定分配配送端点并写入 transportRoutes；使用：recordFacilityTransport；边界：每条边必须绑定真实 2107/2101 端点，未覆盖的副产物硬失败。
func (b *builder) bindFlyTransportEdges(mode int) error {
	if mode != 2 && mode != 5 {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: fmt.Sprintf("配送端点绑定不支持模式 %d", mode)}
	}
	if b.flowGraph == nil || b.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送模式缺少规范物流图"}
	}
	if len(b.flowGraph.Edges) == 0 {
		return nil
	}
	if len(b.flyEndpoints) == 0 {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("模式 %d 没有登记任何配送器端点", mode)}
	}
	if len(b.transportRoutes) != 0 {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送模式物流边被重复绑定"}
	}

	owners := map[string]int{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			owners[node.ID] = node.ProductionIndex
		}
	}
	inputs := map[int]map[int][]*flyTransportEndpoint{}
	outputs := map[int]map[int][]*flyTransportEndpoint{}
	remaining := map[*flyTransportEndpoint]float64{}
	capacity := b.logistics.BeltCapacityPerMinute
	if !finitePositive(capacity) {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "配送模式没有有效的所选等级单带容量"}
	}
	for _, endpoint := range b.flyEndpoints {
		if endpoint == nil || endpoint.link == nil || endpoint.box == nil || endpoint.delivery == nil {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "存在未完整登记的配送端点"}
		}
		if endpoint.delivery.record.ItemID != 2107 || endpoint.box.record.ItemID != 2101 || int(endpoint.delivery.record.FilterID) != endpoint.item {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("产线 %d 的配送端点物品 %d 与箱体/过滤器不匹配", endpoint.owner, endpoint.item)}
		}
		if mode == 2 && endpoint.splitter != nil || mode == 5 && endpoint.splitter == nil {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("模式 %d 的物品 %d 端点分流器形态不匹配", mode, endpoint.item)}
		}
		if endpoint.rate < 0 || math.IsNaN(endpoint.rate) || math.IsInf(endpoint.rate, 0) {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("产线 %d 物品 %d 的配送计划流量无效", endpoint.owner, endpoint.item)}
		}
		if _, duplicate := remaining[endpoint]; duplicate {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("产线 %d 物品 %d 的配送端点被重复登记", endpoint.owner, endpoint.item)}
		}
		// 同一实体接驳带不能通过多写几条诊断 lane 伪装成并行带。
		// 实际机器行/端点拆分必须在装配阶段完成。
		if endpoint.rate > capacity+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("产线 %d 物品 %d 单个配送端点 %.6f/min 超过所选带容量 %.6f/min，需要拆分真实机器行或端点", endpoint.owner, endpoint.item, endpoint.rate, capacity)}
		}
		remaining[endpoint] = endpoint.rate
		target := outputs
		if endpoint.input {
			target = inputs
		}
		if target[endpoint.owner] == nil {
			target[endpoint.owner] = map[int][]*flyTransportEndpoint{}
		}
		target[endpoint.owner][endpoint.item] = append(target[endpoint.owner][endpoint.item], endpoint)
	}

	boundEdges := 0
	plannedRoutes := []*transportRoute{}
	for _, edge := range b.flowGraph.Edges {
		if edge.RatePerMinute <= 0 || math.IsNaN(edge.RatePerMinute) || math.IsInf(edge.RatePerMinute, 0) {
			return transportAllocationError(edge, edge.RatePerMinute, "规范边流量无效")
		}
		var source, target []flyEndpointUse
		if owner, ok := owners[edge.SourceNodeID]; ok {
			var err error
			source, err = allocateFlyEndpoints(outputs[owner][edge.ItemID], remaining, edge.RatePerMinute)
			if err != nil {
				return transportAllocationError(edge, edge.RatePerMinute, "生产输出没有足够的配送端点")
			}
		}
		if owner, ok := owners[edge.TargetNodeID]; ok {
			var err error
			target, err = allocateFlyEndpoints(inputs[owner][edge.ItemID], remaining, edge.RatePerMinute)
			if err != nil {
				return transportAllocationError(edge, edge.RatePerMinute, "生产输入没有足够的配送端点")
			}
		}
		if len(source) == 0 && len(target) == 0 {
			return transportAllocationError(edge, edge.RatePerMinute, "规范边两端都没有本地图内配送端点")
		}
		routes, err := splitFlyTransportLanes(edge, source, target, capacity)
		if err != nil {
			return err
		}
		plannedRoutes = append(plannedRoutes, routes...)
		boundEdges++
	}
	// 按登记顺序核对残余，保证相同错误也不受 map 遍历顺序影响。
	for _, endpoint := range b.flyEndpoints {
		rate := remaining[endpoint]
		if rate > math.Max(1e-6, endpoint.rate*1e-8) {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("产线 %d 物品 %d 仍有 %.6f/min 端点流量未绑定规范边", endpoint.owner, endpoint.item, rate)}
		}
	}
	b.transportRoutes = plannedRoutes
	b.logistics.Checks = append(b.logistics.Checks,
		LogisticsCheck{RuleID: "fly-edge-binding", Status: "passed", Message: fmt.Sprintf("模式 %d 已将 %d 条规范边按端点实际份额拆成 %d 条计划 lane；每路只引用对应的真实本地配送器/箱体接驳链", mode, boundEdges, len(b.transportRoutes))},
		LogisticsCheck{RuleID: "fly-runtime-delivery", Status: "unverified", Message: "配送机队、箱体库存、远程供给和游戏科技状态未在离线蓝图中模拟；逐边速率是按计划供料/取货契约记录"},
	)
	return nil
}

// allocateFlyEndpoints 职责：按登记顺序将一条规范边拆到同物品端点；使用：多机器行/多端点；边界：不创建额外流量，不跨物品复用端点。
func allocateFlyEndpoints(endpoints []*flyTransportEndpoint, remaining map[*flyTransportEndpoint]float64, need float64) ([]flyEndpointUse, error) {
	if need <= 0 || math.IsNaN(need) || math.IsInf(need, 0) {
		return nil, fmt.Errorf("invalid endpoint rate")
	}
	left := need
	used := []flyEndpointUse{}
	seen := map[*flyTransportEndpoint]bool{}
	for _, endpoint := range endpoints {
		if left <= 0 || len(used) > 0 && left <= math.Max(1e-8, need*1e-10) {
			break
		}
		if endpoint == nil {
			return nil, fmt.Errorf("配送端点为空")
		}
		if seen[endpoint] {
			return nil, fmt.Errorf("配送端点重复")
		}
		seen[endpoint] = true
		available := remaining[endpoint]
		if available < 0 || math.IsNaN(available) || math.IsInf(available, 0) {
			return nil, fmt.Errorf("配送端点剩余流量无效")
		}
		if available <= 0 {
			continue
		}
		amount := math.Min(left, available)
		left -= amount
		used = append(used, flyEndpointUse{endpoint: endpoint, rate: amount})
	}
	if left > math.Max(1e-6, need*1e-8) {
		return nil, fmt.Errorf("endpoint shortage %.9g", left)
	}
	// 先确认整条边可分配，再统一提交，失败不得消耗一部分端点份额。
	for _, use := range used {
		remaining[use.endpoint] = math.Max(0, remaining[use.endpoint]-use.rate)
	}
	return used, nil
}

// splitFlyTransportLanes 职责：在供需两侧的精确端点份额之间稳定配对；使用：规范边绑定；边界：计划配对不等价于两侧已经存在连续带或已模拟配送机队吞吐。
func splitFlyTransportLanes(edge quant.FlowEdge, sources, targets []flyEndpointUse, capacity float64) ([]*transportRoute, error) {
	if !finitePositive(edge.RatePerMinute) || !finitePositive(capacity) {
		return nil, transportAllocationError(edge, edge.RatePerMinute, "规范边或单带容量无效")
	}
	if edge.FlowKind != "external-input" && edge.FlowKind != "target-output" && edge.FlowKind != "leftover" && edge.FlowKind != "internal" {
		return nil, transportAllocationError(edge, edge.RatePerMinute, "未知配送流向类型")
	}
	if edge.FlowKind == "external-input" && len(targets) == 0 || (edge.FlowKind == "target-output" || edge.FlowKind == "leftover") && len(sources) == 0 || edge.FlowKind == "internal" && (len(sources) == 0 || len(targets) == 0) {
		return nil, transportAllocationError(edge, edge.RatePerMinute, "规范边缺少对应方向的真实配送端点")
	}
	if len(sources) == 0 && len(targets) == 0 {
		return nil, transportAllocationError(edge, edge.RatePerMinute, "规范边没有实体配送建筑")
	}
	for side, uses := range [][]flyEndpointUse{sources, targets} {
		if len(uses) == 0 {
			continue
		}
		total := 0.0
		for _, use := range uses {
			if use.endpoint == nil || use.endpoint.item != edge.ItemID || use.endpoint.input != (side == 1) || !finitePositive(use.rate) {
				return nil, transportAllocationError(edge, edge.RatePerMinute, "配送端点物品或分配份额无效")
			}
			total += use.rate
		}
		if math.Abs(total-edge.RatePerMinute) > math.Max(1e-6, edge.RatePerMinute*1e-8) {
			return nil, transportAllocationError(edge, total-edge.RatePerMinute, "配送端点份额总和与规范边不守恒")
		}
	}
	si, ti := 0, 0
	sourceUsed, targetUsed := 0.0, 0.0
	left := edge.RatePerMinute
	tolerance := math.Max(1e-8, edge.RatePerMinute*1e-10)
	routes := []*transportRoute{}
	for left > tolerance || len(routes) == 0 {
		part := math.Min(left, capacity)
		var source, target *flyTransportEndpoint
		if len(sources) > 0 {
			for si < len(sources) && (sources[si].rate-sourceUsed <= 0 || sourceUsed > 0 && sources[si].rate-sourceUsed <= tolerance) {
				si++
				sourceUsed = 0
			}
			if si == len(sources) {
				return nil, transportAllocationError(edge, left, "配送供货份额未覆盖全部 lane")
			}
			source = sources[si].endpoint
			part = math.Min(part, sources[si].rate-sourceUsed)
		}
		if len(targets) > 0 {
			for ti < len(targets) && (targets[ti].rate-targetUsed <= 0 || targetUsed > 0 && targets[ti].rate-targetUsed <= tolerance) {
				ti++
				targetUsed = 0
			}
			if ti == len(targets) {
				return nil, transportAllocationError(edge, left, "配送需求份额未覆盖全部 lane")
			}
			target = targets[ti].endpoint
			part = math.Min(part, targets[ti].rate-targetUsed)
		}
		if !finitePositive(part) {
			return nil, transportAllocationError(edge, part, "配送 lane 分配份额无效")
		}
		route := &transportRoute{edge: edge, rate: part, path: appendFlyEndpointEntities(nil, source, target)}
		// from/to 只表达本地图中的供需方向。这里不设置 paired，也不
		// 写连续带证明；最终 InternalConnected 必须继续为 false。
		if source != nil {
			route.from = &transportRow{owner: source.owner}
			route.start = source.link
			sourceUsed += part
		}
		if target != nil {
			route.to = &transportRow{owner: target.owner}
			route.end = target.link
			targetUsed += part
		}
		if edge.FlowKind == "external-input" {
			route.boundary = target.delivery
		} else if edge.FlowKind == "target-output" || edge.FlowKind == "leftover" {
			route.boundary = source.delivery
		}
		routes = append(routes, route)
		left = math.Max(0, left-part)
	}
	return routes, nil
}

// appendFlyEndpointEntities 职责：稳定去重配送端点对应建筑；使用：TransportLane BuildingIndices；边界：只收录真实 builder.all 指针。
func appendFlyEndpointEntities(out []*placed, endpoints ...*flyTransportEndpoint) []*placed {
	seen := map[*placed]bool{}
	for _, p := range out {
		seen[p] = true
	}
	for _, endpoint := range endpoints {
		if endpoint == nil {
			continue
		}
		entities := append([]*placed{endpoint.link, endpoint.splitter, endpoint.box, endpoint.delivery, endpoint.arm}, endpoint.belts...)
		entities = append(entities, endpoint.arms...)
		entities = append(entities, endpoint.filters...)
		for _, p := range entities {
			if p != nil && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// validateFlyEndpointTopology 职责：检查端点本地接驳链；使用：模式收尾回归；边界：不把远程机队行为当成本地带连接。
func (b *builder) validateFlyEndpointTopology(mode int) error {
	for _, endpoint := range b.flyEndpoints {
		if endpoint == nil || endpoint.delivery == nil || endpoint.box == nil || endpoint.link == nil || len(endpoint.belts) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "配送端点缺少箱体、配送器或实体接驳带"}
		}
		if endpoint.delivery.input != endpoint.box {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 配送器未连接箱体", endpoint.owner, endpoint.item)}
		}
		if mode == 5 {
			if endpoint.splitter == nil || endpoint.box.input != endpoint.splitter {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 四向分流器未连接箱体", endpoint.owner, endpoint.item)}
			}
		} else if endpoint.arm == nil {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 配送器缺少箱体分拣器", endpoint.owner, endpoint.item)}
		}
		for _, arm := range endpoint.arms {
			if arm == nil || arm.input == nil || arm.output == nil {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 存在未连接的并联箱体分拣器", endpoint.owner, endpoint.item)}
			}
		}
		for _, filter := range endpoint.filters {
			if filter == nil || filter.input == nil || filter.output == nil || int(filter.record.FilterID) != endpoint.item {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("产线 %d 物品 %d 输出过滤分拣器不完整或物品不匹配", endpoint.owner, endpoint.item)}
			}
		}
	}
	return nil
}
