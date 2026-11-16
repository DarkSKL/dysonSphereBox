package build

// 文件职责：在主路展开前识别目标处已封死的短通道；使用：多终点寻路的必要条件预检。
// 边界：只证明最多64个节点的无分叉封闭分量；遇源点、分叉或预算上限一律交给原A*，不缓存可变占位结论。

import "context"

// transportGoalHasApproach 职责：沿合法有向入边检查目标的短通道是否封闭；使用：过滤密集设施附近的无效整数接驳点。
// 边界：反向查询仍核验previous→current原方向；源节点允许原有占位例外，其他已有带不能充当穿行点，未证明无解就保留目标。
func (env *transportEnvironment) transportGoalHasApproach(ctx context.Context, start, goal vector, source, target *placed, inside func(vector) bool) bool {
	if goal == start {
		return true
	}
	var visited [64]vector
	visited[0] = goal
	count, current := 1, goal
	for {
		if ctx.Err() != nil {
			return false
		}
		var next vector
		found := false
		for _, delta := range transportNeighborOffsets {
			previous := vector{current.x + delta.x, current.y + delta.y, current.z + delta.z}
			if !inside(previous) || env.blocked[beltCell(previous)] && previous != start || !env.denseTransportStepClear(previous, current) || !env.reservedSpanClear(previous, current, source, target) {
				continue
			}
			if previous == start {
				return true
			}
			seen := false
			for _, point := range visited[:count] {
				if previous == point {
					seen = true
					break
				}
			}
			if seen {
				continue
			}
			if found {
				// 有分叉就没有便宜的封闭证明，保留全部原始搜索机会。
				return true
			}
			next, found = previous, true
		}
		if !found {
			return false
		}
		if count == len(visited) {
			return true
		}
		visited[count] = next
		count++
		current = next
	}
}
