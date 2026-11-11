package build

// 文件职责：为逐物料 lane 执行有界三维寻路、增量占位和严格实体接线。
// 使用：所有机器行和过滤端口已落位后；边界：未知实体几何和无路返回错误，不降级为玩家手接内部带。

import (
	"container/heap"
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// transportSearchLimit 职责：限制每条路线的搜索展开；使用：三维 A*；边界：与生成 context 的四分钟上限共同生效。
const transportSearchLimit = 300000

// transportMaxHeight 职责：限定已经过带距离预检的候选高度层；使用：三维路由；边界：失败不无限堆高规避规则。
const transportMaxHeight = 32

// transportPathNode 职责：保存三维 A* 候选；使用：稳定最小堆；边界：精确搜索不重开关闭节点，有界引导搜索允许更低代价重新打开。
type transportPathNode struct {
	point        vector
	parent       *transportPathNode
	distance     float64
	score        float64
	order, index int
	closed       bool
	discarded    bool
	// length 职责：保留真实路径节点数；使用：已有可行候选的安全上界剪枝；边界：不同于带高度代价的distance。
	length int
	// lengthEstimate 职责：保存多终点搜索的节点数下界；使用：同代价时优先较短完整带链；边界：普通单终点队列不读取。
	lengthEstimate int
}

// transportPathQueue 职责：稳定选择估价最低且同分时更接近终点的候选；使用：三维寻路；边界：最终同分按插入顺序而非 map 顺序。
type transportPathQueue []*transportPathNode

// Len 职责：提供堆长度；使用：container/heap；边界：不统计关闭节点。
func (q transportPathQueue) Len() int { return len(q) }

// Less 职责：按估价、已行距离和稳定键排序；使用：container/heap；边界：只在相同估价内优先更小剩余估价，不改变 A* 主序或搜索预算。
func (q transportPathQueue) Less(i, j int) bool {
	if q[i].score != q[j].score {
		return q[i].score < q[j].score
	}
	if q[i].distance != q[j].distance {
		return q[i].distance > q[j].distance
	}
	return q[i].order < q[j].order
}

// Swap 职责：同步堆节点位置；使用：container/heap；边界：不改路径。
func (q transportPathQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i]; q[i].index = i; q[j].index = j }

// Push 职责：登记候选位置；使用：container/heap；边界：仅接收寻路节点。
func (q *transportPathQueue) Push(value interface{}) {
	node := value.(*transportPathNode)
	node.index = len(*q)
	*q = append(*q, node)
}

// Pop 职责：弹出最低估价候选；使用：container/heap；边界：关闭状态由搜索器维护。
func (q *transportPathQueue) Pop() interface{} {
	old := *q
	node := old[len(old)-1]
	old[len(old)-1] = nil
	*q = old[:len(old)-1]
	node.index = -1
	return node
}

// transportPathCacheKey 职责：为同一占位版本缓存路径；使用：候选内重复查询；边界：占位变更后不复用旧路径。
type transportPathCacheKey struct {
	start, end vector
	version    int
	// source/target 职责：保留本路允许使用的端口柱身份；使用：同坐标不同权限的缓存隔离；边界：仅作单个候选内指针键，不输出到诊断。
	source, target *placed
	// merge 职责：区分接入已有合流点的占位权限；使用：缓存匹配；边界：不把普通终点路径用于其它合流语义。
	merge bool
	// shorterThan 职责：隔离只搜索更短候选的缓存；使用：相邻消费端口比较；边界：零表示没有已有路径上界。
	shorterThan int
	corridor    bool
	// maxCost 职责：隔离已有候选代价上界；使用：相邻端口安全剪枝；边界：零表示不限制，同价路径必须保留。
	maxCost float64
}

