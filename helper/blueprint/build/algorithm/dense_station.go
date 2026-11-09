package build

// 文件职责：运行独立的超密度物流塔全场候选，不使用每配方闭合矩形或中央庭院。
// 使用：mode3/4 dense 组合；边界：每个候选均重新装配并执行完整最终规则，普通塔逻辑保持独立。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/algorithm/search"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
	"context"
	"errors"
	"fmt"
	"math"
)

// betterDenseStationCandidate 职责：让新增密铺物流候选同时改善空间与真实线长的综合代价；使用：完成终检且塔数相同的候选比较；边界：既有候选之间沿用原评分，不能用小幅面积收益换取大量绕线。
func betterDenseStationCandidate(a, c *LogisticsSummary) bool {
	if !betterDenseCandidate(a.Metrics, c.Metrics) {
		return false
	}
	combinedCost := false
	for _, summary := range []*LogisticsSummary{a, c} {
		for _, check := range summary.Checks {
			combinedCost = combinedCost || check.RuleID == "refinery-collinear-lanes" || check.RuleID == "dense-station-shared-inputs"
		}
	}
	if combinedCost {
		if a.Metrics.EnvelopeArea > c.Metrics.EnvelopeArea {
			return false
		}
		// 乘积按相对比例比较两种代价，不混加米与格面积，也保留既有长条惩罚。
		first := denseLayoutShapeObjective(a.Metrics) * math.Max(1, a.Metrics.TotalBeltLengthMeters)
		second := denseLayoutShapeObjective(c.Metrics) * math.Max(1, c.Metrics.TotalBeltLengthMeters)
		if math.Abs(first-second) > 1e-6 {
			return first < second
		}
	}
	return true
}

