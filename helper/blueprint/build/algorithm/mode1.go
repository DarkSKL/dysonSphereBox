package build

// 文件职责：把统一供需图一次性装配为过滤端口、紧凑候选与完整直连带工厂。
// 使用：mode 1 生成主链；边界：任何容量、端口、寻路或最终规则失败都不返回部分蓝图。

import (
	"DysonSphereBox/helper/blueprint/build/algorithm/search"
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	mode1common "DysonSphereBox/helper/blueprint/build/adapters/mode1/common"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/quant"
)

// connectedLayoutVariant 职责：兼容旧生成函数的候选类型别名；使用：mode1 生成流程；边界：候选规则由 mode1/common 所有。
type connectedLayoutVariant struct {
	// LayoutVariant 职责：保留普通候选参数；使用：共同编排；边界：不改变普通模式策略。
	mode1common.LayoutVariant
	// denseOrder 职责：选择密铺排序与距离代价；使用：独立候选；边界：不改变规范物料图。
	denseOrder int
	// denseSpan 职责：约束本候选机器行跨度；使用：容量组内细分；边界：不增加单带容量。
	denseSpan float64
	// refine 职责：启用小数位姿搜索；使用：额外可行候选；边界：不取消整数候选或最终校验。
	refine bool
	// street/streetRotation 职责：选择连续容量段街道并比较整场朝向；使用：独立完整候选；边界：不代替旧全场密铺候选。
	street           bool
	streetRotation   int
	streetGroupLimit int
	// pairedOutputs 职责：标识真实双侧过滤布局；使用：有限多去向候选；边界：旧单侧可行候选继续独立保留。
	pairedOutputs bool
	// capacityFill 职责：选择单带能力优先填满的独立密铺分组；使用：有界布局比较；边界：原均衡候选和请求设置保持不变。
	capacityFill bool
	// producerSplits/splitterModel 职责：选择独立产物分流树及其真实形态；使用：有限几何候选；边界：原过滤候选保留，模型改变必须重新装配并通过完整校验。
	producerSplits bool
	splitterModel  int16
	// bentRows 职责：选择真实短run的L/U折行；使用：独立密铺候选；边界：直线方案保留，不按外包矩形独占折角空间。
	bentRows bool
}

// connectedLayoutVariants 职责：生成有限的矩形/轮廓排版候选；使用：布局测试与生成器扩展；边界：不扩大候选预算，不绕过碰撞和路线规则。
func connectedLayoutVariants(req Request, limits []float64) []connectedLayoutVariant {
	surfaceMode := mode1common.SurfacePlanar
	if req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		surfaceMode = mode1common.SurfaceAutoTropic
	}
	planned := mode1common.Variants(surfaceMode, limits)
	variants := make([]connectedLayoutVariant, 0, len(planned))
	for index, variant := range planned {
		variants = append(variants, connectedLayoutVariant{LayoutVariant: variant, denseOrder: index % 4, denseSpan: []float64{12, 18, 24, 32}[(index/2)%4]})
	}
	if req.Density == densityDense {
		for index := range planned {
			if index%4 >= 2 {
				refined := variants[index]
				refined.refine = true
				variants = append(variants, refined)
			}
		}
		if denseConnectedStreetRequestEligible(req) && len(limits) > 0 {
			for _, groupLimit := range []int{0, 2, -1} {
				for turns := 0; turns < 4; turns++ {
					variants = append(variants, connectedLayoutVariant{LayoutVariant: mode1common.LayoutVariant{Limit: limits[0], Compact: true, GlobalPower: true}, street: true, streetRotation: turns, streetGroupLimit: groupLimit})
				}
			}
		}
	}
	return variants
}