// transportEnvironment 职责：维护静态/动态分层空间哈希和路径缓存；使用：逐 lane 增量提交；边界：失败候选不污染其它排布候选。
type transportEnvironment struct {
	blocked          map[vector]bool
	reserved         map[vector]map[*placed]bool
	cache            map[transportPathCacheKey][]vector
	version          int
	minimum, maximum vector
	// searchCalls/cacheHits 职责：累计确定性寻路次数和当前占位版本缓存命中；使用：routing-budget 诊断；边界：不包含墙钟时间。
	searchCalls, cacheHits int
	// guidedCalls 职责：累计大型原生网格的有界代价搜索次数；使用：获选结果诊断；边界：不改变搜索展开计数或实际带流量。
	guidedCalls int
	// totalExpanded/maxExpanded 职责：累计及记录单次最高展开数；使用：检查固定搜索预算；边界：缓存命中为零展开，不因此放宽硬约束。
	totalExpanded, maxExpanded int
	// preferCorridor 职责：为直连带优先复用平面走廊；使用：运输A*；边界：不强制升高，不改变碰撞、端口、高度或预算。
	preferCorridor bool
	// beltCorridors 职责：记录本地与已提交运输带的平面投影；使用：新路线跨高度复用走廊；边界：仅是软代价，不授权连接不同物品或穿过已有实体。
	beltCorridors map[[2]int]bool
	// corridorPotential 职责：缓存当前路线的平面松弛势场；使用：相邻端口下界复用；边界：占位版本变化即失效。
	corridorPotential *transportCorridorPotential
	// densePorts 职责：核验连续低轨接驳与真实整段静态净空；使用：仅密铺路由；边界：普通模式保持为空。
	densePorts *denseRoutePortIndex
}

// routeTransport 职责：将全部规范 lane 写入 placed 指针图；使用：mode 1；边界：所有内部边都成功后才进入编码，不接受部分成功。
func (b *builder) routeTransport(ctx context.Context) error {
	if b.directBoundaryUnits != nil {
		return b.routeTransportWithInputs(ctx, nil)
	}
	minimum, maximum := bounds(b.all)
	positions := transportInputPositions(b.transportRoutes, minimum, maximum, b.busFabric == nil && !b.connectedLegacyInputs)
	if b.denseSpatial && !b.connectedLegacyInputs {
		positions = b.denseInputPositions(minimum, maximum)
	}
	var err error
	positions, err = b.fitDenseInputHeight(ctx, positions, minimum, maximum)
	if err != nil {
		return err
	}
	return b.routeTransportWithInputs(ctx, positions)
}

// routeTransportWithInputs 职责：从给定西侧边界端口完成所有真实物料路线；使用：就近入口、环路基线和同输入回归比较；边界：端口数必须匹配全部外供 lane，之后仍执行同一占位、容量和最终规则。
func (b *builder) routeTransportWithInputs(ctx context.Context, inputPositions []vector) error {
	return b.routeTransportWithStrategy(ctx, inputPositions, b.busFabric == nil)
}

