package build

// 文件职责：把密铺分数坐标的低层带端口接入整数寻路网格，保留每一段真实带链。
// 使用：密铺全场接线；边界：仅新增经过静态盒、节点净距和带长检查的接驳，不移动机器或借接其它物料。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint/build/placement"
)

// denseRouteNodeClearance 职责：保存独立带节点的米制净距；使用：接驳及整链复核；边界：仅实际复用的端点可豁免自身。
const denseRouteNodeClearance = .35

// denseRouteMaximumSpan 职责：限制相邻真实带段的米制长度；使用：分数端口升降和整数搜索；边界：不允许用长跳绕过中间静态盒。
const denseRouteMaximumSpan = 2.15

// denseRoutePortIndex 职责：索引当前真实静态盒和带节点；使用：分数接驳及整数边检查；边界：普通模式额外检查带段交叉，超密度保留原路由规则。
type denseRoutePortIndex struct {
	// footprintLow/High 职责：保存接线前实体包络；使用：外绕软代价；边界：新增路线不扩大此基准，空索引不启用。
	footprintLow, footprintHigh vector
	footprintReady              bool
	// polarFrame 职责：令所有距离及静态盒使用同一原生球面；使用：跨子环接线；边界：nil仍用既有平面预检。
	polarFrame *densePolarFrame
	boxes      []connectedPackingShape
	// strictBelts 职责：普通布局寻路时禁止与已提交的独立带段相交；使用：共享塔/配送全场候选；边界：超密度保留原带段规则，最终仍统一验收。
	strictBelts bool
	belts       []surfaceBeltSpan
	beltCells   map[[3]int][]int
	beltEdges   map[[2]*placed]bool
	// origins 职责：保存每个静态盒所属建筑的切平面原点；使用：临界高架顶部的局部球面投影；边界：不修改模型盒尺寸。
	origins  []placement.Vector
	owners   []*placed
	cells    map[[3]int][]int
	nodes    map[[3]int][]*placed
	previous map[*placed][]*placed
	edges    map[[2]vector]bool
	// integerEdges 职责：缓存实际边界内无权限整数邻边；使用：避免稀疏缓存饱和后重复实算；边界：动态节点及原生端口权限始终独立判断。
	integerEdges *denseIntegerEdgeCache
	registered   map[*placed]bool
}

// newDenseRoutePortIndex 职责：沿用全场真实轮廓的静态模型建立接驳索引；使用：密铺寻路开始；边界：不把分拣器粗矩形加回密铺障碍。
func (b *builder) newDenseRoutePortIndex(ctx context.Context) (*denseRoutePortIndex, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	index := &denseRoutePortIndex{strictBelts: b.adapter != nil && b.adapter.density == densityStandard, cells: map[[3]int][]int{}, nodes: map[[3]int][]*placed{}, previous: map[*placed][]*placed{}, edges: map[[2]vector]bool{}, registered: map[*placed]bool{}, beltCells: map[[3]int][]int{}, beltEdges: map[[2]*placed]bool{}}
	if len(b.all) > 0 {
		index.footprintLow, index.footprintHigh = bounds(b.all)
		index.footprintReady = true
	}
	encoding := newPlanarRuleEncoding()
	if b.polarFrame != nil {
		frame := *b.polarFrame
		index.polarFrame = &frame
		encoding = &surfaceEncoding{}
	}
	for _, entity := range b.all {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if surfaceIsBelt(entity.record.ItemID) || entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
			continue
		}
		shapes, err := catalog.Shapes(entity.record.ItemID, entity.record.ModelIndex)
		if err != nil {
			return nil, err
		}
		origin := placement.Vector{entity.a.x * planarGridLength, entity.a.z*1.3333333 + .2, entity.a.y * planarGridLength}
		if index.polarFrame != nil {
			origin = index.polarFrame.world(entity.a, 0)
		}
		for _, collider := range shapes {
			box, err := encoding.colliderBox(vector{origin[0], origin[1], origin[2]}, entity.record.Yaw, collider)
			if err != nil {
				return nil, err
			}
			shape := connectedBoxShape("static", box)
			n := len(index.boxes)
			index.boxes = append(index.boxes, shape)
			index.origins = append(index.origins, origin)
			index.owners = append(index.owners, entity)
			connectedShapeCells(shape, func(cell [3]int) bool {
				index.cells[cell] = append(index.cells[cell], n)
				return true
			})
		}
	}
	for _, p := range b.all {
		index.addNode(p)
		if surfaceIsBelt(p.record.ItemID) && p.output != nil && surfaceIsBelt(p.output.record.ItemID) {
			index.previous[p.output] = append(index.previous[p.output], p)
		}
	}
	return index, nil
}

