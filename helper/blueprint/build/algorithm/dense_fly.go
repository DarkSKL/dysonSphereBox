package build

// 文件职责：把超密度配送的机器核心和配送设施分开选址，再按真实端口统一布线。
// 使用：mode2 dense 专用装配；边界：不预留产线独占矩形，不改规范物流份额，原料回收与自产剂反馈保留必要局部关系。

import (
	"DysonSphereBox/helper/blueprint/build/algorithm/search"
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// denseFlyWire 职责：关联配送设施真实短尾和待接线端口；使用：全场布线后重建配送证据；边界：临时 transportRoute 不替代规范配送边。
type denseFlyWire struct {
	// endpoint 职责：保留实际配送端点；使用：回写有向接驳；边界：回料链复用自产剂出货端点，不新增净流端点。
	endpoint *flyTransportEndpoint
	// tail 职责：保留设施侧原始短尾；使用：拼接机器侧路线；边界：不能复制带节点。
	tail []*placed
	// route 职责：保存待建立的实际物理路线；使用：借用全场寻路；边界：装配后清除，不作为远程配送规范边发布。
	route *transportRoute
	// recycle 职责：区分自产剂真实回料链；使用：布线后绑定毛流证据；边界：不是外供配送端点。
	recycle *flySelfSpray
}

// generateDenseFlyLayout 职责：比较真正分离机器与设施的矩形密铺候选；使用：模式2超密度入口；边界：失败不退回普通区块布局，所有候选必须通过相同最终检查。
func (b *builder) generateDenseFlyLayout(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	recycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return Result{}, err
	}
	variants := []denseFlyLayoutVariant{{blackbox: true}}
	hasStreet := b.hasDenseFlyStreet(req, dataset)
	if hasStreet {
		variants = append(variants, denseFlyLayoutVariant{blackbox: true, street: true})
	}
	if !recycle {
		variants = append(variants, denseFlyLayoutVariant{})
		if hasStreet {
			variants = append(variants, denseFlyLayoutVariant{street: true})
		}
	}
	return b.generateDenseFlyVariants(ctx, req, dataset, variants)
}

// denseFlyLayoutVariant 职责：声明一次完整配送候选的拓扑和核心策略；使用：公开选择与同条件审计；边界：不同策略必须分别通过全部生成终检。
type denseFlyLayoutVariant struct {
	blackbox  bool
	street    bool
	groups    int
	clearance float64
}

