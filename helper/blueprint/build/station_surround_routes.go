package build

// 文件职责：在中央塔组周围按真实障碍和未来端口净空寻找三维带路。
// 使用：环绕塔平面接驳无解时复用统一有界 A*；边界：不无限加高，不覆盖其它带，任一路线失败即拒绝整组。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// routeSurroundStationBridges 职责：为中央塔组逐路规划低层优先的真实三维连接；使用：物流塔环绕布局后备；边界：复用三十二维高度及三十万展开上限，不发送部分成功建筑。
func (b *builder) routeSurroundStationBridges(ctx context.Context, routes []stationRoute, centers []vector, existing []*placed, leftoverSlots [][]int) error {
	blocked, err := stationBuildingObstacles(ctx, existing)
	if err != nil {
		return err
	}
	low, high := bounds(existing)
	for _, center := range centers {
		low.x, low.y = math.Min(low.x, center.x-10), math.Min(low.y, center.y-10)
		high.x, high.y = math.Max(high.x, center.x+10), math.Max(high.y, center.y+10)
		for x := -3.; x <= 3; x++ {
			for y := -3.; y <= 3; y++ {
				for z := 0.; z <= transportMaxHeight; z++ {
					blocked[vector{center.x + x, center.y + y, z}] = true
				}
			}
		}
	}
	env := &transportEnvironment{blocked: blocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(low.x) - 12, math.Floor(low.y) - 12, 0}, maximum: vector{math.Ceil(high.x) + 12, math.Ceil(high.y) + 12, transportMaxHeight}}
	for tower, slots := range leftoverSlots {
		for _, slot := range slots {
			outer := slotOffset(slot)[0]
			outer.x, outer.y = outer.x+centers[tower].x, outer.y+centers[tower].y
			direction := stationApproach(slot)
			for _, point := range append(stationInner(centers[tower], slot), outer, vector{outer.x + direction.x, outer.y + direction.y, 0}) {
				for z := 0.; z <= transportMaxHeight; z++ {
					env.blocked[beltCell(vector{point.x, point.y, z})] = true
				}
			}
		}
	}
	for _, building := range existing {
		if surfaceIsBelt(building.record.ItemID) {
			env.blocked[beltCell(building.a)] = true
		}
		if building.record.ItemID >= 2011 && building.record.ItemID <= 2014 {
			for x := math.Floor(math.Min(building.a.x, building.b.x)); x <= math.Ceil(math.Max(building.a.x, building.b.x)); x++ {
				for y := math.Floor(math.Min(building.a.y, building.b.y)); y <= math.Ceil(math.Max(building.a.y, building.b.y)); y++ {
					env.blocked[vector{x, y, math.Round(building.a.z)}] = true
				}
			}
		}
	}
	endpoints := make([]*placed, len(routes))
	for i, route := range routes {
		end := slotOffset(route.slot)[0]
		end.x += centers[route.tower].x
		end.y += centers[route.tower].y
		endpoints[i] = &placed{a: end}
		for _, point := range stationInner(centers[route.tower], route.slot) {
			// 自己的塔内尾段同样不可作绕行通路，否则追加尾段时会折返并重叠。
			env.blocked[beltCell(point)] = true
		}
		points := append([]vector{route.head.a, end}, stationInner(centers[route.tower], route.slot)...)
		for _, point := range points {
			owner := endpoints[i]
			if point == route.head.a {
				owner = route.head
			}
			for z := 0.; z <= transportMaxHeight; z++ {
				cell := beltCell(vector{point.x, point.y, z})
				if env.reserved[cell] == nil {
					env.reserved[cell] = map[*placed]bool{}
				}
				env.reserved[cell][owner] = true
			}
		}
	}
	for i := range routes {
		route := &routes[i]
		path, expanded := env.findTransportPath(ctx, route.head.a, endpoints[i].a, route.head, endpoints[i], false)
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "中央物流塔三维接驳已取消或超时"}
		}
		if len(path) < 2 {
			return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("中央物流塔物品 %d、%.3f/min、端口 %d 从 %+v 到 %+v 在固定三维预算内无可用路线（展开 %d/%d，高度 %d）", route.item, route.rate, route.slot, route.head.a, endpoints[i].a, expanded, transportSearchLimit, transportMaxHeight)}
		}
		route.path = append([]vector(nil), path[1:]...)
		for _, point := range append(append([]vector(nil), path...), stationInner(centers[route.tower], route.slot)...) {
			env.blocked[beltCell(point)] = true
		}
		env.version++
	}
	return nil
}
