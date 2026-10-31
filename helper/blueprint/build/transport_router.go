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

// transportPathNode 职责：保存三维 A* 候选；使用：稳定最小堆；边界：已关闭节点不会重新展开。
type transportPathNode struct {
	point        vector
	parent       *transportPathNode
	distance     float64
	score        float64
	order, index int
	closed       bool
}

// transportPathQueue 职责：稳定选择估价最低的候选；使用：三维寻路；边界：同分按插入顺序而非 map 顺序。
type transportPathQueue []*transportPathNode

// Len 职责：提供堆长度；使用：container/heap；边界：不统计关闭节点。
func (q transportPathQueue) Len() int { return len(q) }

// Less 职责：按估价和稳定键排序；使用：container/heap；边界：不依赖运行时哈希顺序。
func (q transportPathQueue) Less(i, j int) bool {
	if q[i].score != q[j].score {
		return q[i].score < q[j].score
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
}

// transportEnvironment 职责：维护静态/动态分层空间哈希和路径缓存；使用：逐 lane 增量提交；边界：失败候选不污染其它排布候选。
type transportEnvironment struct {
	blocked          map[vector]bool
	reserved         map[vector]map[*placed]bool
	cache            map[transportPathCacheKey][]vector
	version          int
	minimum, maximum vector
}

// routeTransport 职责：将全部规范 lane 写入 placed 指针图；使用：mode 1；边界：所有内部边都成功后才进入编码，不接受部分成功。
func (b *builder) routeTransport(ctx context.Context) error {
	minimum, _ := bounds(b.all)
	external := 0
	b.owner = -1
	for _, route := range b.transportRoutes {
		if route.from != nil {
			continue
		}
		p := b.make(2000+b.setting.BeltLv, math.Floor(minimum.x)-5, math.Floor(minimum.y)+float64(external*2), 0)
		p.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		route.start, route.boundary = p, p
		route.path = []*placed{p}
		if route.end == nil {
			// 外供物品直接作为目标时仍生成一段真实带，两端分别表示供给和交付边界。
			route.end = b.make(2000+b.setting.BeltLv, p.a.x+2, p.a.y, 0)
			route.end.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		}
		external++
	}
	minimum, maximum := bounds(b.all)
	buildingBlocked, err := stationBuildingObstacles(ctx, b.all)
	if err != nil {
		return err
	}
	env := &transportEnvironment{blocked: buildingBlocked, reserved: map[vector]map[*placed]bool{}, cache: map[transportPathCacheKey][]vector{}, minimum: vector{math.Floor(minimum.x) - 8, math.Floor(minimum.y) - 8, 0}, maximum: vector{math.Ceil(maximum.x) + 8, math.Ceil(maximum.y) + 8, transportMaxHeight}}
	for _, p := range b.all {
		if surfaceIsBelt(p.record.ItemID) {
			env.blocked[beltCell(p.a)] = true
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			// 分拣器胶囊通道也进入离散占位，最终仍由版本化 slotPose 精确预检决定。
			for x := math.Floor(math.Min(p.a.x, p.b.x)); x <= math.Ceil(math.Max(p.a.x, p.b.x)); x++ {
				for y := math.Floor(math.Min(p.a.y, p.b.y)); y <= math.Ceil(math.Max(p.a.y, p.b.y)); y++ {
					env.blocked[vector{x, y, math.Round(p.a.z)}] = true
				}
			}
		}
	}
	// 为所有未来入口预留落地柱，防止先路由的高架封死尚未接线的端口。
	for _, route := range b.transportRoutes {
		if route.to == nil && route.from != nil {
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
			for z := 0.; z <= transportMaxHeight; z++ {
				cell := vector{math.Round(endpoint.a.x) + direction, math.Round(endpoint.a.y), z}
				if env.reserved[cell] == nil {
					env.reserved[cell] = map[*placed]bool{}
				}
				env.reserved[cell][endpoint] = true
			}
		}
	}
	incoming := map[*placed][]*placed{}
	for index, route := range b.transportRoutes {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全图传送带寻路已取消或超时"}
		}
		reportProgress(ctx, "routing", "正在自动连接内部物料与外部入口", index+1, len(b.transportRoutes))
		if route.to == nil && route.from != nil {
			continue
		}
		start := route.start.a
		if route.from == nil {
			start.x++
		} else {
			start.x--
		}
		end := vector{route.end.a.x - 1, route.end.a.y, route.end.a.z}
		if route.to != nil {
			// 消费端 stub 由机器行外侧向西延伸；从 stub 首格进入可避免把目标分拣器本体当作障碍。
			end = vector{route.end.a.x, route.end.a.y, route.end.a.z}
		}
		tail := incoming[route.end]
		var merge *placed
		if len(tail) > 0 {
			// 上一条路线的首段可能仍处于其生产端预留柱中，不能让后续供应者借用该落地柱。
			for i, candidate := range tail {
				if env.reservedByOther(candidate.a, route.start, route.end) {
					continue
				}
				free := false
				for _, d := range []vector{{-1, 0, 0}, {0, 1, 0}, {1, 0, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}} {
					neighbor := vector{candidate.a.x + d.x, candidate.a.y + d.y, candidate.a.z + d.z}
					if neighbor.z >= 0 && !env.blocked[beltCell(neighbor)] && !env.reservedByOther(neighbor, route.start, route.end) {
						free = true
						break
					}
				}
				if free {
					merge, tail, end = candidate, tail[i:], candidate.a
					break
				}
			}
		}
		destination := end
		points, steps := []vector(nil), 0
		if route.to != nil && merge == nil {
			// 目标本身已有带，不可将寻路末点与既有带叠放；逐个尝试真实可接的邻格并选最短可行路径。
			candidates := []vector{{end.x - 1, end.y, end.z}, {end.x, end.y + 1, end.z}, {end.x, end.y - 1, end.z}, {end.x + 1, end.y, end.z}, {end.x, end.y, end.z + 1}}
			bestLength := math.Inf(1)
			for _, candidate := range candidates {
				if env.blocked[beltCell(candidate)] || env.reservedByOther(candidate, route.start, route.end) {
					continue
				}
				path, expanded := env.findTransportPath(ctx, start, candidate, route.start, route.end, false)
				steps += expanded
				if len(path) > 0 && float64(len(path)) < bestLength {
					destination, points, bestLength = candidate, path, float64(len(path))
				}
				if ctx.Err() != nil {
					return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "全图传送带寻路已取消或超时"}
				}
			}
		} else {
			points, steps = env.findTransportPath(ctx, start, destination, route.start, route.end, merge != nil)
		}
		if len(points) == 0 {
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
			return &blueprint.Error{Code: "GEN_E_ROUTE", Message: fmt.Sprintf("物品 %d、边 %s、%.3f/min 无可行路线；端口 (%.0f,%.0f,%.0f)→(%.0f,%.0f,%.0f)，展开 %d/%d、高度上限 %d，端点占位 %t/%t，可达邻格 %d/%d，边界 %.0f..%.0f/%.0f..%.0f，目标邻格 %v", route.edge.ItemID, route.edge.ID, route.rate, route.start.a.x, route.start.a.y, route.start.a.z, route.end.a.x, route.end.a.y, route.end.a.z, steps, transportSearchLimit, transportMaxHeight, env.blocked[beltCell(start)], env.blocked[beltCell(end)], freeNeighbors(start), freeNeighbors(end), env.minimum.x, env.maximum.x, env.minimum.y, env.maximum.y, targetCells)}
		}
		if len(b.all)+len(points) > blueprint.MaxBuildingCount {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "完整物料路线会使建筑超过 20 万，请拆分目标"}
		}
		b.owner = -1
		if merge != nil {
			points = points[:len(points)-1]
		}
		path := b.byPath(points)
		previous := route.start
		if len(path) > 0 {
			connect(previous, path[0])
			previous = path[len(path)-1]
		}
		if merge != nil {
			connect(previous, merge)
			path = append(path, tail...)
		} else {
			connect(previous, route.end)
			path = append(path, route.end)
		}
		route.path = append(route.path, path...)
		// 合流锚点随新入带向上游推进，避免所有供应者把同一格的四个邻位耗尽。
		// 保存完整下游尾链，诊断检查的仍是此物料实际抵达消费者的连续路线。
		incoming[route.end] = append([]*placed{}, path...)
		for _, point := range points {
			env.blocked[beltCell(point)] = true
		}
		env.version++
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
	return nil
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
	key := transportPathCacheKey{start: start, end: end, version: env.version}
	if path, exists := env.cache[key]; exists {
		return append([]vector{}, path...), 0
	}
	allowed := func(v vector) bool {
		if v.x < env.minimum.x || v.y < env.minimum.y || v.x > env.maximum.x || v.y > env.maximum.y || v.z < 0 || v.z > transportMaxHeight || env.blocked[beltCell(v)] && v != start && v != end && !(merge && v == end) {
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
	heuristic := func(v vector) float64 { return math.Abs(v.x-end.x) + math.Abs(v.y-end.y) + math.Abs(v.z-end.z)*1.5 }
	first := &transportPathNode{point: start, score: heuristic(start)}
	queue := transportPathQueue{first}
	seen := map[vector]*transportPathNode{start: first}
	serial := 0
	steps := 0
	for len(queue) > 0 && steps < transportSearchLimit {
		if ctx.Err() != nil {
			return nil, steps
		}
		current := heap.Pop(&queue).(*transportPathNode)
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
			if !allowed(v) {
				continue
			}
			cost := 1.
			if delta.z != 0 {
				cost = 1.5
			}
			distance := current.distance + cost
			old := seen[v]
			if old != nil && (old.closed || distance >= old.distance) {
				continue
			}
			if old != nil {
				old.distance, old.score, old.parent = distance, distance+heuristic(v), current
				heap.Fix(&queue, old.index)
				continue
			}
			serial++
			node := &transportPathNode{point: v, parent: current, distance: distance, score: distance + heuristic(v), order: serial}
			seen[v] = node
			heap.Push(&queue, node)
		}
	}
	return nil, steps
}
