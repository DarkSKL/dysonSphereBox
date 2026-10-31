package build

// 文件职责：核验可扩展混带环路的守恒、过滤、连续带链、真实并联容量及结果索引。
// 使用：mode6 装配完成与统一编号后重复调用；边界：所有硬约束必须通过，不模拟游戏逐帧堵带和启动时序。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// validateBusFabric 职责：把每条规范边绑定到真实共享环路和过滤端口；使用：统一 finish 及编码前复核；边界：任何缺边、过载、断链或未注册实体都返回硬错误。
func (b *builder) validateBusFabric() error {
	if b.flowGraph == nil || b.logistics == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "混带环路缺少可信流图或诊断"}
	}
	indices := make(map[*placed]int, len(b.all))
	for i, p := range b.all {
		indices[p] = i
	}
	ringSets := map[int]map[*placed]bool{}
	for layer, ring := range b.busLayerBelts {
		if len(ring) < 4 {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带共享环路实体数量不足"}
		}
		set := map[*placed]bool{}
		for i, p := range ring {
			if p == nil || set[p] || !surfaceIsBelt(p.record.ItemID) || p.output != ring[(i+1)%len(ring)] {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("混带共享环路%d在%d处断开", layer+1, i)}
			}
			if _, ok := indices[p]; !ok {
				return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带共享环路引用蓝图外实体"}
			}
			set[p] = true
		}
		ringSets[layer] = set
	}
	edgeTotals := map[string]float64{}
	layerLoads := map[int]float64{}
	beltLoads := map[*placed]float64{}
	armLoads := map[*placed]float64{}
	summary := b.logistics
	summary.Lanes = []TransportLane{}
	summary.ExternalPorts = []BoundaryPort{}
	summary.Metrics.ExternalInputCount = 0
	summary.Metrics.InternalBeltLength = 0
	for _, entry := range b.busFabric {
		if entry == nil || entry.route == nil {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "混带规范lane为空"}
		}
		route := entry.route
		if !finitePositive(route.rate) || route.rate > summary.BeltCapacityPerMinute+1e-6 {
			return transportAllocationError(route.edge, route.rate, "混带lane超过所选等级容量")
		}
		edgeTotals[route.edge.ID] += route.rate
		parts := append([]*placed{}, route.filters...)
		parts = append(parts, route.sharedPath...)
		parts = append(parts, route.path...)
		if route.from != nil {
			if err := validateBusFabricProducer(route); err != nil {
				return err
			}
			if len(route.filters) == 0 {
				return transportAllocationError(route.edge, route.rate, "混带生产者缺少逐物品过滤端口")
			}
			capacity := 0.
			for _, arm := range route.filters {
				if err := validateBusInserterShape(arm, route.edge.ItemID); err != nil {
					return err
				}
				value, err := busInserterCapacity(arm)
				if err != nil {
					return err
				}
				capacity += value
				armLoads[arm] += route.rate / float64(len(route.filters))
			}
			if capacity+1e-6 < route.rate {
				return transportAllocationError(route.edge, route.rate, "混带生产者过滤端口能力不足")
			}
		}
		if route.to != nil {
			if entry.source == nil || entry.target == nil || len(entry.feed) == 0 || len(entry.take) == 0 {
				return transportAllocationError(route.edge, route.rate, "混带供需缺少环路接驳")
			}
			ring := ringSets[entry.layer]
			if len(ring) == 0 {
				return transportAllocationError(route.edge, route.rate, "混带lane未分配真实共享环路")
			}
			layerLoads[entry.layer] += route.rate
			feedChain := append([]*placed{}, entry.source.path...)
			if len(entry.source.sharedPath) == 0 || len(feedChain) == 0 || feedChain[len(feedChain)-1] != entry.source.sharedPath[0] {
				return transportAllocationError(route.edge, route.rate, "混带源接驳没有抵达环路供料带")
			}
			feedChain = append(feedChain, entry.source.sharedPath[1:]...)
			if err := validateBusFabricChain(feedChain, indices); err != nil {
				return err
			}
			if err := validateBusFabricChain(entry.target.path, indices); err != nil {
				return err
			}
			if len(entry.target.path) == 0 || entry.target.path[len(entry.target.path)-1] != route.end {
				return transportAllocationError(route.edge, route.rate, "混带过滤取料没有到达正确消费者")
			}
			consumerTail, err := busFabricConsumerTail(route)
			if err != nil {
				return err
			}
			if err := validateBusFabricChain(consumerTail, indices); err != nil {
				return err
			}
			if err := validateBusFabricArms(entry.feed, route.rate, route.edge.ItemID, ring, true); err != nil {
				return err
			}
			if err := validateBusFabricArms(entry.take, route.rate, route.edge.ItemID, ring, false); err != nil {
				return err
			}
			if route.from == nil {
				if len(entry.recycle) != len(entry.feed) {
					return transportAllocationError(route.edge, route.rate, "原料入口缺少与注入能力相配的真实过滤回收臂")
				}
				if err := validateBusFabricArms(entry.recycle, route.rate, route.edge.ItemID, ring, false); err != nil {
					return err
				}
				if len(entry.source.sharedPath) < 3 || entry.source.sharedPath[len(entry.source.sharedPath)-1].output != entry.source.sharedPath[1] {
					return transportAllocationError(route.edge, route.rate, "原料回收供料带没有形成受外界回压的真实循环")
				}
				for _, recycle := range entry.recycle {
					for _, feed := range entry.feed {
						if recycle.input.a.y >= feed.output.a.y {
							return transportAllocationError(route.edge, route.rate, "原料回收段必须位于当前圈注入段之前，避免刚注入物料立即被回收")
						}
					}
				}
			}
			feedMembers, takeMembers := map[*placed]bool{}, map[*placed]bool{}
			for _, belt := range feedChain {
				feedMembers[belt] = true
			}
			for _, belt := range entry.target.path {
				takeMembers[belt] = true
			}
			for _, arm := range entry.feed {
				if !feedMembers[arm.input] {
					return transportAllocationError(route.edge, route.rate, "环路供料臂未连接当前供料带链")
				}
				armLoads[arm] += route.rate / float64(len(entry.feed))
			}
			for _, arm := range entry.take {
				if !takeMembers[arm.output] {
					return transportAllocationError(route.edge, route.rate, "环路取料臂未连接当前消费带链")
				}
				armLoads[arm] += route.rate / float64(len(entry.take))
			}
			for _, arm := range entry.recycle {
				if !feedMembers[arm.output] {
					return transportAllocationError(route.edge, route.rate, "原料回收臂没有接回当前原料供料循环")
				}
				armLoads[arm] += route.rate / float64(len(entry.recycle))
			}
			parts = append(parts, feedChain...)
			parts = append(parts, entry.target.path...)
			parts = append(parts, consumerTail...)
			parts = append(parts, entry.feed...)
			parts = append(parts, entry.take...)
			parts = append(parts, entry.recycle...)
			parts = append(parts, b.busLayerBelts[entry.layer]...)
			if route.from == nil {
				if route.boundary == nil || route.boundary != entry.source.path[0] || !hasPortItem(route.boundary, route.edge.ItemID) {
					return transportAllocationError(route.edge, route.rate, "外部原料缺少正确物品的开放入口")
				}
			}
		} else {
			if route.from == nil || route.boundary == nil || !hasPortItem(route.boundary, route.edge.ItemID) {
				return transportAllocationError(route.edge, route.rate, "目标或副产物没有独立过滤出口")
			}
			if err := validateBusFabricChain(route.path, indices); err != nil {
				return err
			}
		}
		seen := map[*placed]bool{}
		buildingIndices := []int{}
		for _, part := range parts {
			if part == nil || seen[part] {
				continue
			}
			index, ok := indices[part]
			if !ok {
				return transportAllocationError(route.edge, route.rate, "混带路线引用蓝图外实体")
			}
			seen[part] = true
			buildingIndices = append(buildingIndices, index)
			if surfaceIsBelt(part.record.ItemID) {
				beltLoads[part] += route.rate
			}
			if route.edge.FlowKind == "internal" && surfaceIsBelt(part.record.ItemID) && part.output != nil {
				a, c := part.a, part.output.a
				summary.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
			}
		}
		summary.Lanes = append(summary.Lanes, TransportLane{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, SourceNodeID: route.edge.SourceNodeID, TargetNodeID: route.edge.TargetNodeID, FlowKind: route.edge.FlowKind, RatePerMinute: route.rate, CapacityPerMinute: summary.BeltCapacityPerMinute, Utilization: route.rate / summary.BeltCapacityPerMinute, BuildingIndices: buildingIndices, InternalConnected: route.edge.FlowKind == "internal"})
		if route.edge.FlowKind != "internal" {
			if route.boundary == nil {
				return transportAllocationError(route.edge, route.rate, "混带边界端口为空")
			}
			index, ok := indices[route.boundary]
			if !ok {
				return transportAllocationError(route.edge, route.rate, "混带边界端口不属于蓝图")
			}
			point := route.boundary.a
			summary.ExternalPorts = append(summary.ExternalPorts, BoundaryPort{EdgeID: route.edge.ID, ItemID: route.edge.ItemID, Kind: route.edge.FlowKind, RatePerMinute: route.rate, BuildingIndex: index, X: point.x, Y: point.y, Z: point.z})
			if route.edge.FlowKind == "external-input" {
				summary.Metrics.ExternalInputCount++
			}
		}
	}
	for _, edge := range b.flowGraph.Edges {
		if math.Abs(edgeTotals[edge.ID]-edge.RatePerMinute) > math.Max(1e-6, edge.RatePerMinute*1e-8) {
			return transportAllocationError(edge, edge.RatePerMinute-edgeTotals[edge.ID], "混带规范边未被完整守恒覆盖")
		}
	}
	for layer, load := range layerLoads {
		if load > summary.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("混带第%d圈实际负载%.3f/min超过所选单带%.3f/min", layer+1, load, summary.BeltCapacityPerMinute)}
		}
	}
	for part, load := range beltLoads {
		if load > summary.BeltCapacityPerMinute+1e-6 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("混带实体带%d实际共享负载%.3f/min超过所选单带%.3f/min", indices[part], load, summary.BeltCapacityPerMinute)}
		}
	}
	for arm, load := range armLoads {
		if err := validateBusInserter(arm, load); err != nil {
			return err
		}
	}
	summary.Metrics.LaneCount = len(summary.Lanes)
	checks := summary.Checks[:0]
	for _, check := range summary.Checks {
		if check.RuleID != "bus-flow-coverage" && check.RuleID != "bus-fleet" && check.RuleID != "bus-ring-capacity" {
			checks = append(checks, check)
		}
	}
	summary.Checks = append(checks, LogisticsCheck{RuleID: "bus-flow-coverage", Status: "passed", Message: "所有规范边已绑定实体闭环、逐物品过滤及连续供需接驳；共享干线和并联分拣器均已核算"}, LogisticsCheck{RuleID: "bus-ring-capacity", Status: "passed", Message: fmt.Sprintf("按用户带等级自动创建%d条独立共享混带环路，未升级传送带或缩减量化产线", len(ringSets))}, LogisticsCheck{RuleID: "bus-fleet", Status: "unverified", Message: "离线规则不模拟混带环路启动、堵带时序和真实游戏连续产能"})
	return nil
}

