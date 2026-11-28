package build

// 文件职责：在原生密铺整图已接通后，尝试用更短的真实带弦替换自由直角。
// 使用：共享尾链绑定及实体编号前；边界：保留现有寻路、端口、合流与所有剩余带节点位置。

import (
	"context"
	"math"
	"slices"
)

// denseRouteCornerPortGuard 职责：保护原生接驳端的前三跳；使用：筛选新带链内部折点；边界：不改变既有端口方向或端口碰撞权限。
const denseRouteCornerPortGuard = 3

// cutDenseRouteCorners 职责：从已验证的全局新带移除可直连的自由折角；使用：所有路线已占位且合流锚点已确定；边界：原生球面厂内带，端部三跳和所有汇合点的进出边受保护，失败保留原链。
// 逻辑流：1. 按提交顺序检查互不重叠的自由折角 => 2. 核对实际球面弦长及原净空 => 3. 一次改接并同步所有引用的路径证据。
func (b *builder) cutDenseRouteCorners(ctx context.Context, env *transportEnvironment, routes []*transportRoute, chains [][]*placed, incoming map[*placed]int) int {
	var cuts [][3]*placed
	for _, chain := range chains {
		for i := denseRouteCornerPortGuard; i+denseRouteCornerPortGuard < len(chain); i++ {
			if ctx.Err() != nil {
				return 0
			}
			before, corner, after := chain[i-1], chain[i], chain[i+1]
			if incoming[before] != 1 || incoming[corner] != 1 || incoming[after] != 1 || before.output != corner || corner.output != after {
				continue
			}
			if env.denseRouteCornerClear(before.a, corner.a, after.a) {
				cuts = append(cuts, [3]*placed{before, corner, after})
				i++
			}
		}
	}
	if len(cuts) == 0 || ctx.Err() != nil {
		return 0
	}
	removed := make(map[*placed]bool, len(cuts))
	for _, cut := range cuts {
		connect(cut[0], cut[2])
		removed[cut[1]] = true
	}
	b.all = slices.DeleteFunc(b.all, func(node *placed) bool { return removed[node] })
	for _, route := range routes {
		route.path = withoutDenseRouteCorners(route.path, removed)
		route.sharedPath = withoutDenseRouteCorners(route.sharedPath, removed)
	}
	return len(cuts)
}

// denseRouteCornerClear 职责：证明同层整数直角的对角连接确实更短且有净空；使用：原生密铺单次粘贴的新带链中段；边界：同批带段交叠沿用密铺规则，严格带段区和外部接口不裁剪，不动用建筑端口豁免。
func (env *transportEnvironment) denseRouteCornerClear(before, corner, after vector) bool {
	index := env.densePorts
	if index == nil || index.polarFrame == nil || index.strictBelts || index.boundarySegment(before, after) {
		return false
	}
	if !denseIntegerNode(before) || !denseIntegerNode(corner) || !denseIntegerNode(after) || before.z != corner.z || corner.z != after.z || math.Abs(before.x-after.x) != 1 || math.Abs(before.y-after.y) != 1 {
		return false
	}
	if math.Abs(before.x-corner.x)+math.Abs(before.y-corner.y) != 1 || math.Abs(after.x-corner.x)+math.Abs(after.y-corner.y) != 1 {
		return false
	}
	// 以与最终带长相同的中心弦计量，不使用用于净空的径向抬升点。
	a, c, d := index.polarFrame.world(before, 0), index.polarFrame.world(corner, 0), index.polarFrame.world(after, 0)
	first, second, direct := a.Sub(c), c.Sub(d), a.Sub(d)
	if math.Sqrt(direct.Dot(direct))+1e-3 >= math.Sqrt(first.Dot(first))+math.Sqrt(second.Dot(second)) {
		return false
	}
	return index.spanClear(before, after) && env.reservedSpanClear(before, after, nil, nil)
}

// withoutDenseRouteCorners 职责：修复规范及共享带链引用；使用：改接完成后；边界：未发生裁剪的切片直接复用，共享底层数组不原地修改。
func withoutDenseRouteCorners(path []*placed, removed map[*placed]bool) []*placed {
	isRemoved := func(node *placed) bool { return removed[node] }
	if !slices.ContainsFunc(path, isRemoved) {
		return path
	}
	return slices.DeleteFunc(slices.Clone(path), isRemoved)
}
