package build

// 文件职责：核验模式3/4物流塔实体端口与规范流图的绑定；使用：Generate收尾调用；边界：不模拟物流机队、星际运输或游戏内持续运行。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// stationLinkUse 职责：保存单条规范流边在一个具体物流塔路线上的计划份额；使用：拆分 DTO lane；边界：各片段相加严格等于规范边。
type stationLinkUse struct {
	link *stationTransportLink
	rate float64
}

// validateStationTransport 职责：逐规范边匹配供货/需求塔槽并检查物品与带容量；使用：模式3/4最终 records 后；边界：站间运输和机器分拣器吞吐单独标为未验证。
func (b *builder) validateStationTransport(ctx context.Context, mode int, records []blueprint.Building) error {
	if mode != 3 && mode != 4 {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: fmt.Sprintf("物流塔诊断不支持模式 %d", mode)}
	}
	if b.flowGraph == nil || b.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "物流塔诊断缺少规范流图"}
	}
	if err := validateStationPorts(ctx, records); err != nil {
		return err
	}
	indices := make(map[*placed]int, len(b.all))
	for i, p := range b.all {
		indices[p] = i
	}
	nodeKinds := make(map[string]string, len(b.flowGraph.Nodes))
	productionNodeIDs := make(map[int]string)
	linksByNode := make(map[string][]*stationTransportLink)
	for _, node := range b.flowGraph.Nodes {
		nodeKinds[node.ID] = node.Kind
		if node.Kind == "production" {
			productionNodeIDs[node.ProductionIndex] = node.ID
		}
	}
	for i := range b.stationLinks {
		link := &b.stationLinks[i]
		if err := ctx.Err(); err != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔端点关联检查已取消"}
		}
		if link.tower == nil || link.belt == nil || link.head == nil || len(link.path) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "物流塔路线缺少塔、产线端点或实体带"}
		}
		if link.item <= 0 || link.slot < 0 || link.slot >= 12 || !finitePositive(link.rate) {
			return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "物流塔路线物品、端口或计划流量无效"}
		}
		if err := validateStationLink(link, indices, records); err != nil {
			return err
		}
		if link.rate > b.logistics.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("产线 %d 物品 %d 的物流塔端口带 %.6f/min，超过带容量 %.6f/min", link.owner, link.item, link.rate, b.logistics.BeltCapacityPerMinute)}
		}
		if nodeID, ok := productionNodeIDs[link.owner]; ok {
			linksByNode[nodeID] = append(linksByNode[nodeID], link)
		}
	}
	remaining := make(map[*stationTransportLink]float64, len(b.stationLinks))
	for i := range b.stationLinks {
		remaining[&b.stationLinks[i]] = b.stationLinks[i].rate
	}
	b.logistics.Lanes, b.logistics.ExternalPorts = []TransportLane{}, []BoundaryPort{}
	for _, edge := range b.flowGraph.Edges {
		if err := ctx.Err(); err != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔流边检查已取消"}
		}
		sourceProduction := nodeKinds[edge.SourceNodeID] == "production"
		targetProduction := nodeKinds[edge.TargetNodeID] == "production"
		sources, targets := findStationLinks(edge, linksByNode, true), findStationLinks(edge, linksByNode, false)
		if sourceProduction && len(sources) == 0 {
			return stationEdgeError(edge, "生产供给边没有对应的物品供货塔槽（副产物不得漏出）")
		}
		if targetProduction && len(targets) == 0 {
			return stationEdgeError(edge, "生产需求边没有对应的物品需求塔槽")
		}
		var usedSources, usedTargets []stationLinkUse
		if sourceProduction {
			var err error
			usedSources, err = consumeStationLinks(edge, sources, remaining)
			if err != nil {
				return err
			}
		}
		if targetProduction {
			var err error
			usedTargets, err = consumeStationLinks(edge, targets, remaining)
			if err != nil {
				return err
			}
		}
		if err := appendStationFlowLanes(b.logistics, edge, usedSources, usedTargets, indices); err != nil {
			return err
		}
	}
	if err := validateStationFlowCoverage(b.flowGraph, b.stationLinks); err != nil {
		return err
	}
	b.logistics.Checks = append(b.logistics.Checks,
		LogisticsCheck{RuleID: "station-topology", Status: "passed", Message: "规范流边已绑定具体物流塔物品槽、方向、端口和相连传送带；副产物流边逐项检查"},
		LogisticsCheck{RuleID: "station-fleet", Status: "unverified", Message: "物流塔站间机队吞吐、外界供货/取货和游戏内持续运行未验证"},
		LogisticsCheck{RuleID: "station-machine-inserters", Status: "unverified", Message: "机器分拣器实时吞吐与堵带时序未验证；本检查只核对机器行端点、塔端口及计划净流量"},
	)
	b.logistics.Metrics.LaneCount = len(b.logistics.Lanes)
	b.logistics.Metrics.ExternalInputCount = 0
	for _, port := range b.logistics.ExternalPorts {
		if port.Kind == "external-input" {
			b.logistics.Metrics.ExternalInputCount++
		}
	}
	return nil
}

