package build

// 文件职责：把全场共享塔与内部直连布局用于普通塔模式。
// 使用：mode 3/4 标准密度布局完成后；边界：候选仍执行普通碰撞、间距及完整物流终检。

import (
	"context"
	"errors"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// generateStandardStationField 职责：按实际塔数与普通包络比较共享塔黑盒候选；使用：普通行星塔和星际塔；边界：只返回完整终检的候选，全部失败直接报错。
func (b *builder) generateStandardStationField(ctx context.Context, req Request, dataset *quant.Dataset, config contract.StationConfig) (Result, error) {
	var best *Result
	bestTowers := math.MaxInt
	var failures []error
	diagnostics := []SearchCandidateDiagnostic{}
	for variant := 0; variant < 4; variant++ {
		if err := b.checkGeneration("普通共享塔布局候选"); err != nil {
			return Result{}, err
		}
		candidate, err := b.denseStationBlackboxCandidate(ctx, req, dataset, config, variant)
		stage := "assembly"
		var out Result
		if err == nil {
			stage = "final-validation"
			out, err = candidate.finish(ctx, req, dataset)
		}
		if candidate != nil {
			diagnostics = appendSearchCandidateDiagnostics(diagnostics, newSearchCandidateDiagnostic("standard-station-field", variant, fmt.Sprintf("ordinarySpacing=true;sharedBoundaryTowers=true;variant=%d", variant), stage, err), candidate.logistics)
		}
		if err != nil {
			failures = append(failures, err)
			if candidate != nil {
				candidate.rollbackPreview()
			}
			continue
		}
		if out.LogisticsSummary == nil {
			failures = append(failures, &blueprint.Error{Code: "GEN_E_FLOW", Message: "普通共享塔候选缺少最终物流证据"})
			continue
		}
		count := denseStationFacilityCount(candidate)
		if best == nil || count < bestTowers || count == bestTowers && betterTransportCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best, bestTowers = &out, count
		}
	}
	if err := b.checkGeneration("普通共享塔布局提交"); err != nil {
		return Result{}, err
	}
	if best == nil {
		return Result{}, errors.Join(failures...)
	}
	best.LogisticsSummary.SearchCandidates = diagnostics
	best.LogisticsSummary.Metrics.CandidateBudget = 4
	best.LogisticsSummary.Metrics.EvaluatedCandidates = 4
	best.LogisticsSummary.Metrics.RejectedCandidates = len(failures)
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "standard-station-shared-field", Status: "passed", Message: fmt.Sprintf("普通机器间距与碰撞规则下，以全场机器核心和共享外部库存布局生成；实际%d座塔，内部物料由真实直连带输送", bestTowers)})
	return *best, nil
}
