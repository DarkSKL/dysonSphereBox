package build

// 文件职责：将密铺传送带原料接口统一放在工厂左侧，并尽量对齐消费者。
// 使用：全场密铺布线前；边界：仍保持真实外部入口与独立物料，现有副产物归集契约单独保留。

import "math"

// denseExternalStep 职责：返回外部入口实际向内方向；使用：端口净空和寻路首步；边界：旧模式零值保持向东。
func denseExternalStep(route *transportRoute) vector {
	if route.entryDirection.x == 0 && route.entryDirection.y == 0 {
		return vector{x: 1}
	}
	return route.entryDirection
}

// denseInputPositions 职责：在西侧边界搜索与消费者齐平的独立输入口；使用：密铺空间固定后；边界：有副产物时保留已验证的同侧回收接口，端口至少相距两格。
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
