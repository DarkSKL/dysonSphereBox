package build

// 文件职责：剔除多终点下界计算中被同一平面格其它后缀完全支配的项。
// 使用：每次运输搜索固定目标之后；边界：只缩减启发式计算，不删除真实可达目标、改变其端口权限或后缀代价。

import "math"

// transportHeuristicGoals 职责：保留足以计算相同代价及长度下界的目标下标；使用：大量不同高度接驳点共享平面坐标时；边界：同一平面格的势能相同，垂直松弛距离为 1.5 倍高度差，严格比较不添加容差。
func transportHeuristicGoals(goals []transportDestination) []int {
	indices := make([]int, 0, len(goals))
	columns := make(map[[2]float64][]int)
	for i, goal := range goals {
		column := [2]float64{goal.point.x, goal.point.y}
		columns[column] = append(columns[column], i)
	}
	for i, candidate := range goals {
		dominated := false
		for _, j := range columns[[2]float64{candidate.point.x, candidate.point.y}] {
			if i == j {
				continue
			}
			other := goals[j]
			height := math.Abs(candidate.point.z - other.point.z)
			if other.cost+1.5*height <= candidate.cost && other.length <= candidate.length-int(math.Ceil(height)) {
				// 完全相等时保留原来的第一项，避免相互支配导致集合为空。
				if height != 0 || other.cost < candidate.cost || other.length < candidate.length || j < i {
					dominated = true
					break
				}
			}
		}
		if !dominated {
			indices = append(indices, i)
		}
	}
	return indices
}
