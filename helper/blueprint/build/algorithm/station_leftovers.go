package build

// 文件职责：把物流塔规范剩余份额装配成塔真实出带到本组原料入口侧的独立回收链。
// 使用：mode3/4 每个闭合塔组完成后；边界：生产塔槽与内部份额保持原值，回收不再记为第二份生产，不依赖机队送回。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
	"DysonSphereBox/helper/quant"
)

// stationLeftoverReturn 职责：保存规范剩余边的一份真实塔出带；使用：最终独立校验和摘要绑定；边界：sourceBelt仍是唯一生产账，path是塔库存离开后的物理链。
type stationLeftoverReturn struct {
	edge           quant.FlowEdge
	rate           float64
	tower          *placed
	sourceBelt     *placed
	rawHead        *placed
	rawItemMatched bool
	slot           int
	itemSlot       int
	path           []*placed
	offset         vector
}

// stationLeftoverKinds 职责：找出生产节点有规范剩余边的物品；使用：保守预留真实塔出口；边界：不从配方毛产物猜测剩余或改变规范流量。
func (b *builder) stationLeftoverKinds(owner int) map[int]bool {
	return stationlayout.LeftoverKinds(b.flowGraph, owner)
}

// stationLeftoverReservedSlots 职责：为每个可能含剩余份额的机器行预留独立出带口；使用：普通路线寻路之前；边界：最多十二口，真实剩余率仍在组内装配时精确分配。
func (b *builder) stationLeftoverReservedSlots(lines []*line, groups [][]stationItem) [][]int {
	reserved := make([][]int, len(groups))
	if len(lines) == 0 {
		return reserved
	}
	kinds := b.stationLeftoverKinds(lines[0].machines[0].owner)
	priority := []int{9, 10, 11, 8, 0, 7, 1, 6, 2, 5, 3, 4}
	for tower, group := range groups {
		count := 0
		for _, item := range group {
			if item.provide && kinds[item.id] {
				count += len(lines)
			}
		}
		reserved[tower] = append([]int(nil), priority[:min(count, len(priority))]...)
	}
	return reserved
}

// stationUnreservedSlots 职责：按既有稳定顺序过滤副产物专用口；使用：普通塔路线三维回退；边界：不能用回退覆盖已预留的物理端口。
func stationUnreservedSlots(priority, reserved []int) []int {
	used := map[int]bool{}
	for _, slot := range reserved {
		used[slot] = true
	}
	available := []int{}
	for _, slot := range priority {
		if !used[slot] {
			available = append(available, slot)
		}
	}
	return available
}

// stationLeftoverUse 职责：保存某真实生产入塔路线与剩余区间的交集；使用：本组实体出带；边界：部分交集不减少或搬走仍服务内部边的生产入塔路线。
type stationLeftoverUse struct {
	linkIndex int
	edge      quant.FlowEdge
	rate      float64
}

// planStationLeftovers 职责：重放规范边和真实生产路线的稳定份额分配；使用：每组生成前；边界：只返回本组交集，不消耗或改写任何原始生产份额。
func (b *builder) planStationLeftovers(ctx context.Context, linkBegin int) ([]stationLeftoverUse, error) {
	byOwner := map[int]string{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			byOwner[node.ProductionIndex] = node.ID
		}
	}
	position := map[string]float64{}
	uses := []stationLeftoverUse{}
	for index, link := range b.stationLinks {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "塔副产物份额规划已取消"}
		}
		if !link.output {
			continue
		}
		key := stationFlowKey(link.owner, link.item, true)
		start, end := position[key], position[key]+link.rate
		position[key] = end
		if index < linkBegin {
			continue
		}
		cursor := 0.
		for _, edge := range b.flowGraph.Edges {
			if edge.SourceNodeID != byOwner[link.owner] || edge.ItemID != link.item {
				continue
			}
			next := cursor + edge.RatePerMinute
			part := math.Min(end, next) - math.Max(start, cursor)
			if edge.FlowKind == "leftover" && part > math.Max(1e-8, edge.RatePerMinute*1e-10) {
				uses = append(uses, stationLeftoverUse{linkIndex: index, edge: edge, rate: part})
			}
			cursor = next
		}
	}
	return uses, nil
}

