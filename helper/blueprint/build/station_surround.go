package build

// 文件职责：为物流塔规划端口受限的环绕机器行，并将独立产线放到塔的两侧。
// 使用：mode 3/4 的拆行和局部装配；边界：只刚性移动真实建筑，不改变量化流量、机器数量或单行容量上限。

import "math"

// stationSurroundCounts 职责：把既有容量拆行均匀分配到中央塔组的两侧；使用：塔模式装配前；边界：不增加机器，不超过原 lineCounts 已验证的最大行长或每塔十二端口。
func stationSurroundCounts(counts []int, itemKinds, slots int) ([]int, int) {
	if len(counts) == 0 {
		return counts, 1
	}
	perTower := min(itemKinds, slots)
	perGroup := min(4, 12/max(1, perTower))
	total, longest := 0, 0
	for _, count := range counts {
		total += count
		longest = max(longest, count)
	}
	rows := max(len(counts), min(2, total))
	if rows > perGroup {
		rows = min(total, ((rows+perGroup-1)/perGroup)*perGroup)
	}
	rows = max(rows, int(math.Ceil(float64(total)/float64(longest))))
	result := make([]int, rows)
	for i := range result {
		result[i] = total / rows
		if i < total%rows {
			result[i]++
		}
	}
	return result, perGroup
}

// surroundStationRows 职责：将机器行分别放在中央塔组的南北两侧，南侧独立旋转使端点分布在另一侧；使用：创建塔端口之前；边界：至少两行时才可能包围塔，单机行不伪造第二台机器。
func surroundStationRows(lines []*line, towerCount int) {
	centerX := float64(towerCount-1) * 15
	north, south := 10., -10.
	for i, row := range lines {
		if i%2 == 1 {
			for _, building := range row.all {
				building.a.x, building.a.y = -building.a.x, -building.a.y
				building.b.x, building.b.y = -building.b.x, -building.b.y
				building.record.Yaw = float32(math.Mod(float64(building.record.Yaw)+180, 360))
				building.record.Yaw2 = float32(math.Mod(float64(building.record.Yaw2)+180, 360))
			}
		}
		low, high := bounds(row.all)
		dx := centerX - math.Floor((low.x+high.x)/2)
		if i%2 == 0 {
			// 外侧行的接驳头向外错开，保留独立纵向走廊，不能被内侧行的带头封死。
			dx -= float64(i/2) * 4
			shift(row.all, dx, north-low.y, 0)
			north += math.Ceil(high.y-low.y) + 1
		} else {
			dx += float64(i/2) * 4
			shift(row.all, dx, south-high.y, 0)
			south -= math.Ceil(high.y-low.y) + 1
		}
	}
}
