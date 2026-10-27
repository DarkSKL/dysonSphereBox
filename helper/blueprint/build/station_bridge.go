package build

// 文件职责：为拥挤入口寻找有界高架接驳；使用：平面物流塔布线失败后；边界：逐格避免已有带及同组路线，不把高架自检冒充游戏碰撞验收。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"math"
)

// beltCell 职责：把微移带点归一到占用格；使用：同高度避碰；边界：保留层高，不利用 0.002 微移绕过真正重叠。
func beltCell(v vector) vector { return vector{math.Round(v.x), math.Round(v.y), math.Round(v.z)} }

// bridgeStationRoutes 职责：先验证升降列，再在高架平面执行有界 A*；使用：塔策略的显式后备；边界：失败整体拒绝，不写出半条接驳。
func (b *builder) bridgeStationRoutes(ctx context.Context, routes []stationRoute, centers []vector) error {
	occupied := map[vector]bool{}
	maxZ := 0.
	for _, p := range b.all {
		if p.record.ItemID > 2000 && p.record.ItemID < 2010 {
			occupied[beltCell(p.a)] = true
			maxZ = math.Max(maxZ, p.a.z)
		}
	}
	reserved := map[vector]int{}
	for i, route := range routes {
		end := slotOffset(route.slot)[0]
		end.x += centers[route.tower].x
		end.y += centers[route.tower].y
		reserved[beltCell(end)] = i
		for _, p := range stationInner(centers[route.tower], route.slot) {
			reserved[beltCell(p)] = i
		}
	}
	for i := range routes {
		route := &routes[i]
		end := slotOffset(route.slot)[0]
		end.x += centers[route.tower].x
		end.y += centers[route.tower].y
		direction := vector{1, 0, 0}
		switch {
		case route.slot <= 2:
			direction = vector{0, 1, 0}
		case route.slot <= 5:
			direction = vector{-1, 0, 0}
		case route.slot <= 8:
			direction = vector{0, -1, 0}
		}
		found := false
		reason := ""
		for attempt := 0; attempt < 192; attempt++ {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔高架寻路超过截止时间"}
			}
			begin := route.head.a
			begin.x -= float64(1 + attempt%4)
			offsets := []float64{0, 1, -1, 2, -2, 3, -3, 4}
			perpendicular := offsets[(attempt/4)%len(offsets)]
			distance := float64(1 + (attempt/32)%6)
			drop := vector{end.x + direction.x*distance - direction.y*perpendicular, end.y + direction.y*distance + direction.x*perpendicular, 0}
			level := int(math.Ceil(maxZ)) + 2 + i + attempt/96
			points := append([]vector{}, manhattan(route.head.a, begin, true)[1:]...)
			for z := 1; z <= level; z++ {
				points = append(points, vector{begin.x + .002*float64(z), begin.y, float64(z)})
			}
			down := []vector{}
			for z := level - 1; z >= 0; z-- {
				down = append(down, vector{drop.x + .002*float64(z), drop.y, float64(z)})
			}
			groundBlocked := map[vector]bool{}
			for p, owner := range reserved {
				if owner != i {
					groundBlocked[p] = true
				}
			}
			for p := range occupied {
				if p.z == 0 {
					groundBlocked[p] = true
				}
			}
			for _, center := range centers {
				for x := -3.; x <= 3; x++ {
					for y := -3.; y <= 3; y++ {
						groundBlocked[vector{center.x + x, center.y + y, 0}] = true
					}
				}
			}
			ground := findPath(ctx, drop, end, attempt%2 == 0, groundBlocked)
			if len(ground) < 2 {
				reason = "落地接驳无通路"
				continue
			}
			down = append(down, ground[1:]...)
			tail := stationInner(centers[route.tower], route.slot)
			valid := true
			for _, p := range append(append(append([]vector{}, points...), down...), tail...) {
				cell := beltCell(p)
				reservedOwner, isReserved := reserved[cell]
				if occupied[cell] || isReserved && reservedOwner != i {
					reason = fmt.Sprintf("入口或落地点被占用 %+v", beltCell(p))
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			blocked := map[vector]bool{}
			for p := range occupied {
				if int(p.z) == level {
					blocked[p] = true
				}
			}
			for _, center := range centers {
				for x := -3.; x <= 3; x++ {
					for y := -3.; y <= 3; y++ {
						blocked[vector{center.x + x, center.y + y, float64(level)}] = true
					}
				}
			}
			high := findPath(ctx, vector{begin.x, begin.y, float64(level)}, vector{drop.x, drop.y, float64(level)}, attempt%2 == 0, blocked)
			if len(high) < 2 {
				reason = "高架平面无通路"
				continue
			}
			points = append(points, high[1:]...)
			points = append(points, down...)
			seen := map[vector]bool{}
			for _, p := range append(append([]vector{}, points...), tail...) {
				cell := beltCell(p)
				if occupied[cell] || seen[cell] {
					reason = fmt.Sprintf("路线格重叠 %+v", cell)
					valid = false
					break
				}
				seen[cell] = true
			}
			if !valid {
				continue
			}
			route.path = points
			for cell := range seen {
				occupied[cell] = true
			}
			found = true
			break
		}
		if !found {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("物流塔物品 %d、端口 %d 无法找到不重叠的高架接驳：%s", route.item, route.slot, reason)}
		}
	}
	return nil
}
