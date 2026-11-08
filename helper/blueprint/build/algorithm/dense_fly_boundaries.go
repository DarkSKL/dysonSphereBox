package build

// 文件职责：将同物料同方向的工厂边界份额合并到有限真实配送箱，不为内部产品建立配送设施。
// 使用：内部机器端口已装配、全场落位之前；边界：一个2107只过滤一种物品，箱体三个已核对西侧槽及带宽共同限制每组容量。

import (
	"context"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// prepareDenseFlyBoundaryUnits 职责：按物料、用途及实际箱臂容量装配边界设施；使用：directBoundaryUnits回调；边界：不删除规范边，输入多去向用真实过滤臂，输出多来源用真实有向合流。
func (b *builder) prepareDenseFlyBoundaryUnits(ctx context.Context, _ []*transportRow) ([]denseSpatialUnit, error) {
	if b.denseFlyBlackbox == nil || !finitePositive(b.directBoundaryLaneLimit) {
		return nil, &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送黑盒缺少真实边界容量规划"}
	}
	type boundaryGroup struct {
		item   int
		input  bool
		rate   float64
		routes []*transportRoute
	}
	groups := []*boundaryGroup{}
	for _, route := range b.transportRoutes {
		if route.from != nil && route.to != nil {
			continue
		}
		if route.from == nil && route.to == nil {
			return nil, transportAllocationError(route.edge, route.rate, "配送黑盒暂不接受无生产机器的原料直接交付")
		}
		if route.rate > b.directBoundaryLaneLimit+1e-6 {
			return nil, transportAllocationError(route.edge, route.rate, "边界份额超过所选箱臂与单带共同能力")
		}
		input := route.from == nil
		var chosen *boundaryGroup
		for _, group := range groups {
			if group.item == route.edge.ItemID && group.input == input && group.rate+route.rate <= b.directBoundaryLaneLimit+1e-6 {
				chosen = group
				break
			}
		}
		if chosen == nil {
			chosen = &boundaryGroup{item: route.edge.ItemID, input: input}
			groups = append(groups, chosen)
		}
		chosen.rate += route.rate
		chosen.routes = append(chosen.routes, route)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].input != groups[j].input {
			return groups[i].input
		}
		return groups[i].item < groups[j].item
	})
	b.owner = -1
	for _, group := range groups {
		if err := b.checkGeneration("配送黑盒边界设施规划"); err != nil {
			return nil, err
		}
		begin := len(b.all)
		var store *denseFlyBoundaryStore
		for _, existing := range b.denseFlyBlackbox.stores {
			if existing.item == group.item && existing.input == group.input && len(existing.ports) < 2 {
				store = existing
				break
			}
		}
		var endpoint *flyTransportEndpoint
		var tail []*placed
		var err error
		if store == nil {
			endpoint, tail, _, err = b.makeDenseFlyFacility(flyIndependentPort{item: group.item, input: group.input}, group.rate)
			if err == nil {
				store = &denseFlyBoundaryStore{box: endpoint.box, delivery: endpoint.delivery, item: group.item, input: group.input}
				b.denseFlyBlackbox.stores = append(b.denseFlyBlackbox.stores, store)
			}
		} else {
			endpoint, tail, err = b.makeDenseFlyOppositeFacility(store, group.rate)
		}
		if err != nil {
			return nil, err
		}
		// 两个真实同高节点把箱臂处的出入口引到箱外，后续portal据其已有有向邻边选择向外方向。
		direction := math.Copysign(1, tail[0].a.x-endpoint.box.a.x)
		middle := b.make(2000+b.setting.BeltLv, tail[0].a.x+direction, tail[0].a.y, tail[0].a.z)
		head := b.make(2000+b.setting.BeltLv, tail[0].a.x+2*direction, tail[0].a.y, tail[0].a.z)
		if group.input {
			connect(tail[0], middle)
			connect(middle, head)
		} else {
			connect(head, middle)
			connect(middle, tail[0])
		}
		endpoint.link, endpoint.belts = head, append([]*placed{middle}, tail...)
		facility := &denseFlyBoundaryFacility{store: store, endpoint: endpoint, routes: group.routes, branches: map[*transportRoute][]*placed{}, monitorHeads: map[*transportRoute]*placed{}}
		ports := []denseStreetPort{}
		if group.input && len(group.routes) > 1 {
			if err := b.attachDenseFlyBoundaryBranches(facility); err != nil {
				return nil, err
			}
			for _, route := range group.routes {
				branch := facility.branches[route]
				ports = append(ports, denseStreetPort{item: group.item, head: route.start, inner: branch[len(branch)-2]})
			}
		} else {
			for _, route := range group.routes {
				if group.input {
					route.start, route.path = head, []*placed{head}
				} else {
					route.end = head
				}
				route.sharedPath = append(route.sharedPath, endpoint.belts...)
			}
			ports = append(ports, denseStreetPort{item: group.item, input: !group.input, head: head, inner: middle})
			if group.input && b.setting.EnableFlowMonitor {
				// 所有箱臂先汇入tail[0]，随后两个地面节点构成真实监测宿主，不能在箱库存上虚标监测。
				tail[0].record.Parameters = []int32{int32(group.item), int32(math.Ceil(group.rate))}
				b.makeDenseFlyBoundaryMonitor(middle)
			}
		}
		for _, route := range group.routes {
			route.boundary = endpoint.delivery
			if group.input && route.start != nil {
				route.start.record.Parameters = []int32{int32(group.item), int32(math.Ceil(route.rate))}
			}
		}
		facility.entities = b.all[begin:]
		b.denseFlyBlackbox.facilities = append(b.denseFlyBlackbox.facilities, facility)
		store.ports = append(store.ports, facility)
		store.entities = append(store.entities, facility.entities...)
		store.spatialPorts = append(store.spatialPorts, ports...)
	}
	units := []denseSpatialUnit{}
	for _, store := range b.denseFlyBlackbox.stores {
		units = append(units, denseSpatialUnit{entities: store.entities, ports: store.spatialPorts})
	}
	if err := b.rejectDenseFlyFacilityDominance(ctx); err != nil {
		return nil, err
	}
	return units, nil
}

