package build

// 文件职责：装配单/双行星或星际物流塔和路由；使用：mode 3/4；边界：容量、端口或寻路不足时明确失败，不静默丢弃物品。
// 站点参数和微调数值核对 dsp-calc LocalStation/InterstellarStation，算法独立实现，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
	"context"
	"fmt"
	"math"
	"sort"
)

// stationItem 职责：保存站点物品角色；使用：参数装配；边界：不改变求解结果。
type stationItem struct {
	// id 职责：原型；使用：槽位；边界：不能为零。
	id int
	// provide 职责：供给角色；使用：输入回塔与输出供货区分；边界：物品列表不截断。
	provide bool
}

// stationRoute 职责：绑定产线头和塔槽；使用：先寻路后装配；边界：保存实际指针，不反推索引。
type stationRoute struct {
	// head 职责：产线首带；使用：连接；边界：归属不变。
	head *placed
	// item 职责：供给物品；使用：站点物品选择；边界：回塔为主产物。
	item int
	// output 职责：回塔标记；使用：决定带方向；边界：与站点供给角色不同层次。
	output bool
	// tower 职责：站点数组下标；使用：双塔；边界：0 基。
	tower int
	// slot 职责：塔端口；使用：连接；边界：0..11。
	slot int
	// path 职责：外部路由；使用：装配；边界：只有成功路径可写出。
	path []vector
	// rate 职责：记录本行分配的每分钟流量；使用：塔端口容量核验；边界：不以额定机器余量扩大可信需求。
	rate float64
}

// slotOffset 职责：取得参考端口路径；使用：物流塔的整数过渡点与内部微调；边界：原样保留经验小数。
func slotOffset(slot int) []vector {
	paths := [][]vector{{{1, 4, 0}, {1, 2, 0}}, {{0, 5, 0}, {0, 2, 0}}, {{-1, 6, 0}, {-1, 2, 0}}, {{-4, 7, 0}, {-4, 1, 0}, {-2, 1, 0}}, {{-5, 8, 0}, {-5, 0, 0}, {-2, 0, 0}}, {{-6, 9, 0}, {-6, -1, 0}, {-2, -1, 0}}, {{-1, -6, 0}, {-1, -2, 0}}, {{0, -5, 0}, {0, -2, 0}}, {{1, -4, 0}, {1, -2, 0}}, {{4, -1, 0}, {2, -1, 0}}, {{4, 0, 0}, {2, 0, 0}}, {{4, 1, 0}, {2, 1, 0}}}
	return append([]vector{}, paths[slot]...)
}

// stationInner 职责：生成塔内部对齐带点；使用：外部路由之后；边界：微调在 f64 层完成后统一转 f32。
func stationInner(center vector, slot int) []vector {
	main := slotOffset(slot)
	var points []vector
	for i, v := range main {
		v.x += center.x
		v.y += center.y
		main[i] = v
		if i == 0 {
			points = append(points, v)
		} else {
			segment := manhattan(main[i-1], v, true)
			points = append(points, segment[1:]...)
		}
	}
	points = points[1:]
	n := len(points)
	for _, index := range []int{n - 1, n - 2} {
		delta := .0207
		if index == n-1 {
			delta = .1464
		}
		switch {
		case slot <= 2:
			points[index].y += delta
		case slot <= 5:
			points[index].x -= delta
		case slot <= 8:
			points[index].y -= delta
		default:
			points[index].x += delta
		}
		switch slot {
		case 2, 6:
			points[index].x += .0015
		case 0, 8:
			points[index].x -= .0015
		case 3, 11:
			points[index].y -= .0015
		case 5, 9:
			points[index].y += .0015
		}
	}
	return points
}

// createStation 职责：写 2048 字参数和物品角色；使用：已完成寻路后；边界：调用者保证不超过物品槽。
func (b *builder) createStation(center vector, items []stationItem, mode int) *placed {
	id := 2103
	if mode == 4 {
		id = 2104
	}
	capacity, _ := transport.StationItemCapacity(id)
	p := b.make(id, center.x, center.y, 0)
	p.record.Yaw = 0
	p.record.Yaw2 = 0
	p.record.InputToSlot = 0
	p.record.Parameters = make([]int32, 2048)
	parameters := p.record.Parameters
	for i, item := range items {
		role := int32(2)
		if item.provide {
			role = 1
		}
		parameters[i*6] = int32(item.id)
		parameters[i*6+1] = role
		if mode == 4 {
			parameters[i*6+2] = role
		}
		parameters[i*6+3] = int32(capacity)
	}
	for i := 0; i < 12; i++ {
		parameters[192+i*4] = 1
		if i >= 9 {
			parameters[193+i*4] = int32(i - 8)
		}
	}
	parameters[320] = 200000
	if mode == 4 {
		parameters[320] = 1000000
		parameters[331] = 1
	}
	parameters[321] = -100000000
	parameters[322] = 240000000
	parameters[323] = 1
	parameters[324] = 480000
	parameters[325] = 1
	parameters[326] = 10
	parameters[327] = 100
	parameters[330] = 1
	return p
}

