package build

// 文件职责：以用户选定位置为基准，允许完整密铺矩形使用相邻原生纬带。
// 使用：显式ExpandAdjacent的独立原生候选；边界：保留严格单带结果，不换产量、设备或锚点语义，完整终检决定可行性。

import (
	"context"
	"fmt"
	"math"
	"time"
)

// denseAdjacentLatitudeKey 职责：传递本候选的扩展许可；使用：球面装箱与编码；边界：请求私有，旧请求默认关闭。
type denseAdjacentLatitudeKey struct{}

// denseLatitudeAllowedBands 职责：按中心/边缘扩展到连续相邻带；使用：布局与终检共用；边界：南边缘向北、北边缘向南、中心两侧，最多112格窗口，不跨行星极点。
func denseLatitudeAllowedBands(selection densityLatitudeSelection, expand bool) []surfacePackedBand {
	selected := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
	if !expand || len(selected) == 0 {
		return selected
	}
	all := surfaceLatitudeBands(200)
	first, last := -1, -1
	for i, band := range all {
		if band.low == selected[0].low {
			first = i
		}
		if band.high == selected[len(selected)-1].high {
			last = i
		}
	}
	if first < 0 || last < first {
		return selected
	}
	low, high := selected[0].low, selected[len(selected)-1].high
	switch selection.position {
	case "south":
		high = max(high, min(all[len(all)-1].high-2, low+111))
	case "north":
		low = min(low, max(all[0].low+2, high-111))
	default:
		center := (low + high) / 2
		low = min(low, max(all[0].low+2, center-56))
		high = max(high, min(all[len(all)-1].high-2, center+55))
	}
	out := []surfacePackedBand{}
	for _, band := range all {
		if band.high < low || band.low > high {
			continue
		}
		band.low = max(band.low, low)
		band.high = min(band.high, high)
		out = append(out, band)
	}
	return out
}

// denseLatitudeReferenceRange 职责：固定最初选择的区域边界；使用：窗口定位和边缘证明；边界：扩展不会把锚点移动到邻带中心。
func denseLatitudeReferenceRange(selection densityLatitudeSelection) (int, int) {
	bands := densityLatitudeTierBands(selection.tier, selection.hemisphere, 200)
	return bands[0].low, bands[len(bands)-1].high
}