// denseRouteWorld 职责：使用最终平面规则的带中心高度换算米制坐标；使用：接驳距离与碰撞；边界：球面终检仍在编码后独立执行。
func denseRouteWorld(p vector) placement.Vector {
	return placement.Vector{p.x * planarGridLength, p.z*1.3333333 + .4, p.y * planarGridLength}
}

// world 职责：在索引固定坐标系内计算带节点中心；使用：全部边和净距检查；边界：不能混用平面盒和球面点。
func (index *denseRoutePortIndex) world(p vector) placement.Vector {
	if index.polarFrame != nil {
		return index.polarFrame.world(p, .2)
	}
	return denseRouteWorld(p)
}

// denseRouteNodeCell 职责：索引带节点所在的八米空间桶；使用：动态占位；边界：查询会枚举净距球覆盖的所有桶。
func denseRouteNodeCell(p placement.Vector) [3]int {
	return [3]int{int(math.Floor(p[0] / 8)), int(math.Floor(p[1] / 8)), int(math.Floor(p[2] / 8))}
}

// addNode 职责：登记已提交真实带节点；使用：增量接线；边界：同一实体仅登记一次。
func (index *denseRoutePortIndex) addNode(p *placed) {
	if !surfaceIsBelt(p.record.ItemID) {
		return
	}
	index.addBeltSpan(p)
	if index.registered[p] {
		return
	}
	index.registered[p] = true
	cell := denseRouteNodeCell(index.world(p.a))
	index.nodes[cell] = append(index.nodes[cell], p)
}

// addBeltSpan 职责：增量登记真实有向带段；使用：普通全场候选每次提交路线后；边界：仅供寻路避碰，不改变实体连接或密铺模式。
func (index *denseRoutePortIndex) addBeltSpan(p *placed) {
	if !index.strictBelts || p.output == nil || !surfaceIsBelt(p.output.record.ItemID) {
		return
	}
	key := [2]*placed{p, p.output}
	if index.beltEdges[key] {
		return
	}
	index.beltEdges[key] = true
	first, second := index.world(p.a), index.world(p.output.a)
	span := connectedBeltShape(first, second)
	index.belts = append(index.belts, span.span)
	number := len(index.belts) - 1
	connectedShapeCells(span, func(cell [3]int) bool {
		index.beltCells[cell] = append(index.beltCells[cell], number)
		return true
	})
}

// beltSpanClear 职责：普通模式拒绝穿越已存在带段的新增边；使用：低轨接驳及整数寻路；边界：共同端点由连通性检查处理，静态净空另行检查。
func (index *denseRoutePortIndex) beltSpanClear(first, second vector) bool {
	if !index.strictBelts {
		return true
	}
	a, c := index.world(first), index.world(second)
	candidate := connectedBeltShape(a, c)
	seen := map[int]bool{}
	clear := connectedShapeCells(candidate, func(cell [3]int) bool {
		for _, number := range index.beltCells[cell] {
			if seen[number] {
				continue
			}
			seen[number] = true
			existing := index.belts[number]
			if a == existing.ends[0] || a == existing.ends[1] || c == existing.ends[0] || c == existing.ends[1] {
				continue
			}
			if surfaceSpanDistanceSquared(candidate.span, existing) < .46*.46-1e-6 {
				return false
			}
		}
		return true
	})
	return clear
}

