package build

// 文件职责：在连续产物主带旁设置可选缓存，并证明主物流不依赖库存吞吐。
// 使用：机器净输出装配及最终验收；边界：回流先行，缓存不充当持续消耗，也不按全产线流量强制升级箱臂。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"fmt"
	"math"
)

// productionBuffer 职责：保存旁路缓存与独立主通道证据；使用：布局和终检；边界：一个库存只处理一种物品。
type productionBuffer struct {
	// row,item,rate 职责：关联原产线及满负荷净流；使用：主带容量核验；边界：rate不是缓存臂必须达到的吞吐。
	row  *transportRow
	item int
	rate float64
	// source,store 职责：保留机器净出口与旁挂库存；使用：来源证明；边界：库存不在主出货路径中。
	source, store *placed
	// feed 职责：保留机器到主通道的已连接短带；使用：共享流量证据；边界：不再交给全场寻路另行接驳。
	feed *transportRoute
	// detached 职责：允许大型流体旁路与回流附件就近独立落位；使用：避免刚性长行挤压机器回收链；边界：主带仍绕过库存，供给必须实际接通。
	detached bool
	// main,intake,outlet 职责：记录绕过库存的完整主通道及入口出口；使用：主路径证明；边界：主通道只允许带与已验证优先门。
	main, intake, outlet []*placed
	// storeIntake,storeOutlet,gates 职责：记录储液罐的溢流和回补支路；使用：罐口及优先参数检查；边界：固体缓存不使用分流门。
	storeIntake, storeOutlet, gates []*placed
	// inArms,outArms,entities 职责：记录固体取放臂及完整刚性模块；使用：随机器行移动；边界：臂速只决定缓存速度。
	inArms, outArms, entities []*placed
}

// bufferFluid 职责：按原版物品选择储液罐；使用：缓存构造；边界：不按名称猜测流体。
func bufferFluid(item int) bool {
	switch item {
	case 1000, 1007, 1114, 1116, 1120, 1121:
		return true
	}
	return false
}

// attachProductionBuffer 职责：将旁路模块直接接在原产物带末端；使用：净出货分流前；边界：主物流保持原带等级，不经过箱臂或库存。
func (b *builder) attachProductionBuffer(row *transportRow, item int, source *placed, cursor float64) (*productionBuffer, error) {
	if row.buffers[item] != nil {
		return nil, &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "同一行产物重复创建缓存"}
	}
	rate := productionItemRate(row.info.Capacity.Outputs, item) * float64(row.count) / float64(row.info.MachineCount)
	if !finitePositive(rate) {
		return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: "缓存主带缺少可信净出货能力"}
	}
	begin := len(b.all)
	buffer := &productionBuffer{row: row, item: item, rate: rate, source: source, detached: len(row.recycles) > 0 || bufferFluid(item) || b.compactBufferLayout()}
	var err error
	if bufferFluid(item) {
		err = b.buildTankSideBuffer(buffer)
	} else {
		err = b.buildBoxSideBuffer(buffer)
	}
	if err != nil {
		return nil, err
	}
	// 库存位于真实输入接驳与其他产物模块以西，主带直接延长，不新增独立寻路任务。
	left := source.a.x - 1
	for _, output := range row.line.outputs {
		if len(output) > 0 {
			left = math.Min(left, output[0].a.x-1)
		}
	}
	for _, port := range row.allInputEndpoints() {
		left = math.Min(left, port.a.x-1)
	}
	targetY := source.a.y
	previousCount := 0
	if len(row.recipe.Results) > 1 {
		left -= float64(2*len(row.recipe.Results) + 2)
	}
	head := buffer.intake[0]
	if !buffer.detached {
		localMin, localMax := bounds(b.all[begin:])
		for _, previous := range b.productionBuffers {
			if previous.row == row {
				previousCount++
				minimum, maximum := bounds(previous.entities)
				if source.a.y >= previous.source.a.y {
					targetY = math.Max(targetY, maximum.y+3-(localMin.y-head.a.y))
				} else {
					targetY = math.Min(targetY, minimum.y-3-(localMax.y-head.a.y))
				}
			}
		}
		shift(b.all[begin:], left-head.a.x, targetY-head.a.y, 0)
		if math.Abs(source.a.z-head.a.z) > 1e-6 {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "旁路缓存主带与净出口高度不一致"}
		}
	}
	var path []*placed
	if buffer.detached {
		path = []*placed{source}
	} else if previousCount == 0 {
		path = b.joinBufferBelts([]*placed{source}, []*placed{head}, true)
	} else {
		turnX := left + float64(2*(previousCount+1))
		turn := b.byPath(manhattan(vector{turnX, source.a.y, 0}, vector{turnX, head.a.y, 0}, false))
		path = b.joinBufferBelts([]*placed{source}, turn, true)
		path = b.joinBufferBelts(path, []*placed{head}, true)
	}
	buffer.feed = &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("production-buffer:%d:%d", row.owner, len(b.productionBuffers)), ItemID: item, FlowKind: "internal"}, from: row, start: source, end: head, path: path}
	buffer.feed.placementWeight = 4
	if buffer.detached {
		buffer.feed.to = &transportRow{owner: -1, inputs: map[int]*placed{item: head}}
	}
	for _, route := range row.outputs {
		if route.edge.ItemID == item {
			buffer.feed.rate += route.rate
		}
	}
	buffer.entities = append([]*placed(nil), b.all[begin:]...)
	for _, entity := range buffer.entities {
		if surfaceIsBelt(entity.record.ItemID) {
			entity.record.Parameters = []int32{int32(item), 0}
		}
	}
	if row.buffers == nil {
		row.buffers = map[int]*productionBuffer{}
	}
	row.buffers[item] = buffer
	b.productionBuffers = append(b.productionBuffers, buffer)
	return buffer, nil
}

