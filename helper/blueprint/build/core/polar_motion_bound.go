package build

// 文件职责：把已证明的碰撞余量转换为保守的经向无效区间；使用：球面单元按四分之一格递增搜索。
// 边界：只跳过必定仍冲突的采样，不接受未验证位置，不改变网格、物理规则或候选的有效顺序。
import (
	"DysonSphereBox/helper/blueprint/build/placement"
	"math"
)

// polarBlockedSteps 职责：利用绕行星轴的运动上界r·θ跳过仍在碰撞内的整步；使用：单调经向扫描；边界：始终扣除数值余量，深度不足一步时返回零。
func polarBlockedSteps(depth, radius, anglePerStep float64) int {
	if depth <= 1e-5 || radius <= 0 || anglePerStep <= 0 {
		return 0
	}
	steps := math.Floor((depth-1e-5)/(radius*anglePerStep) - 1e-7)
	if steps <= 0 || math.IsNaN(steps) || math.IsInf(steps, 0) {
		return 0
	}
	return int(math.Min(steps, 100000))
}

// polarBoxBlockedSteps 职责：以真实盒的相交深度和旋转半径证明连续无效采样；使用：窄相位已确认冲突后；边界：实际边界采样仍由原Overlaps判断。
func polarBoxBlockedSteps(moving, fixed placement.Box, anglePerStep float64) int {
	low, high := moving.Bounds()
	radius := math.Hypot(math.Max(math.Abs(low[0]), math.Abs(high[0])), math.Max(math.Abs(low[2]), math.Abs(high[2])))
	return polarBlockedSteps(moving.OverlapMargin(fixed), radius, anglePerStep)
}