// makeDenseFlyOppositeFacility 职责：用同一箱体对侧的真实3/4/5槽建立第二条独立带；使用：同物料双面端口；边界：仅在同方向库存上复用箱体与配送器，不保留临时箱实体或相同槽重复臂。
func (b *builder) makeDenseFlyOppositeFacility(store *denseFlyBoundaryStore, rate float64) (*flyTransportEndpoint, []*placed, error) {
	begin := len(b.all)
	endpoint, tail, entities, err := b.makeDenseFlyFacility(flyIndependentPort{item: store.item, input: store.input}, rate)
	if err != nil {
		return nil, nil, err
	}
	rotatePlacedQuarter(entities, 2)
	shift(entities, store.box.a.x-endpoint.box.a.x, store.box.a.y-endpoint.box.a.y, 0)
	temporaryBox, temporaryDelivery := endpoint.box, endpoint.delivery
	for _, arm := range endpoint.arms {
		if store.input {
			arm.input = store.box
			arm.record.InputFromSlot = (arm.record.InputFromSlot + 6) % 12
		} else {
			arm.output = store.box
			arm.record.OutputToSlot = (arm.record.OutputToSlot + 6) % 12
		}
	}
	kept := append([]*placed{}, b.all[:begin]...)
	for _, entity := range entities {
		if entity != temporaryBox && entity != temporaryDelivery {
			kept = append(kept, entity)
		}
	}
	b.all = kept
	endpoint.box, endpoint.delivery = store.box, store.delivery
	return endpoint, tail, nil
}

// attachDenseFlyBoundaryBranches 职责：为同一输入箱的多个真实消费者建立选定等级过滤支路；使用：箱体共享；边界：不同主带节点交替出料，不能把普通传送带的单个output伪装成T分流。
func (b *builder) attachDenseFlyBoundaryBranches(facility *denseFlyBoundaryFacility) error {
	endpoint := facility.endpoint
	main := endpoint.link
	shared := append(append([]*placed{}, endpoint.belts...), main)
	capacity := inserterCapacityPerMinute(b.setting.InserterLv, 1)
	if b.nativeCapacityReserve {
		capacity = inserterCapacityPerMinute(b.setting.InserterLv, 3)
	}
	direction := math.Copysign(1, main.a.x-endpoint.box.a.x)
	for i, route := range facility.routes {
		side := 1.
		if i%2 == 1 {
			side = -side
		}
		branch := []*placed{}
		count := max(1, int(math.Ceil((route.rate-1e-8)/capacity)))
		for j := 0; j < count; j++ {
			next := b.make(2000+b.setting.BeltLv, main.a.x+direction, main.a.y, main.a.z)
			connect(main, next)
			main = next
			shared = append(shared, main)
			pickup := b.make(2000+b.setting.BeltLv, main.a.x, main.a.y+side, main.a.z)
			if len(branch) > 0 {
				connect(branch[len(branch)-1], pickup)
			}
			branch = append(branch, pickup)
			arm, err := b.beltTransfer(main, pickup, b.setting.InserterLv, endpoint.item)
			if err != nil {
				return err
			}
			route.filters = append(route.filters, arm)
		}
		exit := b.make(2000+b.setting.BeltLv, main.a.x+direction, main.a.y+side, main.a.z)
		connect(branch[len(branch)-1], exit)
		branch = append(branch, exit)
		if b.setting.EnableFlowMonitor {
			monitorHost := b.make(2000+b.setting.BeltLv, exit.a.x+direction, exit.a.y, exit.a.z)
			final := b.make(2000+b.setting.BeltLv, exit.a.x+2*direction, exit.a.y, exit.a.z)
			connect(exit, monitorHost)
			connect(monitorHost, final)
			branch = append(branch, monitorHost, final)
			facility.monitorHeads[route] = exit
			exit.record.Parameters = []int32{int32(endpoint.item), int32(math.Ceil(route.rate))}
			b.makeDenseFlyBoundaryMonitor(monitorHost)
			exit = final
		}
		route.start, route.path = exit, append([]*placed{}, branch...)
		route.sharedPath = append(route.sharedPath, shared...)
		facility.branches[route] = branch
		if i%2 == 1 && i != len(facility.routes)-1 {
			gapCount := 1
			if b.setting.EnableFlowMonitor {
				gapCount = 3
			}
			for gapIndex := 0; gapIndex < gapCount; gapIndex++ {
				gap := b.make(2000+b.setting.BeltLv, main.a.x+direction, main.a.y, main.a.z)
				connect(main, gap)
				main = gap
				shared = append(shared, gap)
			}
		}
	}
	return nil
}

// makeDenseFlyBoundaryMonitor 职责：把默认监测器放到预构造的真实直线宿主；使用：设施整体参与全场占位；边界：不在此声称净空或供电成功，后续统一终检仍逐项核验。
func (b *builder) makeDenseFlyBoundaryMonitor(host *placed) {
	monitor := b.make(2030, host.a.x, host.a.y, host.a.z)
	monitor.record.Yaw, monitor.record.Yaw2 = host.record.Yaw, host.record.Yaw
}