// nodeClear 职责：检查新节点不会被现有带自动合并；使用：分数接驳；边界：只豁免明确复用的已有端点自身，不豁免其它同物料节点。
func (index *denseRoutePortIndex) nodeClear(p vector, reused *placed) bool {
	world := index.world(p)
	padding := placement.Vector{denseRouteNodeClearance, denseRouteNodeClearance, denseRouteNodeClearance}
	low, high := denseRouteNodeCell(world.Sub(padding)), denseRouteNodeCell(world.Add(padding))
	for x := low[0]; x <= high[0]; x++ {
		for y := low[1]; y <= high[1]; y++ {
			for z := low[2]; z <= high[2]; z++ {
				for _, existing := range index.nodes[[3]int{x, y, z}] {
					if existing == reused && existing.a == p {
						continue
					}
					delta := world.Sub(index.world(existing.a))
					if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
						return false
					}
				}
			}
		}
	}
	return true
}

// spanClear 职责：核对相邻真实带段长度及沿途静态盒；使用：接驳和密铺A*边；边界：不使用只检测两个端点的穿墙近似。
func (index *denseRoutePortIndex) spanClear(first, second vector) bool {
	return index.spanClearWithLinks(first, second, nil)
}

// spanClearWithLinks 职责：只为真实原生端口的有限连接跳数应用逐端点豁免；使用：portal端部；边界：无端口身份时仍按完整静态带段检查，许可不进入无权限缓存。
func (index *denseRoutePortIndex) spanClearWithLinks(first, second vector, links map[*placed][2]bool) bool {
	slot, shift := -1, uint(0)
	if len(links) == 0 {
		slot, shift = index.integerEdges.slot(first, second)
		if slot >= 0 {
			if state := index.integerEdges.states[slot] >> shift & 3; state != 0 {
				return state == 2 && index.beltSpanClear(first, second)
			}
		}
	}
	var key [2]vector
	if slot < 0 {
		key = [2]vector{first, second}
		if first.x > second.x || first.x == second.x && (first.y > second.y || first.y == second.y && first.z > second.z) {
			key[0], key[1] = second, first
		}
		if clear, known := index.edges[key]; known && len(links) == 0 {
			return clear && index.beltSpanClear(first, second)
		}
	}
	a, c := index.world(first), index.world(second)
	delta := a.Sub(c)
	lengthSquared := delta.Dot(delta)
	clear := lengthSquared >= denseRouteNodeClearance*denseRouteNodeClearance-1e-9 && lengthSquared <= denseRouteMaximumSpan*denseRouteMaximumSpan+1e-9
	if clear {
		span := connectedBeltShape(a, c)
		// 曲率只可能把擦顶带向对象局部切平面降低；扩大查询范围以发现跨空间桶的临界顶部，不扩大任何碰撞盒。
		span.low[1] -= .5
		seen := map[int]bool{}
		connectedShapeCells(span, func(cell [3]int) bool {
			for _, n := range index.cells[cell] {
				if seen[n] {
					continue
				}
				seen[n] = true
				if n < len(index.owners) {
					allowed := links[index.owners[n]]
					if (allowed[0] || allowed[1]) && (allowed[0] || index.boxes[n].box.SegmentDistanceSquared(a, a) >= .23*.23-1e-6) && (allowed[1] || index.boxes[n].box.SegmentDistanceSquared(c, c) >= .23*.23-1e-6) {
						continue
					}
				}
				if index.boxes[n].box.SegmentDistanceSquared(a, c) < .23*.23-1e-6 {
					clear = false
					break
				}
				if index.polarFrame == nil && n < len(index.origins) && math.Min(a[1], c[1]) > index.boxes[n].high[1] {
					firstCurved := denseRouteCurvedPoint(a, index.origins[n])
					secondCurved := denseRouteCurvedPoint(c, index.origins[n])
					if index.boxes[n].box.SegmentDistanceSquared(firstCurved, secondCurved) < .23*.23-1e-6 {
						clear = false
						break
					}
				}
			}
			return clear
		})
	}
	// 只缓存静态几何，正反方向共用一个键；大场景达到上限后继续实算，不膨胀为整个三维搜索空间的永久副本。
	if len(links) == 0 {
		if slot >= 0 {
			index.integerEdges.store(slot, shift, clear)
		} else if len(index.edges) < 131072 {
			index.edges[key] = clear
		}
	}
	return clear && index.beltSpanClear(first, second)
}