// findStationLinks 按稳定 owner、物品和生产供需角色过滤本地塔路线。
func findStationLinks(edge quant.FlowEdge, byNode map[string][]*stationTransportLink, source bool) []*stationTransportLink {
	nodeID := edge.TargetNodeID
	if source {
		nodeID = edge.SourceNodeID
	}
	links := []*stationTransportLink{}
	for _, link := range byNode[nodeID] {
		if link.item == edge.ItemID && link.output == source {
			links = append(links, link)
		}
	}
	return links
}

// consumeStationLinks 按 route 声明的精确机器份额分配规范边，不把剩余机器能力当作产出。
func consumeStationLinks(edge quant.FlowEdge, links []*stationTransportLink, remaining map[*stationTransportLink]float64) ([]stationLinkUse, error) {
	left := edge.RatePerMinute
	used := []stationLinkUse{}
	tolerance := math.Max(1e-8, edge.RatePerMinute*1e-8)
	for _, link := range links {
		available := remaining[link]
		if available <= tolerance {
			continue
		}
		part := math.Min(left, available)
		if part > 0 {
			used = append(used, stationLinkUse{link: link, rate: part})
		}
		remaining[link] = math.Max(0, available-part)
		left -= part
		if left <= tolerance {
			return used, nil
		}
	}
	return nil, stationEdgeError(edge, fmt.Sprintf("生产端点声明流量不足，尚缺 %.6f/min", left))
}

