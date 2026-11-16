package build

// 文件职责：拆分真实路线、共享容量、守恒与纯指标实现。
// 使用：逐项规则入口和原有内部组合；边界：每个辅助只拥有一个职责，不在指标阶段补跑规则。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"fmt"
	"math"
)

// checkLogisticsRoutes 职责：核对真实路线并输出实体负载与边份额；使用：路线规则；边界：不执行缓存、回流、总量守恒或指标度量。
func checkLogisticsRoutes(b *builder, policy logisticsFinalization) (map[*placed]float64, map[string]float64, error) {
	summary := b.logistics
	indices := make(map[*placed]int, len(b.all))
	for i, p := range b.all {
		indices[p] = i
	}
	edgeTotals := map[string]float64{}
	// 同一实体带可能承载多个规范边（例如合流或过滤干线）；按建筑链累计，防止每条 lane 单独合格而共享带超载。
	sharedBeltRates := map[*placed]float64{}
	externalInputs := map[*placed]bool{}
	// mode3/4 在塔端核验阶段已经按具体 stationLink 写入 lanes/边界；
	// finish 这里只补空间与索引指标，不能把已核验的实体 lane 清空。
	if !policy.preserveStationLanes {
		summary.Lanes, summary.ExternalPorts = []TransportLane{}, []BoundaryPort{}
	}
	for _, route := range b.transportRoutes {
		if route.rate <= 0 || route.rate > summary.BeltCapacityPerMinute+1e-6 {
			return nil, nil, transportAllocationError(route.edge, route.rate, "lane 运力不足或分配流量无效")
		}
		lane := TransportLane{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, SourceNodeID: route.edge.SourceNodeID, TargetNodeID: route.edge.TargetNodeID, FlowKind: route.edge.FlowKind, RatePerMinute: route.rate, CapacityPerMinute: summary.BeltCapacityPerMinute, Utilization: route.rate / summary.BeltCapacityPerMinute, BuildingIndices: []int{}}
		for _, p := range append(append([]*placed{}, route.filters...), route.path...) {
			index, exists := indices[p]
			if !exists {
				return nil, nil, transportAllocationError(route.edge, route.rate, "路线引用了蓝图外建筑")
			}
			lane.BuildingIndices = append(lane.BuildingIndices, index)
			if surfaceIsBelt(p.record.ItemID) {
				sharedBeltRates[p] += route.rate
			}
		}
		for _, p := range route.sharedPath {
			if _, exists := indices[p]; !exists {
				return nil, nil, transportAllocationError(route.edge, route.rate, "共享干线引用了蓝图外建筑")
			}
			if surfaceIsBelt(p.record.ItemID) {
				sharedBeltRates[p] += route.rate
			}
		}
		if b.denseProducerLeafFor(route) != nil || route.priorityInput != nil || route.from != nil && route.from.buffers[route.edge.ItemID] != nil {
			seen := map[int]bool{}
			for _, index := range lane.BuildingIndices {
				seen[index] = true
			}
			for _, p := range route.sharedPath {
				index := indices[p]
				if !seen[index] {
					lane.BuildingIndices = append(lane.BuildingIndices, index)
					seen[index] = true
				}
			}
		}
		if policy.validateConnectedRoutes {
			if len(route.path) == 0 {
				return nil, nil, transportAllocationError(route.edge, route.rate, "没有完整实体路线")
			}
			for i := 1; i < len(route.path); i++ {
				if route.path[i-1].output != route.path[i] {
					return nil, nil, transportAllocationError(route.edge, route.rate, "逐带输出链中断")
				}
				if route.edge.FlowKind == "internal" {
					a, c := route.path[i-1].a, route.path[i].a
					summary.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
				}
			}
			if route.from != nil {
				if len(route.filters) == 0 {
					if err := b.validateDenseDirectOutput(route); err != nil {
						return nil, nil, err
					}
				}
				for _, arm := range route.filters {
					if arm.input == nil || arm.output == nil || int(arm.record.FilterID) != route.edge.ItemID {
						return nil, nil, transportAllocationError(route.edge, route.rate, "过滤器物品或输入/输出端点不匹配")
					}
				}
			}
			if route.to != nil && route.path[len(route.path)-1] != route.end {
				return nil, nil, transportAllocationError(route.edge, route.rate, "路线没有抵达指定消费者端口")
			}
			if route.to != nil && route.inputRole == sprayInputRole {
				if route.to.line == nil || route.to.line.coaterInput == nil || !flyBeltReaches(route.end, route.to.line.coaterInput) {
					return nil, nil, transportAllocationError(route.edge, route.rate, "供剂用途路线未真实抵达本行喷涂入口")
				}
			}
			lane.InternalConnected = route.edge.FlowKind == "internal"
		}
		summary.Lanes = append(summary.Lanes, lane)
		edgeTotals[route.edge.ID] += route.rate
		if route.boundary != nil {
			index, exists := indices[route.boundary]
			if !exists {
				return nil, nil, transportAllocationError(route.edge, route.rate, "外部接口引用了蓝图外建筑")
			}
			point := route.boundary.a
			summary.ExternalPorts = append(summary.ExternalPorts, BoundaryPort{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, Kind: route.edge.FlowKind, RatePerMinute: route.rate, BuildingIndex: index, X: point.x, Y: point.y, Z: point.z})
			if route.from == nil && !externalInputs[route.boundary] {
				summary.Metrics.ExternalInputCount++
				externalInputs[route.boundary] = true
			}
		}
	}
	b.aggregateMode1DeliveryPorts(summary)
	if plan := b.mode1Deliveries; plan != nil {
		summary.Checks = append(summary.Checks, LogisticsCheck{RuleID: "left-boundary-shared-deliveries", Status: "passed", Message: fmt.Sprintf("%d条对外规范份额按物品及所选带实际毛容量合并为%d根独立西向交付带；已核对真实来源、共享尾链、原料隔离及聚合净产量标注", len(plan.identities), len(plan.groups))})
	}
	return sharedBeltRates, edgeTotals, nil
}