// denseRouteCurvedPoint 职责：把候选点投影到建筑原点的200米局部球面切平面；使用：高架擦顶复核；边界：保留真实径向高度和横向展开，最终选区坐标仍由球面终检决定，低轨不走此附加分支。
func denseRouteCurvedPoint(point, origin placement.Vector) placement.Vector {
	dx, dz := point[0]-origin[0], point[2]-origin[2]
	distance := math.Hypot(dx, dz)
	if distance < 1e-9 {
		return point
	}
	angle := distance / 200.2
	radius := 200 + point[1]
	scale := radius * math.Sin(angle) / distance
	return placement.Vector{origin[0] + dx*scale, radius*math.Cos(angle) - 200, origin[2] + dz*scale}
}

// denseTransportStepClear 职责：为密铺整数A*补充整段静态检查；使用：所有共享寻路器；边界：普通模式索引为空时保持原行为。
func (env *transportEnvironment) denseTransportStepClear(first, second vector) bool {
	return env.densePorts == nil || env.densePorts.spanClear(first, second)
}

// denseIntegerNode 职责：区分可作为整数A*根的真实节点；使用：合流目标筛选；边界：分数尾链仍保留并参与实际接线。
func denseIntegerNode(p vector) bool {
	return p.x == math.Round(p.x) && p.y == math.Round(p.y) && p.z == math.Round(p.z)
}

// outward 职责：从真实低轨邻居恢复端口向外方向；使用：折排和四向街道；边界：未知或合流端点才回退到调用者已有方向。
func (index *denseRoutePortIndex) outward(endpoint *placed, fallback vector, source bool) vector {
	inner := endpoint.output
	if source {
		inner = endpoint.input
		if previous := index.previous[endpoint]; len(previous) == 1 {
			inner = previous[0]
		}
	}
	if inner != nil && (surfaceIsBelt(inner.record.ItemID) || denseNativeBeltPort(inner.record.ItemID)) {
		dx, dy := endpoint.a.x-inner.a.x, endpoint.a.y-inner.a.y
		if length := math.Hypot(dx, dy); length > 1e-6 {
			return vector{dx / length, dy / length, 0}
		}
	}
	if length := math.Hypot(fallback.x, fallback.y); length > 1e-6 {
		return vector{fallback.x / length, fallback.y / length, 0}
	}
	return vector{x: -1}
}

// denseRoutePortal 职责：保存从现有端点到整数格的完整连续带链；使用：源前缀及反向目标后缀；边界：首节点为复用实体，其余节点尚未创建。
type denseRoutePortal struct {
	points []vector
	cost   float64
	height float64
}

// denseRouteLength 职责：计算真实米制接驳长度；使用：最低层内优先短接；边界：不把参考臂端点当成带节点。
func denseRouteLength(points []vector) float64 {
	length := 0.
	for i := 1; i < len(points); i++ {
		delta := denseRouteWorld(points[i]).Sub(denseRouteWorld(points[i-1]))
		length += math.Sqrt(delta.Dot(delta))
	}
	return length
}

