package build

// 文件职责：把私有物理向量交接给共同的单精度球面度量。
// 使用：分拣器真实格距和跨度检查；边界：框架、非有限值和缺少径向位置均以错误返回，不再复制分段表或数学公式。

import (
	buildgeometry "DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfaceLocalGridSize 职责：用带身份的球面世界点查询真实局部格长；使用：端口纠错；边界：不把赤道格长用于高纬。
func surfaceLocalGridSize(position, direction placement.Vector) (float64, error) {
	frame := buildgeometry.PlanetWorldFrame()
	size, err := buildgeometry.LocalGridSize32(geometryWorldPoint(position, frame), buildgeometry.Direction{Frame: frame, X: direction[0], Y: direction[1], Z: direction[2]})
	return float64(size), err
}

// surfaceSegmentsAcross 职责：用共同游戏单精度规则核验球面跨度；使用：分拣器硬约束；边界：输入全部为同一行星世界空间，不是平面细格。
func surfaceSegmentsAcross(reference, first, second placement.Vector) (float64, error) {
	frame := buildgeometry.PlanetWorldFrame()
	size, err := buildgeometry.SegmentsAcross32(geometryWorldPoint(reference, frame), geometryWorldPoint(first, frame), geometryWorldPoint(second, frame))
	return float64(size), err
}
