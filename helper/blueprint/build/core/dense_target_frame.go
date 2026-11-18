package build

// 文件职责：为非极区大厂选择有限的目标球面矩形与装箱顺序。
// 使用：沿用原生单元、路由和编码的同一坐标；边界：不越过选定档位，不改变生产设备或物料链。

import "math"

// denseTargetFrameExtents 职责：给出本档可用的纬向候选；使用：大厂原生生成；边界：窄环不借用相邻档位。
func denseTargetFrameExtents(bands []surfacePackedBand, rows []int) []int {
	span := bands[len(bands)-1].high - bands[0].low + 1
	result := []int{}
	for _, height := range rows {
		if height <= span {
			result = append(result, height)
		}
	}
	// 窄带无法容纳任何配置窗口时使用整带。
	if len(result) == 0 {
		return []int{span}
	}
	return result
}

// denseTargetLongitudeBudget 职责：从真实局部包络估算有限经向搜索上界；使用：非极区大厂装箱；边界：只是搜索预算，实际形状仍逐实体检验，不占满整条纬圈。
func denseTargetLongitudeBudget(frame *densePolarFrame, units []denseSpatialUnit, aspects ...float64) float64 {
	area, longest, scale := 0., 0., 1.
	for _, unit := range units {
		low, high := bounds(unit.entities)
		width, height := high.x-low.x+1, high.y-low.y+1
		area += width * height
		longest = math.Max(longest, width)
		scale = math.Max(scale, frame.unitScale(unit, vector{}, vector{y: float64(frame.height - 1)}))
	}
	budget := math.Ceil(math.Max(longest+16, area/float64(max(1, frame.height-8))*1.6+16) * scale)
	if len(aspects) > 0 && aspects[0] > 0 {
		budget = denseLocalRectangleLongitudeBudget(frame, aspects[0])
	}
	return budget
}

// targetLatitude 职责：按用户所选中心或边缘向内搜索完整单元；使用：非极区大厂球面装箱；边界：返回局部南界，越界候选由装箱器拒绝。
func (frame densePolarFrame) targetLatitude(index int, unitHeight float64) float64 {
	available := float64(frame.height-1) - unitHeight
	if frame.offsetWindow {
		last := int(math.Floor(available))
		if index < 0 || index > last {
			return -1
		}
		anchorValue := frame.referenceCenter - float64(frame.start) - unitHeight/2
		if frame.referenceCenter < 0 {
			anchorValue = math.Ceil(anchorValue)
		} else {
			anchorValue = math.Floor(anchorValue)
		}
		anchor := max(0, min(last, int(anchorValue)))
		paired := min(anchor, last-anchor)
		if index <= paired*2 {
			sign := 1
			if frame.referenceCenter < 0 {
				sign = -1
			}
			if index%2 == 1 {
				return float64(anchor - sign*(index+1)/2)
			}
			return float64(anchor + sign*index/2)
		}
		extra := index - paired*2
		if anchor > last-anchor {
			return float64(anchor - paired - extra)
		}
		return float64(anchor + paired + extra)
	}

	switch frame.position {
	case "south":
		return float64(index)
	case "north":
		return available - float64(index)
	default:
		delta := float64((index + 1) / 2)
		if index%2 == 1 {
			delta = -delta
		}
		return math.Floor(available/2) + delta
	}
}
