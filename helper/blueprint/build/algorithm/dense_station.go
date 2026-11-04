package build

// 文件职责：运行独立的超密度物流塔全场候选，不使用每配方闭合矩形或中央庭院。
// 使用：mode3/4 dense 组合；边界：每个候选均重新装配并执行完整最终规则，普通塔逻辑保持独立。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
	"context"
	"errors"
	"fmt"
	"math"
)

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
	if needsLocalRecycle {
		fieldBudget = 0
	}
	blackboxBudget := 4
	if req.Plan.MachineCount >= 80 {
		blackboxBudget = 7
	}
	budget = fieldBudget + blackboxBudget
	for variant := 0; variant < blackboxBudget; variant++ {
		candidate, err := b.denseStationBlackboxCandidate(ctx, req, dataset, config, variant)
		stage := "assembly"
		var out Result
		if err == nil {
			stage = "final-validation"
			out, err = candidate.finish(ctx, req, dataset)
		}
		street := candidate != nil && candidate.denseStreetCandidate
		parameters := fmt.Sprintf("externalStationsOnly=true;internalDirect=true;continuousStreet=%t;variant=%d", street, variant)
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
			continue
		}
		count := denseStationFacilityCount(candidate)
		if best == nil || count < bestTowers || count == bestTowers && betterDenseCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best, bestTowers = &out, count
		}
	}
	for index := 0; index < fieldBudget; index++ {
		street, variant := index >= 4, index%4
		scope := "dense-station-field"
		if street {
			scope = "dense-station-street"
		}
		if err := b.checkGeneration("密铺塔全场候选"); err != nil {
			return Result{}, err
		}
		candidate, err := b.newCandidateBuilder()
		if err != nil {
			return Result{}, err
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
			continue
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
			continue
		}
		count := denseStationFacilityCount(candidate)
		if best == nil || count < bestTowers || count == bestTowers && betterDenseCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best, bestTowers = &out, count
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
