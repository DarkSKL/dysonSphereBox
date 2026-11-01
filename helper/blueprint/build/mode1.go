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

// generateConnected 职责：在固定预算内选择最小面积可行候选；使用：直连带主链；边界：候选必须通过最终数值、连通和编码检查，不以矩形面积替代可行性。
func (b *builder) generateConnected(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	limits := []float64{math.Max(60, b.setting.BlockWidthLimit), math.Max(100, b.setting.BlockWidthLimit*1.5), math.Max(160, b.setting.BlockWidthLimit*2)}
	layouts := len(limits) * 2
	budget := layouts * 2
	var best *Result
	var lastError error
	evaluated, rejected := 0, 0
	for candidateIndex := 0; candidateIndex < budget; candidateIndex++ {
		// 就近入口改变了前序路径占位；只有整组均不可行时才独立重建旧入口候选，
		// 避免线路节省变成生成兼容性回退，也不在常规成功路径上翻倍装配开销。
		if candidateIndex == layouts && best != nil {
			break
		}
		legacyInputs := candidateIndex >= layouts
		layoutIndex := candidateIndex % layouts
		limit := limits[candidateIndex%len(limits)]
		compact := layoutIndex >= len(limits)
		if ctx.Err() != nil {
			return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成已取消或超过 4 分钟"}
		}
		evaluated++
		inputKind := "就近原料入口"
		if legacyInputs {
			inputKind = "兼容原料入口"
		}
		reportProgress(ctx, "flow-layout", fmt.Sprintf("正在检查直连带布局候选 %d/%d（%s）", evaluated, budget, inputKind), evaluated, budget)
		candidate, err := newBuilder(b.setting)
		if err != nil {
			return Result{}, err
		}
		candidate.flowGraph = b.flowGraph
		candidate.connectedCompactBelts = true
		candidate.connectedLegacyInputs = legacyInputs
		candidate.generationCtx = ctx
		candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
		candidate.logistics.Metrics.CandidateBudget = budget
		if err = candidate.assembleConnectedVariant(ctx, req, dataset, limit, compact); err != nil {
			lastError, rejected = fmt.Errorf("直连候选%d/%d（高度%.0f、轮廓%t、%s）：%w", evaluated, budget, limit, compact, inputKind, err), rejected+1
			continue
		}
		out, err := candidate.finish(ctx, req, dataset)
		if err != nil {
			lastError, rejected = fmt.Errorf("直连候选%d/%d（高度%.0f、轮廓%t、%s）：%w", evaluated, budget, limit, compact, inputKind, err), rejected+1
			continue
		}
		if out.LogisticsSummary == nil {
			lastError, rejected = &blueprint.Error{Code: "GEN_E_FLOW", Message: "直连候选缺少物流摘要"}, rejected+1
			continue
		}
		if best == nil || betterTransportCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best = &out
		}
	}
	if ctx.Err() != nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成已取消或超过 4 分钟"}
	}
	if best == nil {
		if lastError == nil {
			lastError = &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "没有通过容量、路线和碰撞检查的布局候选"}
		}
		return Result{}, lastError
	}
	if best.LogisticsSummary == nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_FLOW", Message: "直连候选缺少物流摘要"}
	}
	best.LogisticsSummary.Metrics.EvaluatedCandidates = evaluated
	best.LogisticsSummary.Metrics.RejectedCandidates = rejected
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "bounded-layout-search", Status: "passed", Message: fmt.Sprintf("最多 %d 个候选：先比较 %d 个就近入口矩形/轮廓布局，仅在全部不可行时独立重建兼容入口布局；评估 %d、拒绝 %d，按面积和内部带长选择可行方案，不代表数学全局最优", budget, layouts, evaluated, rejected)})
	return *best, nil
}