// generateDenseStationField 职责：比较共享塔库存、矩形全场占位与实际路由的有限候选；使用：密铺塔入口；边界：不以失败布局或普通闭合组冒充成功。
func (b *builder) generateDenseStationField(ctx context.Context, req Request, dataset *quant.Dataset, input contract.StationLayoutInput, plan contract.StationLayoutPlan, config contract.StationConfig) (Result, error) {
	var best *Result
	var failures []error
	var diagnostics []SearchCandidateDiagnostic
	bestTowers := math.MaxInt
	budget := 4
	for index, prepared := range input.Productions {
		if b.denseStationStreetEligible(input.Plan.Productions[index], prepared.Recipe) {
			budget = 6
			break
		}
	}
	fieldBudget := budget
	needsLocalRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return Result{}, err
	}
	// 塔场路径尚未建立原生单元坐标；根球面候选即使完成平面接线，也会被最终编码明确拒绝。
	// 提前排除不具备目标坐标能力的候选，把同一总时限留给实际可编码的黑盒与后续单环搜索。
	if needsLocalRecycle || req.SurfaceLayout != nil && req.SurfaceLayout.polarRoot {
		fieldBudget = 0
	}
	blackboxBudget := 4
	narrowTier := false
	if req.SurfaceLayout != nil {
		selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
		narrowTier = ok && selection.tier >= 2 && selection.tier <= 5
	}
	// 窄纬中的中等规模厂也需要低纬向高度候选，避免仅按机器数量决定候选集合而回退到多塔转发。
	if req.Plan.MachineCount >= 80 || narrowTier {
		blackboxBudget = 7
	}
	blackboxIndices := make([]int, blackboxBudget)
	for index := range blackboxIndices {
		blackboxIndices[index] = index
	}
	if !b.setting.EnableFlowMonitor {
		for index, production := range input.Productions {
			p, r := input.Plan.Productions[index], production.Recipe
			if p.MachineType == 4 && denseReferenceSource(p.MachineID) == 2317 && p.IncreasePoints == 0 && len(r.Items) == 3 && len(r.Results) == 1 && !recipeHasMachineRecycle(r) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv) {
				blackboxIndices = append(blackboxIndices, 7, 8, 9)
				break
			}
		}
	}
	for index, production := range input.Productions {
		if b.denseRefineryEligible(input.Plan.Productions[index], production.Recipe) {
			blackboxIndices = append(blackboxIndices, 10, 11, 12)
			break
		}
	}
	// 共享原料使用独立候选；保留既有成功方案，按实际塔数、占地及线长比较完成终检的结果。
	if len(req.Plan.RawInputs) > 0 {
		base := 4
		if blackboxIndices[len(blackboxIndices)-1] >= 10 {
			base = 10
		} else if blackboxIndices[len(blackboxIndices)-1] >= 7 {
			base = 7
		}
		for variant := base; variant < base+3; variant++ {
			blackboxIndices = append(blackboxIndices, denseStationSharedInputVariantOffset+variant)
		}
	}
	blackboxBudget = len(blackboxIndices)
	budget = fieldBudget + blackboxBudget
	allPreviews := b.planCandidatePreviews(budget)
	type stationEvaluation struct {
		out         Result
		count       int
		err         error
		diagnostics []SearchCandidateDiagnostic
	}
	{
		indices := blackboxIndices
		previews := allPreviews[:blackboxBudget]
		waveErr := search.Waves(ctx, indices, b.candidateWorkers(ctx, len(indices)), func(ctx context.Context, localIndex int, variant int) (evaluation stationEvaluation) {
			ctx, finishPerformance := beginCandidatePerformance(ctx, "dense-station-blackbox", variant)
			defer func() { finishPerformance(evaluation.err) }()
			var failures []error
			var diagnostics []SearchCandidateDiagnostic
			defer func() { evaluation.err = errors.Join(failures...); evaluation.diagnostics = diagnostics }()
			candidate, err := b.denseStationBlackboxCandidate(ctx, req, dataset, config, variant, previews[localIndex])
			stage := "assembly"
			var out Result
			if err == nil {
				stage = "final-validation"
				out, err = candidate.finish(ctx, req, dataset)
			}
			street := candidate != nil && candidate.denseStreetCandidate
			parameters := fmt.Sprintf("externalStationsOnly=true;internalDirect=true;continuousStreet=%t;variant=%d", street, variant)
			if variant >= denseStationSharedInputVariantOffset {
				parameters += ";sharedExternalInputs=true"
			}
			if candidate != nil && candidate.denseLatitudeSpan > 0 {
				parameters += fmt.Sprintf(";materialNeighbors=true;sharedInputTracks=true;latitudeWorkBudget=%.0f", candidate.denseLatitudeSpan)
			}
			if out.SurfacePlan != nil {
				parameters += fmt.Sprintf(";nativeArea=%dx%d", out.SurfacePlan.LayoutWidth, out.SurfacePlan.LayoutHeight)
			}
			if candidate != nil {
				diagnostics = appendSearchCandidateDiagnostics(diagnostics, newSearchCandidateDiagnostic("dense-station-blackbox", variant, parameters, stage, err), candidate.logistics)
			}
			if err != nil {
				failures = append(failures, err)
				if candidate != nil {
					candidate.rollbackPreview()
				}
				return
			}

			evaluation.out = out
			evaluation.count = denseStationFacilityCount(candidate)
			return
		}, func(_ int, evaluation stationEvaluation) error {
			diagnostics = append(diagnostics, evaluation.diagnostics...)
			if evaluation.err != nil {
				failures = append(failures, evaluation.err)
				return nil
			}
			out, count := evaluation.out, evaluation.count
			if best == nil || count < bestTowers || count == bestTowers && betterDenseStationCandidate(out.LogisticsSummary, best.LogisticsSummary) {
				best, bestTowers = &out, count
			}
			return nil
		})
		if waveErr != nil {
			return Result{}, previewContextError(waveErr)
		}
	}
	{
		indices := make([]int, fieldBudget)
		for index := range indices {
			indices[index] = index
		}
		previews := allPreviews[blackboxBudget:]
		waveErr := search.Waves(ctx, indices, b.candidateWorkers(ctx, len(indices)), func(ctx context.Context, localIndex int, index int) (evaluation stationEvaluation) {
			ctx, finishPerformance := beginCandidatePerformance(ctx, "dense-station-field", index)
			defer func() { finishPerformance(evaluation.err) }()
			var failures []error
			var diagnostics []SearchCandidateDiagnostic
			defer func() { evaluation.err = errors.Join(failures...); evaluation.diagnostics = diagnostics }()
			street, variant := index >= 4, index%4
			scope := "dense-station-field"
			if street {
				scope = "dense-station-street"
			}
			if err := b.checkGeneration("密铺塔全场候选"); err != nil {
				failures = append(failures, err)
				return
			}
			candidate, err := b.newCandidateBuilder()
			if err != nil {
				failures = append(failures, err)
				return
			}
			if previews[localIndex] != nil {
				candidate.preview = previews[localIndex]
			}
			candidate.generationCtx, candidate.flowGraph = ctx, b.flowGraph
			candidate.denseSpatial = true
			candidate.connectedGlobalPower = street
			candidate.warnings = append([]string(nil), b.warnings...)
			candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
			candidate.beginPreview()
			parameters := fmt.Sprintf("globalDense=true;variant=%d;sharedRoleSlots=true;continuousStreet=%t", variant, street)
			stage := "assembly"
			span := 24. + float64(variant%2)*8
			rows, routes, err := candidate.assembleDenseStationRows(ctx, input, plan, config, span, street)
			if err == nil {
				var towers []denseStationTower
				towers, err = planDenseStationTowers(ctx, rows, routes, config.ItemSlots)
				if err == nil && len(towers) > bestTowers {
					err = &blueprint.Error{Code: "GEN_I_DENSITY_DOMINATED", Message: fmt.Sprintf("此候选需要%d塔，多于已完成完整终检的%d塔黑盒，按最少设施目标淘汰", len(towers), bestTowers)}
				}
				if err == nil {
					stage = "placement"
					err = candidate.placeDenseStationField(ctx, rows, routes, towers, config, variant)
				}
				if err == nil {
					stage = "routing"
					err = candidate.connectDenseStationField(ctx, rows, routes, towers, config)
				}
				if err == nil {
					candidate.surfaceBlocks = nil
					candidate.zones = make([]Zone, 0, len(plan.Zones))
					for _, zone := range plan.Zones {
						candidate.zones = append(candidate.zones, zoneFromLayout(zone.Zone))
					}
					stage = "power"
					if req.SurfaceLayout != nil && req.SurfaceLayout.Mode != "planar" {
						err = candidate.completeConnectedSurfacePower(ctx, req)
					} else {
						err = candidate.completeBoundaryFlowMonitors(ctx, req.Mode)
						if err == nil {
							err = candidate.completeConnectedPower(ctx)
						}
					}
				}
			}
			if err != nil {
				failures = append(failures, err)
				diagnostics = appendSearchCandidateDiagnostics(diagnostics, newSearchCandidateDiagnostic(scope, index, parameters, stage, err), candidate.logistics)
				candidate.rollbackPreview()
				return
			}
			for _, link := range candidate.stationLinks {
				previous := link.head
				for _, belt := range link.path {
					a, c := previous.a, belt.a
					candidate.logistics.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
					previous = belt
				}
			}
			out, err := candidate.finish(ctx, req, dataset)
			diagnostics = appendSearchCandidateDiagnostics(diagnostics, newSearchCandidateDiagnostic(scope, index, parameters, "final-validation", err), candidate.logistics)
			if err != nil {
				failures = append(failures, err)
				candidate.rollbackPreview()
				return
			}

			evaluation.out = out
			evaluation.count = denseStationFacilityCount(candidate)
			return
		}, func(_ int, evaluation stationEvaluation) error {
			diagnostics = append(diagnostics, evaluation.diagnostics...)
			if evaluation.err != nil {
				failures = append(failures, evaluation.err)
				return nil
			}
			out, count := evaluation.out, evaluation.count
			if best == nil || count < bestTowers || count == bestTowers && betterDenseStationCandidate(out.LogisticsSummary, best.LogisticsSummary) {
				best, bestTowers = &out, count
			}
			return nil
		})
		if waveErr != nil {
			return Result{}, previewContextError(waveErr)
		}
	}
	if ctx.Err() != nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺塔全场生成已取消或超时"}
	}
	if best == nil {
		return Result{}, errors.Join(failures...)
	}
	best.LogisticsSummary.SearchCandidates = diagnostics
	best.LogisticsSummary.Metrics.CandidateBudget, best.LogisticsSummary.Metrics.EvaluatedCandidates, best.LogisticsSummary.Metrics.RejectedCandidates = budget, budget, len(failures)
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "dense-station-field", Status: "passed", Message: fmt.Sprintf("先最少实际物流塔（获选%d塔），同塔数再比较紧凑矩形和真实线路；对外边界黑盒与跨产线共享塔场均执行完整终检，保留塔间距、十二带口、供剂和副产物证明", bestTowers)})
	return *best, nil
}