// appendStationFlowLanes 将一条图边按供需两侧的具体路线份额切成实体端口 lane。
func appendStationFlowLanes(summary *LogisticsSummary, edge quant.FlowEdge, sources, targets []stationLinkUse, indices map[*placed]int) error {
	if len(sources) == 0 && len(targets) == 0 {
		return stationEdgeError(edge, "边没有可定位的生产塔端口")
	}
	if edge.FlowKind == "external-input" && len(targets) == 0 || (edge.FlowKind == "target-output" || edge.FlowKind == "leftover") && len(sources) == 0 {
		return stationEdgeError(edge, "边界物料没有具体物流塔端口")
	}
	si, ti := 0, 0
	var sourceUsed, targetUsed float64
	left := edge.RatePerMinute
	for left > math.Max(1e-8, edge.RatePerMinute*1e-8) {
		var source, target *stationLinkUse
		part := left
		if len(sources) > 0 {
			for si < len(sources) && sourceUsed >= sources[si].rate-math.Max(1e-8, edge.RatePerMinute*1e-8) {
				si++
				sourceUsed = 0
			}
			if si >= len(sources) {
				return stationEdgeError(edge, "供货塔路线分配不足")
			}
			source = &sources[si]
			part = math.Min(part, source.rate-sourceUsed)
		}
		if len(targets) > 0 {
			for ti < len(targets) && targetUsed >= targets[ti].rate-math.Max(1e-8, edge.RatePerMinute*1e-8) {
				ti++
				targetUsed = 0
			}
			if ti >= len(targets) {
				return stationEdgeError(edge, "需求塔路线分配不足")
			}
			target = &targets[ti]
			part = math.Min(part, target.rate-targetUsed)
		}
		if part <= 0 {
			return stationEdgeError(edge, "路线拆分产生非正流量")
		}
		links := []*stationTransportLink{}
		if source != nil {
			links = append(links, source.link)
		}
		if target != nil {
			links = append(links, target.link)
		}
		capacity := summary.BeltCapacityPerMinute
		if part > capacity+1e-6 {
			return stationEdgeError(edge, "拆分后的单塔带流量超过所选带容量")
		}
		summary.Lanes = append(summary.Lanes, TransportLane{
			EdgeID: edge.ID, ItemID: edge.ItemID, SourceNodeID: edge.SourceNodeID, TargetNodeID: edge.TargetNodeID,
			FlowKind: edge.FlowKind, RatePerMinute: part, CapacityPerMinute: capacity, Utilization: part / capacity,
			BuildingIndices: stationLinkBuildingIndices(links, indices), InternalConnected: false,
		})
		if edge.FlowKind != "internal" {
			portUse := target
			if edge.FlowKind == "target-output" || edge.FlowKind == "leftover" {
				portUse = source
			}
			if portUse == nil {
				return stationEdgeError(edge, "边界物料没有具体物流塔端口")
			}
			link := portUse.link
			summary.ExternalPorts = append(summary.ExternalPorts, BoundaryPort{
				EdgeID: edge.ID, ItemID: edge.ItemID, Kind: edge.FlowKind, RatePerMinute: part,
				BuildingIndex: indices[link.tower], X: link.tower.a.x, Y: link.tower.a.y, Z: link.tower.a.z,
			})
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

// validateStationFlowCoverage 拒绝没有任何规范边的额外物料供需端点，避免未追踪物料被站点静默吞吐。
func validateStationFlowCoverage(graph *quant.FlowGraph, links []stationTransportLink) error {
	flow := map[string]float64{}
	nodes := make(map[string]quant.FlowNode, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range graph.Edges {
		if node, ok := nodes[edge.SourceNodeID]; ok && node.Kind == "production" {
			flow[stationFlowKey(node.ProductionIndex, edge.ItemID, true)] += edge.RatePerMinute
		}
		if node, ok := nodes[edge.TargetNodeID]; ok && node.Kind == "production" {
			flow[stationFlowKey(node.ProductionIndex, edge.ItemID, false)] += edge.RatePerMinute
		}
	}
	routes := map[string]float64{}
	for _, link := range links {
		routes[stationFlowKey(link.owner, link.item, link.output)] += link.rate
	}
	for key, rate := range flow {
		if math.Abs(routes[key]-rate) > math.Max(1e-6, rate*1e-8) {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("物流塔端点流量与规范边不一致：%s route=%.6f graph=%.6f/min", key, routes[key], rate)}
		}
	}
	for key, rate := range routes {
		if math.Abs(flow[key]-rate) > math.Max(1e-6, rate*1e-8) {
			return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("未进入规范流图的物流塔物料端点：%s route=%.6f graph=%.6f/min", key, rate, flow[key])}
		}
	}
	return nil
}

func stationFlowKey(owner, item int, output bool) string {
	return fmt.Sprintf("%d:%d:%t", owner, item, output)
}

func stationLinkBuildingIndices(links []*stationTransportLink, indices map[*placed]int) []int {
	seen := map[int]bool{}
	result := []int{}
	for _, link := range links {
		for _, p := range link.path {
			if i, ok := indices[p]; ok && !seen[i] {
				seen[i] = true
				result = append(result, i)
			}
		}
		for _, p := range []*placed{link.tower, link.head} {
			if i, ok := indices[p]; ok && !seen[i] {
				seen[i] = true
				result = append(result, i)
			}
		}
	}
	return result
}

// validateStationLink 核对塔槽方向、物品槽、末端带指针与全程有向带链。
func validateStationLink(link *stationTransportLink, indices map[*placed]int, records []blueprint.Building) error {
	towerIndex, towerExists := indices[link.tower]
	beltIndex, beltExists := indices[link.belt]
	if !towerExists || !beltExists {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "物流塔路线引用蓝图外建筑"}
	}
	tower, belt := records[towerIndex], records[beltIndex]
	if !surfaceIsStation(tower.ItemID) || !surfaceIsBelt(belt.ItemID) || len(tower.Parameters) != 2048 {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "物流塔路线建筑类型或参数不匹配"}
	}
	base := 192 + link.slot*4
	if link.output {
		if link.belt.output != link.tower || int(belt.OutputToSlot) != link.slot || tower.Parameters[base] != 2 || tower.Parameters[base+1] != 0 || len(belt.Parameters) == 0 || int(belt.Parameters[0]) != link.item {
			return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "供货带未连接指定塔槽或物品标记不匹配"}
		}
		for i := 1; i < len(link.path); i++ {
			if link.path[i-1].output != link.path[i] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "供货塔带链中断"}
			}
		}
		if link.path[len(link.path)-1] != link.belt || link.head.output != link.path[0] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "供货塔带链未连接产线端点"}
		}
		return nil
	}
	if link.belt.input != link.tower || int(belt.InputFromSlot) != link.slot || tower.Parameters[base] != 1 || len(belt.Parameters) == 0 || int(belt.Parameters[0]) != link.item {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "需求带未连接指定塔槽或物品标记不匹配"}
	}
	itemSlot := int(tower.Parameters[base+1])
	if itemSlot <= 0 || itemSlot > 5 || tower.Parameters[(itemSlot-1)*6] != int32(link.item) || tower.Parameters[(itemSlot-1)*6+1] != 2 {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "需求塔槽未配置对应物品"}
	}
	for i := 0; i+1 < len(link.path); i++ {
		if link.path[i+1].output != link.path[i] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "需求塔带链中断"}
		}
	}
	// 输入带在 attachStations 中先 reverse，再把产线侧首段接到产线头：
	// path[0] -> head，后续段依次反向指向前一段，belt 是塔侧末段。
	if link.path[len(link.path)-1] != link.belt || link.path[0].output != link.head {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "需求塔带链未连接产线端点"}
	}
	return nil
}

func productionItemRate(rates []quant.ItemRate, itemID int) float64 {
	for _, rate := range rates {
		if rate.ItemID == itemID {
			return rate.RatePerMinute
		}
	}
	return 0
}

func stationEdgeError(edge quant.FlowEdge, reason string) error {
	return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("物品 %d、边 %s、%.6f/min：%s", edge.ItemID, edge.ID, edge.RatePerMinute, reason)}
}