// checkLogisticsSharedCapacity 职责：检查已聚合真实带负载；使用：共享容量规则；边界：不重新生成路线或更改等级。
func checkLogisticsSharedCapacity(b *builder, rates map[*placed]float64) error {
	summary := b.logistics
	sharedBeltRates := rates
	indices := map[*placed]int{}
	for i, p := range b.all {
		indices[p] = i
	}
	maxShared := 0.0
	for belt, rate := range sharedBeltRates {
		if rate > maxShared {
			maxShared = rate
		}
		if rate > summary.BeltCapacityPerMinute+1e-6 {
			index := indices[belt]
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("共享传送带 %d 承载 %.6f/min，超过等级 %d 单带容量 %.6f/min", index, rate, summary.BeltLevel, summary.BeltCapacityPerMinute)}
		}
	}
	summary.Checks = append(summary.Checks, LogisticsCheck{RuleID: "shared-belt-capacity", Status: "passed", Message: fmt.Sprintf("逐实体传送带聚合合流与过滤支路流量；最大共享负载 %.3f/min，不超过等级 %d 容量 %.3f/min", maxShared, summary.BeltLevel, summary.BeltCapacityPerMinute)})
	return nil
}

// checkLogisticsConservation 职责：比较规范净流并复核内部总线；使用：守恒规则；边界：不修改量化账或用吞吐估算代替实体链。
func checkLogisticsConservation(b *builder, policy logisticsFinalization, edgeTotals map[string]float64) error {
	summary := b.logistics
	if policy.validateEdgeConservation {
		for _, edge := range b.flowGraph.Edges {
			if math.Abs(edgeTotals[edge.ID]-edge.RatePerMinute) > math.Max(1e-6, edge.RatePerMinute*1e-8) {
				return transportAllocationError(edge, edge.RatePerMinute-edgeTotals[edge.ID], "路线流量总和不守恒")
			}
		}
	}
	if policy.validateBusTransport {
		// finish 前的预检不持有最终编号；在 Area/records 归一后重建 mode6 lane 和外部端口。
		checks := summary.Checks[:0]
		for _, check := range summary.Checks {
			if check.RuleID != "shared-belt-capacity" {
				checks = append(checks, check)
			}
		}
		summary.Checks = checks
		if err := b.validateBusTransport(); err != nil {
			return err
		}
	}
	return checkReportedLogisticsConservation(b)
}