// betterTransportCandidate 职责：按明确且稳定的空间目标比较可行候选；使用：候选选择；边界：不可行候选不得参与此比较。
func betterTransportCandidate(a, b LayoutMetrics) bool {
	if a.EnvelopeArea != b.EnvelopeArea {
		return a.EnvelopeArea < b.EnvelopeArea
	}
	if a.InternalBeltLength != b.InternalBeltLength {
		return a.InternalBeltLength < b.InternalBeltLength
	}
	if a.CrossAreaConnections != b.CrossAreaConnections {
		return a.CrossAreaConnections < b.CrossAreaConnections
	}
	return a.ExternalInputCount < b.ExternalInputCount
}

// assembleConnected 职责：按上游到下游顺序放置真实端口并路由全部 lane；使用：每个有界候选；边界：SCC 回边不被删除，跨行连接后禁止按独立机器块重排。
func (b *builder) assembleConnected(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64) error {
	return b.assembleConnectedVariant(ctx, req, dataset, heightLimit, false)
}

// assembleConnectedVariant 职责：在不改变机器、端口和规范 lane 的前提下装配一种空间候选；使用：基线与轮廓靠拢对比；边界：所有候选仍须完成寻路和 finish 的严格规则检查。
func (b *builder) assembleConnectedVariant(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64, compact bool) error {
	if err := b.prepareConnectedRows(ctx, req, dataset, heightLimit, compact); err != nil {
		return err
	}
	if err := b.routeTransport(ctx); err != nil {
		return err
	}
	if b.connectedCompactBelts {
		return b.trimConnectedTails()
	}
	return nil
}

// prepareConnectedRows 职责：装配统一机器行和逐物品过滤端口但不决定全局运输拓扑；使用：直连带与可扩展混带环路共用局部几何；边界：调用方必须继续完成全图路由与最终规则，不能直接编码半成品。
func (b *builder) prepareConnectedRows(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64, compact bool) error {
	b.generationCtx = ctx
	rows, err := b.planTransportRows(ctx, req, dataset)
	if err != nil {
		return err
	}
	x, y, columnWidth := 0., 0., 0.
	packing := &connectedPacking{cells: map[[3]int][]int{}}
	for i, row := range rows {
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
				row.inputs[item] = links[len(links)-1]
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
			stubLength := 5
			if req.Mode == 6 {
				// 混带环路的远端接驳为每份供给保留独立汇入格，避免后续供给借用上一条长路线的源头。
				count := 0
				for _, route := range b.transportRoutes {
					if route.to == row && route.edge.ItemID == item {
						count++
					}
				}
				stubLength = max(stubLength, count*3+2)
			}
			points := []vector{}
			for x := endpoint.a.x - float64(stubLength); x < endpoint.a.x; x++ {
				points = append(points, vector{x, endpoint.a.y, endpoint.a.z})
			}
			stub := b.byPath(points)
			connect(stub[len(stub)-1], endpoint)
			row.inputStubs[item] = stub
			row.inputs[item] = stub[0]
			inputIndex := 0
			for _, route := range b.transportRoutes {
				if route.to == row && route.edge.ItemID == item {
					route.end = stub[0]
					if req.Mode == 6 {
						route.end = stub[inputIndex*3+1]
						inputIndex++
					}
				}
			}
		}
		if err := b.prepareMachineRecycles(row); err != nil {
			return err
		}
		if req.Mode == 6 {
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
		// 接入干线属于机器行真实占地，必须在装箱前生成，否则后补的西侧带会侵入上一列。
		block := b.all[begin:]
		minimum, maximum := bounds(block)
		width, height := math.Ceil(maximum.x-minimum.x)+1, math.Ceil(maximum.y-minimum.y)+1
		// 机器行的带端已在块内预留，块间只保留一格最小走廊；
		// 路由占位和碰撞预检会拒绝真正不可行的候选，不再用固定大留白
		// 把可用面积浪费在每一行之间。
		const rowGap = 1.0
		const columnGap = 1.0
		if y > 0 && y+height > heightLimit {
			x, y, columnWidth = x+columnWidth+columnGap, 0, 0
		}
		placementX, placementY := x, y
		if compact {
			shape, err := b.connectedBlockShape(ctx, row, block)
			if err != nil {
				return err
			}
			placementX, placementY, err = packing.place(ctx, shape, minimum, maximum, x, y, heightLimit)
			if err != nil {
				return err
			}
		}
		shift(block, placementX-minimum.x, placementY-minimum.y, 0)
		y = math.Max(y, placementY+height+rowGap)
		columnWidth = math.Max(columnWidth, placementX-x+width)
		if len(b.all) > blueprint.MaxBuildingCount {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器和过滤端口超过 20 万建筑，请拆分目标"}
		}
	}
	// 规范边全部接好后已成为整体物理图，不能再由旧的逐产线块装箱移动并拉断跨块路线。
	b.surfaceBlocks = nil
	b.zones = planZones(req.Plan)
	for _, route := range b.transportRoutes {
		if route.to != nil {
			if req.Mode == 6 && route.end != nil {
				continue
			}
			route.end = route.to.inputs[route.edge.ItemID]
			if route.end == nil {
				return transportAllocationError(route.edge, route.rate, "配方输入或增产剂没有已验证实体端口")
			}
		}
	}
	return nil
}