// generateDenseFlyVariants 职责：以相同完整流水线比较显式配送候选；使用：公开生成及独立旧方案对照；边界：不读取先前候选的实体或复用其验证结论。
func (b *builder) generateDenseFlyVariants(ctx context.Context, req Request, dataset *quant.Dataset, variants []denseFlyLayoutVariant) (Result, error) {
	var best *Result
	bestFacilities := math.MaxInt
	var lastError error
	diagnostics := []SearchCandidateDiagnostic{}
	index, rejected := 0, 0
	plannedPreviews := b.planCandidatePreviews(len(variants) * 6)
	for variantIndex := 0; variantIndex < len(variants); variantIndex++ {
		option := variants[variantIndex]
		useStreet := option.street
		clearance := option.clearance
		if clearance == 0 {
			clearance = .04
		}
		passed, assemblyFailures := 0, 0
		type flyEvaluation struct {
			out              Result
			count            int
			parameters       string
			err              error
			diagnostics      []SearchCandidateDiagnostic
			assemblyFailures int
		}
		type flyInput struct {
			scale float64
			order int
		}
		inputs := []flyInput{{.85, 2}, {.85, 3}, {1, 2}, {1, 3}, {1.2, 2}, {1.2, 3}}
		if denseNativeLatitude(req) {
			// 高纬高度预算会让多个尺度收敛到同一排布；保留面积优先的两种次序作为独立候选。
			inputs[4].order, inputs[5].order = 0, 1
		}
		previews := plannedPreviews[variantIndex*6 : (variantIndex+1)*6]
		groupBase := index
		waveErr := search.Waves(ctx, inputs, b.candidateWorkers(ctx, len(inputs)), func(ctx context.Context, localIndex int, input flyInput) (evaluation flyEvaluation) {
			ctx, finishPerformance := beginCandidatePerformance(ctx, "dense-fly", groupBase+localIndex)
			defer func() { finishPerformance(evaluation.err) }()
			scale, order := input.scale, input.order
			index := groupBase + localIndex
			var lastError error
			var diagnostics []SearchCandidateDiagnostic
			assemblyFailures := 0
			defer func() {
				evaluation.err = lastError
				evaluation.diagnostics = diagnostics
				evaluation.assemblyFailures = assemblyFailures
			}()
			if err := b.checkGeneration("配送全场密铺候选"); err != nil {
				lastError = err
				return
			}
			candidate, err := b.newCandidateBuilder()
			if err != nil {
				lastError = err
				return
			}
			if previews[localIndex] != nil {
				candidate.preview = previews[localIndex]
			}
			candidateContext := ctx
			if useStreet {
				candidateContext = context.WithValue(ctx, denseStreetFoldClearanceKey{}, clearance)
			}
			if best != nil {
				candidateContext = context.WithValue(candidateContext, denseFlyFacilityBoundKey{}, denseFlyFacilityBound{count: bestFacilities, aspect: best.LogisticsSummary.Metrics.EnvelopeAspectRatio})
			}
			candidate.generationCtx, candidate.flowGraph = candidateContext, b.flowGraph
			candidate.denseSpatial, candidate.denseSpatialOrder, candidate.flyCompact = true, order, true
			candidate.nativeCapacityReserve = denseNativeLatitude(req) && localIndex >= 4
			candidate.denseStreetGroupLimit = option.groups
			candidate.warnings = append(candidate.warnings, b.warnings...)
			candidate.logistics = newLogisticsSummary(*b.flowGraph, candidate.setting)
			candidate.logistics.Checks = append(candidate.logistics.Checks, b.logistics.Checks...)
			candidate.beginPreview()
			parameters := fmt.Sprintf("detachedFacilities=true;street=%t;streetGroups=%d;streetClearance=%.2f;extentScale=%.2f;order=%d", useStreet, option.groups, clearance, scale, order)
			parameters += fmt.Sprintf(";blackbox=%t", option.blackbox)
			if option.blackbox {
				span := 12
				if order%2 == 1 {
					span = 18
				}
				parameters += fmt.Sprintf(";maxSpan=%d", span)
			}
			stage := "assembly"
			if option.blackbox {
				err = candidate.assembleDenseFlyBlackbox(candidateContext, req, dataset, scale, useStreet)
			} else {
				err = candidate.assembleDenseFlyLayout(candidateContext, req, dataset, scale, useStreet)
			}
			if err == nil {
				stage = "final-validation"
				var result Result
				result, err = candidate.finish(candidateContext, req, dataset)
				if err == nil && result.LogisticsSummary != nil {
					proof := newSearchCandidateDiagnostic("dense-fly-layout", index, parameters, stage, nil)
					m := result.LogisticsSummary.Metrics
					facilities := denseFlyFacilityCount(candidate.all)
					proof.Reason = fmt.Sprintf("完整通过：配送器%d个，包络%.0f格²，长宽比%.3f，真实带%d段/%.3f米", facilities, m.EnvelopeArea, m.EnvelopeAspectRatio, m.BeltCount, m.TotalBeltLengthMeters)
					diagnostics = appendSearchCandidateDiagnostics(diagnostics, proof, result.LogisticsSummary)
					evaluation.out, evaluation.count = result, facilities
					evaluation.parameters = parameters

					return
				}
			}
			if err == nil {
				err = &blueprint.Error{Code: "GEN_E_FLOW", Message: "密铺配送缺少最终物流证据"}
			}
			if denseFlyFacilityDominated(err) {
				stage = "facility-bound"
			} else {
				assemblyFailures++
			}
			diagnostics = appendSearchCandidateDiagnostics(diagnostics, newSearchCandidateDiagnostic("dense-fly-layout", index, parameters, stage, err), candidate.logistics)
			lastError = err
			reportProgress(ctx, "layout-rejected", fmt.Sprintf("配送候选%d（%s）：%v", index+1, parameters, err), index+1, len(inputs))
			candidate.rollbackPreview()

			return
		}, func(_ int, evaluation flyEvaluation) error {
			index++
			diagnostics = append(diagnostics, evaluation.diagnostics...)
			assemblyFailures += evaluation.assemblyFailures
			if evaluation.err != nil {
				lastError = evaluation.err
				rejected++
				return nil
			}
			passed++
			result, facilities := evaluation.out, evaluation.count
			if best == nil || betterDenseFlyCandidate(result.LogisticsSummary.Metrics, facilities, best.LogisticsSummary.Metrics, bestFacilities) {
				result.LogisticsSummary.Checks = append(result.LogisticsSummary.Checks, LogisticsCheck{RuleID: "dense-fly-selected-layout", Status: "passed", Message: evaluation.parameters})
				best, bestFacilities = &result, facilities
			}
			return nil
		})
		if waveErr != nil {
			return Result{}, previewContextError(waveErr)
		}
		if useStreet && !option.blackbox && option.groups == 0 && passed == 0 && assemblyFailures > 0 {
			variants = append(variants, denseFlyLayoutVariant{street: true, groups: 3}, denseFlyLayoutVariant{street: true, groups: 2, clearance: .2}, denseFlyLayoutVariant{street: true, groups: 2, clearance: .4}, denseFlyLayoutVariant{street: true, groups: 2, clearance: .6}, denseFlyLayoutVariant{street: true, groups: 1})
			plannedPreviews = append(plannedPreviews, b.planCandidatePreviews(30)...)
		}
	}
	if err := b.checkGeneration("配送全场密铺提交"); err != nil {
		return Result{}, err
	}
	if best == nil {
		return Result{}, lastError
	}
	best.LogisticsSummary.Metrics.CandidateBudget = index
	best.LogisticsSummary.Metrics.EvaluatedCandidates = index
	best.LogisticsSummary.Metrics.RejectedCandidates = rejected
	best.LogisticsSummary.SearchCandidates = diagnostics
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "dense-fly-facility-priority", Status: "passed", Message: fmt.Sprintf("完整矩形候选优先减少实际配送器，再比较面积与真实带路；当前选择%d个配送器，内部直连黑盒与原可行配送方案均参与比较", bestFacilities)})
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "dense-fly-independent-facilities", Status: "passed", Message: "机器核心与配送箱/臂/短尾在同一矩形占位场独立选址后真实接线；自产剂及副产物回收保留必要反馈关系，不以长尾矩形隔离产线"})
	return *best, nil
}

