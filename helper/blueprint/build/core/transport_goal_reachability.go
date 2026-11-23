package build

// 文件职责：在主路展开前识别目标处已封死的小连通分量；使用：多终点寻路的必要条件预检。
// 边界：最多核对64个整数节点，触及未探索节点预算即交原A*；不缓存可变占位结论，不把有分叉的小房间当作一定可进入。

import (
	"context"
	"math"
)

// transportGoalProbeNodes 职责：限制必要条件预检的状态数；使用：固定栈队列；边界：超过即未知，不增加或替代主路预算。
const transportGoalProbeNodes = 64

// transportGoalExactGrid 职责：只在加减相邻整数可逆的坐标中作封闭证明；使用：反向遍历前。
// 边界：分数坐标或精度边缘只保留原搜索机会，不能用逆向浮点舍入误删前向可达路径。
func transportGoalExactGrid(point vector) bool {
	return denseIntegerNode(point) && math.Abs(point.x) <= 1<<52 && math.Abs(point.y) <= 1<<52 && math.Abs(point.z) <= 1<<52
}

// transportGoalHasApproach 职责：沿合法有向入边检查目标的小分量是否封闭；使用：过滤密集设施附近的无效整数接驳点。
// 边界：反向查询仍核验previous→current原方向；源节点允许原有占位例外，其他已有带不能充当穿行点，未证明无解就保留目标。
func (env *transportEnvironment) transportGoalHasApproach(ctx context.Context, start, goal vector, source, target *placed, inside func(vector) bool) bool {
	if ctx.Err() != nil {
		return false
	}
	if goal == start {
		return true
	}
	if !transportGoalExactGrid(start) || !transportGoalExactGrid(goal) {
		return true
	}
	// 反向可达分量只需证明超过 64 格。深度优先避免反复展开同一开放空间的内圈。
	// 从目标最多走 64 次，三个有符号坐标差各占一字节；半满的开地址表只登记录入分量的点。
	var visited [2 * transportGoalProbeNodes]uint32
	slot := func(v vector) (*uint32, uint32) {
		key := uint32(int(v.x-goal.x)+65)<<16 | uint32(int(v.y-goal.y)+65)<<8 | uint32(int(v.z-goal.z)+65)
		i := (key * 2654435761) >> 25
		for visited[i] != 0 && visited[i] != key {
			i = (i + 1) & 127
		}
		return &visited[i], key
	}
	stamp, key := slot(goal)
	*stamp = key
	var pending [transportGoalProbeNodes]vector
	pending[0] = goal
	count := 1
	for size := 1; size > 0; {
		if ctx.Err() != nil {
			return false
		}
		size--
		current := pending[size]
		for _, delta := range transportNeighborOffsets {
			previous := vector{current.x + delta.x, current.y + delta.y, current.z + delta.z}
			stamp, key := slot(previous)
			if *stamp != 0 {
				continue
			}
			if !inside(previous) || env.blocked[beltCell(previous)] && previous != start || !env.denseTransportStepClear(previous, current) || !env.reservedSpanClear(previous, current, source, target) {
				continue
			}
			if previous == start {
				return true
			}
			if count == transportGoalProbeNodes {
				// 还有未核对节点时只能判为未知，保留原主路搜索。
				return true
			}
			*stamp = key
			pending[size] = previous
			size++
			count++
		}
	}
	return false
}