// isProductionBuffer 职责：识别已登记库存；使用：配送完整性；边界：不豁免未知箱体。
func (b *builder) isProductionBuffer(entity *placed) bool {
	for _, buffer := range b.productionBuffers {
		if buffer.store == entity {
			return true
		}
	}
	return false
}

// appendProductionBufferEdges 职责：补充已核验库存的可选物料路径；使用：最终机器图；边界：主通道另行证明，不依赖这些库存边才能交付。
func (b *builder) appendProductionBufferEdges(graph map[*placed][]finalMaterialEdge) {
	for _, buffer := range b.productionBuffers {
		if bufferFluid(buffer.item) {
			last := buffer.storeIntake[len(buffer.storeIntake)-1]
			graph[last] = append(graph[last], finalMaterialEdge{target: buffer.store, item: buffer.item})
			graph[buffer.store] = append(graph[buffer.store], finalMaterialEdge{target: buffer.storeOutlet[0], item: buffer.item})
		} else {
			for _, arm := range append(append([]*placed{}, buffer.inArms...), buffer.outArms...) {
				graph[arm.input] = append(graph[arm.input], finalMaterialEdge{target: arm.output, item: buffer.item})
			}
		}
	}
}

// validateBufferedDirectOutput 职责：确认直接出货起于旁路主带末端；使用：单去向路线检查；边界：不把箱臂作为生产输出能力。
func (b *builder) validateBufferedDirectOutput(route *transportRoute) error {
	buffer := route.from.buffers[route.edge.ItemID]
	if buffer == nil || len(route.path) == 0 || route.start != buffer.outlet[len(buffer.outlet)-1] || route.path[0] != route.start {
		return transportAllocationError(route.edge, route.rate, "缓存主带未接到真实出货带")
	}
	for _, other := range route.from.outputs {
		if other != route && other.edge.ItemID == route.edge.ItemID {
			return transportAllocationError(route.edge, route.rate, "缓存多去向缺少独立过滤分支")
		}
	}
	return nil
}

