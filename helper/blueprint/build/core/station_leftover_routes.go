package build

// 文件职责：在预留塔口与本组原料入口侧之间装配真实有向回收带。
// 使用：副产物组内事务；边界：固定所选带等级、有限三维寻路与完整占位检查，不穿其它塔或占用普通物料端口。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// addStationLeftover 职责：从真实供货库存槽建立筛选出带并导到独立开放端；使用：逐规范剩余份额；边界：不更改本来供货角色，不减少内部份额或假造另一个生产节点。
func (b *builder) addStationLeftover(ctx context.Context, use stationLeftoverUse, rawHead *placed, rawItemMatched bool, slot int, target vector, blockBegin int, future map[vector]bool) error {
	link := b.stationLinks[use.linkIndex]
	tower := link.tower
	route := stationLeftoverReturn{edge: use.edge, rate: use.rate, tower: tower, sourceBelt: link.belt, rawHead: rawHead, rawItemMatched: rawItemMatched, slot: slot}
	if tower == nil || len(tower.record.Parameters) != 2048 || slot < 0 || slot >= 12 || !finitePositive(use.rate) || use.rate > b.logistics.BeltCapacityPerMinute+1e-6 {
		return stationLeftoverFailure(route, "塔、物理端口或所选单带容量无效")
	}
	for _, other := range b.stationLinks {
		if other.tower == tower && other.slot == slot {
			return stationLeftoverFailure(route, "预留出带口已被普通物料路线占用")
		}
	}
	for _, other := range b.stationLeftovers {
		if other.tower == tower && other.slot == slot {
			return stationLeftoverFailure(route, "真实副产物出带口被重复占用")
		}
	}
	for itemSlot := 0; itemSlot < 5; itemSlot++ {
		if tower.record.Parameters[itemSlot*6] == int32(link.item) && tower.record.Parameters[itemSlot*6+1] == 1 {
			route.itemSlot = itemSlot + 1
			break
		}
	}
	if route.itemSlot == 0 {
		return stationLeftoverFailure(route, "源塔没有保留同物品的真实供货库存槽")
	}
	prefix := stationInner(tower.a, slot)
	if b.denseSpatial {
		prefix = denseStationInner(tower.a, slot)
	}
	for i, j := 0, len(prefix)-1; i < j; i, j = i+1, j-1 {
		prefix[i], prefix[j] = prefix[j], prefix[i]
	}
	outer := slotOffset(slot)[0]
	outer.x, outer.y = outer.x+tower.a.x, outer.y+tower.a.y
	if b.denseSpatial {
		outer = denseStationOuter(tower.a, slot)
	}
	prefix = append(prefix, outer)
	existing := b.all[blockBegin:]
	withoutTower := make([]*placed, 0, len(existing)-1)
	for _, p := range existing {
		if p != tower {
			withoutTower = append(withoutTower, p)
		}
	}
	if err := b.validateFlyPathCandidate(prefix, tower, nil, withoutTower); err != nil {
		return stationLeftoverFailure(route, err.Error())
	}
	points, err := b.stationLeftoverPath(ctx, outer, target, existing, future, stationApproach(slot))
	if err != nil {
		return stationLeftoverFailure(route, err.Error())
	}
	points = append(prefix, points[1:]...)
	route.path = b.byPath(points)
	first, last := route.path[0], route.path[len(route.path)-1]
	first.input, first.record.InputFromSlot = tower, int8(slot)
	first.record.Parameters = []int32{int32(link.item), int32(math.Ceil(use.rate))}
	last.record.Parameters = []int32{int32(link.item), int32(math.Ceil(use.rate))}
	base := 192 + slot*4
	tower.record.Parameters[base], tower.record.Parameters[base+1] = 1, int32(route.itemSlot)
	route.offset = vector{last.a.x - rawHead.a.x, last.a.y - rawHead.a.y, last.a.z - rawHead.a.z}
	b.stationLeftovers = append(b.stationLeftovers, route)
	return nil
}

// stationLeftoverPath 职责：为塔口外侧到原料接入侧建立有界三维路径；使用：塔内微调路径已独立验证后；边界：组外建筑不在当前局部坐标中，禁止误当障碍或创建跨组连接。
func (b *builder) stationLeftoverPath(ctx context.Context, start, end vector, existing []*placed, future map[vector]bool, approach vector) ([]vector, error) {
	minimum, maximum := bounds(existing)
	blocked, err := stationBuildingObstacles(ctx, existing)
	if err != nil {
		return nil, err
	}
	for _, p := range existing {
		if surfaceIsBelt(p.record.ItemID) {
			blocked[beltCell(p.a)] = true
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			for x := math.Floor(math.Min(p.a.x, p.b.x)); x <= math.Ceil(math.Max(p.a.x, p.b.x)); x++ {
				for y := math.Floor(math.Min(p.a.y, p.b.y)); y <= math.Ceil(math.Max(p.a.y, p.b.y)); y++ {
					blocked[vector{x, y, math.Round(p.a.z)}] = true
				}
			}
		}
	}
	for point := range future {
		if point == beltCell(start) || point == beltCell(vector{start.x + approach.x, start.y + approach.y, 0}) {
			continue
		}
		for z := 0.; z <= transportMaxHeight; z++ {
			blocked[vector{point.x, point.y, z}] = true
		}
	}
	env := &transportEnvironment{
		blocked: blocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{},
		minimum: vector{math.Floor(math.Min(minimum.x, end.x)) - 8, math.Floor(math.Min(minimum.y, end.y)) - 8, 0},
		maximum: vector{math.Ceil(math.Max(maximum.x, end.x)) + 8, math.Ceil(math.Max(maximum.y, end.y)) + 8, transportMaxHeight},
	}
	points, expanded := env.findTransportPath(ctx, start, end, nil, nil, false)
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "副产物塔回收带寻路已取消"}
	}
	if len(points) < 2 || len(points) > 4096 {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("副产物塔回收带从%+v到%+v没有预算内的真实通路（展开%d）", start, end, expanded)}
	}
	if err := b.validateFlyPathCandidate(points, &placed{owner: b.owner}, nil, existing); err != nil {
		return nil, err
	}
	return points, nil
}