// densePortals 职责：先尝试直接短接，再按高度建立连续抬升的有限候选；使用：密铺端口，可按完整整数高度层延迟展开源端；边界：最多保留每高度两个不同出口，省略高度上限时覆盖全部层，所有新增节点逐个验净距。
func (env *transportEnvironment) densePortals(endpoint, source, target *placed, preferred vector, sourceSide, fixedGroundStem bool, heightLimits ...float64) []denseRoutePortal {
	index := env.densePorts
	maximumHeight := float64(transportMaxHeight)
	if len(heightLimits) > 0 {
		maximumHeight = math.Min(maximumHeight, heightLimits[0])
	}
	outward := index.outward(endpoint, preferred, sourceSide)
	linked := index.linkedPortBuildings(endpoint, sourceSide)
	base := []vector{endpoint.a}
	if fixedGroundStem {
		stem := vector{endpoint.a.x + 1, endpoint.a.y, endpoint.a.z}
		if !index.nodeClear(stem, nil) || !index.spanClear(endpoint.a, stem) || env.reservedByOther(stem, source, target) {
			return nil
		}
		base = append(base, stem)
		outward = vector{x: 1}
	}
	inside := func(p vector) bool {
		return p.x >= env.minimum.x && p.x <= env.maximum.x && p.y >= env.minimum.y && p.y <= env.maximum.y && p.z >= 0 && p.z <= maximumHeight
	}
	clearNew := func(path []vector, p vector) bool {
		permissions := densePortalLinkPermissions(linked, len(path)-1, len(path))
		if !inside(p) || !index.nodeClear(p, nil) || env.reservedByOther(p, source, target) || !index.spanClearWithLinks(path[len(path)-1], p, permissions) {
			return false
		}
		for _, previous := range path {
			delta := index.world(previous).Sub(index.world(p))
			if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
				return false
			}
		}
		return true
	}
	byExit := map[vector]denseRoutePortal{}
	appendCandidate := func(path []vector, p vector) {
		if !denseIntegerNode(p) || !clearNew(path, p) {
			return
		}
		if len(path) == 1 && (p.x-endpoint.a.x)*outward.x+(p.y-endpoint.a.y)*outward.y < -1e-6 {
			return
		}
		points := append(append([]vector{}, path...), p)
		candidate := denseRoutePortal{points: points, cost: denseRouteLength(points)}
		for _, point := range points {
			candidate.height = math.Max(candidate.height, point.z)
		}
		old, exists := byExit[p]
		if !exists || candidate.cost < old.cost-1e-9 || math.Abs(candidate.cost-old.cost) <= 1e-9 && len(points) < len(old.points) {
			byExit[p] = candidate
		}
	}
	last := base[len(base)-1]
	// 老整数口优先保持原有短接，分数口允许邻近整格，但不能退回低轨内部。
	appendCandidate(base, vector{math.Round(last.x + outward.x), math.Round(last.y + outward.y), math.Round(last.z)})
	stems := [][]vector{base}
	if !fixedGroundStem {
		for _, length := range []float64{.5, 1} {
			stem := vector{last.x + outward.x*length, last.y + outward.y*length, last.z}
			if clearNew(base, stem) {
				stems = append(stems, append(append([]vector{}, base...), stem))
			}
		}
	}
	for _, stem := range stems {
		origin := stem[len(stem)-1]
		reachable := [][]vector{stem}
		for z := math.Ceil(origin.z*2) / 2; z <= maximumHeight; z += .5 {
			path := stem
			if z > origin.z+1e-9 {
				point := vector{origin.x, origin.y, z}
				path = nil
				for i := len(reachable) - 1; i >= 0; i-- {
					previous := reachable[i]
					if z-previous[len(previous)-1].z > denseRouteMaximumSpan/1.3333333 {
						break
					}
					if clearNew(previous, point) && (path == nil || len(previous)+1 < len(path)) {
						path = append(append([]vector{}, previous...), point)
					}
				}
				if path == nil {
					continue
				}
				reachable = append(reachable, path)
			}
			point := path[len(path)-1]
			if denseIntegerNode(point) && len(path) > 1 {
				candidate := denseRoutePortal{points: path, cost: denseRouteLength(path), height: point.z}
				if old, exists := byExit[point]; !exists || candidate.cost < old.cost {
					byExit[point] = candidate
				}
			}
			for dx := -1.; dx <= 1; dx++ {
				for dy := -1.; dy <= 1; dy++ {
					appendCandidate(path, vector{math.Round(point.x) + dx, math.Round(point.y) + dy, math.Round(z)})
				}
			}
		}
	}
	portals := make([]denseRoutePortal, 0, len(byExit))
	for _, portal := range byExit {
		portals = append(portals, portal)
	}
	sort.Slice(portals, func(i, j int) bool {
		a, c := portals[i], portals[j]
		if a.height != c.height {
			return a.height < c.height
		}
		if math.Abs(a.cost-c.cost) > 1e-9 {
			return a.cost < c.cost
		}
		if len(a.points) != len(c.points) {
			return len(a.points) < len(c.points)
		}
		x, y := a.points[len(a.points)-1], c.points[len(c.points)-1]
		if x.x != y.x {
			return x.x < y.x
		}
		if x.y != y.y {
			return x.y < y.y
		}
		return x.z < y.z
	})
	limited := make([]denseRoutePortal, 0, 2*(transportMaxHeight+1))
	levels := map[int]int{}
	for _, portal := range portals {
		level := int(math.Ceil(portal.height))
		if levels[level] < 2 {
			limited = append(limited, portal)
			levels[level]++
		}
	}
	return limited
}