// routeTransportWithStrategy 职责：以相同实体端点选择走廊软代价；使用：公开默认策略与同布局回归比较；边界：仅改变寻路目标，不改变物料图、硬约束或预算，不新增用户配置。
func (b *builder) routeTransportWithStrategy(ctx context.Context, inputPositions []vector, preferCorridor bool) error {
	routes := append([]*transportRoute{}, b.denseProducerSplitRoutes()...)
	routes = append(routes, b.transportRoutes...)
	routes = append(routes, b.denseStreetRoutes...)
	routes = append(routes, b.denseMaterialNetworkRoutes()...)
	for _, recycle := range b.machineRecycles {
		routes = append(routes, recycle.route)
	}
	external := 0
	b.owner = -1
	for _, route := range routes {
		if route.from != nil {
			continue
		}
		if b.directBoundaryUnits != nil {
			if route.start == nil || route.end == nil || route.boundary == nil || len(route.path) == 0 {
				return transportAllocationError(route.edge, route.rate, "黑盒外供路线缺少预先分配的真实设施或带头")
			}
			continue
		}
		if external >= len(inputPositions) {
			return transportAllocationError(route.edge, route.rate, "外供入口位置数量不足")
		}
		position := inputPositions[external]
		p := b.make(2000+b.setting.BeltLv, position.x, position.y, position.z)
		p.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		route.start, route.boundary = p, p
		route.path = []*placed{p}
		if route.end == nil {
			// 外供物品直接作为目标时仍生成一段真实带，两端分别表示供给和交付边界。
			route.end = b.make(2000+b.setting.BeltLv, p.a.x+2, p.a.y, 0)
			route.end.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		}
		b.appendDenseInputStem(route)
		external++
	}
	if external != len(inputPositions) {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "外供入口位置数量与真实 lane 不符"}
	}
	if err := b.prepareLeftoverBoundaries(ctx, inputPositions); err != nil {
		return err
	}
	b.previewCheckpoint("routing")
	minimum, maximum := bounds(b.all)
	var buildingBlocked map[vector]bool
	var err error
	if b.polarFrame != nil {
		buildingBlocked = map[vector]bool{}
	} else if b.denseSpatial {
		buildingBlocked, err = denseTransportObstacles(ctx, b.all)
	} else {
		buildingBlocked, err = stationBuildingObstacles(ctx, b.all)
	}
	if err != nil {
		return err
	}
	env := &transportEnvironment{blocked: buildingBlocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(minimum.x) - 8, math.Floor(minimum.y) - 8, 0}, maximum: vector{math.Ceil(maximum.x) + 8, math.Ceil(maximum.y) + 8, transportMaxHeight}, preferCorridor: preferCorridor, beltCorridors: map[[2]int]bool{}}
	if b.polarFrame != nil {
		env.minimum.x = math.Max(env.minimum.x, -8)
		env.maximum.x = math.Min(env.maximum.x, b.polarFrame.longitudeLimit()+8)
	}
	if b.denseSpatial && b.denseLatitudeSpan > 0 {
		env.minimum.y = math.Max(env.minimum.y, 0)
		env.maximum.y = math.Min(env.maximum.y, b.denseLatitudeSpan-1)
	}
	if b.denseSpatial {
		env.densePorts, err = b.newDenseRoutePortIndex(ctx)
		if err != nil {
			return err
		}
		if b.denseBoundary != nil {
			env.minimum.x = max(env.minimum.x, b.denseBoundary.minimumX)
			env.densePorts.boundary = b.denseBoundary
			for _, entity := range b.all {
				if surfaceIsBelt(entity.record.ItemID) {
					env.densePorts.addBeltSpan(entity)
				}
			}
		}
		env.densePorts.integerEdges = newDenseIntegerEdgeCache(env.minimum, env.maximum)
	}
	env.reserveDenseBoundaryEscapes(routes)
	for _, p := range b.all {
		if surfaceIsBelt(p.record.ItemID) {
			if b.denseSpatial {
				env.densePorts.blockNode(env.blocked, p.a)
			} else {
				env.blocked[beltCell(p.a)] = true
			}
			env.beltCorridors[beltColumn(p.a)] = true
		}
		if !b.denseSpatial && p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			// 分拣器胶囊通道也进入离散占位，最终仍由版本化 slotPose 精确预检决定。
			for x := math.Floor(math.Min(p.a.x, p.b.x)); x <= math.Ceil(math.Max(p.a.x, p.b.x)); x++ {
				for y := math.Floor(math.Min(p.a.y, p.b.y)); y <= math.Ceil(math.Max(p.a.y, p.b.y)); y++ {
					env.blocked[vector{x, y, math.Round(p.a.z)}] = true
				}
			}
		}
	}
	// 为所有未来入口预留落地柱，防止先路由的高架封死尚未接线的端口。
	for _, route := range routes {
		if b.denseSpatial {
			// 低轨可共享XY而位于不同真实高度；整数整列预留会把彼此的合法接驳全部封死。
			// 密铺由portal逐节点/逐段检查并在提交后登记真实占位，普通模式保留原列预留。
			continue
		}
		if route.to == nil && route.from != nil && route.end == nil {
			continue
		}
		for i, endpoint := range []*placed{route.start, route.end} {
			if endpoint == nil {
				return transportAllocationError(route.edge, route.rate, "路线端点为空")
			}
			direction := -1.
			if i == 0 && route.from == nil {
				direction = 1
			}
			if i == 1 && route.to == nil && (route.edge.FlowKind == "leftover" || b.mode1DeliveryGroup(route) != nil) {
				direction = 1
			}
			offset := vector{x: direction}
			if i == 0 && route.from == nil {
				offset = denseExternalStep(route)
			}
			if i == 0 && route.from != nil {
				offset = denseRotateVector(offset, route.from.quarterTurns)
			}
			if i == 1 && route.to != nil {
				offset = denseRotateVector(offset, route.to.quarterTurns)
			}
			if env.densePorts != nil {
				offset = env.densePorts.outward(endpoint, offset, i == 0)
			}
			for z := 0.; z <= transportMaxHeight; z++ {
				cell := vector{math.Round(endpoint.a.x + offset.x), math.Round(endpoint.a.y + offset.y), z}
				if env.reserved[cell] == nil {
					env.reserved[cell] = map[*placed]bool{}
				}
				env.reserved[cell][endpoint] = true
				if b.busFabric != nil && i == 1 {
					// 环路模式的并联汇入格位于连续干线中，落地需要正上方的独立柱，不能让较早路线封住未来消费者。
					landing := vector{math.Round(endpoint.a.x), math.Round(endpoint.a.y), z}
					if env.reserved[landing] == nil {
						env.reserved[landing] = map[*placed]bool{}
					}
					env.reserved[landing][endpoint] = true
				}
			}
		}
	}
	incoming := map[*placed][]*placed{}
	incomingCounts := map[*placed]int{}
	for _, p := range b.all {
		if p.output != nil {
			incomingCounts[p.output]++
		}
	}
	// 外供端口及球面位置已冻结；之后每条路线只追加带节点并更新源端 output。
	b.previewCheckpoint("routing")
	b.orderDenseBoundaryRoutes(routes)
	for index, route := range routes {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全图传送带寻路已取消或超时"}
		}
		reportProgress(ctx, "routing", "正在自动连接内部物料与外部入口", index+1, len(routes))
		if route.to == nil && route.from != nil && route.end == nil {
			continue
		}
		source := route.transportSource()
		start := source.a
		if route.from == nil {
			offset := denseExternalStep(route)
			start.x += offset.x
			start.y += offset.y
		} else {
			offset := denseRotateVector(vector{x: -1}, route.from.quarterTurns)
			start.x += offset.x
			start.y += offset.y
		}
		end := vector{route.end.a.x - 1, route.end.a.y, route.end.a.z}
		if route.to == nil && (route.edge.FlowKind == "leftover" || b.mode1DeliveryGroup(route) != nil) {
			// 独立交付从原料区向西出货，最后一段由东侧进入，避免把交付口变成朝机器的反向出口。
			end.x = route.end.a.x + 1
		}
		if route.to != nil {
			// 消费端 stub 由机器行外侧向西延伸；从 stub 首格进入可避免把目标分拣器本体当作障碍。
			end = vector{route.end.a.x, route.end.a.y, route.end.a.z}
		}
		tail := incoming[route.end]
		var merge *placed
		destination := end
		points, steps := []vector(nil), 0
		maxSearchExpanded, searches := 0, 0
		if b.denseSpatial {
			var tailIndex int
			preferred := vector{start.x - source.a.x, start.y - source.a.y, 0}
			points, tailIndex, steps, maxSearchExpanded, searches = env.findDenseTransportPath(ctx, route, tail, incomingCounts, preferred)
			if tailIndex >= 0 {
				merge, tail = tail[tailIndex], tail[tailIndex:]
				end, destination = merge.a, merge.a
			}
		} else if len(tail) > 0 {
			// 从全部合法合流格反向搜索，不能凭一个空邻格就认定首个锚点可达。
			// 多源搜索共用一次预算，封闭锚点不会令整张工厂徒劳扫描。
			var tailIndex int
			points, tailIndex, steps = env.findTransportMergePath(ctx, start, tail, route.start, route.end, incomingCounts)
			maxSearchExpanded, searches = steps, 1
			if tailIndex >= 0 {
				merge, tail = tail[tailIndex], tail[tailIndex:]
				end, destination = merge.a, merge.a
			}
		} else if route.to != nil && env.preferCorridor {
			candidates := []vector{{end.x - 1, end.y, end.z}, {end.x, end.y + 1, end.z}, {end.x, end.y - 1, end.z}, {end.x + 1, end.y, end.z}, {end.x, end.y, end.z + 1}}
			points, destination, steps = env.findTransportDestinationPath(ctx, start, candidates, route.start, route.end)
			maxSearchExpanded, searches = steps, 1
		} else if route.to != nil {
			// 比较真实合法邻格；走廊策略按投影空间代价排序，兼容策略保留节点数剪枝。
			candidates := []vector{{end.x - 1, end.y, end.z}, {end.x, end.y + 1, end.z}, {end.x, end.y - 1, end.z}, {end.x + 1, end.y, end.z}, {end.x, end.y, end.z + 1}}
			bestLength := 0
			bestCost := math.Inf(1)
			for _, candidate := range candidates {
				if env.blocked[beltCell(candidate)] || env.reservedByOther(candidate, route.start, route.end) {
					continue
				}
				lengthLimit := bestLength
				if env.preferCorridor {
					// 节点更少但外扩的路线未必更好，不能以长度剪掉分层复用候选。
					lengthLimit = 0
				}
				costLimit := 0.
				if env.preferCorridor && bestLength > 0 {
					costLimit = bestCost
				}
				path, expanded := env.findTransportPathWithinBounds(ctx, start, candidate, route.start, route.end, false, lengthLimit, costLimit)
				steps, searches = steps+expanded, searches+1
				maxSearchExpanded = max(maxSearchExpanded, expanded)
				cost := env.transportPathCost(path)
				better := bestLength == 0 || len(path) < bestLength
				if env.preferCorridor && bestLength > 0 {
					better = cost < bestCost || cost == bestCost && len(path) < bestLength
				}
				if len(path) > 0 && better {
					destination, points, bestLength = candidate, path, len(path)
					bestCost = cost
				}
				if ctx.Err() != nil {
					return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全图传送带寻路已取消或超时"}
				}
			}
		} else {
			points, steps = env.findTransportPath(ctx, start, destination, route.start, route.end, merge != nil)
			maxSearchExpanded, searches = steps, 1
		}
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全图传送带寻路已取消或超时"}
		}
		if len(points) == 0 {
			reason := "已穷尽当前候选空间仍无可行路线"
			if maxSearchExpanded >= transportSearchLimit {
				reason = "在固定展开预算内未找到可行路线（预算耗尽不代表几何无解）"
			}
			freeNeighbors := func(v vector) int {
				count := 0
				for _, d := range []vector{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
					n := vector{v.x + d.x, v.y + d.y, v.z + d.z}
					if n.x >= env.minimum.x && n.y >= env.minimum.y && n.x <= env.maximum.x && n.y <= env.maximum.y && n.z >= 0 && n.z <= transportMaxHeight && (!env.blocked[beltCell(n)] || n == end) {
						count++
					}
				}
				return count
			}
			var targetCells []string
			for _, d := range []vector{{-1, 0, 0}, {1, 0, 0}, {0, -1, 0}, {0, 1, 0}, {0, 0, 1}} {
				v := vector{end.x + d.x, end.y + d.y, end.z + d.z}
				targetCells = append(targetCells, fmt.Sprintf("(%.0f,%.0f,%.0f):blocked=%t,reserved=%t", v.x, v.y, v.z, env.blocked[beltCell(v)], env.reservedByOther(v, route.start, route.end)))
			}
			return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("物品 %d、边 %s、%.3f/min %s；端口 (%.0f,%.0f,%.0f)→(%.0f,%.0f,%.0f)，单次最高展开 %d/%d、本路 %d 次共 %d、全图累计 %d、高度上限 %d，端点占位 %t/%t，可达邻格 %d/%d，边界 %.0f..%.0f/%.0f..%.0f，目标邻格 %v", route.edge.ItemID, route.edge.ID, route.rate, reason, route.start.a.x, route.start.a.y, route.start.a.z, route.end.a.x, route.end.a.y, route.end.a.z, maxSearchExpanded, transportSearchLimit, searches, steps, env.totalExpanded, transportMaxHeight, env.blocked[beltCell(start)], env.blocked[beltCell(end)], freeNeighbors(start), freeNeighbors(end), env.minimum.x, env.maximum.x, env.minimum.y, env.maximum.y, targetCells)}
		}
		if b.denseSpatial && b.adapter != nil && b.adapter.density == densityDense {
			target := route.end
			if merge != nil {
				target = merge
			}
			var removed int
			points, removed = env.compactDenseLongRoute(points, source, target, denseLongBeltLongitudeFactor(ctx))
			if removed > 0 && b.logistics != nil {
				b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-long-belt-segments", Status: "passed", Message: fmt.Sprintf("物品%d自由直带采用最长1.5格节点间距，减少%d个带节点；保留端部接驳及转角，实际带速和容量不变，最终球面规则继续核验", route.edge.ItemID, removed)})
			}
		}
		if len(b.all)+len(points) > generationBuildingBudget(ctx) {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("完整物料路线会使建筑超过本次预算 %d，请拆分目标", generationBuildingBudget(ctx))}
		}
		b.owner = -1
		if merge != nil || b.denseSpatial {
			points = points[:len(points)-1]
		}
		previewBase := len(b.all)
		path := b.byPath(points)
		for _, point := range path {
			incomingCounts[point]++
		}
		previous := source
		if len(path) > 0 {
			connect(previous, path[0])
			previous = path[len(path)-1]
		}
		if merge != nil {
			connect(previous, merge)
			incomingCounts[merge]++
			path = append(path, tail...)
		} else {
			connect(previous, route.end)
			incomingCounts[route.end]++
			path = append(path, route.end)
		}
		route.path = append(route.path, path...)
		// 合流锚点随新入带向上游推进，避免所有供应者把同一格的四个邻位耗尽。
		// 保存完整下游尾链，诊断检查的仍是此物料实际抵达消费者的连续路线。
		incoming[route.end] = append([]*placed{}, path...)
		if b.denseSpatial && denseIntegerNode(source.a) {
			// 整数源本身也是合法有向尾链的一部分；保留它供短接路线的下一供应者合流。
			incoming[route.end] = append([]*placed{source}, incoming[route.end]...)
		}
		for _, point := range points {
			if b.denseSpatial {
				env.densePorts.blockNode(env.blocked, point)
			} else {
				env.blocked[beltCell(point)] = true
			}
			env.beltCorridors[beltColumn(point)] = true
		}
		if env.densePorts != nil {
			env.densePorts.addNode(source)
			for _, point := range path {
				env.densePorts.addNode(point)
			}
		}
		env.version++
		b.previewRouteAppend(previewBase, source)
	}
	inserterStatus := "passed"
	inserterMessage := "机器单臂及逐物品过滤分支按实际跨度和已验证保守吞吐检查，不把带容量当作分拣器容量"
	for _, check := range b.logistics.Checks {
		if check.RuleID == "inserter-capacity" && check.Status == "unverified" {
			inserterStatus = "unverified"
			inserterMessage = check.Message
			break
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "belt-lanes", Status: "passed", Message: "每条规范边已分配到具体端口与不超所选等级容量的实体 lane"}, LogisticsCheck{RuleID: "mode1-internal-connectivity", Status: "passed", Message: "全部内部物料已自动接线；仅标记的原料、目标和剩余物品端口需要玩家连接外部设施"}, LogisticsCheck{RuleID: "inserter-capacity", Status: inserterStatus, Message: inserterMessage})
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "routing-budget", Status: "passed", Message: fmt.Sprintf("确定性路线预算：searchCalls=%d，cacheHits=%d，totalExpanded=%d，maxExpanded=%d，perSearchLimit=%d，maxHeight=%d，occupancyVersions=%d；缓存按端口身份、合流权限和占位版本隔离，带等级和高度上限固定于本次环境", env.searchCalls, env.cacheHits, env.totalExpanded, env.maxExpanded, transportSearchLimit, transportMaxHeight, env.version)})
	if env.preferCorridor {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "stacked-corridor-routing", Status: "passed", Message: "独立传送带优先复用既有带的平面走廊，以不同高度区分并在端点真实逐格升降；走廊水平代价1、外扩1.25、升降1.5，不强制空场高架；平面下界预计算最多300000格，碰撞、端口权限、高度32和单次三维300000预算不变"})
	}
	if env.guidedCalls > 0 {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "bounded-native-routing", Status: "passed", Message: fmt.Sprintf("大型原生网格执行%d次有界代价搜索；单次整数主路及目标后缀保留2倍代价界，允许重开节点，碰撞、容量、高度及展开预算不变；不宣称整厂全局最优", env.guidedCalls)})
	}
	if err := b.validateDenseBoundaryRoutes(ctx); err != nil {
		return err
	}
	if b.denseBoundary != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-boundary-separation", Status: "passed", Message: "外部接口位于生产实体的实际高度内，最多三层；入口区禁止独立带段相交，固定短带向内接入，交付口保留独立地面槽"})
	}
	return b.commitMode1Deliveries()
}

