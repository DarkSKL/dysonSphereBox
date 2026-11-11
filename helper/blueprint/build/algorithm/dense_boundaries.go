package build

// 文件职责：将密铺传送带原料接口统一放在工厂左侧，并尽量对齐消费者。
// 使用：全场密铺布线前；边界：仍保持真实外部入口与独立物料，现有副产物归集契约单独保留。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"math"
)

// denseExternalStep 职责：返回外部入口实际向内方向；使用：端口净空和寻路首步；边界：旧模式零值保持向东。
func denseExternalStep(route *transportRoute) vector {
	if route.entryDirection.x == 0 && route.entryDirection.y == 0 {
		return vector{x: 1}
	}
	return route.entryDirection
}

// denseInputPositions 职责：按真实消费者生成西侧入口草案；使用：密铺空间固定后；边界：直连模式交给联合三层规划分配独立槽，其它模式保留既有两格入口搜索。
func (b *builder) denseInputPositions(low, high vector) []vector {
	if b.adapter != nil && b.adapter.mode == 1 {
		positions := []vector{}
		for _, route := range b.transportRoutes {
			if route.from != nil {
				continue
			}
			point := vector{math.Floor(low.x) - 5, low.y, 0}
			if route.end != nil {
				point.y = route.end.a.y
			}
			route.entryDirection = vector{x: 1}
			positions = append(positions, point)
		}
		return positions
	}
	fallback := func() []vector {
		for _, route := range b.transportRoutes {
			route.entryDirection = vector{}
		}
		return transportInputPositions(b.transportRoutes, low, high, true)
	}
	for _, route := range b.transportRoutes {
		if route.edge.FlowKind == "leftover" || route.from == nil && route.end == nil {
			return fallback()
		}
	}
	positions := []vector{}
	left, bottom, top := math.Floor(low.x)-2, math.Floor(low.y)-2, math.Ceil(high.y)+2
	for _, route := range b.transportRoutes {
		if route.from != nil {
			continue
		}
		point := route.end.a
		chosen := vector{}
		found := false
		for offset := 0; offset <= len(b.transportRoutes)*2+2; offset++ {
			for _, sign := range []float64{1, -1} {
				y := math.Max(bottom, math.Min(top, math.Round(point.y)+sign*float64(offset)))
				candidate := vector{left, y, 0}
				free := true
				for _, old := range positions {
					if math.Abs(old.y-candidate.y) < 2 {
						free = false
						break
					}
				}
				if free {
					chosen = candidate
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return fallback()
		}
		route.entryDirection = vector{x: 1}
		positions = append(positions, chosen)
	}
	return positions
}

// fitDenseInputHeight 职责：将直连接口约束在生产实体纬向范围内，联合安排最多三层入口和独立地面出口。
// 使用：入口草案完成、实体尚未创建；边界：不合并物料或改变份额、带等级及内部目标，其它模式保持原生球面适配。
func (b *builder) fitDenseInputHeight(ctx context.Context, positions []vector, low, high vector) ([]vector, error) {
	if len(positions) == 0 || b.adapter == nil || b.adapter.density != densityDense {
		return positions, nil
	}
	if b.adapter.mode == 1 && len(b.all) > 0 {
		return b.planDenseBoundaries(ctx, positions, low, high)
	}
	leftovers := 0
	for _, r := range b.transportRoutes {
		if r.edge.FlowKind == "leftover" {
			leftovers++
		}
	}
	minimum, maximum := positions[0].y, positions[0].y
	insideNative := true
	for _, p := range positions {
		minimum = math.Min(minimum, p.y)
		maximum = math.Max(maximum, p.y)
		if b.polarFrame != nil && (p.y < 0 || p.y > float64(b.polarFrame.height-1) || p.z < 0 || p.z > transportMaxHeight) {
			insideNative = false
		}
	}
	span := math.Max(maximum, high.y) - math.Min(minimum, low.y) + 1 + float64(leftovers*2)
	if b.polarFrame != nil {
		if insideNative && span <= float64(b.polarFrame.height) {
			return positions, nil
		}
		return fitDenseInputRows(positions, 0, float64(b.polarFrame.height-1), leftovers)
	}
	zone, _ := ctx.Value(densityLatitudeKey{}).(string)
	selection, ok := densityLatitudeSelectionFor(zone)
	if !ok || selection.tier < 4 {
		return positions, nil
	}
	band, _, err := densityContextBand(ctx, zone, 200)
	if err != nil {
		return nil, err
	}
	if span <= float64(band.high-band.low+1) {
		return positions, nil
	}
	bottom, top := math.Ceil(low.y), math.Min(math.Floor(high.y), math.Ceil(low.y)+float64(band.high-band.low))
	if int((top-bottom)/2)+1 <= leftovers {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选窄环缺少独立原料与副产物接入口"}
	}
	return fitDenseInputRows(positions, bottom, top, leftovers)
}

// fitDenseInputRows 职责：在指定局部纬向区间内分配独立分层原料口；使用：原生球面及旧高纬入口适配；边界：保留原两格行距、两层层距、地面交付预留和最高带层，地面恰好满额时原料仍可用上层，容量不足返回错误，不修改原位置切片。
func fitDenseInputRows(positions []vector, bottom, top float64, leftovers int) ([]vector, error) {
	if top < bottom {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选球面缺少独立原料接入口"}
	}
	count := int((top-bottom)/2) + 1
	if count < leftovers {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选窄环缺少独立原料与副产物接入口"}
	}
	result := append([]vector(nil), positions...)
	left := positions[0].x
	for _, p := range positions {
		left = math.Min(left, p.x)
	}
	used := map[[2]int]bool{}
	for i, p := range positions {
		best, distance := [2]int{-1, 0}, math.Inf(1)
		for level := 0; level <= int(transportMaxHeight)/2; level++ {
			for row := 0; row < count; row++ {
				key := [2]int{row, level}
				if used[key] || level == 0 && row < leftovers {
					continue
				}
				cost := math.Abs(bottom+float64(row*2)-p.y) + float64(level*4)
				if cost < distance {
					best, distance = key, cost
				}
			}
		}
		if best[0] < 0 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "窄环分层原料入口超过高度容量"}
		}
		used[best] = true
		result[i] = vector{left, bottom + float64(best[0]*2), float64(best[1] * 2)}
	}
	return result, nil
}