// denseAdjacentFrameExtents 职责：按规模比较有限近方形窗口；使用：跨纬带候选；边界：窗口始终能围绕所选参考点放入允许区域。
func denseAdjacentFrameExtents(selection densityLatitudeSelection, bands []surfacePackedBand, machines int) []int {
	south, north := denseLatitudeReferenceRange(selection)
	limit := bands[len(bands)-1].high - bands[0].low + 1
	switch selection.position {
	case "south":
		limit = bands[len(bands)-1].high - south + 1
	case "north":
		limit = north - bands[0].low + 1
	default:
		center := (south + north) / 2
		limit = 2*min(center-bands[0].low, bands[len(bands)-1].high-center) + 1
	}
	// 实体坐标边界外仍有真实模型半径，两端各留两格，不用跨带许可吞掉外侧碰撞盒。
	limit -= 4
	ideal := int(math.Ceil(math.Sqrt(float64(max(1, machines))*12)*1.25)) + 12
	out := []int{}
	seen := map[int]bool{}
	for _, value := range []int{ideal, ideal * 4 / 5, ideal * 6 / 5, limit} {
		value = min(limit, max(8, value))
		if value > 0 && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// generateTransportWithAdjacent 职责：按选区宽度安排严格带与跨带搜索；使用：密铺运输模式；边界：窄纬带优先已授权的跨带，完整成功后不浪费时限重跑窄带，用户取消立即终止。
func generateTransportWithAdjacent(ctx context.Context, prepared preparedGeneration, adapter *modeAdapter) (Result, error) {
	req := prepared.request
	if req.Density != densityDense || req.SurfaceLayout == nil || req.SurfaceLayout.LatitudeZone == "" || !req.SurfaceLayout.ExpandAdjacent {
		return generateFixedTransportAttempt(ctx, prepared, adapter)
	}
	strict := prepared
	layout := *req.SurfaceLayout
	layout.ExpandAdjacent = false
	strict.request.SurfaceLayout = &layout
	// 窄纬带已获跨带授权时先搜索目标球面矩形，避免严格单带重试耗尽整次请求预算。
	selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	var early Result
	var earlyErr error
	earlyTried := selection.tier >= 2
	if earlyTried {
		early, earlyErr = generateAdjacentTransport(ctx, req, adapter)
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		if earlyErr == nil && early.LogisticsSummary.Metrics.EnvelopeAspectRatio <= 1.6 {
			early.LogisticsSummary.Checks = append(early.LogisticsSummary.Checks, LogisticsCheck{RuleID: "adjacent-latitude-first", Status: "passed", Message: "已按跨纬带许可优先完成近方形球面方案，省略严格窄纬带的重复搜索；真实接线、设备、碰撞及供电均已终检"})
			return early, nil
		}
	}
	strictContext := ctx
	cancelStrict := func() {}
	if earlyTried && earlyErr == nil {
		strictContext, cancelStrict = context.WithTimeout(ctx, 20*time.Second)
	}
	base, baseErr := generateFixedTransportAttempt(strictContext, strict, adapter)
	cancelStrict()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if baseErr == nil && base.LogisticsSummary != nil && base.LogisticsSummary.Metrics.EnvelopeAspectRatio <= densePreferredAspect {
		return base, nil
	}
	if baseErr != nil && !denseNativeGeometryFailure(baseErr) {
		if earlyTried && earlyErr == nil {
			return early, nil
		}
		return base, baseErr
	}
	out, err := early, earlyErr
	if !earlyTried {
		out, err = generateAdjacentTransport(ctx, req, adapter)
	}
	return selectAdjacentTransportResult(ctx, base, baseErr, out, err)
}

// generateAdjacentTransport 职责：独立分配跨带搜索预算；使用：优先跨带或原单带之后的比较；边界：局部截止不取消父任务，不提高运输等级。
func generateAdjacentTransport(ctx context.Context, req Request, adapter *modeAdapter) (Result, error) {
	// 额外比较按规模有界，不把已经成功的小产线拖入一分钟搜索。
	comparisonSeconds := min(60, max(12, req.Plan.MachineCount/2))
	extra, cancel := context.WithTimeout(ctx, time.Duration(comparisonSeconds)*time.Second)
	defer cancel()
	extra = context.WithValue(extra, denseAdjacentLatitudeKey{}, true)
	extra = context.WithValue(extra, denseBalancedFrameKey{}, true)
	extra = context.WithValue(extra, denseUnitPowerRequestedKey{}, req.Setting.NeedPower == nil || *req.Setting.NeedPower)
	selection, _ := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	if selection.tier >= 3 {
		extra = context.WithValue(extra, denseGroundTracksKey{}, true)
	}
	return generateDensePolar(extra, req, adapter)
}

// selectAdjacentTransportResult 职责：归约完整候选并保留已通过结果；使用：跨带与严格带搜索结束后；边界：父任务取消仍失败，不提交半成品。
func selectAdjacentTransportResult(ctx context.Context, base Result, baseErr error, out Result, err error) (Result, error) {
	if err == nil && baseErr == nil {
		a, c := out.LogisticsSummary.Metrics, base.LogisticsSummary.Metrics
		reportProgress(ctx, "adjacent-compared", fmt.Sprintf("相邻纬带完整方案：原长宽比%.3f/面积%.0f，候选长宽比%.3f/面积%.0f", c.EnvelopeAspectRatio, c.EnvelopeArea, a.EnvelopeAspectRatio, a.EnvelopeArea), 0, 0)
	}
	if err != nil {
		reportProgress(ctx, "adjacent-rejected", "相邻纬带候选未通过："+err.Error(), 0, 0)
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if err == nil && (baseErr != nil || betterAdjacentLayout(out, base)) {
		out.LogisticsSummary.Checks = append(out.LogisticsSummary.Checks, LogisticsCheck{RuleID: "adjacent-latitude-shape", Status: "passed", Message: "以所选中心或边缘为基准比较相邻原生纬带，按整厂形状获选；仍通过全部球面、物料和设施规则"})
		return out, nil
	}
	if baseErr == nil {
		base.LogisticsSummary.Checks = append(base.LogisticsSummary.Checks, LogisticsCheck{RuleID: "adjacent-latitude-shape", Status: "passed", Message: "已比较以所选位置为基准的相邻纬带；本次有界搜索未找到形状代价更低的完整可行方案，保留已通过全部检查的原方案"})
		return base, nil
	}
	return out, err
}

// betterAdjacentLayout 职责：在额外面积有界且最长边真正缩短时优先近方形；使用：严格带与相邻带比较；边界：不增加物流设施，不奖励只扩大短边的填空。
func betterAdjacentLayout(candidate, base Result) bool {
	facilities := func(result Result) int {
		count := 0
		for _, r := range result.statisticsRecords {
			if surfaceIsStation(r.ItemID) || r.ItemID == 2107 {
				count++
			}
		}
		return count
	}
	if facilities(candidate) > facilities(base) {
		return false
	}
	a, c := candidate.LogisticsSummary.Metrics, base.LogisticsSummary.Metrics
	if a.EnvelopeAspectRatio <= densePreferredAspect && c.EnvelopeAspectRatio > densePreferredAspect &&
		a.EnvelopeArea <= c.EnvelopeArea*1.5 && a.EnvelopeArea*a.EnvelopeAspectRatio < c.EnvelopeArea*c.EnvelopeAspectRatio-1e-6 {
		return true
	}
	return betterDenseCandidate(a, c)
}

// updateDenseRootShapeMetrics 职责：按跨带共同主区的整厂包络计形状；使用：正式候选终检后；边界：不将每条窄Area单独评分，也不更改面积或真实带长。
func updateDenseRootShapeMetrics(surface *surfaceEncoding, metrics *LayoutMetrics) {
	if surface == nil || !surface.rootCoordinates || surface.plan == nil {
		return
	}
	w, h := float64(surface.plan.LayoutWidth), float64(surface.plan.LayoutHeight)
	if w > 0 && h > 0 {
		metrics.EnvelopeAspectRatio = math.Max(w, h) / math.Min(w, h)
		metrics.ShapeObjective = denseLayoutShapeObjective(*metrics)
	}
}
