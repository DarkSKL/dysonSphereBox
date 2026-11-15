package build

// 文件职责：为非极区大厂选择有限的目标球面矩形与装箱顺序。
// 使用：沿用原生单元、路由和编码的同一坐标；边界：不越过选定档位，不改变生产设备或物料链。

import "math"

// denseNativeLargeMachineThreshold 职责：让大厂从装箱开始固定目标球面坐标；使用：公开生成分流；边界：沿用大厂搜索阈值，小厂继续使用已验收的紧凑路径。
const denseNativeLargeMachineThreshold = 256

// denseTargetFrameExtents 职责：给出本档可用的纬向候选；使用：大厂原生生成；边界：窄环不借用相邻档位。
func denseTargetFrameExtents(bands []surfacePackedBand) []int {
	span := bands[len(bands)-1].high - bands[0].low + 1
	if span <= 64 {
		return []int{span}
	}
	result := []int{}
	for _, height := range []int{64, 80, 96, 112} {
		if height <= span {
			result = append(result, height)
		}
	}
	return result
}

// denseTargetLongitudeBudget 职责：从真实局部包络估算有限经向搜索上界；使用：非极区大厂装箱；边界：只是搜索预算，实际形状仍逐实体检验，不占满整条纬圈。
func denseTargetLongitudeBudget(frame *densePolarFrame, units []denseSpatialUnit) float64 {
	area, longest, scale := 0., 0., 1.
	for _, unit := range units {
		low, high := bounds(unit.entities)
		width, height := high.x-low.x+1, high.y-low.y+1
		area += width * height
		longest = math.Max(longest, width)
		scale = math.Max(scale, frame.unitScale(unit, vector{}, vector{y: float64(frame.height - 1)}))
	}
	return math.Ceil(math.Max(longest+16, area/float64(max(1, frame.height-8))*1.6+16) * scale)
}

// targetLatitude 职责：按用户所选中心或边缘向内搜索完整单元；使用：非极区大厂球面装箱；边界：返回局部南界，越界候选由装箱器拒绝。
func (frame densePolarFrame) targetLatitude(index int, unitHeight float64) float64 {
	available := float64(frame.height-1) - unitHeight
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
