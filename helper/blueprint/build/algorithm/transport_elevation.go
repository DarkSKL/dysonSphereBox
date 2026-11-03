package build

// 文件职责：为独立传送带复用已占用的平面走廊提供软代价。
// 使用：普通与反向合流 A* 共用；边界：相同平面格的不同高度仍是独立实体，不强制高架或放宽占位与连接规则。

import "math"

// transportStepCost 职责：优先沿既有带的平面投影铺设独立分层路线；使用：逐格计分；边界：升降有代价，空场不强迫升高，两端均在走廊才算复用以保证正反搜索代价一致。
func (env *transportEnvironment) transportStepCost(from, to vector) float64 {
	if from.z != to.z {
		return 1.5
	}
	if env.preferCorridor && !(env.beltCorridors[beltColumn(from)] && env.beltCorridors[beltColumn(to)]) {
		return 1.25
	}
	return 1
}

// beltColumn 职责：获得真实带格的平面投影；使用：跨高度走廊索引；边界：不代替三维碰撞或连接核验。
func beltColumn(point vector) [2]int {
	return [2]int{int(math.Round(point.x)), int(math.Round(point.y))}
}

// transportHeuristic 职责：给出忽略三维障碍的保守一致下界；使用：正向与反向 A*；边界：势场过期或场外时只使用曼哈顿，不假定未知路径必定复用走廊。
func (env *transportEnvironment) transportHeuristic(from, to vector) float64 {
	horizontal := math.Abs(from.x-to.x) + math.Abs(from.y-to.y)
	if field := env.corridorPotential; env.preferCorridor && field != nil && field.version == env.version {
		left, leftOK := field.distanceAt(from)
		right, rightOK := field.distanceAt(to)
		if leftOK && rightOK {
			horizontal = math.Max(horizontal, math.Abs(left-right))
		}
	}
	vertical := math.Abs(from.z-to.z) * 1.5
	return horizontal + vertical
}

// transportPathCost 职责：按同一软代价比较已找到的端点候选；使用：消费端合法邻格择优；边界：不以较少节点覆盖能有效复用走廊的路线。
func (env *transportEnvironment) transportPathCost(path []vector) float64 {
	total := 0.
	for i := 1; i < len(path); i++ {
		total += env.transportStepCost(path[i-1], path[i])
	}
	return total
}