// validateProductionBuffers 职责：证明连续主带、可选取放与液体优先旁路；使用：最终物流验收；边界：缓存关闭、空或满不应破坏主路径，库存吞吐不计为主带运力。
func (b *builder) validateProductionBuffers() error {
	fail := func(message string) error {
		return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "旁路缓存：" + message}
	}
	if !b.setting.EnableIntermediateBuffer {
		if len(b.productionBuffers) > 0 {
			return fail("未启用缓存却生成了缓存")
		}
		return nil
	}
	alive := map[*placed]bool{}
	indices := map[*placed]int{}
	for i, entity := range b.all {
		alive[entity] = true
		indices[entity] = i
	}
	if len(b.productionBuffers) == 0 {
		return fail("未建立任何缓存")
	}
	for _, route := range b.transportRoutes {
		if route.from != nil && route.from.buffers[route.edge.ItemID] == nil {
			return fail("存在未缓存的净产物")
		}
	}
	for _, buffer := range b.productionBuffers {
		if buffer.row.buffers[buffer.item] != buffer || !alive[buffer.store] || len(buffer.main) < 3 || len(buffer.intake) == 0 || len(buffer.outlet) == 0 || buffer.feed == nil || len(buffer.feed.path) < 2 || buffer.feed.path[0] != buffer.source || buffer.feed.path[len(buffer.feed.path)-1] != buffer.main[0] {
			return fail("缺少主带或来源")
		}
		want := buffer.row.line.outputForItem(buffer.item)[0]
		for _, recycle := range buffer.row.recycles {
			if recycle.item == buffer.item {
				want = recycle.surplus[len(recycle.surplus)-1]
			}
		}
		if buffer.source != want {
			return fail("库存来源不是机器净输出")
		}
		for _, path := range [][]*placed{buffer.feed.path, buffer.main, buffer.storeIntake, buffer.storeOutlet} {
			for i, node := range path {
				if !alive[node] {
					return fail("引用了已删除实体")
				}
				if i > 0 && path[i-1].output != node && !(path[i-1].record.ItemID == 2020 && node.input == path[i-1]) {
					return fail("主带或缓存支路断开")
				}
			}
		}
		for i, node := range buffer.main {
			if !surfaceIsBelt(node.record.ItemID) && node.record.ItemID != 2020 {
				return fail("主出货路径经过库存或分拣器")
			}
			if surfaceIsBelt(node.record.ItemID) && node.record.ItemID != int16(2000+b.setting.BeltLv) {
				return fail("主带等级被缓存降档")
			}
			if node.record.ItemID == 2020 {
				if len(buffer.gates) != 2 || (node != buffer.gates[0] && node != buffer.gates[1]) || i == 0 || i+1 == len(buffer.main) || buffer.main[i-1].record.OutputToSlot != 0 || buffer.main[i+1].record.InputFromSlot != 2 {
					return fail("液体主通道没有使用已验证的直行槽")
				}
			}
		}
		if buffer.main[len(buffer.main)-1] != buffer.outlet[len(buffer.outlet)-1] {
			return fail("主带出口不一致")
		}
		if bufferFluid(buffer.item) {
			if len(buffer.gates) != 2 || len(buffer.storeIntake) < 2 || len(buffer.storeOutlet) < 2 {
				return fail("储液罐缺少独立旁路")
			}
			for i, gate := range buffer.gates {
				priority := int8(2)
				if i == 1 {
					priority = 0
				}
				turns := (int(math.Round(float64(gate.record.Yaw)/90)) + 3) % 4
				if err := validateRecycleGate(gate, priority, buffer.item, turns, indices); err != nil {
					return err
				}
			}
			last, first := buffer.storeIntake[len(buffer.storeIntake)-1], buffer.storeOutlet[0]
			if buffer.store.record.ItemID != 2106 || len(buffer.store.record.Parameters) != 2 || buffer.store.record.Parameters[0] != 1 || buffer.store.record.Parameters[1] != 1 || last.output != buffer.store || last.record.OutputToSlot != 1 || first.input != buffer.store || first.record.InputFromSlot != 3 {
				return fail("罐体进出开关或端口错误")
			}
			if buffer.intake[len(buffer.intake)-1].output != buffer.gates[0] || buffer.intake[len(buffer.intake)-1].record.OutputToSlot != 0 || buffer.outlet[0].input != buffer.gates[1] || buffer.outlet[0].record.InputFromSlot != 2 || buffer.storeIntake[0].input != buffer.gates[0] || buffer.storeIntake[0].record.InputFromSlot != 3 || buffer.storeOutlet[len(buffer.storeOutlet)-1].output != buffer.gates[1] || buffer.storeOutlet[len(buffer.storeOutlet)-1].record.OutputToSlot != 3 {
				return fail("液体主带与旁路接错槽")
			}
		} else {
			if buffer.store.record.ItemID != 2101 || len(buffer.store.record.Parameters) != 110 || buffer.store.record.Parameters[0] != 0 || len(buffer.inArms) != 1 || len(buffer.outArms) != 1 {
				return fail("箱体或取放臂缺失")
			}
			used := map[int8]bool{}
			for index, arm := range []*placed{buffer.inArms[0], buffer.outArms[0]} {
				slot, box, belt := arm.record.OutputToSlot, arm.output, arm.input
				if index == 1 {
					slot, box, belt = arm.record.InputFromSlot, arm.input, arm.output
				}
				found := false
				for _, node := range buffer.main {
					found = found || node == belt
				}
				if !alive[arm] || box != buffer.store || !found || slot < 0 || slot > 11 || used[slot] || int(arm.record.FilterID) != buffer.item {
					return fail("箱臂方向、物品或槽位错误")
				}
				used[slot] = true
				if err := flyInserterSpanMatchesGeometry(arm); err != nil {
					return err
				}
				capacity, err := flyEntityInserterCapacity(arm)
				if err != nil {
					return err
				}
				if capacity <= 0 {
					return fail("箱臂无有效能力")
				}
			}
		}
		if err := b.validateBufferIsolation(buffer); err != nil {
			return err
		}
	}
	for _, check := range b.logistics.Checks {
		if check.RuleID == "production-buffers" {
			return nil
		}
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "production-buffers", Status: "passed", Message: fmt.Sprintf("%d组旁路缓存均保留独立主通道：固体同带取放，液体优先直行、拥堵溢流、空隙回补；箱臂和库存不承担主带吞吐", len(b.productionBuffers))})
	return nil
}

