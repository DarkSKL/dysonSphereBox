package build

// 文件职责：把普通和密铺直连边界的三层槽位落实为直入短带及严格带段净空。
// 使用：原料实体创建和全局路线提交前后；边界：只约束生产实体左侧的接口区，厂内继续使用既有密铺规则。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// boundarySegment 职责：识别经过左侧接口区的真实带段；使用：动态登记和精确查询；边界：跨边界的整段同样受保护。
func (index *denseRoutePortIndex) boundarySegment(first, second vector) bool {
	return index.boundary != nil && min(first.x, second.x) <= index.boundary.maximumX
}

// boundarySpanAllowed 职责：限制边界内的外伸与水平带层；使用：portal、A* 和最终边界复核；边界：平直接口只用 0/2/4 三层，盒外过渡区可升降接入厂内高度，始终执行带段净空。
func (index *denseRoutePortIndex) boundarySpanAllowed(first, second vector) bool {
	if !index.boundarySegment(first, second) {
		return true
	}
	plan := index.boundary
	entry := min(first.x, second.x) <= plan.levelLimitX
	maximumHeight := float64(transportMaxHeight)
	if entry {
		maximumHeight = 4
	}
	for _, point := range []vector{first, second} {
		if point.x < plan.minimumX-1e-6 || point.y < plan.minimumY-1e-6 || point.y > plan.maximumY+1e-6 || point.z < -1e-6 || point.z > maximumHeight+1e-6 {
			return false
		}
	}
	if entry && math.Abs(first.z-second.z) < 1e-6 {
		return math.Abs(first.z-math.Round(first.z/2)*2) < 1e-6
	}
	return true
}

// appendDenseInputStem 职责：在边界与交接列之间建立固定向东短带；使用：普通或密铺分层原料头创建后；边界：原头仍是外部接口，完整真实前缀保留在 route.path 中。
func (b *builder) appendDenseInputStem(route *transportRoute) {
	if b.denseBoundary == nil {
		return
	}
	previous := route.start
	for x := previous.a.x + 1; x <= b.denseBoundary.handoffX+1e-6; x++ {
		next := b.make(2000+b.setting.BeltLv, x, previous.a.y, previous.a.z)
		connect(previous, next)
		route.path = append(route.path, next)
		previous = next
	}
	route.routingEntry = previous
}

// reserveDenseBoundaryEscapes 职责：为未接线入口保护朝厂内的两格出口，避免先行路线横穿后封死整排入口。
// 使用：固定短带已创建、全局寻路开始前；边界：仅保留入口实际高度的有限格位，不预留整列或额外升层，仍执行真实净空检查。
func (env *transportEnvironment) reserveDenseBoundaryEscapes(routes []*transportRoute) {
	if env.densePorts == nil || env.densePorts.boundary == nil {
		return
	}
	for _, route := range routes {
		if route.routingEntry == nil {
			continue
		}
		source := route.transportSource()
		for step := 1.; step <= 2; step++ {
			point := beltCell(vector{source.a.x + step, source.a.y, source.a.z})
			if env.reserved[point] == nil {
				env.reserved[point] = make(map[*placed]bool)
			}
			env.reserved[point][source] = true
		}
	}
}

// orderDenseBoundaryRoutes 职责：先保护厂内受限端口，再从低层到高层接入原料；使用：边界位置冻结后；边界：内部路线及同层外供各自保序，不改变规范份额或真实端点。
func (b *builder) orderDenseBoundaryRoutes(routes []*transportRoute) {
	if b.denseBoundary == nil {
		return
	}
	sort.SliceStable(routes, func(i, j int) bool {
		a, c := routes[i].routingEntry, routes[j].routingEntry
		if a == nil || c == nil {
			return a == nil && c != nil
		}
		return a.a.z < c.a.z
	})
}

// validateDenseBoundaryRoutes 职责：从已提交实体重建接口带段并独立检查交叠；使用：全部路线接完之后；边界：共享真实端点的合流沿用原规则，不能以寻路曾成功代替此项检查。
func (b *builder) validateDenseBoundaryRoutes(ctx context.Context) error {
	if b.denseBoundary == nil {
		return nil
	}
	index := &denseRoutePortIndex{polarFrame: b.polarFrame, boundary: b.denseBoundary, nodes: make(map[[3]int][]*placed), beltCells: make(map[[3]int][]int), beltEdges: make(map[[2]*placed]bool)}
	seen := make(map[*placed]bool)
	for _, entity := range b.all {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !surfaceIsBelt(entity.record.ItemID) {
			continue
		}
		target := entity.output
		if target == nil || !surfaceIsBelt(target.record.ItemID) {
			target = entity
		}
		if !index.boundarySegment(entity.a, target.a) {
			continue
		}
		// 端点坐标相同只有同一真实节点才可豁免，不能把两条独立带误当作合流。
		for _, node := range []*placed{entity, target} {
			if seen[node] {
				continue
			}
			if !index.nodeClear(node.a, nil) {
				return &blueprint.Error{Code: "GEN_E_COLLISION", Message: "直连输入输出区存在重叠的独立带节点"}
			}
			seen[node] = true
			cell := denseRouteNodeCell(index.world(node.a))
			index.nodes[cell] = append(index.nodes[cell], node)
		}
		if !index.boundarySpanAllowed(entity.a, target.a) || !index.beltSpanClear(entity.a, target.a) {
			return &blueprint.Error{Code: "GEN_E_COLLISION", Message: "直连输入输出区存在带段交叠或超出三层接入范围"}
		}
		index.addBeltSpan(entity)
	}
	return nil
}
