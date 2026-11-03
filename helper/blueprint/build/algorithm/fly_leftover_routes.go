package build

// 文件职责：为配送副产物装配独立回收支路、箱体和完整有向带。
// 使用：副产物迁移事务内部；边界：不放宽占位、等级或槽位规则，无法路由由事务整体恢复。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// flyLocalInputAnchor 职责：取得当前闭合块真实机器原料配送箱；使用：本组副产物回收；边界：允许原料来自内部物流，但必须是实体输入链，不能用其它组或假箱体。
func (b *builder) flyLocalInputAnchor(entities []*placed) *placed {
	anchor, _ := b.flyLocalInputAnchorForItem(entities, 0, -1)
	return anchor
}

// flyLocalInputAnchorForItem 职责：优先取得与副产物同物品的真实原料配送箱；使用：副产物导回原料侧；边界：找不到同物品入口时只能退回本闭合块的原料侧空间锚点，不把回收带伪装成物料循环。
func (b *builder) flyLocalInputAnchorForItem(entities []*placed, item, owner int) (*placed, bool) {
	inside := map[*placed]bool{}
	for _, p := range entities {
		inside[p] = true
	}
	var anchor, fallback *placed
	for _, endpoint := range b.flyEndpoints {
		if !endpoint.input || endpoint.box == nil || !inside[endpoint.box] || !inside[endpoint.link] {
			continue
		}
		if owner >= 0 && endpoint.owner != owner {
			continue
		}
		if fallback == nil || endpoint.box.a.x < fallback.a.x || endpoint.box.a.x == fallback.a.x && endpoint.box.a.y > fallback.a.y {
			fallback = endpoint.box
		}
		if item > 0 && endpoint.item != item {
			continue
		}
		if anchor == nil || endpoint.box.a.x < anchor.a.x || endpoint.box.a.x == anchor.a.x && endpoint.box.a.y > anchor.a.y {
			anchor = endpoint.box
		}
	}
	if anchor == nil {
		return fallback, false
	}
	return anchor, true
}

