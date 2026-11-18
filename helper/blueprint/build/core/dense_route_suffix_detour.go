package build

// 文件职责：在原搜索全部失败后，为已被整链拒绝的目标后缀补足自身节点净距约束。
// 使用：公共前后缀拼接的有限备用；边界：最多处理原四次拒绝，每个后缀只重试一次，不改变原成功路径、单次展开上限或物理终检。

import "context"

// denseRouteRejectedSuffix 职责：保留原搜索已找到主路但整链不合格的真实目标。
// 使用：全部原尝试失败后逐目标私有重试；边界：不保存合流旧尾链、不新增或永久删除目标。
type denseRouteRejectedSuffix struct {
	// goal 职责：保存原整数接驳点及完整后缀成本；使用：同目标重试；边界：不把真实端点取整后替换。
	goal transportDestination
	// points 职责：保存从整数目标到真实端点的连续后缀；使用：临时净距及最终拼接；边界：首末节点身份不变。
	points []vector
}

// guardDenseRouteSuffixNodes 职责：仅在一个目标的搜索期间保护其分数后缀内部节点净距。
// 使用：备用主路绕行前，调用方必须执行返回的恢复函数；边界：使用原米制净距及容差，保留已有占位；固定起点或目标与后缀自身冲突时不搜索。
func (env *transportEnvironment) guardDenseRouteSuffixNodes(start vector, suffix denseRouteRejectedSuffix) (restore func(), possible bool) {
	added := map[vector]bool{}
	restore = func() {
		for point, existed := range added {
			if existed {
				env.blocked[point] = false
			} else {
				delete(env.blocked, point)
			}
		}
	}
	for _, point := range suffix.points[1 : len(suffix.points)-1] {
		cells := map[vector]bool{}
		env.densePorts.blockNode(cells, point)
		world := env.densePorts.world(point)
		for cell := range cells {
			delta := env.densePorts.world(cell).Sub(world)
			if delta.Dot(delta) >= denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
				continue
			}
			if cell == start || cell == suffix.goal.point {
				return restore, false
			}
			if blocked, existed := env.blocked[cell]; !blocked {
				env.blocked[cell] = true
				added[cell] = existed
			}
		}
	}
	// 没有新增任何整数约束时重搜只会重复原结果，不占用备用搜索预算。
	return restore, len(added) > 0
}

// findDenseRouteSuffixDetour 职责：在目标私有后缀约束下重试一次原整数寻路并复核完整带链。
// 使用：仅由原四次尝试的实际拒绝记录调用；边界：恢复所有临时节点，取消不返回部分结果，仍由原搜索器执行预算、静态盒、带段、预留及原生权限规则。
func (env *transportEnvironment) findDenseRouteSuffixDetour(ctx context.Context, route *transportRoute, prefix denseRoutePortal, suffix denseRouteRejectedSuffix) (full []vector, expanded, maximum, searches int) {
	if ctx.Err() != nil {
		return
	}
	start := prefix.points[len(prefix.points)-1]
	restore, possible := env.guardDenseRouteSuffixNodes(start, suffix)
	defer restore()
	if !possible {
		return
	}
	path, _, expanded, maximum, searches := env.findDenseNativeDestinationPath(ctx, start, []transportDestination{suffix.goal}, route.transportSource(), route.end)
	if ctx.Err() != nil || len(path) == 0 {
		return nil, expanded, maximum, searches
	}
	candidate := append(append([]vector{}, prefix.points[:len(prefix.points)-1]...), path...)
	candidate = append(candidate, suffix.points[1:]...)
	if env.densePathClear(candidate, route.transportSource(), route.end) {
		full = candidate
	}
	return
}