// attachFilteredOutputs 职责：从逐产物独立出货带按消费者流量拆出过滤支路；使用：机器行平移前；边界：不同产物不共享机器干线，同物品各份额仍合并核算带容量。
func (b *builder) attachFilteredOutputs(row *transportRow) error {
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
		for _, input := range row.inputs {
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
	}
	return nil
}

// connectedPackingShape 职责：保存候选机器行的真实静态盒、修正分拣器盒、带段和端口通道；使用：轮廓靠拢宽窄相位；边界：不裁小游戏碰撞体，不把包围盒重叠直接当作实体冲突。
type connectedPackingShape struct {
	// kind 职责：区分静态、分拣器、带、供电间距和端口净空；使用：应用不同碰撞规则；边界：未知类别不被当作安全。
	kind string
	// box 职责：保存静态/动态盒或端口预留柱；使用：窄相位；边界：带段使用 span。
	box placement.Box
	// span 职责：保存真实有向带段两端；使用：带交叉和建筑占用；边界：不会在候选阶段合并不同物品路线。
	span surfaceBeltSpan
	// item 职责：保存电力节点原型；使用：调用版本化节点间距规则；边界：其它类别只作诊断。
	item int16
	// low/high 职责：缓存米制包围盒；使用：空间哈希；边界：只用作宽相位。
	low, high placement.Vector
}

// connectedBlockContour 职责：保存一个尚未全局接线的局部产线轮廓；使用：每行增量装箱；边界：所有建筑指针由原 builder 持有，不复制机器。
type connectedBlockContour struct {
	shapes []connectedPackingShape
}

// connectedPacking 职责：维护已放置产线的真实几何空间哈希；使用：有界轮廓靠拢；边界：最终物流路由和球面规则仍由统一出口验证。
type connectedPacking struct {
	shapes []connectedPackingShape
	cells  map[[3]int][]int
	high   vector
	ready  bool
}

// connectedBlockShape 职责：从当前机器行生成可平移的规则轮廓；使用：局部装配完成、全局接线之前；边界：未知碰撞目录或非法分拣器端点必须失败，不通过粗矩形缩小几何。
func (b *builder) connectedBlockShape(ctx context.Context, row *transportRow, block []*placed) (connectedBlockContour, error) {
	out := connectedBlockContour{shapes: []connectedPackingShape{}}
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
			out.shapes = append(out.shapes, connectedBeltShape(first, second))
			continue
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			continue
		}
		shapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return out, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for _, shape := range shapes {
			position := world(p.a)
			box, err := encoding.colliderBox(vector{position[0], position[1], position[2]}, p.record.Yaw, shape)
			if err != nil {
				return out, err
			}
			out.shapes = append(out.shapes, connectedBoxShape("static", box))
		}
		if surfaceIsPowerNode(p.record.ItemID) {
			point := world(p.a)
			out.shapes = append(out.shapes, connectedPackingShape{kind: "power", item: p.record.ItemID, span: surfaceBeltSpan{ends: [2]placement.Vector{point, point}}, low: point.Sub(placement.Vector{12, 12, 12}), high: point.Add(placement.Vector{12, 12, 12})})
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
		out.shapes = append(out.shapes, connectedBoxShape("inserter", box.Geometry))
	}
	ports := []*placed{}
	items := make([]int, 0, len(row.inputs))
	for item := range row.inputs {
		items = append(items, item)
	}
	sort.Ints(items)
	for _, item := range items {
		ports = append(ports, row.inputs[item])
	}
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
		out.shapes = append(out.shapes, connectedBoxShape("port", box))
	}
	return out, nil
}