// attachStations 职责：为一组机器行创建单/双塔；使用：mode 3/4；边界：所有路径可达才写出站点，失败不留下半个布局。
func (b *builder) attachStations(ctx context.Context, lines []*line, mode int) error {
	if len(lines) == 0 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "没有可连接的产线"}
	}
	recipe := lines[0].recipe
	slots := 4
	if mode == 4 {
		slots = 5
	}
	items := []stationItem{}
	seen := map[int]bool{}
	for _, id := range recipe.Results {
		if !seen[id] {
			items = append(items, stationItem{id: id, provide: true})
			seen[id] = true
		}
	}
	inputIDs := append([]int(nil), recipe.Items...)
	if spray := productionSprayItem(lines[0].info); spray > 0 {
		inputIDs = append(inputIDs, spray)
	}
	for _, id := range inputIDs {
		if !seen[id] {
			items = append(items, stationItem{id: id})
			seen[id] = true
		}
	}
	if len(items) > slots*2 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物品种类超过双塔容量"}
	}
	groups := [][]stationItem{items}
	if len(items) > slots {
		groups = [][]stationItem{items[:slots], items[slots:]}
		b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 使用双塔，请确认所有原料供给和副产物消耗", recipe.ID))
	}
	itemTower := map[int]int{}
	for t, group := range groups {
		for _, item := range group {
			itemTower[item.id] = t
		}
	}
	routes := []stationRoute{}
	resultIDs := uniqueStationItems(recipe.Results)
	for _, l := range lines {
		share := 0.
		if l.info.MachineCount > 0 {
			share = float64(len(l.machines)) / float64(l.info.MachineCount)
		}
		for i, row := range l.inputs {
			routes = append(routes, stationRoute{head: row[0], item: recipe.Items[i], tower: itemTower[recipe.Items[i]], rate: productionItemRate(l.info.Inputs, recipe.Items[i]) * share})
		}
		if spray := productionSprayItem(l.info); spray > 0 {
			if l.coaterInput == nil {
				return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("产线 %d 缺少增产剂 %d 的真实供剂入口", l.machines[0].owner, spray)}
			}
			routes = append(routes, stationRoute{head: l.coaterInput, item: spray, tower: itemTower[spray], rate: productionItemRate(l.info.Inputs, spray) * share})
		}
		for _, item := range resultIDs {
			head, err := b.stationOutputHead(l, item)
			if err != nil {
				return err
			}
			routes = append(routes, stationRoute{head: head, item: item, output: true, tower: itemTower[item], rate: productionItemRate(l.info.Outputs, item) * share})
		}
	}
	used := make([]int, len(groups))
	priority := []int{6, 7, 8, 9, 10, 11, 0, 1, 2, 3, 4, 5}
	if len(routes) <= 6 {
		priority = priority[3:]
	}
	for i := range routes {
		t := routes[i].tower
		if used[t] >= len(priority) {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "站点传送带端口不足，需要拆分机器行"}
		}
		routes[i].slot = priority[used[t]]
		used[t]++
	}
	startY, endY := routes[0].head.a.y, routes[0].head.a.y
	for _, route := range routes {
		startY, endY = math.Min(startY, route.head.a.y), math.Max(endY, route.head.a.y)
	}
	centerY := math.Floor((endY-startY)/2) + startY
	centers := make([]vector, len(groups))
	var existing []*placed
	for _, l := range lines {
		existing = append(existing, l.all...)
	}
	buildingObstacles, err := stationBuildingObstacles(ctx, existing)
	if err != nil {
		return err
	}
	failure := ""
	ready := false
	for attempt := 0; attempt < 20; attempt++ {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过截止时间"}
		}
		for i := range centers {
			// 30 格是本地端口走廊的候选间隔，不冒充游戏距离常量；
			// 行星/星际塔分别按 15/29 米在最终规则中核验。
			// 塔先放到产线外侧，让普通地面寻路优先绕过机器。
			centers[i] = vector{-30 - float64(attempt)*2 - float64(i)*30, centerY, 0}
		}
		blocked := map[vector]bool{}
		for point := range buildingObstacles {
			if point.z == 0 {
				blocked[point] = true
			}
		}
		occupiedBelts := map[vector]bool{}
		// 外部物流路线不能穿过已经装配的产线带；混料 B 入口旁已有收集带，仅保留入口本身作为可出发点。
		for _, l := range lines {
			for _, p := range l.all {
				if p.a.z != 0 {
					continue
				}
				if p.record.ItemID > 2000 && p.record.ItemID < 2010 {
					blocked[p.a] = true
					occupiedBelts[beltCell(p.a)] = true
				}
				if p.record.RecipeID > 0 {
					lo, hi := footprint(p.record.ItemID)
					for x := math.Floor(p.a.x + lo.x); x <= math.Ceil(p.a.x+hi.x); x++ {
						for y := math.Floor(p.a.y + lo.y); y <= math.Ceil(p.a.y+hi.y); y++ {
							blocked[vector{x, y, 0}] = true
						}
					}
				}
			}
		}
		for _, route := range routes {
			blocked[route.head.a] = true
		}
		for _, center := range centers {
			// 物流塔路线必须绕过完整的静态建造盒，不能只屏蔽中心五格后从塔角擦行。
			for dx := -3.; dx <= 3; dx++ {
				for dy := -3.; dy <= 3; dy++ {
					blocked[vector{center.x + dx, center.y + dy, 0}] = true
				}
			}
			for _, slot := range priority {
				for _, point := range stationInner(center, slot) {
					blocked[point] = true
				}
				p := slotOffset(slot)[0]
				p.x += center.x
				p.y += center.y
				blocked[p] = true
			}
		}
		ready = true
		sort.SliceStable(routes, func(i, j int) bool { return routes[i].head.a.y < routes[j].head.a.y })
		usedSlots := make([]map[int]bool, len(groups))
		for i := range usedSlots {
			usedSlots[i] = map[int]bool{}
		}
		for i := range routes {
			route := &routes[i]
			begin := route.head.a
			candidates := []int{9, 10, 11, 8, 0, 7, 1, 6, 2, 5, 3, 4}
			center := centers[route.tower]
			sort.SliceStable(candidates, func(i, j int) bool {
				a, c := slotOffset(candidates[i])[0], slotOffset(candidates[j])[0]
				return math.Abs(begin.x-center.x-a.x)+math.Abs(begin.y-center.y-a.y) < math.Abs(begin.x-center.x-c.x)+math.Abs(begin.y-center.y-c.y)
			})
			var path []vector
			for _, slot := range candidates {
				if usedSlots[route.tower][slot] {
					continue
				}
				target := slotOffset(slot)[0]
				target.x += center.x
				target.y += center.y
				// 寻路器允许进入被屏蔽的终点，因此必须另核对端口及追加的塔内带，不能侵占其他产线入口。
				occupied := occupiedBelts[beltCell(target)]
				for _, point := range stationInner(center, slot) {
					occupied = occupied || occupiedBelts[beltCell(point)]
				}
				if occupied {
					continue
				}
				candidate := findPath(ctx, begin, target, math.Abs(begin.x-target.x) >= math.Abs(begin.y-target.y), blocked)
				if len(candidate) >= 2 {
					path = candidate
					route.slot = slot
					usedSlots[route.tower][slot] = true
					break
				}
			}
			if len(path) < 2 {
				failure = fmt.Sprintf("配方 %d 的物品 %d，入口 (%g,%g)，塔 %d", recipe.ID, route.item, begin.x, begin.y, route.tower)
				ready = false
				break
			}
			if stationPathCrosses(path, routes[:i]) {
				failure = fmt.Sprintf("配方 %d 的物品 %d 路线与已分配塔端路线交叉", recipe.ID, route.item)
				ready = false
				break
			}
			for _, v := range path {
				blocked[v] = true
				occupiedBelts[beltCell(v)] = true
			}
			for _, v := range stationInner(center, route.slot) {
				occupiedBelts[beltCell(v)] = true
			}
			path = path[1:]
			route.path = path
		}
		if ready {
			break
		}
	}
	if !ready {
		// 拥挤的混带入口允许显式跨层桥接，垂直爬升沿用参考 A/B 带的微移规则，避免平面路线互相封死。
		usedSlots := make([]int, len(groups))
		for i := range centers {
			centers[i] = vector{-45 - float64(i)*30, centerY, 0}
		}
		priority := []int{9, 10, 11, 8, 0, 7, 1, 6, 2, 5, 3, 4}
		for i := range routes {
			r := &routes[i]
			r.slot = priority[usedSlots[r.tower]]
			usedSlots[r.tower]++
		}
		// 本组仍在局部坐标中；已排好的其他组不在同一坐标系，不能当作障碍或叠加其高架层数。
		if err := b.bridgeStationRoutes(ctx, routes, centers, existing); err != nil {
			return err
		}
		b.warnings = append(b.warnings, "复杂混料入口使用跨层物流桥接，建议在游戏中检查高度与贴合："+failure)
	}
	towers := make([]*placed, len(groups))
	for i, group := range groups {
		towers[i] = b.createStation(centers[i], group, mode)
	}
	for _, route := range routes {
		points := append(append([]vector{}, route.path...), stationInner(centers[route.tower], route.slot)...)
		// 堆叠/精炼机的高架候选可能包含机器端点；byPath 不能在同一坐标重复创建一节带，否则最终交叉检查会看到未连接的重叠路线。
		if len(points) > 0 && points[0] == route.head.a {
			points = points[1:]
		}
		if len(points) < 2 {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: fmt.Sprintf("物流塔物品 %d 的路线去重后没有可用带段", route.item)}
		}
		belts := b.byPath(points)
		tower := towers[route.tower]
		last := belts[len(belts)-1]
		base := 192 + route.slot*4
		if route.output {
			connect(route.head, belts[0])
			last.output = tower
			last.record.OutputToSlot = int8(route.slot)
			last.record.Parameters = []int32{int32(route.item), 0}
			tower.record.Parameters[base] = 2
			tower.record.Parameters[base+1] = 0
			b.stationLinks = append(b.stationLinks, stationTransportLink{item: route.item, output: true, tower: tower, belt: last, slot: route.slot, head: route.head, path: append([]*placed{}, belts...), owner: route.head.owner, rate: route.rate})
		} else {
			reverse(belts)
			connect(belts[0], route.head)
			last.input = tower
			last.record.InputFromSlot = int8(route.slot)
			last.record.Parameters = []int32{int32(route.item), 0}
			tower.record.Parameters[base] = 1
			for i, item := range groups[route.tower] {
				if item.id == route.item {
					tower.record.Parameters[base+1] = int32(i + 1)
					break
				}
			}
			b.stationLinks = append(b.stationLinks, stationTransportLink{item: route.item, tower: tower, belt: last, slot: route.slot, head: route.head, path: append([]*placed{}, belts...), owner: route.head.owner, rate: route.rate})
		}
	}
	if *b.setting.NeedPower {
		for _, center := range centers {
			// 塔旁补电只服务物流塔；近邻产线杆已经覆盖塔心时复用它，避免追加一个游戏限距内的节点。
			if !stationPowerCovered(center, existing) {
				b.make(2201, center.x+3, center.y+3, 0)
			}
		}
	}
	return nil
}

