package build

// 文件职责：把统一供需图一次性装配为过滤端口、紧凑候选与完整直连带工厂。
// 使用：mode 1 生成主链；边界：任何容量、端口、寻路或最终规则失败都不返回部分蓝图。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// betterTransportCandidate 职责：把直连及局部排布比较交给共同形状规则；使用：两侧设施数无差异时；边界：不修改实体或物料。
func betterTransportCandidate(a, b LayoutMetrics) bool { return BetterLayoutCandidate(a, 0, b, 0) }

// assembleConnectedVariant 职责：在不改变机器、端口和规范 lane 的前提下装配一种空间候选；使用：基线与轮廓靠拢对比；边界：所有候选仍须完成寻路和 finish 的严格规则检查。
func (b *builder) assembleConnectedVariant(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64, compact bool) error {
	if err := b.prepareConnectedRows(ctx, req, dataset, heightLimit, compact); err != nil {
		return err
	}
	return b.completePreparedConnected(ctx, req)
}

// completePreparedConnected 职责：在已完成排布的真实端口图上接线并补齐附件；使用：直连及复用该装配能力的模式。
// 边界：不重新装箱；集中或就近入口的差异只在这一阶段生效，之后仍须完整几何与物理规则。
func (b *builder) completePreparedConnected(ctx context.Context, req Request) error {
	if err := b.preflightStrictStationCapacity(req); err != nil {
		return err
	}
	if err := b.prepareStationTransportEntries(ctx); err != nil {
		return err
	}
	if err := b.routeTransport(ctx); err != nil {
		return err
	}
	if err := b.commitDenseConnectedStreets(); err != nil {
		return err
	}
	if b.connectedCompactBelts {
		if err := b.trimConnectedTails(); err != nil {
			return err
		}
	}
	b.previewCheckpoint("routing")
	if !b.connectedGlobalPower {
		if err := b.completeBoundaryFlowMonitors(ctx, req.Mode); err != nil {
			return err
		}
		if b.denseStreetCandidate {
			// 与球面出口一致，先归一到最终局部原点再搜索供电格点，避免小数整体平移改变可用电塔候选。
			low, _ := bounds(b.all)
			shift(b.all, -low.x, -low.y, 0)
			b.connectedGlobalPower = true
			defer func() { b.connectedGlobalPower = false }()
			return b.completeConnectedPower(ctx)
		}
		return nil
	}
	if req.SurfaceLayout == nil || req.SurfaceLayout.Mode == "planar" {
		return b.completeConnectedPower(ctx)
	}
	return b.completeConnectedSurfacePower(ctx, req)
}

// preflightStrictStationCapacity 职责：提前拒绝必留设施已超过单纬带高度的标准黑盒；使用：布局固定后、运输寻路前；边界：不计算可裁尾带的包络，不影响原生跨带或后续可重排的候选。
func (b *builder) preflightStrictStationCapacity(req Request) error {
	if b.policy == nil || b.policy.density != densityStandard || b.policy.records != recordStationBlackbox || b.directBoundaryUnits == nil || len(b.surfaceBlocks) != 0 {
		return nil
	}
	layout := req.SurfaceLayout
	if layout == nil || layout.Mode != "explicitTropic" || layout.LatitudeZone == "" || layout.ExpandAdjacent || layout.polarRoot || layout.nativeBand != nil || b.polarFrame != nil {
		return nil
	}
	band, _, err := densityLayoutBand(layout)
	if err != nil {
		return err
	}
	maximumHeight := 0
	for _, native := range surfaceLatitudeBands(layout.GridSegment) {
		low, high := max(band.low, native.low), min(band.high, native.high)
		if low <= high {
			maximumHeight = max(maximumHeight, high-low+1)
		}
	}
	permanent := make([]*placed, 0, len(b.all))
	for _, entity := range b.all {
		if !surfaceIsBelt(entity.record.ItemID) {
			permanent = append(permanent, entity)
		}
	}
	if len(permanent) == 0 || maximumHeight == 0 {
		return nil
	}
	low, high := bounds(permanent)
	// 后续只补接线及裁掉尾带，不能缩小这些设施的纬向跨度；忽略浮点临界误差，仍由最终完整包络核验。
	if high.y-low.y > float64(maximumHeight-1)+1e-6 {
		width, height := int(math.Ceil(high.x-low.x))+1, int(math.Ceil(high.y-low.y))+1
		return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("标准黑盒必留设施包络%dx%d超过所选区域单纬带最大高度%d，当前严格候选无法完成运输前落位", width, height, maximumHeight)}
	}
	return nil
}