// generateConnected 职责：在固定预算内选择最小面积可行候选；使用：直连带主链；边界：候选必须通过最终数值、连通和编码检查，不以矩形面积替代可行性。
func (b *builder) generateConnected(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	if denseNativeLatitude(req) && req.Plan.MachineCount >= 90 {
		ctx = context.WithValue(ctx, denseMaterialNetworksKey{}, true)
	}
	limits, err := b.layoutHeightCandidates(ctx, req, dataset, true)
	if err != nil {
		return Result{}, err
	}
	variants := connectedLayoutVariants(req, limits)
	removed := 0
	if req.Density == densityDense {
		variants = appendDenseCapacityFillVariants(req, variants)
		variants = appendDenseSplitterVariants(req, variants)
		variants = appendDenseBentVariants(req, limits, variants)
		keys, err := b.denseConnectedStreetPlanKeys(ctx, req, dataset)
		if err != nil {
			return Result{}, err
		}
		capacityKeys, err := b.denseConnectedStreetPlanKeys(ctx, req, dataset, true)
		if err != nil {
			return Result{}, err
		}
		variants, removed = deduplicateDenseStreetVariants(variants, keys, capacityKeys)
		paired, err := b.hasDensePairedOutputGroups(ctx, req, dataset)
		if err != nil {
			return Result{}, err
		}
		if paired {
			original := append([]connectedLayoutVariant{}, variants...)
			for _, variant := range original {
				if variant.street && variant.streetGroupLimit == 0 && !variant.capacityFill {
					variant.pairedOutputs = true
					variants = append(variants, variant)
				}
			}
		}
	}
	result, err := b.generateConnectedVariants(ctx, req, dataset, variants)
	if err == nil && removed > 0 {
		result.LogisticsSummary.Checks = append(result.LogisticsSummary.Checks, LogisticsCheck{RuleID: "dense-equivalent-plan-dedup", Status: "passed", Message: fmt.Sprintf("按实际容量组和相同朝向/布局参数去除%d个完全等价街道候选；保留所有不同计划，不减少真实校验或复用失败证明", removed)})
	}
	return result, err
}

