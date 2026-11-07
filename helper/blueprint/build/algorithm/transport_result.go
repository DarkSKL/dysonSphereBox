package build

// 文件职责：从最终指针图提取物流证据、端点与空间指标。
// 使用：统一编号后编码前；边界：不把理论 lane 数写成不存在的实体带，不因表面解码成功省略流边检查。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// finalizeLogistics 职责：核验逐边流量、真实带链和最终 Area 索引并生成 DTO；使用：finish；边界：内部断带或超容量都是硬错误。
func (b *builder) finalizeLogistics(policy logisticsFinalization, records []blueprint.Building, areas []blueprint.Area) (*LogisticsSummary, error) {
	if b.logistics == nil || b.flowGraph == nil {
		return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: "缺少统一物流规划"}
	}
	summary := b.logistics
	if err := b.validateDenseMaterialNetworks(); err != nil {
		return nil, err
	}
	indices := make(map[*placed]int, len(b.all))
	for i, p := range b.all {
		indices[p] = i
	}
	edgeTotals := map[string]float64{}
	// 同一实体带可能承载多个规范边（例如合流或过滤干线）；按建筑链累计，防止每条 lane 单独合格而共享带超载。
	sharedBeltRates := map[*placed]float64{}
	// mode3/4 在塔端核验阶段已经按具体 stationLink 写入 lanes/边界；
	// finish 这里只补空间与索引指标，不能把已核验的实体 lane 清空。
	if !policy.preserveStationLanes {
		summary.Lanes, summary.ExternalPorts = []TransportLane{}, []BoundaryPort{}
	}
	for _, route := range b.transportRoutes {
		if route.rate <= 0 || route.rate > summary.BeltCapacityPerMinute+1e-6 {
			return nil, transportAllocationError(route.edge, route.rate, "lane 运力不足或分配流量无效")
		}
		lane := TransportLane{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, SourceNodeID: route.edge.SourceNodeID, TargetNodeID: route.edge.TargetNodeID, FlowKind: route.edge.FlowKind, RatePerMinute: route.rate, CapacityPerMinute: summary.BeltCapacityPerMinute, Utilization: route.rate / summary.BeltCapacityPerMinute, BuildingIndices: []int{}}
		for _, p := range append(append([]*placed{}, route.filters...), route.path...) {
			index, exists := indices[p]
			if !exists {
				return nil, transportAllocationError(route.edge, route.rate, "路线引用了蓝图外建筑")
			}
			lane.BuildingIndices = append(lane.BuildingIndices, index)
			if surfaceIsBelt(p.record.ItemID) {
				sharedBeltRates[p] += route.rate
			}
		}
		for _, p := range route.sharedPath {
			if _, exists := indices[p]; !exists {
				return nil, transportAllocationError(route.edge, route.rate, "共享干线引用了蓝图外建筑")
			}
			if surfaceIsBelt(p.record.ItemID) {
				sharedBeltRates[p] += route.rate
			}
		}
		if policy.validateConnectedRoutes {
			if len(route.path) == 0 {
				return nil, transportAllocationError(route.edge, route.rate, "没有完整实体路线")
			}
			for i := 1; i < len(route.path); i++ {
				if route.path[i-1].output != route.path[i] {
					return nil, transportAllocationError(route.edge, route.rate, "逐带输出链中断")
				}
				if route.edge.FlowKind == "internal" {
					a, c := route.path[i-1].a, route.path[i].a
					summary.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
				}
			}
			if route.from != nil {
				if len(route.filters) == 0 {
					if err := b.validateDenseDirectOutput(route); err != nil {
						return nil, err
					}
				}
				for _, arm := range route.filters {
					if arm.input == nil || arm.output == nil || int(arm.record.FilterID) != route.edge.ItemID {
						return nil, transportAllocationError(route.edge, route.rate, "过滤器物品或输入/输出端点不匹配")
					}
				}
			}
			if route.to != nil && route.path[len(route.path)-1] != route.end {
				return nil, transportAllocationError(route.edge, route.rate, "路线没有抵达指定消费者端口")
			}
			if route.to != nil && route.inputRole == sprayInputRole {
				if route.to.line == nil || route.to.line.coaterInput == nil || !flyBeltReaches(route.end, route.to.line.coaterInput) {
					return nil, transportAllocationError(route.edge, route.rate, "供剂用途路线未真实抵达本行喷涂入口")
				}
			}
			lane.InternalConnected = route.edge.FlowKind == "internal"
		}
		summary.Lanes = append(summary.Lanes, lane)
		edgeTotals[route.edge.ID] += route.rate
		if route.boundary != nil {
			index, exists := indices[route.boundary]
			if !exists {
				return nil, transportAllocationError(route.edge, route.rate, "外部接口引用了蓝图外建筑")
			}
			point := route.boundary.a
			summary.ExternalPorts = append(summary.ExternalPorts, BoundaryPort{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, Kind: route.edge.FlowKind, RatePerMinute: route.rate, BuildingIndex: index, X: point.x, Y: point.y, Z: point.z})
			if route.from == nil {
				summary.Metrics.ExternalInputCount++
			}
		}
	}
	if err := b.validateMachineRecycles(indices, sharedBeltRates); err != nil {
		return nil, err
	}
	if policy.validateSelfSprays {
		if err := b.validateConnectedSelfSprays(indices, sharedBeltRates); err != nil {
			return nil, err
		}
	}
	if policy.validateLeftovers {
		if err := b.validateLeftoverBoundaries(); err != nil {
			return nil, err
		}
	}
	maxShared := 0.0
	for belt, rate := range sharedBeltRates {
		if rate > maxShared {
			maxShared = rate
		}
		if rate > summary.BeltCapacityPerMinute+1e-6 {
			index := indices[belt]
			return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("共享传送带 %d 承载 %.6f/min，超过等级 %d 单带容量 %.6f/min", index, rate, summary.BeltLevel, summary.BeltCapacityPerMinute)}
		}
	}
	summary.Checks = append(summary.Checks, LogisticsCheck{RuleID: "shared-belt-capacity", Status: "passed", Message: fmt.Sprintf("逐实体传送带聚合合流与过滤支路流量；最大共享负载 %.3f/min，不超过等级 %d 容量 %.3f/min", maxShared, summary.BeltLevel, summary.BeltCapacityPerMinute)})
	if policy.validateEdgeConservation {
		for _, edge := range b.flowGraph.Edges {
			if math.Abs(edgeTotals[edge.ID]-edge.RatePerMinute) > math.Max(1e-6, edge.RatePerMinute*1e-8) {
				return nil, transportAllocationError(edge, edge.RatePerMinute-edgeTotals[edge.ID], "路线流量总和不守恒")
			}
		}
	}
	for _, area := range areas {
		summary.Metrics.EnvelopeArea += float64(area.Width) * float64(area.Height)
	}
	updateLayoutShapeMetrics(summary, areas)
	if b.adapter != nil && b.adapter.density == densityDense {
		summary.Metrics.ShapeObjective = denseLayoutShapeObjective(summary.Metrics)
	}
	summary.Metrics.FootprintArea = finalRecordFootprintArea(records)
	if summary.Metrics.EnvelopeArea > 0 {
		summary.Metrics.BlankRatio = math.Max(0, 1-summary.Metrics.FootprintArea/summary.Metrics.EnvelopeArea)
	}
	summary.Metrics.LaneCount = len(summary.Lanes)
	if err := updateBeltConnectionMetrics(&summary.Metrics, records); err != nil {
		return nil, err
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
			return nil, err
		}
	}
	if err := finalizeBoundaryCoordinates(summary, records, areas); err != nil {
		return nil, err
	}
	return summary, nil
}

