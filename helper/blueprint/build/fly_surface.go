package build

// 文件职责：为交错配送组合块比较有限球面宽度。
// 使用：保留旧单行候选之外的自动球面方案；边界：至多四宽、固定几何预算，全部最终数值规则通过才参与原形状目标比较。

import (
	"context"
	"errors"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// planFlySurfaceWidths 职责：比较交错配送块的完整可行球面落点；使用：组合块回退装箱；边界：不改变实体或机器图，不把容量失败当几何无解，取消不提交旧候选。
func (b *builder) planFlySurfaceWidths(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap int, records [][]blueprint.Building, accept func(int, surfacePackedBand, int) bool, probeError func() error) (*surfacePacking, error) {
	widths := surfaceBlockWidthCandidates(layout, sizes, gap, .95)
	var best *surfacePacking
	var bestMetrics LayoutMetrics
	rejected, bestWidth := 0, 0
	var failures []error
	for i, width := range widths {
		reportProgress(ctx, "fly-surface-widths", "正在比较配送组合块球面宽度", i+1, len(widths))
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送球面宽度候选已取消或超时"}
		}
		var packing *surfacePacking
		var err error
		stage := "packing"
		parameters := fmt.Sprintf("widthLimit=%d;gap=%d", width, gap)
		if len(widths) == 4 && width == widths[1] {
			packing, err = planSurfaceBlockRectanglesBounded(ctx, layout, sizes, gap, width, max(1, width*3/5), accept, nil)
		} else {
			packing, err = planSurfaceBlocksHeightSorted(ctx, layout, sizes, gap, width, accept, nil)
		}
		if geometryErr := probeError(); geometryErr != nil {
			if b.logistics != nil {
				b.logistics.SearchCandidates = append(b.logistics.SearchCandidates, newSearchCandidateDiagnostic("fly-surface-width", i, parameters, "geometry-probe", geometryErr))
			}
			return nil, geometryErr
		}
		var metrics LayoutMetrics
		if err == nil {
			stage = "final-validation"
			metrics, err = b.validatePackedSurfaceLayout(ctx, layout, packing, sizes, records)
		}
		if b.logistics != nil {
			b.logistics.SearchCandidates = append(b.logistics.SearchCandidates, newSearchCandidateDiagnostic("fly-surface-width", i, parameters, stage, err))
		}
		if err != nil {
			var failure *blueprint.Error
			if !errors.As(err, &failure) || !surfaceAnchorRetryable(failure) && failure.Code != "GEN_E_SURFACE_CAPACITY" {
				return nil, err
			}
			failures, rejected = append(failures, err), rejected+1
			continue
		}
		if best == nil || betterTransportCandidate(metrics, bestMetrics) {
			best, bestMetrics, bestWidth = packing, metrics, width
		}
	}
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "配送球面宽度候选已取消或超时"}
	}
	if best == nil {
		return nil, errors.Join(failures...)
	}
	b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "fly-surface-rectangular-packing", Status: "passed", Message: fmt.Sprintf("配送组合块比较 %d/4 个有限宽度（拒绝 %d），选择可用宽度 %d；面积 %.0f、长宽比 %.3f；完整球面和最终数值规则通过，机队运行吞吐仍未验证", len(widths), rejected, bestWidth, bestMetrics.EnvelopeArea, bestMetrics.EnvelopeAspectRatio)})
	return best, nil
}
