package build

// 文件职责：在精确线段到盒距离之前应用局部投影的保守距离下界。
// 使用：供电候选的大量胶囊净空判定；边界：仅提前排除明确远离的线段，临界值仍调用原精确公式，不更改碰撞半径。

import (
	"math"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// collisionSegmentWithinRadius 职责：以逐轴区间下界跳过肯定超出半径的线段；使用：空间桶命中后的胶囊窄相位；边界：仅返回原严格距离比较的结果，浮点临界附近保守回退精确计算。
func collisionSegmentWithinRadius(box placement.Box, first, second placement.Vector, radiusSquared float64) bool {
	relative, displacement := first.Sub(box.Center), second.Sub(first)
	lowerBound := 0.
	for axis, basis := range box.Axes {
		origin := relative.Dot(basis)
		end := origin + displacement.Dot(basis)
		gap := math.Max(0, math.Max(math.Min(origin, end)-box.HalfSize[axis], -math.Max(origin, end)-box.HalfSize[axis]))
		lowerBound += gap * gap
		// 下界等于阈值或非常接近时仍交给原公式，避免浮点投影舍入影响严格小于边界。
		if lowerBound > radiusSquared+1e-8*(1+lowerBound) {
			return false
		}
	}
	return box.SegmentDistanceSquared(first, second) < radiusSquared
}
