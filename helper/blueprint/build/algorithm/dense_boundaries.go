package build

// 文件职责：将密铺原料接口放到最近的实际外边界，减少统一西侧入口形成的长尾。
// 使用：全场密铺布线前；边界：仍保持真实外部入口与独立物料，现有副产物归集契约单独保留。

import "math"

// denseExternalStep 职责：返回外部入口实际向内方向；使用：端口净空和寻路首步；边界：旧模式零值保持向东。
func denseExternalStep(route *transportRoute) vector {
	if route.entryDirection.x == 0 && route.entryDirection.y == 0 {
		return vector{x: 1}
	}
	return route.entryDirection
}

// denseInputPositions 职责：在四侧边界搜索最近的独立输入口；使用：密铺空间固定后；边界：有副产物时保留已验证的同侧回收接口，端口至少相距两格。
func (b *builder) denseInputPositions(low, high vector) []vector {
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
	left, right, bottom, top := math.Floor(low.x)-2, math.Ceil(high.x)+2, math.Floor(low.y)-2, math.Ceil(high.y)+2
	for _, route := range b.transportRoutes {
		if route.from != nil {
			continue
		}
		point := route.end.a
		best := math.Inf(1)
		chosen, direction := vector{}, vector{}
		consider := func(candidate, step vector) {
			for _, old := range positions {
				if math.Abs(old.x-candidate.x)+math.Abs(old.y-candidate.y) < 2 {
					return
				}
			}
			cost := math.Abs(candidate.x-point.x) + math.Abs(candidate.y-point.y)
			if cost < best {
				best, chosen, direction = cost, candidate, step
			}
		}
		// 先查与端口齐平的位置，再查有限侧向槽；边界方向始终从外部指向工厂。
		for offset := 0; offset <= len(b.transportRoutes)*2+2; offset++ {
			for _, sign := range []float64{1, -1} {
				x := math.Max(left+1, math.Min(right-1, math.Round(point.x)+sign*float64(offset)))
				y := math.Max(bottom+1, math.Min(top-1, math.Round(point.y)+sign*float64(offset)))
				consider(vector{left, y, 0}, vector{x: 1})
				consider(vector{right, y, 0}, vector{x: -1})
				consider(vector{x, bottom, 0}, vector{y: 1})
				consider(vector{x, top, 0}, vector{y: -1})
			}
		}
		if math.IsInf(best, 1) {
			return fallback()
		}
		route.entryDirection = direction
		positions = append(positions, chosen)
	}
	return positions
}
