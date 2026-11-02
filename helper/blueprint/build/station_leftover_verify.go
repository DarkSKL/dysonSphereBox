package build

// 文件职责：独立核对塔副产物回收出带的真实库存槽、有向实体链及最终摘要边界。
// 使用：mode3/4 最终编号后；边界：保持普通生产塔账和内部份额，回收口不能仍指向源供货塔冒充实体导回。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// validateStationLeftovers 职责：核对全部实体回收路径、塔出货槽与真实原料锚点；使用：普通塔规范边提交前；边界：失效、重复、假锚点或跨闭合块均硬失败。
func (b *builder) validateStationLeftovers(ctx context.Context, indices map[*placed]int, records []blueprint.Building) error {
	type sourceKey struct {
		edge string
		belt *placed
	}
	seen := map[sourceKey]bool{}
	membership := func(p *placed) (blueprint.Building, bool) {
		index, ok := indices[p]
		if !ok || index < 0 || index >= len(records) {
			return blueprint.Building{}, false
		}
		return records[index], true
	}
	for _, route := range b.stationLeftovers {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物塔回收实体校验已取消"}
		}
		key := sourceKey{route.edge.ID, route.sourceBelt}
		if seen[key] || route.edge.FlowKind != "leftover" || !finitePositive(route.rate) || route.rate > b.logistics.BeltCapacityPerMinute+1e-6 || len(route.path) < 2 {
			return stationLeftoverFailure(route, "回收份额、路径或源路线重复/无效")
		}
		seen[key] = true
		expectedEdge := false
		for _, edge := range b.flowGraph.Edges {
			expectedEdge = expectedEdge || edge == route.edge
		}
		if !expectedEdge {
			return stationLeftoverFailure(route, "回收对象不属于未更改的规范剩余边")
		}
		tower, ok := membership(route.tower)
		_, rawExists := membership(route.rawHead)
		if !ok || !rawExists || !surfaceIsStation(tower.ItemID) || len(tower.Parameters) != 2048 || route.slot < 0 || route.slot >= 12 || route.itemSlot < 1 || route.itemSlot > 5 {
			return stationLeftoverFailure(route, "回收源塔、物品槽或原料锚点不在蓝图内")
		}
		sourceFound, rawFound, rawItemFound, rawItemAvailable := false, false, false, false
		sourceOwner := -1
		for _, link := range b.stationLinks {
			if link.output && link.tower == route.tower && link.belt == route.sourceBelt && link.item == route.edge.ItemID && link.rate+1e-6 >= route.rate {
				sourceFound, sourceOwner = true, link.owner
			}
		}
		for _, link := range b.stationLinks {
			if !link.output && link.owner == sourceOwner {
				if link.item == route.edge.ItemID {
					rawItemAvailable = true
					rawItemFound = rawItemFound || link.head == route.rawHead
				}
				if link.head == route.rawHead {
					rawFound = true
				}
			}
		}
		if !sourceFound || !rawFound {
			return stationLeftoverFailure(route, "没有真实同组生产入塔与原料接入路线")
		}
		if route.rawItemMatched != rawItemAvailable || route.rawItemMatched && !rawItemFound {
			return stationLeftoverFailure(route, "同物品入口优先选择标记与真实同组入口不一致")
		}
		cargo, port := (route.itemSlot-1)*6, 192+route.slot*4
		if tower.Parameters[cargo] != int32(route.edge.ItemID) || tower.Parameters[cargo+1] != 1 || tower.Parameters[port] != 1 || tower.Parameters[port+1] != int32(route.itemSlot) {
			return stationLeftoverFailure(route, "真实出带口物品选择错误或源生产供货角色被改写")
		}
		pathSeen := map[*placed]bool{}
		for index, belt := range route.path {
			record, exists := membership(belt)
			if !exists || pathSeen[belt] || belt.owner != sourceOwner || record.ItemID != int16(2000+b.setting.BeltLv) {
				return stationLeftoverFailure(route, "回收带包含幽灵、重复实体、跨产线或错误等级")
			}
			pathSeen[belt] = true
			if index == 0 {
				if belt.input != route.tower || record.InputObjIdx != int32(indices[route.tower]) || int(record.InputFromSlot) != route.slot || len(record.Parameters) < 2 || record.Parameters[0] != int32(route.edge.ItemID) || record.Parameters[1] != int32(math.Ceil(route.rate)) {
					return stationLeftoverFailure(route, "真实出带没有从所声明塔端口取出正确库存物品")
				}
			} else {
				previous := route.path[index-1]
				previousRecord, _ := membership(previous)
				if previous.output != belt || previousRecord.OutputObjIdx != int32(indices[belt]) || previousRecord.OutputToSlot != 1 {
					return stationLeftoverFailure(route, "回收链存在断带、错向或错误带槽")
				}
			}
		}
		last := route.path[len(route.path)-1]
		end, _ := membership(last)
		if last.output != nil || end.OutputObjIdx != -1 || len(end.Parameters) < 2 || end.Parameters[0] != int32(route.edge.ItemID) || end.Parameters[1] != int32(math.Ceil(route.rate)) {
			return stationLeftoverFailure(route, "开放回收终点未保留真实剩余物品与份额标记")
		}
		offset := vector{last.a.x - route.rawHead.a.x, last.a.y - route.rawHead.a.y, last.a.z - route.rawHead.a.z}
		if !flySamePoint(offset, route.offset) || route.offset.x > -4 || route.offset.y < 2 || math.Abs(last.a.z) > 1e-6 {
			return stationLeftoverFailure(route, "真实回收终点已离开本组原料接入侧的独立位置")
		}
		// 球面会刚性移动闭合块；塔、原料带和整条新链必须同块。
		if len(b.surfaceBlocks) > 0 {
			blockOf := func(p *placed) int {
				for index, block := range b.surfaceBlocks {
					if indices[p] >= block.begin && indices[p] < block.end {
						return index
					}
				}
				return -1
			}
			block := blockOf(route.tower)
			if block < 0 || blockOf(route.rawHead) != block {
				return stationLeftoverFailure(route, "副产物回收跨越可独立平移的闭合塔组")
			}
			for _, p := range route.path {
				if blockOf(p) != block {
					return stationLeftoverFailure(route, "回收带没有完整包含在所属闭合塔组")
				}
			}
		}
	}
	return nil
}