// checkReportedLogisticsConservation 职责：所有拓扑从实际保留lane统一复核规范边份额和存活实体；使用：独立守恒规则；边界：不能因沿用塔场lane策略而允许空报告通过，不代替专属拓扑或真实带链证明。
func checkReportedLogisticsConservation(b *builder) error {
	if b.flowGraph == nil || b.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "规范流或实际物流报告缺失"}
	}
	edges := make(map[string]quant.FlowEdge, len(b.flowGraph.Edges))
	for _, edge := range b.flowGraph.Edges {
		edges[edge.ID] = edge
	}
	totals := map[string]float64{}
	for _, lane := range b.logistics.Lanes {
		edge, known := edges[lane.EdgeID]
		if !known || lane.ItemID != edge.ItemID || lane.SourceNodeID != edge.SourceNodeID || lane.TargetNodeID != edge.TargetNodeID || lane.FlowKind != edge.FlowKind {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "实际lane没有匹配的规范物料边身份"}
		}
		if !finitePositive(lane.RatePerMinute) || !finitePositive(lane.CapacityPerMinute) || lane.RatePerMinute > lane.CapacityPerMinute+1e-6 {
			return transportAllocationError(edge, lane.RatePerMinute, "实际lane报告流量无效或超过真实容量")
		}
		if len(lane.BuildingIndices) == 0 {
			return transportAllocationError(edge, lane.RatePerMinute, "实际lane缺少任何存活实体路线证据")
		}
		for _, index := range lane.BuildingIndices {
			if index < 0 || index >= len(b.all) {
				return transportAllocationError(edge, lane.RatePerMinute, "实际lane引用蓝图外实体")
			}
		}
		totals[edge.ID] += lane.RatePerMinute
	}
	for _, edge := range b.flowGraph.Edges {
		if math.Abs(totals[edge.ID]-edge.RatePerMinute) > math.Max(1e-6, edge.RatePerMinute*1e-8) {
			return transportAllocationError(edge, edge.RatePerMinute-totals[edge.ID], "实际lane报告流量总和不守恒")
		}
	}
	return nil
}

// measureLogisticsLayout 职责：只度量最终Area、足迹、带连接和已有lane；使用：独立指标步骤；边界：不执行任何机器、物料、容量或供电规则。
func measureLogisticsLayout(b *builder, records []blueprint.Building, areas []blueprint.Area) error {
	summary := b.logistics
	for _, area := range areas {
		summary.Metrics.EnvelopeArea += float64(area.Width) * float64(area.Height)
	}
	updateLayoutShapeMetrics(summary, areas)
	if b.policy != nil && b.policy.density == densityDense {
		summary.Metrics.ShapeObjective = layoutShapeObjective(summary.Metrics)
	}
	summary.Metrics.FootprintArea = finalRecordFootprintArea(records)
	if summary.Metrics.EnvelopeArea > 0 {
		summary.Metrics.BlankRatio = math.Max(0, 1-summary.Metrics.FootprintArea/summary.Metrics.EnvelopeArea)
	}
	summary.Metrics.LaneCount = len(summary.Lanes)
	if err := updateBeltConnectionMetrics(&summary.Metrics, records); err != nil {
		return err
	}
	return nil
}