// generateConnectedVariants 职责：用同一完整流水线比较显式候选集合；使用：公开候选及同条件回归；边界：每个候选独立完成装配与终检，不复用其它候选的实体。
func (b *builder) generateConnectedVariants(ctx context.Context, req Request, dataset *quant.Dataset, variants []connectedLayoutVariant) (Result, error) {
	layouts := len(variants)
	budget := layouts * 2
	var best *Result
	var bestKey string
	var lastError error
	var searchCandidates []SearchCandidateDiagnostic
	evaluated, rejected := 0, 0
	stoppedAfterFeasibleWave := false
	type connectedEvaluation struct {
		out         Result
		dense       bool
		key         string
		err         error
		diagnostics []SearchCandidateDiagnostic
	}
	if layouts == 0 {
		return Result{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "没有可执行的布局候选"}
	}
	for group := 0; group < 2; group++ {
		if group == 1 && best != nil {
			break
		}
		indices := make([]int, layouts)
		for index := range indices {
			indices[index] = group*layouts + index
		}
		previews := b.planCandidatePreviews(layouts)
		workers := b.candidateWorkers(ctx, layouts)
		feasibleWave := newConnectedFeasibleWave(ctx, req)
		executionCtx := ctx
		if feasibleWave != nil {
			executionCtx = feasibleWave.ctx
		}
		execute := func(_ context.Context, localIndex int, candidateIndex int) (evaluation connectedEvaluation) {
			ctx := executionCtx
			ctx, finishPerformance := beginCandidatePerformance(ctx, "connected", candidateIndex)
			defer func() { finishPerformance(evaluation.err) }()
			var lastError error
			var searchCandidates []SearchCandidateDiagnostic
			defer func() {
				evaluation.err = lastError
				evaluation.diagnostics = searchCandidates
				if lastError == nil && evaluation.out.LogisticsSummary != nil {
					feasibleWave.complete()
				}
			}()
			legacyInputs := candidateIndex >= layouts
			layoutIndex := candidateIndex % layouts
			limit, compact := variants[layoutIndex].Limit, variants[layoutIndex].Compact
			if ctx.Err() != nil {
				lastError = &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "生成已取消或超过 4 分钟"}
				return
			}
			evaluated := candidateIndex + 1
			inputKind := "就近原料入口"
			if legacyInputs {
				inputKind = "兼容原料入口"
			}
			reportProgress(ctx, "flow-layout", fmt.Sprintf("正在检查直连带布局候选 %d/%d（%s）", evaluated, budget, inputKind), evaluated, budget)
			candidate, err := b.newCandidateBuilder()
			if err != nil {
				lastError = err
				return
			}
			if previews[localIndex] != nil {
				candidate.preview = previews[localIndex]
			}
			candidate.flowGraph = b.flowGraph
			if best != nil {
				objective := best.LogisticsSummary.Metrics
				candidate.connectedBestObjective = &objective
			}
			candidate.connectedCompactBelts = true
			candidate.connectedLegacyInputs = legacyInputs
			candidate.connectedHorizontalPacking = variants[layoutIndex].Horizontal
			candidate.connectedGlobalPower = variants[layoutIndex].GlobalPower
			if req.Density == densityDense {
				candidate.denseSpatial, candidate.denseSpatialOrder = true, variants[layoutIndex].denseOrder
				candidate.denseMaxSpan, candidate.denseRefine = variants[layoutIndex].denseSpan, variants[layoutIndex].refine
				candidate.denseStreetCandidate, candidate.denseStreetRotation = variants[layoutIndex].street, variants[layoutIndex].streetRotation
				candidate.denseStreetGroupLimit = variants[layoutIndex].streetGroupLimit
				candidate.densePairedOutputs = variants[layoutIndex].pairedOutputs
				candidate.denseCapacityFill = req.Mode == 1 && variants[layoutIndex].capacityFill
				if variants[layoutIndex].producerSplits {
					ctx = context.WithValue(ctx, denseProducerSplittersKey{}, true)
				}
				if variants[layoutIndex].splitterModel != 0 {
					ctx = context.WithValue(ctx, splitterGateModelKey{}, variants[layoutIndex].splitterModel)
				}
				if variants[layoutIndex].bentRows {
					ctx = context.WithValue(ctx, denseBentRowsKey{}, true)
				}
				candidate.connectedGlobalPower = req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar"
			}
			candidate.generationCtx = ctx
			candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
			candidate.logistics.Metrics.CandidateBudget = budget
			candidate.beginPreview()
			parameters := fmt.Sprintf("heightLimit=%g;compact=%t;horizontal=%t;globalPower=%t;legacyInputs=%t", limit, compact, variants[layoutIndex].Horizontal, variants[layoutIndex].GlobalPower, legacyInputs)
			if candidate.denseSpatial {
				parameters = fmt.Sprintf("globalDense=true;extent=%g;maxSpan=%g;order=%d;refine=%t;globalPower=%t;legacyInputs=%t", limit, candidate.denseMaxSpan, candidate.denseSpatialOrder, candidate.denseRefine, candidate.connectedGlobalPower, legacyInputs)
				if candidate.denseStreetCandidate {
					parameters += fmt.Sprintf(";continuousStreet=true;streetRotation=%d;streetGroupLimit=%d", candidate.denseStreetRotation, candidate.denseStreetGroupLimit)
				}
				parameters += fmt.Sprintf(";pairedOutputs=%t;capacityFill=%t", candidate.densePairedOutputs, candidate.denseCapacityFill)
				producerSplits, _ := ctx.Value(denseProducerSplittersKey{}).(bool)
				splitterModel, _ := splitterGateModel(ctx)
				parameters += fmt.Sprintf(";producerSplits=%t;splitterModel=%d", producerSplits, splitterModel)
				parameters += fmt.Sprintf(";bentRows=%t", variants[layoutIndex].bentRows)
			}
			if err = candidate.assembleConnectedVariant(ctx, req, dataset, limit, compact); err != nil {
				searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("connected-layout", candidateIndex, parameters, "assembly", err), candidate.logistics)
				lastError = fmt.Errorf("直连候选%d/%d（高度%.0f、轮廓%t、%s）：%w", evaluated, budget, limit, compact, inputKind, err)
				reportProgress(ctx, "layout-rejected", lastError.Error(), evaluated, budget)
				candidate.rollbackPreview()
				return
			}
			out, err := candidate.finish(ctx, req, dataset)
			if err != nil {
				searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("connected-layout", candidateIndex, parameters, "final-validation", err), candidate.logistics)
				lastError = fmt.Errorf("直连候选%d/%d（高度%.0f、轮廓%t、%s）：%w", evaluated, budget, limit, compact, inputKind, err)
				reportProgress(ctx, "layout-rejected", lastError.Error(), evaluated, budget)
				candidate.rollbackPreview()
				return
			}
			if out.LogisticsSummary == nil {
				lastError = &blueprint.Error{Code: "GEN_E_FLOW", Message: "直连候选缺少物流摘要"}
				searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("connected-layout", candidateIndex, parameters, "final-validation", lastError), candidate.logistics)
				candidate.rollbackPreview()
				return
			}
			parameters += fmt.Sprintf(";area=%.3f;beltMeters=%.3f;externalInputs=%d", out.LogisticsSummary.Metrics.EnvelopeArea, out.LogisticsSummary.Metrics.TotalBeltLengthMeters, out.LogisticsSummary.Metrics.ExternalInputCount)
			diagnostic := newSearchCandidateDiagnostic("connected-layout", candidateIndex, parameters, "final-validation", nil)
			searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, diagnostic, out.LogisticsSummary)

			evaluation.out = out
			evaluation.dense = candidate.denseSpatial
			evaluation.key = diagnostic.Key
			return
		}
		reduce := func(_ int, evaluation connectedEvaluation) error {
			evaluated++
			searchCandidates = append(searchCandidates, evaluation.diagnostics...)
			if evaluation.err != nil {
				rejected++
				lastError = evaluation.err
				return nil
			}
			out := evaluation.out
			better := best == nil
			if best != nil {
				better = betterTransportCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics)
				if evaluation.dense {
					better = betterDenseCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics)
					if req.Density == densityDense {
						if edgeOrder := compareDenseLatitudeEdge(req.SurfaceLayout, out.SurfacePlan, best.SurfacePlan); edgeOrder != 0 {
							better = edgeOrder < 0
						}
					}
				}
			}
			if better {
				best = &out
				bestKey = evaluation.key
			}
			return nil
		}
		var waveErr error
		if feasibleWave != nil {
			waveErr = search.Stream(ctx, executionCtx, indices, workers, execute, reduce)
		} else {
			waveErr = search.Waves(ctx, indices, workers, execute, reduce)
		}
		feasibleWave.close()
		if waveErr == nil && feasibleWave != nil && best != nil {
			stoppedAfterFeasibleWave = true
			break
		}
		if waveErr != nil {
			return Result{}, previewContextError(waveErr)
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
	for i := range searchCandidates {
		if searchCandidates[i].Key == bestKey {
			searchCandidates[i].Reason = "通过完整检查并获选"
		}
	}
	best.LogisticsSummary.SearchCandidates = searchCandidates
	searchMessage := fmt.Sprintf("最多 %d 个候选：先比较 %d 个就近入口矩形/轮廓布局，仅在全部不可行时独立重建兼容入口布局；评估 %d、拒绝 %d，按面积和内部带长选择可行方案，不代表数学全局最优", budget, layouts, evaluated, rejected)
	if req.Density == densityDense {
		searchMessage = fmt.Sprintf("全场密铺评估 %d 个候选，拒绝 %d 个；比较四向排布、容量组内短行和小数位置微调。优先完整紧凑矩形，再减小包络与真实带成本，不奖励铺带填空；不代表数学全局最优", evaluated, rejected)
		if req.SurfaceLayout != nil {
			if selected, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone); ok && selected.tier == 3 && selected.position != "center" {
				searchMessage += "；第3档指定边缘优先选择实际靠边且通过完整校验的候选，再比较空间与线路"
			}
		}
	}
	if stoppedAfterFeasibleWave {
		searchMessage += "；大型原生直连连续复用空闲计算槽，首个完整候选通过终检后最多比较5秒，并按原索引归约所有已完成候选；临近父截止时预留5秒返回，未完成候选取消且不作为部分蓝图返回"
	}
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "bounded-layout-search", Status: "passed", Message: searchMessage})
	return *best, nil
}