// prepareConnectedRows 职责：装配统一机器行和逐物品过滤端口但不决定全局运输拓扑；使用：直连带与可扩展混带环路共用局部几何；边界：调用方必须继续完成全图路由与最终规则，不能直接编码半成品。
func (b *builder) prepareConnectedRows(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64, compact bool) error {
	b.generationCtx = ctx
	if b.setting.EnableIntermediateBuffer {
		b.denseSpatial, b.connectedGlobalPower = true, true
	}
	if b.connectedGlobalPower {
		originalPower := b.setting.NeedPower
		disabled := false
		b.setting.NeedPower = &disabled
		defer func() { b.setting.NeedPower = originalPower }()
	}
	rows, err := b.planTransportRows(ctx, req, dataset)
	if err != nil {
		return err
	}
	if len(byproductPriorityGroups(b.transportRoutes)) > 0 {
		b.denseSpatial, b.connectedGlobalPower = true, true
	}
	x, y, columnWidth := 0., 0., 0.
	horizontal := req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" && (!compact || b.connectedHorizontalPacking)
	packing := &packingIndex{cells: map[[3]int][]int{}}
	var denseUnits []denseSpatialUnit
	streetOwners := map[int]bool{}
	if b.denseStreetCandidate {
		denseUnits, streetOwners, err = b.prepareDenseConnectedStreets(ctx, rows, req.Setting.NeedPower == nil || *req.Setting.NeedPower)
		if err != nil {
			return err
		}
	}
	for i, row := range rows {
		if streetOwners[row.owner] {
			continue
		}
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "机器行装配已取消或超时"}
		}
		reportProgress(ctx, "ports", "正在装配机器行与逐物品过滤端口", i+1, len(rows))
		b.owner = row.owner
		begin := len(b.all)
		row.line, err = b.buildLine(row.info, row.recipe, row.count, 1, 0, true)
		if err != nil {
			return err
		}
		for item, belts := range row.line.inputs {
			row.inputs[row.recipe.Items[item]] = belts[0]
		}
		if err := b.coatSupply([]*line{row.line}); err != nil {
			return err
		}
		if len(row.line.coaters) > 0 {
			item := []int{0, 1141, 1142, 1143}[row.info.IncreaseLv]
			links := b.coaterLinks[item]
			if len(links) > 0 {
				if req.Mode == 1 || b.directBoundaryUnits != nil {
					row.bindSprayInput(item, links[len(links)-1])
				} else {
					row.inputs[item] = links[len(links)-1]
				}
			}
		}
		// 同一生产行的过滤分支按内部消费者优先，再接目标交付和剩余副产物；
		// 每路仍保留独立 rate，边界端只承诺按该计划速率供料/取货，不宣称过滤臂精确限流。
		sort.SliceStable(row.outputs, func(i, j int) bool {
			priority := func(r *transportRoute) int {
				if r.to != nil {
					return 0
				}
				if r.edge.FlowKind == "target-output" {
					return 1
				}
				return 2
			}
			if pi, pj := priority(row.outputs[i]), priority(row.outputs[j]); pi != pj {
				return pi < pj
			}
			return row.outputs[i].edge.ID < row.outputs[j].edge.ID
		})
		// 先落位完整输入接驳；低等级分拣器可能把产物移到内轨，过滤区必须
		// 知道所有输入带真实西端（包括mode6动态长度），才能避免向外取料撞带。
		items := make([]int, 0, len(row.inputs))
		for item := range row.inputs {
			items = append(items, item)
		}
		sort.Ints(items)
		for _, item := range items {
			endpoint := row.inputs[item]
			role := recipeInputRole
			if (req.Mode == 1 || b.directBoundaryUnits != nil) && item == productionSprayItem(row.info) && productionRecipeInputCount(row.recipe, item) == 0 {
				role = sprayInputRole
			}
			stub := b.prepareConnectedInputStub(row, item, role, endpoint)
			row.inputStubs[item] = stub
			row.inputs[item] = stub[0]
			if role == sprayInputRole {
				row.sprayInput, row.sprayStub = stub[0], stub
			}
		}
		if (req.Mode == 1 || b.directBoundaryUnits != nil) && row.sprayInput != nil && productionRecipeInputCount(row.recipe, productionSprayItem(row.info)) > 0 {
			row.sprayStub = b.prepareConnectedInputStub(row, productionSprayItem(row.info), sprayInputRole, row.sprayInput)
			row.sprayInput = row.sprayStub[0]
		}
		if err := b.prepareMachineRecycles(row); err != nil {
			return err
		}
		if b.policy != nil && b.policy.connectedRows.preferFreshRecycleInput {
			for _, route := range b.transportRoutes {
				if route.to != row {
					continue
				}
				for _, recycle := range row.recycles {
					if recycle.item == route.edge.ItemID {
						// 混带取料须进入新料侧优先门，不能沿原stub绕开回流门。
						route.end = recycle.fresh
						break
					}
				}
			}
		}
		if err := b.attachFilteredOutputs(row); err != nil {
			return err
		}
		if b.policy != nil && b.policy.connectedRows.prepareConnectedSelfSpray {
			if err := b.prepareConnectedSelfSpray(row, begin); err != nil {
				return err
			}
		}
		// 接入干线属于机器行真实占地，必须在装箱前生成，否则后补的西侧带会侵入上一列。
		block := b.all[begin:]
		if b.denseSpatial {
			if b.setting.EnableIntermediateBuffer {
				var units []denseSpatialUnit
				block, units = b.detachProductionBuffers(row, block)
				denseUnits = append(denseUnits, units...)
			}
			denseUnits = append(denseUnits, denseSpatialUnit{entities: block, row: row})
			// 本行的端口、回流与缓存附件也按局部事务提交，下一行无需补扫全部前缀实体。
			b.previewLocalCheckpoint("assembly", begin)
			continue
		}
		minimum, maximum := bounds(block)
		width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
		// 机器行的带端已在块内预留，块间只保留一格最小走廊；
		// 路由占位和碰撞预检会拒绝真正不可行的候选，不再用固定大留白
		// 把可用面积浪费在每一行之间。
		const rowGap = 1.0
		const columnGap = 1.0
		if horizontal && x > 0 && x+width > heightLimit {
			x, y, columnWidth = 0, y+columnWidth+columnGap, 0
		} else if !horizontal && y > 0 && y+height > heightLimit {
			x, y, columnWidth = x+columnWidth+columnGap, 0, 0
		}
		placementX, placementY := x, y
		if compact {
			shape, err := b.connectedBlockShape(ctx, row, block)
			if err != nil {
				return err
			}
			if horizontal {
				placementX, placementY, err = packing.placeHorizontal(ctx, shape, minimum, maximum, x, y, heightLimit)
			} else {
				placementX, placementY, err = packing.place(ctx, shape, minimum, maximum, x, y, heightLimit)
			}
			if err != nil {
				return err
			}
		}
		shift(block, placementX-minimum.x, placementY-minimum.y, 0)
		b.previewCheckpoint("layout")
		if horizontal {
			if placementY+height > y {
				x = math.Max(x, placementX+width+rowGap)
				columnWidth = math.Max(columnWidth, placementY-y+height)
			}
		} else {
			y = math.Max(y, placementY+height+rowGap)
			columnWidth = math.Max(columnWidth, placementX-x+width)
		}
		if len(b.all) > blueprint.MaxBuildingCount {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器和过滤端口超过 30 万建筑，请拆分目标"}
		}
	}
	if b.denseMaterialNetworks != nil {
		denseUnits, err = b.prepareDenseMaterialNetworkGeometry(ctx, denseUnits)
		if err != nil {
			return err
		}
	}
	if b.denseProducerSplits != nil {
		denseUnits, err = b.prepareDenseProducerSplitters(ctx, denseUnits)
		if err != nil {
			return err
		}
	}
	denseUnits, err = b.prepareByproductPriorityInputs(ctx, denseUnits)
	if err != nil {
		return err
	}
	if b.directBoundaryUnits != nil {
		if !b.denseSpatial {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "黑盒边界设施必须参与全场密铺占位"}
		}
		for _, route := range b.transportRoutes {
			if route.to != nil {
				route.end = route.to.inputEndpoint(route.edge.ItemID, route.inputRole)
				if route.priorityInput != nil {
					route.end = route.priorityInput
				}
			}
		}
		boundaryUnits, err := b.directBoundaryUnits(ctx, rows)
		if err != nil {
			return err
		}
		denseUnits = append(denseUnits, boundaryUnits...)
	}
	if b.directBoundaryAttach != nil {
		denseUnits, err = b.directBoundaryAttach(denseUnits)
		if err != nil {
			return err
		}
	}
	if b.denseSpatial {
		// 先删除不被机器、分支或入口使用的尾带，防止不存在于最终产物的长尾参与全场占位。
		if err := b.trimConnectedTails(); err != nil {
			return err
		}
		retained := map[*placed]bool{}
		for _, entity := range b.all {
			retained[entity] = true
		}
		for i := range denseUnits {
			entities := make([]*placed, 0, len(denseUnits[i].entities))
			for _, entity := range denseUnits[i].entities {
				if retained[entity] {
					entities = append(entities, entity)
				}
			}
			denseUnits[i].entities = entities
		}
		denseUnits, err = b.prepareDenseBentRows(ctx, denseUnits)
		if err != nil {
			return err
		}
		if b.directBoundaryUnits != nil && (b.directBoundaryExtentScale > 0 || len(b.byproductInputs) > 0) {
			// 包络只用于估算有限搜索空间；实际摆放仍允许这些独立实体轮廓交错，不占据整块矩形。
			area, shortest := 0., 0.
			for _, unit := range denseUnits {
				lo, hi := bounds(unit.entities)
				width, height := math.Ceil(hi.x)-math.Floor(lo.x)+1, math.Ceil(hi.y)-math.Floor(lo.y)+1
				area += width * height
				shortest = math.Max(shortest, math.Min(width, height))
			}
			scale := b.directBoundaryExtentScale
			if len(b.byproductInputs) > 0 {
				scale = math.Max(scale, 1.2)
			}
			heightLimit = math.Max(heightLimit, math.Max(shortest+4, math.Ceil(math.Sqrt(area)*scale)))
			if b.candidateExtentProbe != nil {
				b.candidateExtentProbe.area, b.candidateExtentProbe.shortest = area, shortest
				b.candidateExtentProbe.byproducts = len(b.byproductInputs) > 0
				return errCandidateExtentReady
			}
		}
		if b.denseStreetCandidate && len(b.denseStreetRoutes) > 0 {
			heightLimit, err = b.denseConnectedStreetExtent(ctx, denseUnits, heightLimit)
			if err != nil {
				return err
			}
		}
		if b.denseUnitOrder != nil {
			denseUnits, err = b.denseUnitOrder(ctx, denseUnits)
			if err != nil {
				return err
			}
		}
		if err := b.placeDenseSpatialUnits(ctx, denseUnits, heightLimit); err != nil {
			return err
		}
		// 平面全场落位已经真实提交，立即展示排布；后续球面补电与装箱不能让画面仍停在原点机器行。
		b.previewCheckpoint("layout")
		if b.denseStreetCandidate && b.denseStreetRotation != 0 {
			rotatePlacedQuarter(b.all, b.denseStreetRotation)
			for _, row := range rows {
				row.quarterTurns = (row.quarterTurns + b.denseStreetRotation) % 4
			}
			low, _ := bounds(b.all)
			shift(b.all, -math.Floor(low.x), -math.Floor(low.y), 0)
		}
		if err := b.placePolarUnitFrames(ctx, denseUnits); err != nil {
			return err
		}
	}
	// 规范边全部接好后已成为整体物理图，不能再由旧的逐产线块装箱移动并拉断跨块路线。
	b.surfaceBlocks = nil
	b.zones = planZones(req.Plan)
	for _, route := range b.transportRoutes {
		if route.to != nil {
			if b.policy != nil && b.policy.connectedRows.keepAssignedRouteEnd && route.end != nil {
				continue
			}
			route.end = route.to.inputEndpoint(route.edge.ItemID, route.inputRole)
			if route.priorityInput != nil {
				route.end = route.priorityInput
			}
			if route.end == nil {
				return transportAllocationError(route.edge, route.rate, "配方输入或增产剂没有已验证实体端口")
			}
		}
	}
	// 全场落位与原生坐标变换已经完成；在开始耗时布线前发布完整排布，避免画面一直停留在逐行装配阶段。
	b.previewCheckpoint("layout")
	return nil
}