// moveFlyLeftover 职责：迁移全份额箱体或为部分份额新建过滤支路；使用：逐剩余 lane；边界：完整源链、箱臂和回收路径必须同时成功，调用方负责整体回滚。
func (b *builder) moveFlyLeftover(ctx context.Context, move flyLeftoverMove) error {
	original := move.endpoint
	if original == nil || original.link == nil || original.box == nil || original.delivery == nil || original.input {
		return transportAllocationError(move.edge, move.rate, "副产物迁移缺少真实机器带、箱体或供货配送端点")
	}
	anchor, matched := b.flyLocalInputAnchorForItem(b.all, move.edge.ItemID, original.owner)
	if anchor == nil {
		return transportAllocationError(move.edge, move.rate, "副产物回收没有真实外部原料箱体可作为空间接口区")
	}
	b.owner = original.owner
	endpoint := original
	boxTemplate, deliveryTemplate := original.box.record, original.delivery.record
	if math.Abs(original.rate-move.rate) <= math.Max(1e-6, original.rate*1e-8) {
		removed := map[*placed]bool{original.box: true, original.delivery: true}
		for _, entity := range append(append([]*placed(nil), original.belts...), original.arms...) {
			removed[entity] = true
		}
		if original.arm != nil {
			removed[original.arm] = true
		}
		remaining := make([]*placed, 0, len(b.all))
		for _, entity := range b.all {
			if !removed[entity] {
				remaining = append(remaining, entity)
			}
		}
		b.all = remaining
		original.link.output = nil
		original.link.record.OutputToSlot = -1
	} else {
		if move.rate <= 0 || move.rate >= original.rate {
			return transportAllocationError(move.edge, move.rate, "副产物拆分份额不在真实端点流量范围内")
		}
		source := original.link
		// 单产物旧端点有带间过滤臂，必须从其机器侧干线另取，
		// 不把两个串联过滤级的能力错误相加为并行吞吐。
		if len(original.filters) > 0 {
			source = original.filters[len(original.filters)-1].input
		}
		var branch *placed
		for _, delta := range []vector{{0, -1, 0}, {0, 1, 0}, {-1, 0, 0}, {1, 0, 0}} {
			point := vector{source.a.x + delta.x, source.a.y + delta.y, source.a.z}
			if err := b.validateFlyPathCandidate([]vector{source.a, point}, source, nil, b.all); err == nil {
				branch = b.make(2000+b.setting.BeltLv, point.x, point.y, point.z)
				break
			}
		}
		if branch == nil {
			return transportAllocationError(move.edge, move.rate, "共享产物没有独立副产物过滤支路的真实槽位")
		}
		filter, err := b.beltTransfer(source, branch, b.setting.InserterLv, move.edge.ItemID)
		if err != nil {
			return err
		}
		endpoint = &flyTransportEndpoint{owner: original.owner, item: original.item, link: branch, filters: []*placed{filter}, rate: move.rate}
		original.rate -= move.rate
		b.flyEndpoints = append(b.flyEndpoints, endpoint)
	}
	endpoint.flowEdgeID, endpoint.leftoverAnchor, endpoint.leftoverAnchorMatched, endpoint.rate = move.edge.ID, anchor, matched, move.rate
	endpoint.arm, endpoint.arms, endpoint.belts = nil, nil, nil
	_, high := bounds(b.all)
	center := vector{anchor.a.x, math.Ceil(high.y) + 6, 0}
	box := b.make(2101, center.x, center.y, center.z)
	box.record = boxTemplate
	box.record.Parameters = append([]int32(nil), boxTemplate.Parameters...)
	delivery := b.make(2107, center.x, center.y, 1.875)
	delivery.record = deliveryTemplate
	delivery.record.Parameters = append([]int32(nil), deliveryTemplate.Parameters...)
	delivery.input = box
	endpoint.box, endpoint.delivery = box, delivery
	storage, err := planFlyStorageArms(move.rate, b.setting.InserterLv)
	if err != nil {
		return err
	}
	tail := storage.beltPoints(center)
	points, err := b.flyLeftoverPath(ctx, endpoint.link, tail)
	if err != nil {
		return transportAllocationError(move.edge, move.rate, err.Error())
	}
	endpoint.belts = b.byPath(points[1:])
	connect(endpoint.link, endpoint.belts[0])
	endpoint.belts[len(endpoint.belts)-1].record.Parameters = []int32{int32(endpoint.item), int32(math.Ceil(move.rate))}
	for index, slot := range storage.slots {
		arm := b.make(2010+b.setting.InserterLv, 0, 0, 0)
		belt := endpoint.belts[len(endpoint.belts)-len(storage.slots)+index]
		if err := flyArmGeometry(arm, box, belt, slot, false); err != nil {
			return err
		}
		arm.record.FilterID = int16(endpoint.item)
		endpoint.arms = append(endpoint.arms, arm)
		if endpoint.arm == nil {
			endpoint.arm = arm
		}
	}
	if len(b.all) > blueprint.MaxBuildingCount {
		return transportAllocationError(move.edge, move.rate, "副产物实体回收带超过20万建筑预算")
	}
	return nil
}

// flyLeftoverPath 职责：复用有界三维寻路为独立回收箱建立完整真带；使用：箱体已落位、箱臂未装配时；边界：尾带槽位预留，最终严格碰撞仍为准入条件。
func (b *builder) flyLeftoverPath(ctx context.Context, link *placed, tail []vector) ([]vector, error) {
	minimum, maximum := bounds(b.all)
	blocked, err := stationBuildingObstacles(ctx, b.all)
	if err != nil {
		return nil, err
	}
	for _, entity := range b.all {
		if surfaceIsBelt(entity.record.ItemID) {
			blocked[beltCell(entity.a)] = true
		}
		if entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
			for x := math.Floor(math.Min(entity.a.x, entity.b.x)); x <= math.Ceil(math.Max(entity.a.x, entity.b.x)); x++ {
				for y := math.Floor(math.Min(entity.a.y, entity.b.y)); y <= math.Ceil(math.Max(entity.a.y, entity.b.y)); y++ {
					blocked[vector{x, y, math.Round(entity.a.z)}] = true
				}
			}
		}
	}
	for _, point := range tail[1:] {
		blocked[beltCell(point)] = true
	}
	env := &transportEnvironment{blocked: blocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(minimum.x) - 8, math.Floor(minimum.y) - 8, 0}, maximum: vector{math.Ceil(maximum.x) + 8, math.Ceil(maximum.y) + 8, transportMaxHeight}}
	points, _ := env.findTransportPath(ctx, link.a, tail[0], link, nil, false)
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送副产物实体路由已取消或超时"}
	}
	if len(points) < 2 || len(points)+len(tail)-1 > 4096 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配送副产物原料区回收箱没有预算内可达的独立带路"}
	}
	points = append(points, tail[1:]...)
	if err := b.validateFlyPathCandidate(points, link, nil, b.all); err != nil {
		return nil, err
	}
	return points, nil
}