// betterTransportCandidate 职责：按明确且稳定的空间目标比较可行候选；使用：候选选择；边界：不可行候选不得参与此比较。
func betterTransportCandidate(a, b LayoutMetrics) bool {
	if first, second := layoutShapeObjective(a), layoutShapeObjective(b); math.Abs(first-second) > 1e-9 {
		return first < second
	}
	if a.EnvelopeArea != b.EnvelopeArea {
		return a.EnvelopeArea < b.EnvelopeArea
	}
	// 包络相同的候选优先填充真实建筑/路线并集；该次序只在硬规则均通过后生效，
	// 不以缩小碰撞盒或省略必要线路换取表面利用率。
	if math.Abs(a.BlankRatio-b.BlankRatio) > 1e-9 {
		return a.BlankRatio < b.BlankRatio
	}
	if a.InternalBeltLength != b.InternalBeltLength {
		return a.InternalBeltLength < b.InternalBeltLength
	}
	if a.CrossLayerConnections != b.CrossLayerConnections {
		return a.CrossLayerConnections < b.CrossLayerConnections
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

// prepareConnectedRows 职责：装配统一机器行和逐物品过滤端口但不决定全局运输拓扑；使用：直连带与可扩展混带环路共用局部几何；边界：调用方必须继续完成全图路由与最终规则，不能直接编码半成品。
func (b *builder) prepareConnectedRows(ctx context.Context, req Request, dataset *quant.Dataset, heightLimit float64, compact bool) error {
	b.generationCtx = ctx
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
	x, y, columnWidth := 0., 0., 0.
	horizontal := req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" && (!compact || b.connectedHorizontalPacking)
	packing := &connectedPacking{cells: map[[3]int][]int{}}
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
		if b.adapter != nil && b.adapter.connectedRows.preferFreshRecycleInput {
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
		if b.adapter != nil && b.adapter.connectedRows.prepareConnectedSelfSpray {
			if err := b.prepareConnectedSelfSpray(row, begin); err != nil {
				return err
			}
		}
		// 接入干线属于机器行真实占地，必须在装箱前生成，否则后补的西侧带会侵入上一列。
		block := b.all[begin:]
		if b.denseSpatial {
			denseUnits = append(denseUnits, denseSpatialUnit{entities: block, row: row})
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
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "机器和过滤端口超过 20 万建筑，请拆分目标"}
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
	if b.directBoundaryUnits != nil {
		if !b.denseSpatial {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "黑盒边界设施必须参与全场密铺占位"}
		}
		for _, route := range b.transportRoutes {
			if route.to != nil {
				route.end = route.to.inputEndpoint(route.edge.ItemID, route.inputRole)
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
		if b.directBoundaryUnits != nil && b.directBoundaryExtentScale > 0 {
			// 包络只用于估算有限搜索空间；实际摆放仍允许这些独立实体轮廓交错，不占据整块矩形。
			area, shortest := 0., 0.
			for _, unit := range denseUnits {
				lo, hi := bounds(unit.entities)
				width, height := math.Ceil(hi.x)-math.Floor(lo.x)+1, math.Ceil(hi.y)-math.Floor(lo.y)+1
				area += width * height
				shortest = math.Max(shortest, math.Min(width, height))
			}
			heightLimit = math.Max(heightLimit, math.Max(shortest+4, math.Ceil(math.Sqrt(area)*b.directBoundaryExtentScale)))
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
			if b.adapter != nil && b.adapter.connectedRows.keepAssignedRouteEnd && route.end != nil {
				continue
			}
			route.end = route.to.inputEndpoint(route.edge.ItemID, route.inputRole)
			if route.end == nil {
				return transportAllocationError(route.edge, route.rate, "配方输入或增产剂没有已验证实体端口")
			}
		}
	}
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
		if b.denseSpatial && !recycled && len(group.routes) == 1 && productionSprayItem(row.info) != group.routes[0].edge.ItemID {
			route := group.routes[0]
			route.start, route.path = trunk, []*placed{trunk}
			route.sharedPath = append([]*placed(nil), group.belts[1:]...)
			trunk.record.Parameters = []int32{int32(route.edge.ItemID), int32(math.Ceil(route.rate))}
			if route.to == nil {
				route.boundary = trunk
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
	}
	return nil
}

// connectedPackingShape 职责：保存候选机器行的真实静态盒、修正分拣器盒、带段和端口通道；使用：轮廓靠拢宽窄相位；边界：不裁小游戏碰撞体，不把包围盒重叠直接当作实体冲突。
type connectedPackingShape struct {
	// gridPort 职责：把连续位姿下的路由出口柱对齐到真正整数接驳点；使用：密铺微调；边界：不移动实际端口或偷改机器坐标。
	gridPort bool
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
	// dense 职责：使用同次密铺带间规则及节点净距；使用：全场密铺索引；边界：普通轮廓检查不改变。
	dense  bool
	shapes []connectedPackingShape
	cells  map[[3]int][]int
	high   vector
	ready  bool
	// queryMarks 职责：复用宽相位去重标记，避免每个形状查询分配map；使用：本候选单线程查询；边界：不跨候选/协程共享。
	queryMarks []uint32
	// querySerial 职责：区分本次形状查询；使用：O(1)去重；边界：回绕时清空标记，不跳过真实冲突。
	querySerial uint32
	// kindCells 职责：按碰撞规则类别索引密铺形状，避免查询必定互略的类型；使用：增量只读候选筛选；边界：未知类别保留保守检查。
	kindCells map[string]map[[3]int][]int
	// kindCount 职责：记录已进入分类索引的形状数；使用：新增实体后补齐；边界：shapes仅追加。
	kindCount int
	// infillAnchors 职责：保存已落位块的有限边缘候选；使用：水平行架回填旧空槽；边界：只尝试不扩展既有包络的全局落点。
	infillAnchors []vector
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
		shapes, err := splitterClearanceShapes(catalog, p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return out, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for shapeIndex, shape := range shapes {
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
			out.shapes = append(out.shapes, connectedBoxShape(kind, box))
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
	if shape.gridPort {
		shape.box.Center[0] = math.Round(shape.box.Center[0]/planarGridLength) * planarGridLength
		shape.box.Center[2] = math.Round(shape.box.Center[2]/planarGridLength) * planarGridLength
		shape.low, shape.high = shape.box.Bounds()
	}
	return shape
}

// connectedShapesConflict 职责：按静态/动态/带/电力规则区分轮廓冲突；使用：空间哈希窄相位；边界：最终检查的连接豁免只用于块内，跨块不允许借用别的物品端点。
func connectedShapesConflict(a, c *connectedPackingShape) bool {
	if a.kind == "splitter-selection" || c.kind == "splitter-selection" {
		return splitterSelectionPackingConflict(a, c)
	}
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
	if len(packing.queryMarks) < len(packing.shapes) {
		packing.queryMarks = append(packing.queryMarks, make([]uint32, len(packing.shapes)-len(packing.queryMarks))...)
	}
	if packing.dense {
		packing.indexDenseKinds()
	}
	for _, source := range contour.shapes {
		if ctx.Err() != nil {
			return false
		}
		shape := shiftedConnectedShape(source, dx, dy)
		packing.querySerial++
		if packing.querySerial == 0 {
			clear(packing.queryMarks)
			packing.querySerial = 1
		}
		serial := packing.querySerial
		buckets, bucketCount := [6]map[[3]int][]int{packing.cells}, 1
		if packing.dense {
			buckets, bucketCount = packing.denseQueryBuckets(shape.kind)
		}
		if !connectedShapeCells(shape, func(cell [3]int) bool {
			for bucketIndex := 0; bucketIndex < bucketCount; bucketIndex++ {
				bucket := buckets[bucketIndex]
				for _, index := range bucket[cell] {
					if packing.queryMarks[index] != serial {
						packing.queryMarks[index] = serial
						var conflict bool
						if packing.dense {
							conflict = denseSpatialConflict(&shape, &packing.shapes[index])
						} else {
							conflict = connectedShapesConflict(&shape, &packing.shapes[index])
						}
						if conflict {
							return false
						}
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