// connectedBoxShape 职责：缓存有效定向盒的宽相位范围；使用：轮廓建模；边界：不修改半尺寸或旋转。
func connectedBoxShape(kind string, box placement.Box) connectedPackingShape {
	low, high := box.Bounds()
	return connectedPackingShape{kind: kind, box: box, low: low, high: high}
}

// connectedBeltShape 职责：用最终带半径保存实际线段；使用：带与带/建筑的跨块避碰；边界：末端可退化成球，不以无连接接触作为合法汇流。
func connectedBeltShape(first, second placement.Vector) connectedPackingShape {
	low, high := first, first
	for i := range low {
		low[i] = math.Min(first[i], second[i]) - .23
		high[i] = math.Max(first[i], second[i]) + .23
	}
	return connectedPackingShape{kind: "belt", span: surfaceBeltSpan{ends: [2]placement.Vector{first, second}}, low: low, high: high}
}

// shiftedConnectedShape 职责：产生候选平移后的几何视图；使用：试探位置而不移动真实建筑；边界：只做刚体平移，不改变连接和朝向。
func shiftedConnectedShape(shape connectedPackingShape, dx, dy float64) connectedPackingShape {
	delta := placement.Vector{dx * planarGridLength, 0, dy * planarGridLength}
	shape.box.Center = shape.box.Center.Add(delta)
	shape.span.ends[0] = shape.span.ends[0].Add(delta)
	shape.span.ends[1] = shape.span.ends[1].Add(delta)
	shape.low, shape.high = shape.low.Add(delta), shape.high.Add(delta)
	return shape
}

// connectedShapesConflict 职责：按静态/动态/带/电力规则区分轮廓冲突；使用：空间哈希窄相位；边界：最终检查的连接豁免只用于块内，跨块不允许借用别的物品端点。
func connectedShapesConflict(a, c connectedPackingShape) bool {
	known := func(kind string) bool {
		return kind == "static" || kind == "inserter" || kind == "belt" || kind == "power" || kind == "port"
	}
	if !known(a.kind) || !known(c.kind) {
		return true
	}
	if a.kind == "power" || c.kind == "power" {
		if a.kind != "power" || c.kind != "power" {
			return false
		}
		delta := a.span.ends[0].Sub(c.span.ends[0])
		return delta.Dot(delta) < float64(surfacePowerDistanceLimit(a.item, c.item))
	}
	for i := range a.low {
		if a.high[i] < c.low[i] || c.high[i] < a.low[i] {
			return false
		}
	}
	if a.kind == "inserter" || c.kind == "inserter" {
		if a.kind == "inserter" && c.kind == "inserter" {
			return a.box.Overlaps(c.box)
		}
		if a.kind == "port" || c.kind == "port" {
			return a.box.Overlaps(c.box)
		}
		return false
	}
	if a.kind == "belt" && c.kind == "belt" {
		return surfaceSpanDistanceSquared(a.span, c.span) < .46*.46-1e-6
	}
	if a.kind == "belt" {
		return c.box.SegmentDistanceSquared(a.span.ends[0], a.span.ends[1]) < .23*.23-1e-6
	}
	if c.kind == "belt" {
		return a.box.SegmentDistanceSquared(c.span.ends[0], c.span.ends[1]) < .23*.23-1e-6
	}
	return a.box.Overlaps(c.box)
}

