package build

// 文件职责：全图接线完成后缩短普通布局中独立的新带链。
// 使用：绑定共享物流链前；边界：只删除自由带节点，不移动端口、转接设施或其它路线。

import (
	"context"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// transportShortcutWindow 是一次探测允许替换的最大原带段数，限制每个位置的试探成本。
const transportShortcutWindow = 6

// transportShortcutOriginReach 是普通旧带段使用源节点桶筛选的查询扩展，较长旧带段进入补充索引。
const transportShortcutOriginReach = 2.5

// transportShortcutSpan 保存超出源节点查询范围的旧带段及端点。
type transportShortcutSpan struct {
	from, to *placed
	span     surfaceBeltSpan
}

// transportShortcutPlan 记录尚未提交的改接。查询沿用终态带节点桶，另登记少量桶外节点和超长旧段。
type transportShortcutPlan struct {
	extraNodes map[[3]int][]*placed
	longSpans  []transportShortcutSpan
	longIndex  polarSpatialIndex
	next       map[*placed]*placed
	removed    map[*placed]bool
}

// newTransportShortcutPlan 仅在首个候选通过静态检查后准备原带补充项，真实带节点位置在整个终态阶段保持固定。
func newTransportShortcutPlan(ctx context.Context, index *denseRoutePortIndex, all []*placed) *transportShortcutPlan {
	plan := &transportShortcutPlan{next: map[*placed]*placed{}, removed: map[*placed]bool{}}
	for _, node := range all {
		if ctx.Err() != nil {
			return nil
		}
		if !surfaceIsBelt(node.record.ItemID) {
			continue
		}
		first := transportShortcutNodeWorld(index, node)
		cached, known := index.nodeWorld[node]
		if !known || cached.point != node.a || !index.registered[node] {
			if plan.extraNodes == nil {
				plan.extraNodes = make(map[[3]int][]*placed)
			}
			cell := denseRouteNodeCell(first)
			plan.extraNodes[cell] = append(plan.extraNodes[cell], node)
		}
		next := node.output
		if next != nil && surfaceIsBelt(next.record.ItemID) {
			second := transportShortcutNodeWorld(index, next)
			delta := second.Sub(first)
			if delta.Dot(delta) > transportShortcutOriginReach*transportShortcutOriginReach {
				span := surfaceBeltSpan{ends: [2]placement.Vector{first, second}}
				low, high := polarSpanBounds(span, ordinaryBeltRadius)
				plan.longIndex.add(low, high)
				plan.longSpans = append(plan.longSpans, transportShortcutSpan{node, next, span})
			}
		}
	}
	return plan
}

// transportShortcutNodeWorld 复用实际节点的世界坐标，未登记的终态节点按当前框架投影。
func transportShortcutNodeWorld(index *denseRoutePortIndex, node *placed) placement.Vector {
	if cached, known := index.nodeWorld[node]; known && cached.point == node.a {
		return cached.world
	}
	return index.world(node.a)
}

// clear 从源节点的空间桶读取现行出边，包含已获选的新边及末端节点。旧长段另用自身包围盒查询。
func (plan *transportShortcutPlan) clear(index *denseRoutePortIndex, window []*placed) bool {
	from, to := window[0], window[len(window)-1]
	span := surfaceBeltSpan{ends: [2]placement.Vector{transportShortcutNodeWorld(index, from), transportShortcutNodeWorld(index, to)}}
	interior := window[1 : len(window)-1]
	const clearance = 2 * ordinaryBeltRadius
	const limit = clearance*clearance - 1e-6
	ignores := func(first, second *placed) bool {
		return plan.removed[first] || first == from || second == from || first == to || second == to || slices.Contains(interior, first)
	}
	queryLow, queryHigh := polarSpanBounds(span, clearance)
	originLow, originHigh := polarSpanBounds(span, clearance+transportShortcutOriginReach+.001)
	cells := worldSpatialRange{low: denseRouteNodeCell(originLow), high: denseRouteNodeCell(originHigh)}
	conflicts := func(nodes []*placed) bool {
		for _, first := range nodes {
			second := plan.next[first]
			if second == nil {
				second = first.output
				if second == nil || !surfaceIsBelt(second.record.ItemID) {
					second = first
				}
			}
			if ignores(first, second) {
				continue
			}
			a := transportShortcutNodeWorld(index, first)
			if a[0] < originLow[0] || a[0] > originHigh[0] || a[1] < originLow[1] || a[1] > originHigh[1] || a[2] < originLow[2] || a[2] > originHigh[2] {
				continue
			}
			c := transportShortcutNodeWorld(index, second)
			if max(a[0], c[0]) < queryLow[0] || min(a[0], c[0]) > queryHigh[0] ||
				max(a[1], c[1]) < queryLow[1] || min(a[1], c[1]) > queryHigh[1] ||
				max(a[2], c[2]) < queryLow[2] || min(a[2], c[2]) > queryHigh[2] {
				continue
			}
			if surfaceSpanDistanceSquared(span, surfaceBeltSpan{ends: [2]placement.Vector{a, c}}) < limit {
				return true
			}
		}
		return false
	}
	if !cells.each(func(cell [3]int) bool { return !conflicts(index.nodes[cell]) && !conflicts(plan.extraNodes[cell]) }) {
		return false
	}
	low, high := polarSpanBounds(span, ordinaryBeltRadius)
	return !plan.longIndex.any(low, high, func(id int) bool {
		other := &plan.longSpans[id]
		return plan.next[other.from] == nil && !ignores(other.from, other.to) && surfaceSpanDistanceSquared(span, other.span) < limit
	})
}

// shortenStrictTransportRoutes 试探已完成的普通直连和塔黑盒接线。失败的试探不修改原链，取消时丢弃整个改接计划。
func (b *builder) shortenStrictTransportRoutes(ctx context.Context, env *transportEnvironment, routes []*transportRoute, incoming map[*placed]int) int {
	index := env.densePorts
	if index == nil || !index.strictBelts || usesDenseBeltCollisions(ctx) || ctx.Err() != nil {
		return 0
	}
	protected := b.transportShortcutProtected(ctx, routes)
	if protected == nil {
		return 0
	}
	// 静态检查沿用原盒、带长及接驳区规则。动态碰撞由终态计划精确排除被替换的边。
	static := *index
	static.strictBelts = false
	static.integerEdges = nil
	static.edges = make(map[[2]vector]bool)
	static.staticQuery = collisionQueryMarks{}
	var plan *transportShortcutPlan
	for _, route := range b.transportRoutes {
		if ctx.Err() != nil {
			return 0
		}
		path := route.path
		source := route.transportSource()
		if source == nil {
			continue
		}
		if at := slices.Index(path, source); at >= 0 {
			path = path[at+1:]
		}
		if len(path) == 0 || path[0] != source.output || path[len(path)-1] != route.end {
			continue
		}
		last := len(path) - 1 - denseRouteCornerPortGuard
		for first := denseRouteCornerPortGuard - 1; first+2 <= last; first++ {
			if ctx.Err() != nil {
				return 0
			}
			before := path[first]
			if path[first-1].output != before || !surfaceIsBelt(before.record.ItemID) {
				continue
			}
			end := first
			var lengths [transportShortcutWindow + 1]float64
			var centers [transportShortcutWindow + 1]placement.Vector
			for ; end <= last && end-first <= transportShortcutWindow; end++ {
				point := path[end]
				if protected[point] || incoming[point] != 1 || point.input != nil || len(index.previous[point]) != 0 ||
					point.owner != -1 || point.a.z != before.a.z || point.record.ItemID != before.record.ItemID || point.output != path[end+1] ||
					index.boundarySegment(point.a, point.a) || env.reservedByOther(point.a, nil, nil) {
					break
				}
				offset := end - first
				centers[offset] = transportShortcutCenter(index, point.a)
				if offset > 0 {
					lengths[offset] = lengths[offset-1] + transportShortcutLength(centers[offset-1], centers[offset])
				}
			}
			for after := end - 1; after >= first+2; after-- {
				from, to := before.a, path[after].a
				length := transportShortcutLength(centers[0], centers[after-first])
				if length > denseRouteMaximumSpan || length+1e-3 >= lengths[after-first] ||
					!transportShortcutForward(path[first-1].a, from, to) || !transportShortcutForward(from, to, path[after+1].a) ||
					!env.reservedSpanClear(from, to, nil, nil) || !static.spanClear(from, to) ||
					!static.transportShortcutNodeClear(path[first-1].a, from, to) || !static.transportShortcutNodeClear(from, to, path[after+1].a) {
					continue
				}
				if plan == nil {
					plan = newTransportShortcutPlan(ctx, index, b.all)
					if plan == nil {
						return 0
					}
				}
				if !plan.clear(index, path[first:after+1]) {
					continue
				}
				plan.next[before] = path[after]
				for _, node := range path[first+1 : after] {
					plan.removed[node] = true
				}
				first = after
				break
			}
		}
	}
	if plan == nil || len(plan.removed) == 0 || ctx.Err() != nil {
		return 0
	}
	for from, to := range plan.next {
		connect(from, to)
	}
	b.all = slices.DeleteFunc(b.all, func(node *placed) bool { return plan.removed[node] })
	for _, route := range routes {
		route.path = withoutDenseRouteCorners(route.path, plan.removed)
	}
	return len(plan.removed)
}

// transportShortcutProtected 收集边界、共享尾链、附件和本地路线的原始节点，保持这些接线及其证据。
func (b *builder) transportShortcutProtected(ctx context.Context, routes []*transportRoute) map[*placed]bool {
	protected := map[*placed]bool{}
	seen := map[*placed]bool{}
	localEvidence := map[*[]*placed]bool{}
	for _, route := range routes {
		if ctx.Err() != nil {
			return nil
		}
		localEvidence[&route.path], localEvidence[&route.sharedPath] = true, true
		for _, endpoint := range []*placed{route.start, route.end, route.boundary, route.priorityInput, route.routingEntry} {
			protected[endpoint] = true
		}
		for _, node := range route.sharedPath {
			protected[node] = true
		}
		for _, node := range route.path {
			if seen[node] {
				protected[node] = true
			}
			seen[node] = true
		}
	}
	for _, path := range b.transportPathEvidence() {
		if !localEvidence[path] {
			for _, node := range *path {
				protected[node] = true
			}
		}
	}
	for _, link := range b.stationLinks {
		protected[link.head], protected[link.belt] = true, true
		for _, node := range link.path {
			protected[node] = true
		}
	}
	for _, node := range b.all {
		if ctx.Err() != nil {
			return nil
		}
		if !surfaceIsBelt(node.record.ItemID) {
			protected[node.input], protected[node.output], protected[node.flowMonitorHost] = true, true, true
		}
		if len(node.record.Parameters) != 0 || node.splitterPortLink != nil || node.productionRate != nil {
			protected[node] = true
		}
		for _, belt := range node.densityCargo.track {
			protected[belt] = true
		}
	}
	if preserveFlowMonitorHosts(b.all, protected) != nil {
		return nil
	}
	return protected
}

// transportShortcutCenter 使用当前路由坐标系去掉仅用于碰撞的 0.2 米抬升，获得实体中心。
func transportShortcutCenter(index *denseRoutePortIndex, point vector) placement.Vector {
	world := index.world(point)
	if index.polarFrame == nil {
		return world.Sub(placement.Vector{0, .2, 0})
	}
	return world.Sub(surfaceUnit(world).Scale(.2))
}

// transportShortcutLength 按与最终带长相同的实体中心弦计量。
func transportShortcutLength(first, second placement.Vector) float64 {
	delta := first.Sub(second)
	return math.Sqrt(delta.Dot(delta))
}

// transportShortcutNodeClear 用改接后的前后邻居核对节点胶囊，既有静态索引内不借用建筑许可。
func (index *denseRoutePortIndex) transportShortcutNodeClear(before, point, after vector) bool {
	encoding := surfaceEncoding{planar: index.polarFrame == nil}
	probe := encoding.beltNodeProbe(transportShortcutCenter(index, point), transportShortcutCenter(index, before), transportShortcutCenter(index, after), true)
	shape := beltPackingShape(probe.center, probe.center)
	index.staticQuery.begin(len(index.boxes))
	return packingShapeCells(shape, func(cell [3]int) bool {
		for _, id := range index.cells[cell] {
			if !index.staticQuery.seen(id) && probe.overlapsBox(index.boxes[id].box) {
				return false
			}
		}
		return true
	})
}

// transportShortcutForward 拒绝改接后在保留端点处折返，同层直角仍有效。
func transportShortcutForward(first, middle, last vector) bool {
	return (middle.x-first.x)*(last.x-middle.x)+(middle.y-first.y)*(last.y-middle.y) >= -1e-9
}