// validateBusFabricProducer 职责：证明过滤器实际从指定生产行输出干线抓取并进入此 lane 支路；使用：混带源端核验；边界：过滤标记和额定能力不能代替生产者/消费者的物理连接。
func validateBusFabricProducer(route *transportRoute) error {
	if route.from == nil || route.from.line == nil || len(route.from.line.output) == 0 || len(route.path) == 0 || route.start != route.path[len(route.path)-1] {
		return transportAllocationError(route.edge, route.rate, "混带生产者缺少真实输出干线或过滤支路")
	}
	trunk, branch := map[*placed]bool{}, map[*placed]bool{}
	row := route.from.line.output
	for current := row[len(row)-1]; current != nil && !trunk[current] && len(trunk) <= blueprint.MaxBuildingCount; current = current.output {
		trunk[current] = true
	}
	for _, belt := range route.path {
		branch[belt] = true
	}
	for _, arm := range route.filters {
		if arm == nil || !trunk[arm.input] || !branch[arm.output] || arm.input.owner != route.from.owner {
			return transportAllocationError(route.edge, route.rate, "混带过滤器没有连接指定生产者输出干线与此物品支路")
		}
	}
	return nil
}

// busFabricConsumerTail 职责：证明并联合流格沿原机器输入干线抵达正确产线；使用：环路取料带链核验；边界：未知物品、绕圈或错误输入行均失败，不把任意带端冒充消费者。
func busFabricConsumerTail(route *transportRoute) ([]*placed, error) {
	if route.to == nil || route.to.line == nil || route.end == nil {
		return nil, transportAllocationError(route.edge, route.rate, "混带消费者缺少真实机器输入行")
	}
	var terminal *placed
	for index, item := range route.to.recipe.Items {
		if item == route.edge.ItemID && index < len(route.to.line.inputs) && len(route.to.line.inputs[index]) > 0 {
			row := route.to.line.inputs[index]
			terminal = row[len(row)-1]
			break
		}
	}
	if terminal == nil && productionSprayItem(route.to.info) == route.edge.ItemID && route.to.line.coaterInput != nil && len(route.to.line.coaters) > 0 {
		// 增产剂不占机器配方槽；其物理消费者是跨过喷涂机附加区域的供剂带，不能要求存在配方输入分拣器。
		var last *placed
		for _, coater := range route.to.line.coaters {
			if coater != nil && (last == nil || coater.a.y > last.a.y) {
				last = coater
			}
		}
		if last != nil {
			wanted := vector{last.a.x - 1, last.a.y, 1}
			seen := map[*placed]bool{}
			for current := route.to.line.coaterInput; current != nil && !seen[current] && len(seen) <= blueprint.MaxBuildingCount; current = current.output {
				if current.a == wanted {
					terminal = current
					break
				}
				seen[current] = true
			}
		}
	}
	if terminal == nil {
		return nil, transportAllocationError(route.edge, route.rate, "混带物品不属于指定机器输入端")
	}
	path, seen := []*placed{}, map[*placed]bool{}
	for current := route.end; current != nil && !seen[current] && len(path) <= blueprint.MaxBuildingCount; current = current.output {
		path = append(path, current)
		if current == terminal {
			return path, nil
		}
		seen[current] = true
	}
	return nil, transportAllocationError(route.edge, route.rate, "混带取料未沿正确输入干线到达消费机器")
}