// reservedByOther 职责：在路径终点筛选前排除未来端口的垂直净空；使用：同物品 lane 寻路；边界：仅放行当前 route 自己拥有的端点柱。
func (env *transportEnvironment) reservedByOther(v vector, source, target *placed) bool {
	for owner := range env.reserved[beltCell(v)] {
		if owner != source && owner != target {
			return true
		}
	}
	return false
}

// findTransportPath 职责：在分层空间哈希中执行固定邻居序的三维 A*；使用：每条 lane；边界：最多指定展开次数，取消后退出，占位版本不同则缓存失效。
func (env *transportEnvironment) findTransportPath(ctx context.Context, start, end vector, source, target *placed, merge bool) ([]vector, int) {
	return env.findTransportPathWithinLength(ctx, start, end, source, target, merge, 0)
}

// findTransportPathWithinLength 职责：在原加权 A* 内安全剪除不可能缩短已有带链的前缀；使用：消费端有限邻格比较；边界：节点数上界和端口权限进入缓存键，不增加任何搜索预算。
func (env *transportEnvironment) findTransportPathWithinLength(ctx context.Context, start, end vector, source, target *placed, merge bool, shorterThan int) ([]vector, int) {
	return env.findTransportPathWithinBounds(ctx, start, end, source, target, merge, shorterThan, 0)
}