// connectedShapeCells 职责：遍历米制八格哈希桶；使用：候选轮廓宽相位；边界：调用方提供已核对且有限的原型几何。
func connectedShapeCells(shape connectedPackingShape, visit func([3]int) bool) bool {
	for x := int(math.Floor(shape.low[0] / 8)); x <= int(math.Floor(shape.high[0]/8)); x++ {
		for y := int(math.Floor(shape.low[1] / 8)); y <= int(math.Floor(shape.high[1]/8)); y++ {
			for z := int(math.Floor(shape.low[2] / 8)); z <= int(math.Floor(shape.high[2]/8)); z++ {
				if !visit([3]int{x, y, z}) {
					return false
				}
			}
		}
	}
	return true
}

// fits 职责：查询一条轮廓是否可平移到指定位置；使用：有界候选选择；边界：不把失败候选写入占位索引。
func (packing *connectedPacking) fits(ctx context.Context, contour connectedBlockContour, dx, dy float64) bool {
	for _, source := range contour.shapes {
		if ctx.Err() != nil {
			return false
		}
		shape := shiftedConnectedShape(source, dx, dy)
		seen := map[int]bool{}
		if !connectedShapeCells(shape, func(cell [3]int) bool {
			for _, index := range packing.cells[cell] {
				if !seen[index] {
					seen[index] = true
					if connectedShapesConflict(shape, packing.shapes[index]) {
						return false
					}
				}
			}
			return true
		}) {
			return false
		}
	}
	return true
}

// place 职责：以固定位置预算在矩形基线周边寻找可利用凹槽；使用：每条机器行增量靠拢；边界：只提交真实几何不冲突的最小包络候选，完整路由失败仍由生成候选回退。
func (packing *connectedPacking) place(ctx context.Context, contour connectedBlockContour, low, high vector, shelfX, shelfY, heightLimit float64) (float64, float64, error) {
	width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
	// position 职责：保存整数格候选原点；使用：固定预算和稳定去重；边界：只在本次机器行选择中有效。
	type position struct{ x, y float64 }
	candidates := []position{{shelfX, shelfY}}
	seen := map[position]bool{candidates[0]: true}
	add := func(x, y float64) {
		point := position{math.Floor(x), math.Floor(y)}
		if point.x < shelfX || point.y < 0 || point.y+height > math.Max(heightLimit, height) || seen[point] || len(candidates) >= 128 {
			return
		}
		seen[point] = true
		candidates = append(candidates, point)
	}
	// 当前列内先向下靠拢，再试少量横向错位，以利用过滤区、电杆和
	// 长短机器行形成的凹槽；每个位置用真实轮廓验证，而不是减小 bbox。
	for retreat := 0.; retreat <= 18 && len(candidates) < 128; retreat++ {
		for _, offset := range []float64{0, 1, 2, 3, 4, 6, 8} {
			add(shelfX+offset, shelfY-retreat)
		}
	}
	best, bestScore := position{}, math.Inf(1)
	found := false
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return 0, 0, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "机器行凹槽靠拢已取消或超时"}
		}
		if !packing.fits(ctx, contour, candidate.x-low.x, candidate.y-low.y) {
			continue
		}
		maximumX, maximumY := candidate.x+width, candidate.y+height
		if packing.ready {
			maximumX = math.Max(maximumX, packing.high.x)
			maximumY = math.Max(maximumY, packing.high.y)
		}
		score := maximumX * maximumY
		if !found || score < bestScore || score == bestScore && (candidate.y < best.y || candidate.y == best.y && candidate.x < best.x) {
			best, bestScore, found = candidate, score, true
		}
	}
	if !found {
		return 0, 0, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器行在固定轮廓靠拢预算内没有满足静态、带路、端口与供电净空的位置"}
	}
	dx, dy := best.x-low.x, best.y-low.y
	for _, source := range contour.shapes {
		shape := shiftedConnectedShape(source, dx, dy)
		index := len(packing.shapes)
		packing.shapes = append(packing.shapes, shape)
		connectedShapeCells(shape, func(cell [3]int) bool {
			packing.cells[cell] = append(packing.cells[cell], index)
			return true
		})
	}
	packing.high.x = math.Max(packing.high.x, best.x+width)
	packing.high.y = math.Max(packing.high.y, best.y+height)
	packing.ready = true
	return best.x, best.y, nil
}