// prepareStationLeftovers 职责：事务式提交本组真实副产物塔出口；使用：普通塔链已装配、组装箱之前；边界：错误或取消恢复已有建筑和塔参数，所有新增引用保留在同一闭合块内。
func (b *builder) prepareStationLeftovers(ctx context.Context, enabled bool, blockBegin, linkBegin int, towers []*placed, reserved [][]int) (err error) {
	if !enabled || b.flowGraph == nil {
		return nil
	}
	uses, err := b.planStationLeftovers(ctx, linkBegin)
	if err != nil || len(uses) == 0 {
		return err
	}
	begin, returnBegin := len(b.all), len(b.stationLeftovers)
	snapshots := map[*placed]placed{}
	for _, p := range b.all[blockBegin:] {
		snapshot := *p
		snapshot.record.Parameters = append([]int32(nil), p.record.Parameters...)
		snapshots[p] = snapshot
	}
	owner := b.owner
	defer func() {
		b.owner = owner
		if err == nil {
			return
		}
		for p, snapshot := range snapshots {
			*p = snapshot
		}
		b.all, b.stationLeftovers = b.all[:begin], b.stationLeftovers[:returnBegin]
	}()
	var rawHead *placed
	for _, link := range b.stationLinks[linkBegin:] {
		if !link.output && link.head != nil && (rawHead == nil || link.head.a.x < rawHead.a.x || link.head.a.x == rawHead.a.x && link.head.a.y < rawHead.a.y) {
			rawHead = link.head
		}
	}
	if rawHead == nil {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "副产物塔组没有真实原料接入带"}
	}
	if len(reserved) != len(towers) {
		return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "副产物预留端口与真实塔数量不一致"}
	}
	used := make([]int, len(towers))
	future := map[vector]bool{}
	for towerIndex, slots := range reserved {
		seen := map[int]bool{}
		for _, slot := range slots {
			if towers[towerIndex] == nil || slot < 0 || slot >= 12 || seen[slot] {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: "副产物预留端口越界、重复或缺少真实塔"}
			}
			seen[slot] = true
			outer := slotOffset(slot)[0]
			outer.x, outer.y = outer.x+towers[towerIndex].a.x, outer.y+towers[towerIndex].a.y
			inner := stationInner(towers[towerIndex].a, slot)
			if b.denseSpatial {
				outer = denseStationOuter(towers[towerIndex].a, slot)
				inner = denseStationInner(towers[towerIndex].a, slot)
			}
			direction := stationApproach(slot)
			for _, point := range append(inner, outer, vector{outer.x + direction.x, outer.y + direction.y, 0}) {
				future[beltCell(point)] = true
			}
		}
	}
	minimum, maximum := bounds(b.all[blockBegin:])
	for index, use := range uses {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物塔出带装配已取消"}
		}
		link := b.stationLinks[use.linkIndex]
		towerIndex := -1
		for i, tower := range towers {
			if tower == link.tower {
				towerIndex = i
			}
		}
		if towerIndex < 0 || used[towerIndex] >= len(reserved[towerIndex]) {
			return stationEdgeError(use.edge, "本组预留的真实副产物出带口不足")
		}
		selectedRawHead, matched := b.stationLeftoverInputHead(linkBegin, use)
		if selectedRawHead == nil {
			selectedRawHead = rawHead
			if b.denseSpatial {
				// 全场共享塔不再隐含“当前组只有一个 owner”；非同物品回收也必须关联本生产的真实原料入口。
				selectedRawHead = nil
				for _, candidate := range b.stationLinks[linkBegin:] {
					if !candidate.output && candidate.owner == link.owner && (selectedRawHead == nil || candidate.head.a.x < selectedRawHead.a.x) {
						selectedRawHead = candidate.head
					}
				}
				if selectedRawHead == nil {
					return stationEdgeError(use.edge, "共享塔副产物没有本生产的真实原料入口")
				}
			}
		}
		slot := reserved[towerIndex][used[towerIndex]]
		used[towerIndex]++
		target := vector{math.Floor(minimum.x) - 5, math.Ceil(selectedRawHead.a.y) + 2 + 3*float64(index), 0}
		if b.denseSpatial {
			target = b.denseStationLeftoverTarget(selectedRawHead, minimum, maximum)
		}
		b.owner = link.owner
		if err = b.addStationLeftover(ctx, use, selectedRawHead, matched, slot, target, blockBegin, future); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物塔出带提交已取消"}
	}
	if len(b.all) > blueprint.MaxBuildingCount {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "副产物真实塔出带超过20万建筑预算"}
	}
	return nil
}

// stationLeftoverInputHead 职责：优先选取与副产物同生产节点、同物品的真实机器入口；使用：定位对应原料侧回收端；边界：无匹配入口时返回空值，调用方只能退回原料侧空间归集，不能把不同物料接成循环。
func (b *builder) stationLeftoverInputHead(linkBegin int, use stationLeftoverUse) (*placed, bool) {
	if use.linkIndex < 0 || use.linkIndex >= len(b.stationLinks) {
		return nil, false
	}
	owner := b.stationLinks[use.linkIndex].owner
	var head *placed
	for _, link := range b.stationLinks[linkBegin:] {
		if link.output || link.owner != owner || link.item != use.edge.ItemID || link.head == nil {
			continue
		}
		if head == nil || link.head.a.x < head.a.x || link.head.a.x == head.a.x && link.head.a.y < head.a.y {
			head = link.head
		}
	}
	return head, head != nil
}

// stationLeftoverFailure 职责：补齐真实回收链的规范边上下文；使用：实体和最终解码检查；边界：不发布缺链回收边界。
func stationLeftoverFailure(route stationLeftoverReturn, reason string) error {
	return stationEdgeError(route.edge, fmt.Sprintf("副产物真实塔出带 %.6f/min：%s", route.rate, reason))
}
