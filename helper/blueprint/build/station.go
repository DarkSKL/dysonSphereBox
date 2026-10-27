package build

// 文件职责：装配单/双行星或星际物流塔和路由；使用：mode 3/4；边界：超容量明确告警，不静默丢弃物品。
// 站点参数和微调数值核对 dsp-calc LocalStation/InterstellarStation，算法独立实现，见 ../NOTICE.md。

import (
	"DysonSphereBox/helper/blueprint"
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
	id, capacity := 2103, 10000
	if mode == 4 {
		id, capacity = 2104, 20000
	}
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
	for _, id := range recipe.Items {
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
	for _, l := range lines {
		for i, row := range l.inputs {
			routes = append(routes, stationRoute{head: row[0], item: recipe.Items[i], tower: itemTower[recipe.Items[i]]})
		}
		routes = append(routes, stationRoute{head: l.output[0], item: recipe.Results[0], output: true, tower: 0})
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
	startY := lines[0].inputs[0][0].a.y
	endY := lines[len(lines)-1].output[0].a.y
	centerY := math.Floor((endY-startY)/2) + startY
	centers := make([]vector, len(groups))
	extra := lines[0].info.MachineType == 3 || lines[0].info.MachineType == 4 || lines[0].info.MachineType == 6
	failure := ""
	ready := false
	for attempt := 0; attempt < 20; attempt++ {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成超过截止时间"}
		}
		for i := range centers {
			centers[i] = vector{-5 - float64(attempt) - float64(i)*14, centerY + float64(i)*10, 0}
		}
		blocked := map[vector]bool{}
		// 外部物流路线不能穿过已经装配的产线带；混料 B 入口旁已有收集带，仅保留入口本身作为可出发点。
		for _, l := range lines {
			for _, p := range l.all {
				if p.a.z != 0 {
					continue
				}
				if p.record.ItemID > 2000 && p.record.ItemID < 2010 {
					blocked[p.a] = true
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
			if extra {
				v := route.head.a
				v.x--
				blocked[v] = true
			}
		}
		for _, center := range centers {
			for dx := -2.; dx <= 2; dx++ {
				for dy := -2.; dy <= 2; dy++ {
					blocked[vector{center.x + dx, center.y + dy, 0}] = true
				}
			}
			blocked[vector{center.x + 3, center.y + 2, 0}] = true
			blocked[vector{center.x + 2, center.y + 3, 0}] = true
			blocked[vector{center.x + 3, center.y + 3, 0}] = true
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
			if extra {
				begin.x--
			}
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
			for _, v := range path {
				blocked[v] = true
			}
			if !extra {
				path = path[1:]
			}
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
			centers[i] = vector{-26 - float64(i)*14, centerY + float64(i)*10, 0}
		}
		priority := []int{9, 10, 11, 8, 0, 7, 1, 6, 2, 5, 3, 4}
		for i := range routes {
			r := &routes[i]
			r.slot = priority[usedSlots[r.tower]]
			usedSlots[r.tower]++
		}
		if err := b.bridgeStationRoutes(ctx, routes, centers); err != nil {
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
		belts := b.byPath(points)
		tower := towers[route.tower]
		last := belts[len(belts)-1]
		base := 192 + route.slot*4
		if route.output {
			connect(route.head, belts[0])
			last.output = tower
			last.record.OutputToSlot = int8(route.slot)
			tower.record.Parameters[base] = 2
			tower.record.Parameters[base+1] = 0
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
		}
	}
	if *b.setting.NeedPower {
		for _, center := range centers {
			b.make(2201, center.x+3, center.y+3, 0)
		}
	}
	return nil
}