// uniqueStationItems 职责：按配方结果顺序去重多产物；使用：独立出货带与塔槽位规划；边界：不排序、不丢副产物。
func uniqueStationItems(items []int) []int {
	seen := map[int]bool{}
	result := make([]int, 0, len(items))
	for _, item := range items {
		if item <= 0 || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

// stationPathCrosses 职责：在平面塔路线候选阶段拒绝未连接的轴向带段相交；使用：attachStations 逐路线寻路；边界：只检查同一高度的几何交叉，不替代最终球面/PhysX 校验。
func stationPathCrosses(path []vector, previous []stationRoute) bool {
	const epsilon = 1e-6
	pointOn := func(p, a, c vector) bool {
		if math.Abs(p.z-a.z) > epsilon || math.Abs(a.z-c.z) > epsilon {
			return false
		}
		return p.x >= math.Min(a.x, c.x)-epsilon && p.x <= math.Max(a.x, c.x)+epsilon &&
			p.y >= math.Min(a.y, c.y)-epsilon && p.y <= math.Max(a.y, c.y)+epsilon
	}
	for i := 1; i < len(path); i++ {
		a, c := path[i-1], path[i]
		for _, route := range previous {
			prior := route.path
			if route.head != nil {
				prior = append([]vector{route.head.a}, prior...)
			}
			for j := 1; j < len(prior); j++ {
				x, y := prior[j-1], prior[j]
				if math.Abs(a.z-c.z) > epsilon || math.Abs(x.z-y.z) > epsilon || math.Abs(a.z-x.z) > epsilon {
					continue
				}
				// 候选路径仅允许轴向段；若发生共线重叠或正交相交，都需要重新寻路。
				if math.Abs(a.x-c.x) < epsilon && math.Abs(x.x-y.x) < epsilon {
					if math.Abs(a.x-x.x) < epsilon && math.Max(math.Min(a.y, c.y), math.Min(x.y, y.y)) <= math.Min(math.Max(a.y, c.y), math.Max(x.y, y.y))+epsilon {
						return true
					}
				} else if math.Abs(a.y-c.y) < epsilon && math.Abs(x.y-y.y) < epsilon {
					if math.Abs(a.y-x.y) < epsilon && math.Max(math.Min(a.x, c.x), math.Min(x.x, y.x)) <= math.Min(math.Max(a.x, c.x), math.Max(x.x, y.x))+epsilon {
						return true
					}
				} else if math.Abs(a.x-c.x) < epsilon && math.Abs(x.y-y.y) < epsilon {
					if pointOn(vector{a.x, x.y, a.z}, a, c) && pointOn(vector{a.x, x.y, a.z}, x, y) {
						return true
					}
				} else if math.Abs(a.y-c.y) < epsilon && math.Abs(x.x-y.x) < epsilon {
					if pointOn(vector{x.x, a.y, a.z}, a, c) && pointOn(vector{x.x, a.y, a.z}, x, y) {
						return true
					}
				}
			}
		}
	}
	return false
}

// stationOutputHead 职责：取得机器端已经按物品过滤的独立出货带首端并写入物品图标；使用：mode3/4逐物品塔路；边界：多产物缺少独立带时硬失败，不再从共享干线二次过滤。
func (b *builder) stationOutputHead(l *line, item int) (*placed, error) {
	row := l.outputForItem(item)
	if len(row) == 0 || len(l.recipe.Results) > 1 && len(l.outputs) != len(l.recipe.Results) {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 产物 %d 缺少独立机器出货带", l.recipe.ID, item)}
	}
	members := map[*placed]bool{}
	for _, belt := range row {
		if belt == nil || !surfaceIsBelt(belt.record.ItemID) || members[belt] {
			return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 产物 %d 的塔出货带存在空节点、非带实体或重复引用", l.recipe.ID, item)}
		}
		members[belt] = true
	}
	for _, other := range l.recipe.Results {
		if other == item {
			continue
		}
		for _, belt := range l.outputForItem(other) {
			if members[belt] {
				return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: fmt.Sprintf("配方 %d 不同产物共用物流塔出货带", l.recipe.ID)}
			}
		}
	}
	row[0].record.Parameters = []int32{int32(item), 0}
	return row[0], nil
}

// stationPowerCovered 职责：冲突时复用同一产线块内覆盖塔心的感应塔；使用：物流塔补电；边界：仅合并追加点三格内且覆盖塔心六格内的杆，不改变其他补电点或跨块网络。
func stationPowerCovered(center vector, existing []*placed) bool {
	for _, p := range existing {
		if p.record.ItemID != 2201 || p.a.z != center.z {
			continue
		}
		dx, dy := p.a.x-center.x, p.a.y-center.y
		px, py := dx-3, dy-3
		// 六格约 7.55 米，保守小于感应塔原型 10.5 米覆盖；不以贴着覆盖边界为复用依据。
		if dx*dx+dy*dy <= 36 && px*px+py*py < 9 {
			return true
		}
	}
	return false
}

// stationBlockClearance 职责：求沿正 Y 平移候选塔区块的最小安全位移；使用：普通装箱按真实塔型与米制距离补齐净空；边界：使用赤道切平面格长，球面映射后的距离仍由最终规则检查。
func stationBlockClearance(existing, block []*placed, dx, dy float64) float64 {
	type interval struct{ low, high float64 }
	forbidden := []interval{}
	for _, left := range existing {
		if !surfaceIsStation(left.record.ItemID) {
			continue
		}
		for _, right := range block {
			if !surfaceIsStation(right.record.ItemID) {
				continue
			}
			x := right.a.x + dx - left.a.x
			y := right.a.y + dy - left.a.y
			z := (right.a.z - left.a.z) * 1.3333333
			radiusSquared := (float64(surfaceStationDistanceLimit(left.record.ItemID, right.record.ItemID)) - z*z) / (planarGridLength * planarGridLength)
			if x*x >= radiusSquared {
				continue
			}
			reach := math.Sqrt(radiusSquared - x*x)
			forbidden = append(forbidden, interval{low: -y - reach, high: -y + reach})
		}
	}
	sort.SliceStable(forbidden, func(i, j int) bool {
		if forbidden[i].low != forbidden[j].low {
			return forbidden[i].low < forbidden[j].low
		}
		return forbidden[i].high < forbidden[j].high
	})
	shift := 0.
	for _, span := range forbidden {
		if shift > span.low+1e-9 && shift < span.high-1e-9 {
			// 最终坐标转 float32，保留千分之一格避免边界舍入回禁区。
			shift = math.Ceil((span.high+1e-3)*1000) / 1000
		}
	}
	return shift
}