// denseFlyConstrainedOwners 职责：识别已有原料侧回收关系的生产行；使用：分离设施之前；边界：仅保留回收所需局部关系，其余设施独立选址。
func (b *builder) denseFlyConstrainedOwners() map[int]bool {
	owners := map[string]int{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			owners[node.ID] = node.ProductionIndex
		}
	}
	constrained := map[int]bool{}
	for _, edge := range b.flowGraph.Edges {
		if edge.FlowKind == "leftover" {
			if owner, ok := owners[edge.SourceNodeID]; ok {
				constrained[owner] = true
			}
		}
	}
	return constrained
}

// assembleDenseFlyLayout 职责：建立核心/设施独立集合并全场布线、补电；使用：每个密铺候选；边界：布局完成后不允许旧球面流程逐块重排，失败整候选丢弃。
func (b *builder) assembleDenseFlyLayout(ctx context.Context, req Request, dataset *quant.Dataset, scale float64, streetVariant ...bool) error {
	if recycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset); err != nil {
		return err
	} else if recycle {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "机器内部毛流回收必须使用配送黑盒真实闭环，不能落回旧远程净流端点候选"}
	}
	originalPower := b.setting.NeedPower
	disabled := false
	b.setting.NeedPower = &disabled
	defer func() { b.setting.NeedPower = originalPower }()
	constrained := b.denseFlyConstrainedOwners()
	units := []denseSpatialUnit{}
	wires := []denseFlyWire{}
	streetWorks := []*denseFlyStreetWork{}
	useStreet := len(streetVariant) > 0 && streetVariant[0]
	streetOwners := 0
	// 先创建需要局部反馈的产线，单独完成原料侧回收，随后才与其它实体一起全场排布。
	for _, owner := range b.flowGraph.ProductionOrder {
		if !constrained[owner] {
			continue
		}
		p := req.Plan.Productions[owner]
		recipe, err := datasetRecipeForProduction(dataset, p)
		if err != nil {
			return err
		}
		counts, err := denseFlyLineCounts(p, recipe, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return err
		}
		b.owner = owner
		for _, count := range counts {
			begin := len(b.all)
			if _, err := b.buildLine(p, recipe, count, 2, 0, true); err != nil {
				return err
			}
			b.flyLayoutBlocks = append(b.flyLayoutBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
		}
	}
	if len(b.flyLayoutBlocks) > 0 {
		fullGraph := b.flowGraph
		partial := *fullGraph
		partial.Edges = nil
		owners := map[string]int{}
		for _, node := range fullGraph.Nodes {
			if node.Kind == "production" {
				owners[node.ID] = node.ProductionIndex
			}
		}
		for _, edge := range fullGraph.Edges {
			if owner, ok := owners[edge.SourceNodeID]; ok && constrained[owner] {
				partial.Edges = append(partial.Edges, edge)
			}
		}
		b.flowGraph = &partial
		err := b.prepareFlyLeftovers(ctx, 2)
		b.flowGraph = fullGraph
		if err != nil {
			return err
		}
		for _, group := range b.flyLayoutBlocks {
			units = append(units, denseSpatialUnit{entities: b.all[group.begin:group.end]})
		}
	}
	for _, owner := range b.flowGraph.ProductionOrder {
		if constrained[owner] {
			continue
		}
		if err := b.checkGeneration("配送核心与设施拆分装配"); err != nil {
			return err
		}
		p := req.Plan.Productions[owner]
		recipe, err := datasetRecipeForProduction(dataset, p)
		if err != nil {
			return err
		}
		counts, err := denseFlyLineCounts(p, recipe, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return err
		}
		b.owner = owner
		var street *denseStreet
		var streetRows []*transportRow
		var streetWork *denseFlyStreetWork
		if useStreet && denseFlyStreetEligible(p, recipe, counts, b.setting) {
			var err error
			streetWork, err = b.prepareDenseFlyStreet(ctx, p, recipe, counts, owner)
			if err != nil {
				return err
			}
			street, streetRows = streetWork.assembly.street, streetWork.rows
			units = append(units, streetWork.units...)
			streetWorks = append(streetWorks, streetWork)
			streetOwners++
		}
		for group, count := range counts {
			begin := len(b.all)
			var machineLine *line
			var core *transportRow
			if street != nil {
				machineLine, core = street.groups[group], streetRows[group]
			} else {
				originalAdapter := b.adapter
				coreAdapter := *originalAdapter
				coreAdapter.lineAssembly.useFlyTransport = false
				coreAdapter.lineAssembly.reverseOutputRows = true
				b.adapter = &coreAdapter
				var err error
				machineLine, err = b.buildLine(p, recipe, count, 2, 0, true)
				b.adapter = originalAdapter
				if err != nil {
					return err
				}
				if err := b.coatSupply([]*line{machineLine}); err != nil {
					return err
				}
				core = &transportRow{owner: owner, count: count, info: p, recipe: recipe, line: machineLine, inputs: map[int]*placed{}}
				machineLine.all = b.all[begin:]
				coreUnit := denseSpatialUnit{entities: machineLine.all, row: core}
				units = append(units, coreUnit)
			}
			ports := []flyIndependentPort{}
			sprayItem := productionSprayItem(p)
			selfSpray := machineLine.coaterInput != nil && productionItemRate(p.Inputs, sprayItem) == 0 && productionItemRate(p.Outputs, sprayItem) > 0
			for i, row := range machineLine.inputs {
				core.inputs[recipe.Items[i]] = row[0]
				if streetWork == nil {
					ports = append(ports, flyIndependentPort{item: recipe.Items[i], input: true, link: row[0]})
				}
			}
			if streetWork != nil {
				for _, port := range streetWork.assembly.ports {
					if port.group == group {
						ports = append(ports, flyIndependentPort{item: port.item, input: port.input, link: port.head})
					}
				}
			}
			if machineLine.coaterInput != nil {
				item := productionSprayItem(p)
				if streetWork == nil {
					core.inputs[item] = machineLine.coaterInput
				} else {
					streetWork.byEntity[machineLine.coaterInput].inputs[item] = machineLine.coaterInput
				}
				if !selfSpray {
					ports = append(ports, flyIndependentPort{item: item, input: true, link: machineLine.coaterInput, role: sprayInputRole})
				}
			}
			for i, row := range machineLine.outputs {
				if streetWork == nil {
					ports = append(ports, flyIndependentPort{item: recipe.Results[i], link: row[0]})
				}
			}
			for _, port := range ports {
				rate, err := flyPortRate(p, recipe, port, count)
				if err != nil {
					return err
				}
				returned, net := 0., rate
				if selfSpray && !port.input && port.item == sprayItem {
					var err error
					net, returned, err = flySelfSprayRates(machineLine)
					if err != nil {
						return err
					}
					rate = net + returned
				}
				facilityBegin := len(b.all)
				facilityRate := rate
				portRow := core
				if street != nil {
					portRow = streetWork.byEntity[port.link]
					for _, required := range append(append([]denseStreetPort(nil), streetWork.assembly.ports...), streetWork.assembly.sprayPorts...) {
						if required.group == group && required.head == port.link && required.item == port.item && required.input == port.input {
							facilityRate = math.Max(facilityRate, required.requiredRate)
							break
						}
					}
				}
				endpoint, tail, entities, err := b.makeDenseFlyFacility(port, facilityRate)
				if err != nil {
					return err
				}
				endpoint.rate = rate
				facility := &transportRow{owner: owner, inputs: map[int]*placed{}}
				route := &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("dense-fly:%d:%d", owner, len(wires)), ItemID: port.item, FlowKind: "internal"}, rate: rate}
				if port.input {
					route.from, route.to, route.start, route.end = facility, portRow, tail[0], port.link
					facility.outputs = []*transportRoute{route}
				} else {
					route.from, route.to, route.start, route.end = portRow, facility, port.link, tail[0]
					portRow.outputs = append(portRow.outputs, route)
					facility.inputs[port.item] = tail[0]
				}
				route.path = []*placed{route.start}
				b.transportRoutes = append(b.transportRoutes, route)
				wires = append(wires, denseFlyWire{endpoint: endpoint, tail: tail, route: route})
				if returned > 0 {
					returnHead, arm, err := b.makeDenseFlyReturnArm(endpoint, returned)
					if err != nil {
						return err
					}
					endpoint.rate = net
					recycle := &flySelfSpray{line: machineLine, arm: arm, path: []*placed{returnHead}}
					endpoint.selfSpray = recycle
					returnRoute := &transportRoute{edge: quant.FlowEdge{ID: fmt.Sprintf("dense-fly-return:%d:%d", owner, len(wires)), ItemID: sprayItem, FlowKind: "internal"}, rate: returned, from: facility, to: core, start: returnHead, end: machineLine.coaterInput, path: []*placed{returnHead}}
					if streetWork != nil {
						returnRoute.to = streetWork.byEntity[machineLine.coaterInput]
					}
					facility.outputs = append(facility.outputs, returnRoute)
					b.transportRoutes = append(b.transportRoutes, returnRoute)
					wires = append(wires, denseFlyWire{endpoint: endpoint, route: returnRoute, recycle: recycle})
					entities = b.all[facilityBegin:]
					b.warnings = append(b.warnings, fmt.Sprintf("产线 %d 的自产增产剂 %d 从真实出货箱回到喷涂入口；首次启动仍需预充库存", owner, sprayItem))
				}
				units = append(units, denseSpatialUnit{entities: entities, row: facility})
				b.flyEndpoints = append(b.flyEndpoints, endpoint)
			}
		}
	}
	if err := b.rejectDenseFlyFacilityDominance(ctx); err != nil {
		return err
	}
	if useStreet && streetOwners == 0 {
		return &blueprint.Error{Code: "GEN_E_DENSITY_STREET_SCOPE", Message: "本配送候选没有可用的真实参考机器街道"}
	}
	if err := b.trimDenseFlyOrdinaryTails(streetWorks); err != nil {
		return err
	}
	retained := map[*placed]bool{}
	for _, entity := range b.all {
		retained[entity] = true
	}
	area := 0.
	for i := range units {
		entities := make([]*placed, 0, len(units[i].entities))
		for _, entity := range units[i].entities {
			if retained[entity] {
				entities = append(entities, entity)
			}
		}
		units[i].entities = entities
		lo, hi := bounds(entities)
		area += (math.Ceil(hi.x-lo.x) + 1) * (math.Ceil(hi.y-lo.y) + 1)
	}
	limit := math.Max(16, math.Ceil(math.Sqrt(area)*scale))
	if err := b.placeDenseSpatialUnits(ctx, units, limit); err != nil {
		return err
	}
	if err := b.routeDenseFlyStreets(ctx, streetWorks); err != nil {
		return err
	}
	if err := b.completeDenseFlyStreets(streetWorks); err != nil {
		return err
	}
	for _, wire := range wires {
		path := wire.route.path
		if len(path) < 2 || path[len(path)-1] != wire.route.end {
			return flyEndpointFailure(wire.endpoint, "全场接驳未到达端口")
		}
		if wire.recycle != nil {
			wire.recycle.path = append([]*placed(nil), path...)
			for _, entity := range path {
				entity.owner = wire.endpoint.owner
			}
			continue
		}
		if wire.endpoint.input {
			middle := append([]*placed(nil), path[1:len(path)-1]...)
			slices.Reverse(middle)
			wire.endpoint.belts = append(middle, wire.tail...)
		} else {
			wire.endpoint.belts = append(append([]*placed(nil), path[1:len(path)-1]...), wire.tail...)
		}
		for _, entity := range wire.endpoint.belts {
			entity.owner = wire.endpoint.owner
		}
	}
	b.transportRoutes = nil
	b.logistics.Metrics.InternalBeltLength = b.denseFlyLocalBeltLength() + b.denseFlyAccessoryLength(streetWorks)
	checks := b.logistics.Checks[:0]
	for _, check := range b.logistics.Checks {
		if check.RuleID != "mode1-internal-connectivity" && check.RuleID != "belt-lanes" {
			checks = append(checks, check)
		}
	}
	b.logistics.Checks = checks
	b.setting.NeedPower = originalPower
	if *originalPower {
		// 独立设施密铺同样取消了旧地面长头，所有密铺候选统一使用真实空隙和共享覆盖求解。
		b.connectedGlobalPower = true
		var err error
		if req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar" {
			err = b.completeConnectedSurfacePower(ctx, req)
		} else {
			err = b.completeConnectedPower(ctx)
		}
		if err != nil {
			return err
		}
	}
	b.surfaceBlocks = nil
	b.zoneRanges = nil
	b.zones = planZones(req.Plan)
	if err := b.validateFlyEndpointTopology(2); err != nil {
		return err
	}
	b.previewCheckpoint("routing")
	return nil
}