// attachFilteredOutputs 职责：从逐产物独立出货带按消费者流量拆出过滤支路；使用：机器行平移前；边界：不同产物不共享机器干线，同物品各份额仍合并核算带容量。
func (b *builder) attachFilteredOutputs(row *transportRow) error {
	if handled, err := b.planDenseProducerSplit(row); handled {
		return err
	}
	// output 职责：保存同一产物的独立机器带和消费者分支；使用：物理轨位排序；边界：不同产物绝不借用同一机器输出干线。
	type output struct {
		belts  []*placed
		routes []*transportRoute
	}
	outputs := []output{}
	cursor := math.Inf(1)
	for _, item := range row.recipe.Results {
		belts := row.line.outputForItem(item)
		if len(belts) == 0 {
			return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行产物%d没有独立输出带", item)}
		}
		group := output{belts: belts}
		for _, route := range row.outputs {
			if route.edge.ItemID == item {
				group.routes = append(group.routes, route)
			}
		}
		outputs = append(outputs, group)
		cursor = math.Min(cursor, belts[0].a.x-1)
	}
	// 外侧轨先分支，内侧轨沿西侧越过外轨尾端后再向外取料，避免一格过滤带
	// 落在邻近产物干线上；逻辑route顺序不变，仅调整各item的实际引出位置。
	machineY := row.line.machines[0].a.y
	sort.SliceStable(outputs, func(i, j int) bool {
		isRecycle := func(group output) bool {
			for _, recycle := range row.recycles {
				if group.belts[0] == row.line.outputForItem(recycle.item)[0] {
					return true
				}
			}
			return false
		}
		if first, second := isRecycle(outputs[i]), isRecycle(outputs[j]); first != second {
			return !first
		}
		return math.Abs(outputs[i].belts[0].a.y-machineY) > math.Abs(outputs[j].belts[0].a.y-machineY)
	})
	poles := []*placed{}
	for _, group := range outputs {
		groupBegin := len(b.all)
		if len(group.routes) == 0 {
			continue
		}
		trunk := group.belts[0]
		shared := append([]*placed{}, group.belts...)
		recycled := false
		for _, recycle := range row.recycles {
			if recycle.item != group.routes[0].edge.ItemID {
				continue
			}
			if len(recycle.surplus) == 0 {
				return transportAllocationError(group.routes[0].edge, group.routes[0].rate, "回流物品没有真实净产物出口")
			}
			trunk = recycle.surplus[len(recycle.surplus)-1]
			shared = append(shared, recycle.source...)
			shared = append(shared, recycle.surplus...)
			cursor = math.Min(cursor, trunk.a.x-1)
			recycled = true
		}
		if b.setting.EnableIntermediateBuffer {
			buffer, err := b.attachProductionBuffer(row, group.routes[0].edge.ItemID, trunk, cursor)
			if err != nil {
				return err
			}
			shared = append(shared, buffer.main...)
			group.belts = append([]*placed(nil), buffer.outlet...)
			for i, j := 0, len(group.belts)-1; i < j; i, j = i+1, j-1 {
				group.belts[i], group.belts[j] = group.belts[j], group.belts[i]
			}
			trunk = group.belts[0]
			cursor = trunk.a.x - 1
		}
		if b.denseSpatial && (!recycled || b.policy != nil && b.policy.density == densityDense) && len(group.routes) == 1 && productionSprayItem(row.info) != group.routes[0].edge.ItemID {
			route := group.routes[0]
			route.start, route.path = trunk, []*placed{trunk}
			route.sharedPath = append([]*placed(nil), group.belts[1:]...)
			if row.buffers[group.routes[0].edge.ItemID] != nil {
				route.sharedPath = append([]*placed(nil), shared[:len(shared)-1]...)
			}
			if recycled {
				// 优先门已将回流与纯物品净产出分开；单一去向直接取净出口，保留全部共同毛流段用于容量核验。
				route.sharedPath = append([]*placed(nil), shared[:len(shared)-1]...)
			}
			trunk.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
			if route.to == nil {
				route.boundary = trunk
			}
			if buffer := row.buffers[route.edge.ItemID]; buffer != nil {
				buffer.entities = append([]*placed(nil), b.all[groupBegin:]...)
			}
			continue
		}
		if !recycled && b.usesDensePairedOutputs(row) {
			var err error
			cursor, err = b.attachDensePairedOutputs(row, group.belts, group.routes, cursor)
			if err != nil {
				return err
			}
			continue
		}
		direction := 1.
		if trunk.a.y < machineY {
			direction = -1
		}
		if recycled {
			// 回收口沿原出货轨向西，净产品必须从优先门另一侧向机器侧
			// 分支，不能把过滤支带重新放到西向回流带所在的同一轨。
			direction = -direction
		} else {
			// 多产物中的回流轨仍占用真实格位，即使净流为零、没有规范外送路线。
			// 先避开另一产物的机器带，再建立净产品过滤支路，不能等最终碰撞审计才发现交叉。
			for _, other := range outputs {
				if other.belts[0] != group.belts[0] && math.Abs(other.belts[0].a.y-(trunk.a.y+direction)) < 1e-6 {
					direction = -direction
					break
				}
			}
		}
		for _, input := range row.allInputEndpoints() {
			if direction*(input.a.y-trunk.a.y) > 0 {
				// 产物位于同侧原料内轨时，先在原料接驳西端外留一格净空再分支；
				// 不能把(-1,y±1)过滤带放到之后才延伸的原料带末格上。
				cursor = math.Min(cursor, input.a.x-2)
			}
		}
		// 前一个产物的过滤干线更短；当前干线逐格延长到独立取料区，不能跨格接线。
		for trunk.a.x-1 > cursor {
			main := b.make(2000+b.setting.BeltLv, trunk.a.x-1, trunk.a.y, trunk.a.z)
			connect(trunk, main)
			trunk = main
			shared = append(shared, main)
		}
		for _, route := range group.routes {
			capacity := inserterCapacityPerMinute(b.setting.InserterLv, 1)
			arms := int(math.Ceil((route.rate - 1e-8) / capacity))
			arms = max(1, arms)
			// 每个过滤器有自己的抓取点；输入共享带的总流量另在拆行阶段核算。
			branch := []*placed{}
			for i := 0; i < arms; i++ {
				main := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y, trunk.a.z)
				connect(trunk, main)
				trunk = main
				shared = append(shared, main)
				pickup := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y+direction, trunk.a.z)
				if len(branch) > 0 {
					connect(branch[len(branch)-1], pickup)
				}
				branch = append(branch, pickup)
				arm, err := b.beltTransfer(main, pickup, b.setting.InserterLv, route.edge.ItemID)
				if err != nil {
					return err
				}
				route.filters = append(route.filters, arm)
				cursor--
			}
			// 抓取行末另留一格作为寻路端点，避免路由与分拣器本体争用同一个格。
			end := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y+direction, trunk.a.z)
			connect(branch[len(branch)-1], end)
			branch = append(branch, end)
			route.start, route.path = end, branch
			route.sharedPath = append([]*placed{}, shared...)
			end.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
			if route.to == nil {
				route.boundary = end
			}
			if *b.setting.NeedPower {
				position := vector{branch[0].a.x, branch[0].a.y + 2*direction, 0}
				if len(poles) == 0 || math.Hypot(poles[len(poles)-1].a.x-position.x, poles[len(poles)-1].a.y-position.y) >= 6 {
					poles = append(poles, b.make(2201, position.x, position.y, 0))
				}
			}
			// 一格隔离两条不同物品/消费者支路，不能直接把相邻过滤行合并；无用的最终尾带在整体落位后统一裁剪。
			for i := 0; i < 2; i++ {
				main := b.make(2000+b.setting.BeltLv, cursor, trunk.a.y, trunk.a.z)
				connect(trunk, main)
				trunk = main
				shared = append(shared, main)
				cursor--
			}
		}
		if buffer := row.buffers[group.routes[0].edge.ItemID]; buffer != nil {
			buffer.entities = append([]*placed(nil), b.all[groupBegin:]...)
		}
	}
	return nil
}

