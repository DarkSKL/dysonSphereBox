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

// stationApproach 职责：给出端口外侧的接入方向；使用：入口预留与高架落点共用；边界：3/4 号端口避开西侧相邻内部带。
func stationApproach(slot int) vector {
	switch {
	case slot <= 2:
		return vector{0, 1, 0}
	case slot == 3 || slot == 4:
		return vector{1, 0, 0}
	case slot == 5:
		return vector{-1, 0, 0}
	case slot <= 8:
		return vector{0, -1, 0}
	default:
		return vector{1, 0, 0}
	}
}

// bridgeStationRoutes 职责：先验证升降列，再在高架平面执行有界 A*；使用：塔策略的显式后备；边界：existing 仅含尚未整体平移的本组产线，失败整体拒绝，不写出半条接驳。
func (b *builder) bridgeStationRoutes(ctx context.Context, routes []stationRoute, centers []vector, existing []*placed) error {
	buildings, err := stationBuildingObstacles(ctx, existing)
	if err != nil {
		return err
	}
	occupied := map[vector]bool{}
	maxZ := 0.
	for _, p := range existing {
		// 堆叠生产机的最高层决定桥接净空；只看现有传送带会把高架平面放进机器层。
		maxZ = math.Max(maxZ, p.a.z)
		if p.record.ItemID > 2000 && p.record.ItemID < 2010 {
			occupied[beltCell(p.a)] = true
		}
	}
	reserved := map[vector]int{}
	for i, route := range routes {
		end := slotOffset(route.slot)[0]
		end.x += centers[route.tower].x
		end.y += centers[route.tower].y
		reserved[beltCell(end)] = i
		// 为尚未布线的端口保留外侧一格，避免先完成的路线封死其唯一入口。
		direction := stationApproach(route.slot)
		reserved[beltCell(vector{end.x + direction.x, end.y + direction.y, 0})] = i
		for _, p := range stationInner(centers[route.tower], route.slot) {
			reserved[beltCell(p)] = i
		}
	}
	for i := range routes {
		route := &routes[i]
		end := slotOffset(route.slot)[0]
		end.x += centers[route.tower].x
		end.y += centers[route.tower].y
		direction := stationApproach(route.slot)
		tail := stationInner(centers[route.tower], route.slot)
		found := false
		reason := ""
		// 优先复用低层空闲通路；只有该层全部落点失败才升层，避免按路线序号无条件堆高升降列。
		for attempt := 0; attempt < 192*(i+2); attempt++ {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔高架寻路超过截止时间"}
			}
			begin := route.head.a
			// 先在产线外侧寻找升降列，再扩展到反侧和上下偏移；仅试四格会把堆叠机的建造盒当成可用净空。
			xOffsets := []float64{1, 2, 3, 4, 5, 6, 7, 8, 10, 12, 14, 16, -1, -2, -3, -4, -5, -6, -8, -10}
			yOffsets := []float64{0, 1, -1, 2, -2, 3, -3, 4, -4, 6, -6, 8, -8, 10, -10, 12, -12, 16, -16, 20, -20}
			begin.x -= xOffsets[attempt%len(xOffsets)]
			begin.y += yOffsets[(attempt/len(xOffsets))%len(yOffsets)]
			offsets := []float64{0, 1, -1, 2, -2, 3, -3, 4, -4, 6, -6, 8, -8, 10, -10, 12, -12}
			perpendicular := offsets[(attempt/4)%len(offsets)]
			distance := float64(1 + (attempt/32)%16)
			drop := vector{end.x + direction.x*distance - direction.y*perpendicular, end.y + direction.y*distance + direction.x*perpendicular, 0}
			level := int(math.Ceil(maxZ)) + 2 + attempt/192
			// 先沿纵向离开产线带，再横向到升降列；反向时横穿机器输出带会在第一格立即碰撞。
			firstX := math.Abs(begin.y-route.head.a.y) < 1e-9
			points := append([]vector{}, manhattan(route.head.a, begin, firstX)[1:]...)
			for z := 1; z <= level; z++ {
				points = append(points, vector{begin.x + .002*float64(z), begin.y, float64(z)})
			}
			down := []vector{}
			for z := level - 1; z >= 0; z-- {
				down = append(down, vector{drop.x + .002*float64(z), drop.y, float64(z)})
			}
			// 先排除不可升降的候选，再执行 A*；实体盒使部分近机器落点天然不可用。
			valid := true
			for _, p := range append(append(append([]vector{}, points...), down...), tail...) {
				cell := beltCell(p)
				reservedOwner, isReserved := reserved[vector{cell.x, cell.y, 0}]
				if occupied[cell] || buildings[cell] || isReserved && reservedOwner != i {
					reason = fmt.Sprintf("入口或落地点被占用 %+v", cell)
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			groundBlocked := map[vector]bool{}
			for p := range buildings {
				if p.z == 0 {
					groundBlocked[p] = true
				}
			}
			for p, owner := range reserved {
				if owner != i {
					groundBlocked[p] = true
				}
			}
			// 自己的塔内带也不能作为落地绕路，否则到达端口后再次追加会形成回折重叠。
			for _, p := range tail {
				groundBlocked[beltCell(p)] = true
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
			valid = true
			for _, p := range append(append(append([]vector{}, points...), down...), tail...) {
				cell := beltCell(p)
				reservedOwner, isReserved := reserved[vector{cell.x, cell.y, 0}]
				if occupied[cell] || buildings[cell] || isReserved && reservedOwner != i {
					reason = fmt.Sprintf("入口或落地点被占用 %+v", beltCell(p))
					valid = false
					break
				}
			}
			if !valid {
				continue
			}
			blocked := map[vector]bool{}
			for p := range buildings {
				if int(p.z) == level {
					blocked[p] = true
				}
			}
			// 端口预留格向上延伸为升降净空，低层复用不能从未来落地点上方横穿并封死下降列。
			for p, owner := range reserved {
				if owner != i {
					blocked[vector{p.x, p.y, float64(level)}] = true
				}
			}
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
