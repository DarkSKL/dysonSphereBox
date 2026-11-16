package build

// 文件职责：为完整实体集合提供四分之一圈刚性旋转。
// 使用：全场机器与设施候选排布；边界：保留连接、槽位、数量和双端高度。

import "math"

// rotatePlacedQuarter 职责：以原点顺时针刚性旋转整块实体；使用：全场候选布局；边界：不改槽位编号、引用与高度，双端姿态同步，重复指针只处理一次。
func rotatePlacedQuarter(buildings []*placed, turns int) {
	turns = (turns%4 + 4) % 4
	seen := make(map[*placed]bool, len(buildings))
	for _, building := range buildings {
		if seen[building] {
			continue
		}
		seen[building] = true
		for step := 0; step < turns; step++ {
			building.a.x, building.a.y = building.a.y, -building.a.x
			building.b.x, building.b.y = building.b.y, -building.b.x
			building.densityCargo.relative.x, building.densityCargo.relative.y = building.densityCargo.relative.y, -building.densityCargo.relative.x
		}
		building.record.Yaw = float32(math.Mod(float64(building.record.Yaw)+float64(turns)*90, 360))
		building.record.Yaw2 = float32(math.Mod(float64(building.record.Yaw2)+float64(turns)*90, 360))
	}
}
