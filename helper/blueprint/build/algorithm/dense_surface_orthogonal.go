package build

// 文件职责：按目标纬度的实际横向格距迁移前三档布局，同时保留机器正交朝向。
// 使用：单Area最终坐标与cargo目标采用相同变换；边界：不放宽碰撞、分拣器角度、运输能力或供电要求。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
)

// mapOrthogonalPoint 职责：保持纬向行与朝向，沿每条纬线补偿横向物理长度；使用：前三档纬带；边界：不是刚体变换，输出必须完整复验，不对单个端口单独拉伸。
func (move *denseSurfaceRelocation) mapOrthogonalPoint(point blueprint.Point, yaw float32) (blueprint.Point, float32) {
	step := float64(float32(2*float32(math.Pi)) / 1000)
	x, y := float64(point.X), float64(point.Y)
	oldLatitude := (float64(move.sourceAnchor) + y) * step
	if move.halfTurn {
		x, y = 2*move.centerX-x, move.height-1-y
		yaw = float32(math.Mod(float64(yaw)+180, 360))
	}
	newLatitude := (float64(move.targetAnchor) + y) * step
	// 横向物理格长正比于cos(纬度)/经度段数；围绕中心缩放，减小纵向线路的侧向偏移。
	scale := math.Cos(oldLatitude) / math.Cos(newLatitude) * float64(move.targetSegments) / float64(move.sourceSegments)
	x = move.centerX + (x-move.centerX)*scale - move.shiftX
	return blueprint.Point{X: float32(x), Y: float32(y - move.shiftY), Z: point.Z}, yaw
}