// densePathClear 职责：复核完整接驳带链的节点、静态盒和距离；使用：提交前；边界：只复用两端实体，跨物料或路径自身近点直接拒绝。
func (env *transportEnvironment) densePathClear(points []vector, source, target *placed) bool {
	firstLinks := env.densePorts.linkedPortBuildings(source, true)
	lastLinks := env.densePorts.linkedPortBuildings(target, false)
	for i, point := range points {
		var reused *placed
		if i == 0 {
			reused = source
		}
		if i == len(points)-1 {
			reused = target
		}
		if reused == nil && !env.densePorts.nodeClear(point, nil) {
			return false
		}
		if i > 0 {
			permissions := densePortalLinkPermissions(firstLinks, i-1, i)
			for owner, allowed := range densePortalLinkPermissions(lastLinks, len(points)-i, len(points)-i-1) {
				previous := permissions[owner]
				permissions[owner] = [2]bool{previous[0] || allowed[0], previous[1] || allowed[1]}
			}
			if !env.densePorts.spanClearWithLinks(points[i-1], point, permissions) {
				return false
			}
		}
		for j := 0; j < i; j++ {
			delta := env.densePorts.world(point).Sub(env.densePorts.world(points[j]))
			if delta.Dot(delta) < denseRouteNodeClearance*denseRouteNodeClearance-1e-9 {
				return false
			}
		}
	}
	return true
}

// findDenseTransportPath 职责：按完整端口路线代价比较最低可行源高度层；使用：密铺直连及配送；边界：返回不含源且包含复用目标的路径，合流索引保持原尾链身份，取消丢弃未提交结果。
func (env *transportEnvironment) findDenseTransportPath(ctx context.Context, route *transportRoute, tail []*placed, incoming map[*placed]int, preferred vector) (points []vector, tailIndex, expanded, maximum, searches int) {
	tailIndex = -1
	source, target := route.start, route.end
	var targetPortals []denseRoutePortal
	if len(tail) == 0 {
		fallback := vector{x: -1}
		if route.to != nil {
			fallback = denseRotateVector(fallback, route.to.quarterTurns)
		}
		// 裸带回收边界保持既有东向入口；塔/箱黑盒由真实内侧邻居确定方向。
		targetPortals = env.densePortals(target, source, target, fallback, false, route.to == nil && route.edge.FlowKind == "leftover" && route.boundary == route.end)
	}
	tried := map[vector]bool{}
	initialHeight := math.Min(transportMaxHeight, math.Max(0, math.Ceil(source.a.z)))
	for height := initialHeight; height <= transportMaxHeight; height++ {
		var best []vector
		bestIndex, bestLength, bestCost := -1, math.MaxInt, math.Inf(1)
		for _, prefix := range env.densePortals(source, source, target, preferred, true, false, height) {
			if ctx.Err() != nil {
				return nil, -1, expanded, maximum, searches
			}
			start := prefix.points[len(prefix.points)-1]
			if tried[start] {
				continue
			}
			tried[start] = true
			if env.transportPathCost(prefix.points) > bestCost {
				continue
			}
			full, selected, cost, length, steps, peak, calls := env.findDenseTransportPrefixPath(ctx, route, tail, incoming, prefix, targetPortals)
			expanded, maximum, searches = expanded+steps, max(maximum, peak), searches+calls
			if len(full) > 0 && (cost < bestCost || cost == bestCost && length < bestLength) {
				best, bestIndex, bestCost, bestLength = full, selected, cost, length
			}
		}
		if ctx.Err() != nil {
			return nil, -1, expanded, maximum, searches
		}
		if len(best) > 0 {
			return best[1:], bestIndex, expanded, maximum, searches
		}
	}
	return
}