// footprintForMetrics 职责：按原有保守 footprint 和分拣器两端统计格网并集；使用：空间指标；边界：只用于度量，不代替碰撞目录。
func footprintForMetrics(p *placed) (vector, vector) {
	low, high := footprint(p.record.ItemID)
	return vector{math.Min(p.a.x+low.x, p.b.x), math.Min(p.a.y+low.y, p.b.y), p.a.z}, vector{math.Max(p.a.x+high.x, p.b.x), math.Max(p.a.y+high.y, p.b.y), p.a.z}
}

// finalRecordFootprintArea 职责：按最终编码坐标及区域身份度量格网足迹；使用：球面重定位和多区域结果；边界：沿用保守轮廓估计，不作为真实碰撞或地表面积证明。
func finalRecordFootprintArea(records []blueprint.Building) float64 {
	cells := map[[3]int]bool{}
	for _, record := range records {
		p := placed{record: record, a: vector{float64(record.LocalOffset.X), float64(record.LocalOffset.Y), float64(record.LocalOffset.Z)}, b: vector{float64(record.LocalOffset2.X), float64(record.LocalOffset2.Y), float64(record.LocalOffset2.Z)}}
		low, high := footprintForMetrics(&p)
		for x := int(math.Floor(low.x)); x <= int(math.Floor(high.x)); x++ {
			for y := int(math.Floor(low.y)); y <= int(math.Floor(high.y)); y++ {
				cells[[3]int{int(record.AreaIndex), x, y}] = true
			}
		}
	}
	return float64(len(cells))
}

// validateFlyTransport 职责：核对配送器/四向分流模式的实体端点覆盖；使用：完成局部装配后；边界：不把箱体存在等同于配送成功，无法绑定的边必须失败。
func (b *builder) validateFlyTransport(mode int) error {
	if len(b.flowGraph.Edges) == 0 {
		return nil
	}
	if err := b.bindFlyTransportEdges(mode); err != nil {
		return err
	}
	if err := b.validateFlyEndpointTopology(mode); err != nil {
		return err
	}
	return nil
}