// applyStationLeftoverLanes 职责：把已绑定生产份额的剩余边指向真实回收终点；使用：普通规范边 lane 创建之后；边界：不添加第二份流量，不修改源槽和InternalConnected结论。
func (b *builder) applyStationLeftoverLanes(edge quant.FlowEdge, uses []stationLinkUse, laneBegin, portBegin int, indices map[*placed]int) error {
	if edge.FlowKind != "leftover" {
		return nil
	}
	if len(b.logistics.Lanes)-laneBegin != len(uses) || len(b.logistics.ExternalPorts)-portBegin != len(uses) {
		return stationEdgeError(edge, "副产物规范份额与真实回收lane数量不一致")
	}
	for index, use := range uses {
		var found *stationLeftoverReturn
		for routeIndex := range b.stationLeftovers {
			route := &b.stationLeftovers[routeIndex]
			if route.edge.ID == edge.ID && route.sourceBelt == use.link.belt {
				if found != nil {
					return stationEdgeError(edge, "一个生产份额重复声明回收链")
				}
				found = route
			}
		}
		if found == nil || math.Abs(found.rate-use.rate) > math.Max(1e-6, use.rate*1e-8) {
			return stationEdgeError(edge, "剩余边没有与生产源份额完全一致的真实塔出带")
		}
		lane := &b.logistics.Lanes[laneBegin+index]
		known := map[int]bool{}
		for _, building := range lane.BuildingIndices {
			known[building] = true
		}
		for _, p := range found.path {
			building := indices[p]
			if !known[building] {
				lane.BuildingIndices = append(lane.BuildingIndices, building)
				known[building] = true
			}
		}
		last := found.path[len(found.path)-1]
		port := &b.logistics.ExternalPorts[portBegin+index]
		port.BuildingIndex, port.X, port.Y, port.Z = indices[last], last.a.x, last.a.y, last.a.z
	}
	return nil
}

// stationLeftoverCheck 职责：披露已核对的真实导回范围及运行时限制；使用：塔物流摘要收尾；边界：不把站内取货优先级或机队吞吐当作已达产。
func (b *builder) stationLeftoverCheck() {
	if len(b.stationLeftovers) == 0 {
		return
	}
	matched := 0
	for _, route := range b.stationLeftovers {
		if route.rawItemMatched {
			matched++
		}
	}
	message := fmt.Sprintf("%d 条剩余副产物lane（%d 条靠近同物品的真实原料入口，%d 条仅归集到本组原料侧）已从真实供货塔物品库存经独立筛选带导至开放回收端；回收端不接入或混入原料带，生产供货槽、内部消费及外供份额保持规范图原值。请单独接入回收/储存或其它消费设施；塔库存分配时序、机队和游戏连续产量仍未验证", len(b.stationLeftovers), matched, len(b.stationLeftovers)-matched)
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "leftover-input-boundary", Status: "passed", Message: message})
	b.warnings = append(b.warnings, message)
}
