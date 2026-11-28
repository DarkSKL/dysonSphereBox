package build

// 文件职责：在主路展开前识别目标处已封死的小连通分量；使用：多终点寻路的必要条件预检。
// 边界：最多核对64个整数节点，触及未探索节点预算即交原A*；不缓存可变占位结论，不把有分叉的小房间当作一定可进入。

import (
	"context"
	"math"
	"slices"
)

// transportGoalProbeNodes 职责：限制必要条件预检的状态数；使用：固定栈队列；边界：超过即未知，不增加或替代主路预算。
const transportGoalProbeNodes = 64

// transportGoalProofs 只在一次多终点搜索中共享，起点、端口、临时屏蔽和高度窗口固定。
// true 表示可达或至少有 65 个前驱可达节点，false 表示已穷尽未连到起点的小分量。
type transportGoalProofs map[vector]bool

func (proofs transportGoalProofs) remember(point vector, open bool) {
	if proofs != nil && len(proofs) < 8*transportGoalProbeNodes {
		proofs[point] = open
	}
}

// transportGoalExactGrid 职责：只在加减相邻整数可逆的坐标中作封闭证明；使用：反向遍历前。
// 边界：分数坐标或精度边缘只保留原搜索机会，不能用逆向浮点舍入误删前向可达路径。
func transportGoalExactGrid(point vector) bool {
	return denseIntegerNode(point) && math.Abs(point.x) <= 1<<52 && math.Abs(point.y) <= 1<<52 && math.Abs(point.z) <= 1<<52
}

// transportGoalHasApproach 职责：沿合法有向入边检查目标的小分量是否封闭；使用：过滤密集设施附近的无效整数接驳点。
// 边界：反向查询仍核验previous→current原方向；源节点允许原有占位例外，其他已有带不能充当穿行点，未证明无解就保留目标。
func (env *transportEnvironment) transportGoalHasApproach(ctx context.Context, start, goal vector, source, target *placed, inside func(vector) bool, proofs transportGoalProofs) bool {
	if ctx.Err() != nil {
		return false
	}
	if goal == start {
		return true
	}
	if !transportGoalExactGrid(start) || !transportGoalExactGrid(goal) {
		return true
	}
	if open, known := proofs[goal]; known {
		return open
	}
	var visited [transportGoalProbeNodes]vector
	visited[0] = goal
	count := 1
	for head := 0; head < count; head++ {
		if ctx.Err() != nil {
			return false
		}
		current := visited[head]
		// current 已经通过有向边接回 goal，其已知前驱也都能接回 goal。
		if head > 0 && proofs[current] {
			proofs.remember(goal, true)
			return true
		}
		for _, delta := range transportNeighborOffsets {
			previous := vector{current.x + delta.x, current.y + delta.y, current.z + delta.z}
			if slices.Contains(visited[:count], previous) {
				continue
			}
			if !inside(previous) || env.blocked[beltCell(previous)] && previous != start || !env.denseTransportStepClear(previous, current) || !env.reservedSpanClear(previous, current, source, target) {
				continue
			}
			if previous == start {
				proofs.remember(goal, true)
				return true
			}
			if count == len(visited) {
				// 还有未核对节点时只能判为未知，保留原主路搜索。
				proofs.remember(goal, true)
				return true
			}
			visited[count] = previous
			count++
		}
	}
	// 完整搜尽后，各成员的反向可达集合都是该闭分量的子集。搜索中途不能把已知闭分量跳过，否则会改变 64 点阈值。
	for _, point := range visited[:count] {
		proofs.remember(point, false)
	}
	return false
}