// validateBusFabricChain 职责：检查一条实体带链的有向连续性与成员身份；使用：环路两侧连接器；边界：空链和蓝图外引用拒绝。
func validateBusFabricChain(path []*placed, indices map[*placed]int) error {
	if len(path) == 0 {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带接驳带链为空"}
	}
	for i, part := range path {
		if part == nil || !surfaceIsBelt(part.record.ItemID) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带接驳包含非传送带端点"}
		}
		if _, ok := indices[part]; !ok {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带接驳引用蓝图外实体"}
		}
		if i > 0 && path[i-1].output != part {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("混带接驳带%d→%d的有向链中断", indices[path[i-1]], indices[part])}
		}
	}
	return nil
}

// validateBusFabricArms 职责：核验并联臂的过滤、圈归属、方向及真实容量；使用：环路注入与取料；边界：不能把一个过载臂的能力重复计算成多臂。
func validateBusFabricArms(arms []*placed, rate float64, item int, ring map[*placed]bool, feed bool) error {
	if len(arms) == 0 {
		return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "混带并联端口为空"}
	}
	seen := map[*placed]bool{}
	for _, arm := range arms {
		if seen[arm] {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "混带并联端口重复引用同一分拣器"}
		}
		seen[arm] = true
		if err := validateBusInserterShape(arm, item); err != nil {
			return err
		}
		if feed && (!ring[arm.output] || ring[arm.input]) || !feed && (!ring[arm.input] || ring[arm.output]) {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "混带过滤端口方向或共享环路不匹配"}
		}
		if err := validateBusInserter(arm, rate/float64(len(arms))); err != nil {
			return err
		}
	}
	return nil
}
