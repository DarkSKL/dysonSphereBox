package build

// 文件职责：按规范 lane 实体拆圈，装配可扩展的过滤混带共享环路。
// 使用：mode6 在统一机器行与端口准备后调用；边界：每圈只承载选定带等级的真实容量，不改变机器/流量，不以三圈魔法上限或单臂超载代替真实接口。
// 逻辑流：1. 生成统一端口 => 2. 按容量确定性装箱 lane => 3. 创建独立闭环及并联过滤臂 => 4. 有界寻路 => 5. 逐边/逐实体复核后进入最终规则。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// busFabricRoute 职责：保存一条规范 lane 经共享环路的完整物理证据；使用：装配后逐边核验；边界：boundary 输出不经过环路，不能被误当内部运输。
type busFabricRoute struct {
	// route 职责：保留量化物品、份额、生产/消费行和过滤结构；使用：守恒与归属；边界：不从几何反推物料。
	route *transportRoute
	// source/target 职责：记录机器行/外界与环路之间的连续带连接器；使用：连通和索引核验；边界：不是隐藏的直接上下游连线。
	source, target *transportRoute
	// layer 职责：保存真实共享环路编号；使用：逐圈累计负载；边界：可有多个环路承载同一物品的不同份额。
	layer int
	// feed/take 职责：保存将物品送入环路和过滤取出的实际并联分拣器；使用：吞吐和方向核验；边界：每个分拣器均为一格，份额按臂数均分。
	feed, take []*placed
	// recycle 职责：把上一圈未消耗的外部原料过滤回本入口供料闭环；使用：原料入口回压和回收；边界：位于环路供料段之前，不能先回收尚未到达消费者的新投料。
	recycle []*placed
	// portHeight 职责：保存按实际臂数计算的纵向端口占地；使用：未提交候选的接驳位置估计；边界：只由当前选定等级的能力规划赋值。
	portHeight int
}

// busFabricBin 职责：保存一条选定等级单带环路的 lane 集合；使用：确定性首适应装箱；边界：load 永远不超过单带容量。
type busFabricBin struct {
	// load 职责：保存圈内全部物料计划负载；使用：容量装箱；边界：不按每种物品单独重用同一带宽。
	load float64
	// height 职责：累计实际并联端口的纬向占地；使用：避免小流量集中导致超长环路；边界：和容量同时满足才接纳新lane。
	height int
	// routes 职责：保存共享本圈的规范lane；使用：稳定装配；边界：一份lane只属于一个圈。
	routes []*busFabricRoute
}

// prepareBusFabricVariant 职责：规划一个独立拓扑及可选有限深度收紧；使用：完整可行性候选的局部几何准备；边界：布置阶段返回false表示无进一步改善候选，不代表忽略真实供需。
func (b *builder) prepareBusFabricVariant(ctx context.Context, req Request, dataset *quant.Dataset, tighten bool) (bool, error) {
	for _, production := range req.Plan.Productions {
		_, ok := dataset.RecipeByID[production.RecipeID]
		if !ok {
			return true, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "混带产线配方不存在"}
		}
	}
	// 实际机器端由 prepareConnectedRows 内的六槽位规划与实体复核负责。
	if err := b.prepareConnectedRows(ctx, req, dataset, math.Max(60, b.setting.BlockWidthLimit), false); err != nil {
		return true, err
	}
	original := append([]*transportRoute{}, b.transportRoutes...)
	b.transportRoutes = nil
	b.busFabric = make([]*busFabricRoute, 0, len(original))
	b.busLayerBelts = map[int][]*placed{}
	pending := []*busFabricRoute{}
	for _, route := range original {
		entry := &busFabricRoute{route: route, layer: -1}
		b.busFabric = append(b.busFabric, entry)
		if route.to == nil {
			continue
		}
		pending = append(pending, entry)
	}
	// 优先放入高流量，再按规范边和原行顺序稳定打破并列；不让 map 顺序影响拆圈。
	sort.SliceStable(pending, func(i, j int) bool {
		if pending[i].route.rate != pending[j].route.rate {
			return pending[i].route.rate > pending[j].route.rate
		}
		return pending[i].route.edge.ID < pending[j].route.edge.ID
	})
	minimum, maximum := bounds(b.all)
	layout, err := b.planBusFabricLayout(pending, minimum, maximum, req.SurfaceLayout != nil)
	if err != nil {
		return true, err
	}
	if tighten {
		var improved bool
		layout, improved = tightenBusFabricLayout(layout, minimum, maximum)
		if !improved {
			return false, nil
		}
	}
	b.owner = -1
	for _, placement := range layout.placements {
		index := placement.bin
		bin := layout.bins[index]
		if ctx.Err() != nil {
			return true, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "混带环路规划已取消或超时"}
		}
		height := bin.height
		left, low := placement.origin.x, placement.origin.y
		right := left + 6
		points := []vector{}
		for y := 0; y <= height; y++ {
			points = append(points, vector{right, low + float64(y), 0})
		}
		for x := right - 1; x >= left; x-- {
			points = append(points, vector{x, low + float64(height), 0})
		}
		for y := height - 1; y >= 0; y-- {
			points = append(points, vector{left, low + float64(y), 0})
		}
		for x := left + 1; x < right; x++ {
			points = append(points, vector{x, low, 0})
		}
		belts := b.byPath(points)
		connect(belts[len(belts)-1], belts[0])
		b.busLayerBelts[index] = belts
		byPoint := map[vector]*placed{}
		for _, belt := range belts {
			byPoint[belt.a] = belt
		}
		y := low + 2
		for _, entry := range bin.routes {
			entry.layer = index
			arms, _ := b.busFabricArmCount(entry.route.rate)
			if err := b.attachBusFabricPorts(entry, byPoint, left, right, y, arms); err != nil {
				return true, err
			}
			y += float64(entry.portHeight)
		}
		if *b.setting.NeedPower {
			for y := low + 3; y < low+float64(height); y += 6 {
				b.make(2201, right+5, y, 0)
				b.make(2201, left-5, y, 0)
			}
		}
		if len(b.all) > blueprint.MaxBuildingCount {
			return true, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "混带环路与并联端口超过20万建筑，请拆分目标"}
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "bus-neighbor-layout", Status: "passed", Message: fmt.Sprintf("固定几何候选选择 %s，%d 个真实环路，无障碍带长估计 %.0f；接驳仍须完成有界寻路与最终规则，未按流量忽略小份额空间成本", layout.name, len(layout.bins), layout.estimatedLength)})
	return true, nil
}

