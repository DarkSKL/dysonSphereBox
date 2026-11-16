package build

// 文件职责：对闭合塔组的球面回退比较有限矩形宽度，避免默认沿整条纬圈展开。
// 使用：塔组行内几何与塔间距回调准备后调用；边界：候选保持真实分段、完整连接和机器数，全部数值终检后才提交位置。

import (
	"context"
	"errors"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// stationSurfaceWidthCandidates 职责：指定塔组四宽的中间比例；使用：塔组球面回填；边界：原容量与满纬圈退路不变。
func stationSurfaceWidthCandidates(layout *SurfaceLayout, sizes []surfaceBlockSize, gap int) []int {
	return surfaceBlockWidthCandidates(layout, sizes, gap, .95)
}

// planStationSurfaceWidths 职责：独立验证每个塔球面宽度并按真实区域形状选优；使用：闭合塔块回退；边界：不移动 builder 实体，取消或未知规则直接失败，候选间不复用审计结果。
// 逻辑流：1. 有界宽度装箱 => 2. 构建完整最终编号记录 => 3. 执行同一球面及最终规则 => 4. 比较区域面积与形状，仅返回通过的完整布局。
func (b *builder) planStationSurfaceWidths(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap int, records [][]blueprint.Building, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool, probeError func() error) (*surfacePacking, error) {
	widths := stationSurfaceWidthCandidates(layout, sizes, gap)
	var best *surfacePacking
	var bestMetrics, baselineMetrics LayoutMetrics
	bestWidth, rejected := 0, 0
	var failures []error
	for i, width := range widths {
		reportProgress(ctx, "station-surface-widths", "正在比较物流塔球面矩形宽度", i+1, len(widths))
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔球面宽度候选已取消或超时"}
		}
		stage := "packing"
		parameters := fmt.Sprintf("widthLimit=%d;gap=%d", width, gap)
		packing, err := planStationSurfaceWidth(ctx, layout, sizes, gap, width, accept, acceptPlacement)
		if geometryErr := probeError(); geometryErr != nil {
			if b.logistics != nil {
				b.logistics.SearchCandidates = append(b.logistics.SearchCandidates, newSearchCandidateDiagnostic("station-surface-width", i, parameters, "geometry-probe", geometryErr))
			}
			return nil, geometryErr
		}
		var metrics LayoutMetrics
		if err == nil {
			stage = "final-validation"
			metrics, err = b.validatePackedSurfaceLayout(ctx, layout, packing, sizes, records)
		}
		if b.logistics != nil {
			b.logistics.SearchCandidates = append(b.logistics.SearchCandidates, newSearchCandidateDiagnostic("station-surface-width", i, parameters, stage, err))
		}
		if err != nil {
			var failure *blueprint.Error
			if !errors.As(err, &failure) || !surfaceAnchorRetryable(failure) && failure.Code != "GEN_E_SURFACE_CAPACITY" {
				return nil, err
			}
			failures, rejected = append(failures, fmt.Errorf("物流塔球面宽度 %d 候选：%w", width, err)), rejected+1
			continue
		}
		if width == 0 {
			baselineMetrics = metrics
		}
		if best == nil || betterTransportCandidate(metrics, bestMetrics) {
			best, bestMetrics, bestWidth = packing, metrics, width
		}
	}
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔球面宽度候选已取消或超时"}
	}
	if best == nil {
		return nil, errors.Join(failures...)
	}
	if b.logistics != nil {
		message := fmt.Sprintf("闭合塔组比较 %d/4 个有限球面宽度（拒绝 %d），选择可用宽度 %d；面积 %.0f、长宽比 %.3f，完整带占位、塔间距及最终数值规则均通过", len(widths), rejected, bestWidth, bestMetrics.EnvelopeArea, bestMetrics.EnvelopeAspectRatio)
		if baselineMetrics.EnvelopeArea > 0 {
			message += fmt.Sprintf("；满纬圈参考面积 %.0f、长宽比 %.3f", baselineMetrics.EnvelopeArea, baselineMetrics.EnvelopeAspectRatio)
		}
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "station-surface-rectangular-packing", Status: "passed", Message: message})
	}
	return best, nil
}

// planStationSurfaceWidth 职责：优先为高块预留稀缺的宽纬带行，再按原编号返回完整落点；使用：有限塔宽候选；边界：只排序尺寸副本，几何回调保持原块身份，零宽保留原顺序兼容参考。
func planStationSurfaceWidth(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap, width int, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool) (*surfacePacking, error) {
	if width == 0 {
		return planSurfaceBlocksWidthChecked(ctx, layout, sizes, gap, width, accept, acceptPlacement)
	}
	widths := stationSurfaceWidthCandidates(layout, sizes, gap)
	if layout.Mode == "autoTropic" && len(widths) == 4 && width == widths[1] {
		return planSurfaceBlockRectanglesBounded(ctx, layout, sizes, gap, width, max(1, width*3/5), accept, acceptPlacement)
	}
	return planSurfaceBlocksHeightSorted(ctx, layout, sizes, gap, width, accept, acceptPlacement)
}