// findTransportPathWithinBounds 职责：以真实长度或一致代价下界安全排除不可能胜过已有候选的路径；使用：有限端口比较；边界：代价只剪严格较大者，保留同价短路径，预算和确定性顺序不变。
func (env *transportEnvironment) findTransportPathWithinBounds(ctx context.Context, start, end vector, source, target *placed, merge bool, shorterThan int, maxCost float64) ([]vector, int) {
	env.searchCalls++
	steps := 0
	cacheHit := false
	defer func() {
		env.totalExpanded += steps
		env.maxExpanded = max(env.maxExpanded, steps)
		recordPerformanceSearch(ctx, steps, cacheHit)
	}()
	if ctx.Err() != nil {
		return nil, 0
	}
	key := transportPathCacheKey{start: start, end: end, version: env.version, source: source, target: target, merge: merge, shorterThan: shorterThan, corridor: env.preferCorridor, maxCost: maxCost}
	if path, exists := env.cache[key]; exists {
		env.cacheHits++
		cacheHit = true
		return append([]vector{}, path...), 0
	}
	allowed := func(v vector) bool {
		if v.x < env.minimum.x || v.y < env.minimum.y || v.x > env.maximum.x || v.y > env.maximum.y || v.z < 0 || v.z > transportMaxHeight || env.blocked[beltCell(v)] && v != start && v != end {
			return false
		}
		for owner := range env.reserved[beltCell(v)] {
			if owner != source && owner != target {
				return false
			}
		}
		return true
	}
	if !allowed(start) || !allowed(end) {
		return nil, 0
	}
	canImprove := func(v vector, length int) bool {
		return shorterThan <= 0 || float64(length)+math.Abs(v.x-end.x)+math.Abs(v.y-end.y)+math.Abs(v.z-end.z) < float64(shorterThan)
	}
	if !canImprove(start, 1) {
		return nil, 0
	}
	env.prepareTransportHeuristic(ctx, end)
	heuristic := func(v vector) float64 { return env.transportHeuristic(v, end) }
	if maxCost > 0 && heuristic(start) > maxCost {
		return nil, 0
	}
	first := &transportPathNode{point: start, score: heuristic(start), length: 1}
	queue := transportPathQueue{first}
	seen := map[vector]*transportPathNode{start: first}
	seenFrontier := map[vector][]*transportPathNode{}
	if shorterThan > 0 {
		seenFrontier[start] = []*transportPathNode{first}
	}
	serial := 0
	for len(queue) > 0 && steps < transportSearchLimit {
		if ctx.Err() != nil {
			return nil, steps
		}
		current := heap.Pop(&queue).(*transportPathNode)
		if current.discarded {
			continue
		}
		current.closed = true
		steps++
		if current.point == end {
			path := []vector{}
			for node := current; node != nil; node = node.parent {
				path = append(path, node.point)
			}
			for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
				path[i], path[j] = path[j], path[i]
			}
			env.cache[key] = append([]vector{}, path...)
			return path, steps
		}
		for _, delta := range []vector{{-1, 0, 0}, {0, 1, 0}, {1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
			v := vector{current.point.x + delta.x, current.point.y + delta.y, current.point.z + delta.z}
			length := current.length + 1
			if !canImprove(v, length) || !allowed(v) || !env.denseTransportStepClear(current.point, v) {
				continue
			}
			distance := current.distance + env.transportStepCost(current.point, v)
			if maxCost > 0 && distance+heuristic(v) > maxCost {
				continue
			}
			old := seen[v]
			if shorterThan > 0 {
				// 长度受限搜索是带资源约束的路径问题：同一坐标不能只保留
				// 加权代价最低的前缀，否则较贵但更短的前缀可能被误剪。
				// 这里用 (distance, length) 的 Pareto 前沿，废弃节点仍留在堆中，
				// 由 discarded 标记惰性删除，避免破坏 heap 索引。
				frontier := seenFrontier[v]
				dominated := false
				for _, candidate := range frontier {
					if candidate.discarded {
						continue
					}
					if candidate.distance <= distance && candidate.length <= length {
						dominated = true
						break
					}
				}
				if dominated {
					continue
				}
				for _, candidate := range frontier {
					if candidate.discarded {
						continue
					}
					if distance <= candidate.distance && length <= candidate.length && (distance < candidate.distance || length < candidate.length) {
						candidate.discarded = true
					}
				}
				node := &transportPathNode{point: v, parent: current, distance: distance, score: distance + heuristic(v), order: serial, length: length}
				serial++
				seenFrontier[v] = append(frontier, node)
				heap.Push(&queue, node)
				continue
			}
			if old != nil && (old.closed || distance >= old.distance) {
				continue
			}
			if old != nil {
				old.distance, old.score, old.parent = distance, distance+heuristic(v), current
				old.length = length
				heap.Fix(&queue, old.index)
				continue
			}
			serial++
			node := &transportPathNode{point: v, parent: current, distance: distance, score: distance + heuristic(v), order: serial, length: length}
			seen[v] = node
			heap.Push(&queue, node)
		}
	}
	return nil, steps
}