// busFabricArmCount 职责：按实际一格分拣器能力创建足量并联端口；使用：环路注入/取料；边界：未知等级或超过建筑预算的数量硬失败。
func (b *builder) busFabricArmCount(rate float64) (int, error) {
	capacity, err := transport.InserterCapacityPerMinute(b.setting.InserterLv, 1)
	if err != nil {
		return 0, err
	}
	count := int(math.Ceil((rate - 1e-8) / capacity))
	if count < 1 || count > 10000 {
		return 0, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("混带端口 %.3f/min 无法在已核对分拣器能力内表示", rate)}
	}
	return count, nil
}

// attachBusFabricPorts 职责：为一条 lane 建立供料/过滤提取臂与两侧连续带；使用：共享环路已落位后；边界：两条连接器通过环路传递物料，不互相直连。
func (b *builder) attachBusFabricPorts(entry *busFabricRoute, ring map[vector]*placed, left, right, y float64, arms int) error {
	route := entry.route
	feedPoints := []vector{{right + 3, y, 0}, {right + 2, y, 0}, {right + 1, y, 0}}
	takePoints := []vector{}
	feedOffset, feedHeight := 0, arms-1
	if route.from == nil {
		feedOffset, feedHeight = arms+2, arms*2+1
	}
	for i := 1; i <= feedHeight; i++ {
		feedPoints = append(feedPoints, vector{right + 1, y + float64(i), 0})
	}
	if route.from == nil {
		feedPoints = append(feedPoints, vector{right + 2, y + float64(feedHeight), 0})
		for i := feedHeight - 1; i >= 1; i-- {
			feedPoints = append(feedPoints, vector{right + 2, y + float64(i), 0})
		}
	}
	for i := 0; i < arms; i++ {
		takePoints = append(takePoints, vector{left - 1, y + float64(arms-1-i), 0})
	}
	takePoints = append(takePoints, vector{left - 2, y, 0}, vector{left - 3, y, 0})
	feed, take := b.byPath(feedPoints), b.byPath(takePoints)
	if route.from == nil {
		connect(feed[len(feed)-1], feed[1])
		feed[0].record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
		for i := 0; i < arms; i++ {
			arm, err := b.beltTransfer(ring[vector{right, y + float64(i), 0}], feed[2+i], b.setting.InserterLv, route.edge.ItemID)
			if err != nil {
				return err
			}
			entry.recycle = append(entry.recycle, arm)
		}
	}
	for i := 0; i < arms; i++ {
		arm, err := b.beltTransfer(feed[2+feedOffset+i], ring[vector{right, y + float64(feedOffset+i), 0}], b.setting.InserterLv, route.edge.ItemID)
		if err != nil {
			return err
		}
		entry.feed = append(entry.feed, arm)
		arm, err = b.beltTransfer(ring[vector{left, y + float64(i), 0}], take[arms-1-i], b.setting.InserterLv, route.edge.ItemID)
		if err != nil {
			return err
		}
		entry.take = append(entry.take, arm)
	}
	// 接入带的首端位于环路外侧三格，公共路由器可从空闲邻格接入；提取带尾端向西开放。
	entry.source = &transportRoute{edge: route.edge, rate: route.rate, from: route.from, to: &transportRow{}, start: route.start, end: feed[0], path: append([]*placed{}, route.path...)}
	entry.target = &transportRoute{edge: route.edge, rate: route.rate, from: &transportRow{}, to: route.to, start: take[len(take)-1], end: route.end, path: take}
	// feed 在源路由之后，单独保留在 source.sharedPath 供严格连通检查拼接，不能标成额外流量。
	entry.source.sharedPath = feed
	if route.from == nil {
		entry.source.path = []*placed{feed[0]}
		entry.source.boundary = feed[0]
	} else {
		b.transportRoutes = append(b.transportRoutes, entry.source)
	}
	b.transportRoutes = append(b.transportRoutes, entry.target)
	return nil
}
