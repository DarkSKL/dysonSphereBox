package build

// 文件职责：把设施集中与内部直连黑盒布局用于普通配送模式。
// 使用：mode 2 标准密度候选；边界：机器与路线仍按普通碰撞、间距和完整配送证据验收。

import (
	"context"
	"errors"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/algorithm/search"
	"DysonSphereBox/helper/quant"
)

// generateStandardFlyField 职责：以最少实际配送器选择普通间距的全场布局；使用：普通配送模式；边界：全部候选失败直接报错，不发布旧布局或部分蓝图。
func (b *builder) generateStandardFlyField(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	var best *Result
	bestFacilities := math.MaxInt
	var failures []error
	diagnostics := []SearchCandidateDiagnostic{}
	options := []struct {
		scale, latitudeFactor float64
		order                 int
		materialOrder         bool
		consumerFirst         bool
	}{
		{scale: 1, order: 2},
		{scale: 1.2, order: 3},
		{scale: 1, latitudeFactor: .7, order: 3},
		{scale: 1, latitudeFactor: .85, order: 3},
		{scale: 1.2, latitudeFactor: .75, order: 2},
		{scale: 1, latitudeFactor: .85, order: 3, materialOrder: true},
		{scale: 1.2, latitudeFactor: .7, order: 3, materialOrder: true, consumerFirst: true},
		{scale: 1.2, order: 3, materialOrder: true},
	}
	type evaluation struct {
		out         Result
		err         error
		count       int
		diagnostics []SearchCandidateDiagnostic
	}
	workers := b.candidateWorkers(ctx, len(options))
	previews := b.planCandidatePreviews(len(options))
	err := search.Waves(ctx, options, workers, func(ctx context.Context, variant int, option struct {
		scale, latitudeFactor float64
		order                 int
		materialOrder         bool
		consumerFirst         bool
	}) (evaluated evaluation) {
		ctx, finishPerformance := beginCandidatePerformance(ctx, "standard-fly", variant)
		defer func() { finishPerformance(evaluated.err) }()
		candidate, err := b.newCandidateBuilder()
		if err != nil {
			return evaluation{err: err}
		}
		if previews[variant] != nil {
			candidate.preview = previews[variant]
		}
		candidate.generationCtx, candidate.flowGraph = ctx, b.flowGraph
		candidate.denseSpatial, candidate.denseSpatialOrder, candidate.flyCompact = true, option.order, true
		if option.latitudeFactor > 0 {
			candidate.denseLatitudeSpan = math.Max(24, math.Ceil(math.Sqrt(float64(req.Plan.MachineCount)*32)*option.latitudeFactor))
		}
		if option.materialOrder {
			candidate.denseUnitOrder = func(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
				return candidate.denseStationMaterialUnitOrder(ctx, units, option.consumerFirst)
			}
		}
		candidate.warnings = append(candidate.warnings, b.warnings...)
		candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
		candidate.logistics.Checks = append(candidate.logistics.Checks, b.logistics.Checks...)
		candidate.beginPreview()
		stage := "assembly"
		err = candidate.assembleDenseFlyBlackbox(ctx, req, dataset, option.scale, false)
		var out Result
		if err == nil {
			stage = "final-validation"
			out, err = candidate.finish(ctx, req, dataset)
		}
		parameters := fmt.Sprintf("ordinarySpacing=true;sharedBoundaryBoxes=true;scale=%.2f;order=%d;latitudeBudget=%.0f;materialOrder=%t;consumerFirst=%t", option.scale, option.order, candidate.denseLatitudeSpan, option.materialOrder, option.consumerFirst)
		proof := newSearchCandidateDiagnostic("standard-fly-field", variant, parameters, stage, err)
		if err == nil && out.LogisticsSummary != nil {
			metrics := out.LogisticsSummary.Metrics
			proof.Reason = fmt.Sprintf("完整通过：配送器%d个，包络%.0f格²，长宽比%.3f，真实带%d段/%.3f米", denseFlyFacilityCount(candidate.all), metrics.EnvelopeArea, metrics.EnvelopeAspectRatio, metrics.BeltCount, metrics.TotalBeltLengthMeters)
		}
		evaluated = evaluation{out: out, err: err, count: denseFlyFacilityCount(candidate.all), diagnostics: appendSearchCandidateDiagnostics(nil, proof, candidate.logistics)}
		if err != nil {
			candidate.rollbackPreview()
			return evaluated
		}
		if out.LogisticsSummary == nil {
			evaluated.err = &blueprint.Error{Code: "GEN_E_FLOW", Message: "普通配送全场候选缺少最终物流证据"}
			candidate.rollbackPreview()
		}
		return evaluated
	}, func(_ int, evaluated evaluation) error {
		diagnostics = append(diagnostics, evaluated.diagnostics...)
		if evaluated.err != nil {
			failures = append(failures, evaluated.err)
			return nil
		}
		out, count := evaluated.out, evaluated.count
		if best == nil || count < bestFacilities || count == bestFacilities && betterTransportCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best, bestFacilities = &out, count
		}
		return nil
	})
	if err != nil {
		return Result{}, previewContextError(err)
	}
	if err := b.checkGeneration("普通配送全场布局提交"); err != nil {
		return Result{}, err
	}
	if best == nil {
		return Result{}, errors.Join(failures...)
	}
	best.LogisticsSummary.SearchCandidates = diagnostics
	best.LogisticsSummary.Metrics.CandidateBudget = len(options)
	best.LogisticsSummary.Metrics.EvaluatedCandidates = len(options)
	best.LogisticsSummary.Metrics.RejectedCandidates = len(failures)
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "standard-fly-shared-field", Status: "passed", Message: fmt.Sprintf("普通机器间距与碰撞规则下，以全场机器核心和集中箱体布局生成；实际%d个配送器，内部物料由真实直连带输送", bestFacilities)})
	return *best, nil
}