// validateBufferIsolation 职责：拒绝旁路缓存接入未知物料或外来库存支路；使用：最终核验；边界：只允许已登记主带和取放连接。
func (b *builder) validateBufferIsolation(buffer *productionBuffer) error {
	allowed := map[[2]*placed]bool{}
	protected := map[*placed]bool{buffer.store: true}
	for _, path := range [][]*placed{buffer.feed.path, buffer.main, buffer.storeIntake, buffer.storeOutlet} {
		for i, node := range path {
			protected[node] = true
			if i > 0 {
				allowed[[2]*placed{path[i-1], node}] = true
			}
		}
	}
	for _, arm := range append(append([]*placed{}, buffer.inArms...), buffer.outArms...) {
		allowed[[2]*placed{arm, arm.output}] = true
		allowed[[2]*placed{arm.input, arm}] = true
	}
	if bufferFluid(buffer.item) {
		allowed[[2]*placed{buffer.storeIntake[len(buffer.storeIntake)-1], buffer.store}] = true
		allowed[[2]*placed{buffer.store, buffer.storeOutlet[0]}] = true
		allowed[[2]*placed{buffer.gates[0], buffer.storeIntake[0]}] = true
		allowed[[2]*placed{buffer.storeOutlet[len(buffer.storeOutlet)-1], buffer.gates[1]}] = true
	}
	delete(protected, buffer.source)
	for _, node := range b.all {
		if protected[node.output] && !allowed[[2]*placed{node, node.output}] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "缓存存在未登记的额外供给"}
		}
		if node.input == buffer.store && !allowed[[2]*placed{buffer.store, node}] {
			return &blueprint.Error{Code: "GEN_E_CONNECTIVITY", Message: "缓存存在未登记的库存出货"}
		}
	}
	return nil
}
