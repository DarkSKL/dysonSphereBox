package build

// 文件职责：把已规划的独立机器行放到物流塔两侧。
// 使用：物流塔局部实体装配；边界：只刚性移动建筑，拆行与端口预算由独立布局阶段负责。

import "math"

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