// connectedBlockShape 职责：从当前机器行生成可平移的规则轮廓；使用：局部装配完成、全局接线之前；边界：未知碰撞目录或非法分拣器端点必须失败，不通过粗矩形缩小几何。
func (b *builder) connectedBlockShape(ctx context.Context, row *transportRow, block []*placed) (packingContour, error) {
	out := packingContour{shapes: []packingShape{}}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return out, err
	}
	encoding := newPlanarRuleEncoding()
	world := func(v vector) placement.Vector {
		return placement.Vector{v.x * planarGridLength, v.z*1.3333333 + .2, v.y * planarGridLength}
	}
	for _, p := range block {
		if ctx.Err() != nil {
			return out, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "机器行轮廓登记已取消或超时"}
		}
		if surfaceIsBelt(p.record.ItemID) {
			first, second := world(p.a), world(p.a)
			if p.output != nil && surfaceIsBelt(p.output.record.ItemID) {
				second = world(p.output.a)
			}
			first[1] += .2
			second[1] += .2
			out.shapes = append(out.shapes, beltPackingShape(first, second))
			continue
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			continue
		}
		shapes, err := splitterClearanceShapes(catalog, p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return out, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for shapeIndex, shape := range shapes {
			if b.setting.EnableIntermediateBuffer && (p.record.ItemID == 2101 || p.record.ItemID == 2106) {
				// 缓存只在装箱阶段增加少量球面收缩余量；实际碰撞盒和最终验收仍读取原型。
				shape.Extents[0] += .25
				shape.Extents[2] += .25
			}
			position := world(p.a)
			box, err := encoding.colliderBox(vector{position[0], position[1], position[2]}, p.record.Yaw, shape)
			if err != nil {
				return out, err
			}
			kind := "static"
			if p.record.ItemID == 2020 && shapeIndex == len(shapes)-1 {
				// 原生static与臂仍可互免；额外选择包络独立保留真实臂盒净空。
				kind = "splitter-selection"
			}
			out.shapes = append(out.shapes, boxPackingShape(kind, box))
		}
		if surfaceIsPowerNode(p.record.ItemID) {
			point := world(p.a)
			out.shapes = append(out.shapes, packingShape{kind: "power", item: p.record.ItemID, span: surfaceBeltSpan{ends: [2]placement.Vector{point, point}}, low: point.Sub(placement.Vector{12, 12, 12}), high: point.Add(placement.Vector{12, 12, 12})})
		}
	}
	local := &builder{all: block}
	records, _, _, err := local.records()
	if err != nil {
		return out, err
	}
	boxes, err := encoding.surfaceInserterBoxes(ctx, records)
	if err != nil {
		return out, err
	}
	for _, box := range boxes {
		out.shapes = append(out.shapes, boxPackingShape("inserter", box.Geometry))
	}
	ports := row.allInputEndpoints()
	for _, route := range row.outputs {
		if route.to != nil {
			ports = append(ports, route.start)
		}
	}
	for _, recycle := range row.recycles {
		ports = append(ports, recycle.route.start, recycle.route.end)
	}
	for _, port := range ports {
		point := world(vector{port.a.x - 1, port.a.y, port.a.z})
		box := placement.Box{Center: point.Add(placement.Vector{0, transportMaxHeight * 1.3333333 / 2, 0}), HalfSize: placement.Vector{.31, transportMaxHeight*1.3333333/2 + .3, .31}, Axes: [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
		out.shapes = append(out.shapes, boxPackingShape("port", box))
	}
	return out, nil
}
