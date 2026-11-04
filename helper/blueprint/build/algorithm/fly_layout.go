package build

// 文件职责：在有限候选内按真实单行轮廓压缩配送布局，避免整配方巨块制造跨行留白。
// 使用：mode2/5 在物料图和设备参数核验后调用；边界：只整体平移连接闭合的机器行，不改变物料份额、机器数量或最终几何校验。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	mode2common "DysonSphereBox/helper/blueprint/build/adapters/mode2/common"
	"DysonSphereBox/helper/quant"
)

// flyLayoutVariant 职责：兼容旧装配器字段名；使用：mode2/5 生成流程；边界：候选规则由 mode2/common 所有。
type flyLayoutVariant = mode2common.LayoutVariant

// generateFlyLayout 职责：只从通过完整最终规则的配送候选中选择最小包络；使用：mode2/5 主链；边界：有界启发式不宣称数学全局最优，也不将机队能力标为已验证。
func (b *builder) generateFlyLayout(ctx context.Context, req Request, dataset *quant.Dataset) (Result, error) {
	if req.Mode == 2 && req.Density == densityDense {
		return b.generateDenseFlyLayout(ctx, req, dataset)
	}
	limits, err := b.layoutHeightCandidates(ctx, req, dataset, false)
	if err != nil {
		return Result{}, err
	}
	surfaceMode := mode2common.SurfacePlanar
	if req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
		surfaceMode = mode2common.SurfaceAutoTropic
	}
	variants := mode2common.Variants(b.setting.BlockWidthLimit, limits, surfaceMode, b.adapter != nil && b.adapter.flySurfacePairs)
	var best *Result
	var lastError error
	var searchCandidates []SearchCandidateDiagnostic
	rejected := 0
	for index, variant := range variants {
		if err := b.checkGeneration("配送布局候选"); err != nil {
			return Result{}, err
		}
		reportProgress(ctx, "fly-layout", fmt.Sprintf("正在检查配送紧凑布局候选 %d/%d", index+1, len(variants)), index+1, len(variants))
		candidate, err := b.newCandidateBuilder()
		if err != nil {
			return Result{}, err
		}
		candidate.generationCtx, candidate.flowGraph = ctx, b.flowGraph
		candidate.flyCompact = variant.Compact
		candidate.flySurfacePairs = variant.SurfacePairs
		candidate.warnings = append(candidate.warnings, b.warnings...)
		candidate.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
		candidate.logistics.Checks = append([]LogisticsCheck(nil), b.logistics.Checks...)
		candidate.beginPreview()
		parameters := fmt.Sprintf("heightLimit=%g;groupRows=%t;compact=%t;surfacePairs=%t", variant.HeightLimit, variant.GroupRows, variant.Compact, variant.SurfacePairs)
		stage := "assembly"
		if err = candidate.assembleFlyLayout(ctx, req, dataset, variant); err == nil {
			if variant.SurfacePairs {
				stage = "surface-pairing"
				err = candidate.pairFlySurfaceBlocks(ctx)
			}
			if err != nil {
				searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("fly-layout", index, parameters, stage, err), candidate.logistics)
				lastError, rejected = err, rejected+1
				candidate.rollbackPreview()
				continue
			}
			var out Result
			candidate.previewCheckpoint("layout")
			stage = "final-validation"
			out, err = candidate.finish(ctx, req, dataset)
			if err == nil && out.LogisticsSummary != nil {
				searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("fly-layout", index, parameters, stage, nil), out.LogisticsSummary)
				if best == nil || betterTransportCandidate(out.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
					best = &out
				}
				continue
			}
		}
		if err == nil {
			err = &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送候选缺少最终物流证据"}
		}
		searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("fly-layout", index, parameters, stage, err), candidate.logistics)
		lastError, rejected = err, rejected+1
		candidate.rollbackPreview()
	}
	if err := b.checkGeneration("配送候选提交"); err != nil {
		return Result{}, err
	}
	if best == nil {
		return Result{}, lastError
	}
	metrics := &best.LogisticsSummary.Metrics
	metrics.CandidateBudget, metrics.EvaluatedCandidates, metrics.RejectedCandidates = len(variants), len(variants), rejected
	best.LogisticsSummary.SearchCandidates = searchCandidates
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "bounded-fly-layout-search", Status: "passed", Message: fmt.Sprintf("固定 %d 个候选预算内按真实占地推导偏矩形列高，比较整配方基线、独立机器行和真实轮廓靠拢；自动球面配送另有一个反向交错候选，保留原有退路；拒绝 %d 个，完整核验后按面积×max(1,长宽比/2)、面积和带长选择，不改变规范物料流或机器数量", len(variants), rejected)})
	return *best, nil
}

// assembleFlyLayout 职责：把配送闭合单元独立装箱并保留分区与产线映射；使用：每个有限候选；边界：整组基线仅为可行性对照，单行位置以全体带/箱体/臂/电杆轮廓检查。
func (b *builder) assembleFlyLayout(ctx context.Context, req Request, dataset *quant.Dataset, variant flyLayoutVariant) error {
	b.flyLayoutHeight = variant.HeightLimit
	zones := planZones(req.Plan)
	rank := make(map[int]int, len(b.flowGraph.ProductionOrder))
	for index, owner := range b.flowGraph.ProductionOrder {
		rank[owner] = index
	}
	const gap = 2.
	zoneX, completed := 0., 0
	for zoneIndex := range zones {
		zone := &zones[zoneIndex]
		sort.SliceStable(zone.ProductionIndices, func(i, j int) bool { return rank[zone.ProductionIndices[i]] < rank[zone.ProductionIndices[j]] })
		zoneBegin := len(b.all)
		nextX, nextY, columnWidth := zoneX, 0., 0.
		packing := &connectedPacking{cells: map[[3]int][]int{}}
		for _, owner := range zone.ProductionIndices {
			if err := b.checkGeneration("配送机器行规划"); err != nil {
				return err
			}
			production := req.Plan.Productions[owner]
			recipe, err := datasetRecipeForProduction(dataset, production)
			if err != nil {
				return err
			}
			counts, err := layoutLineCounts(production, recipe, b.setting, dataset.ByID[production.MachineID].Speed, req.Density)
			if err != nil {
				return err
			}
			b.owner = owner
			completed++
			reportProgress(ctx, "production", "正在紧凑排布"+dataset.ByID[production.MainItemID].Name+"配送产线", completed, len(req.Plan.Productions))
			perGroup := 1
			if variant.GroupRows && req.SurfaceLayout == nil {
				perGroup = len(counts)
			}
			for groupStart := 0; groupStart < len(counts); groupStart += perGroup {
				begin, lineY := len(b.all), 0.
				for index := groupStart; index < min(groupStart+perGroup, len(counts)); index++ {
					row, err := b.buildLine(production, recipe, counts[index], req.Mode, 0, true)
					if err != nil {
						return err
					}
					low, high := bounds(row.all)
					shift(row.all, -low.x, lineY-low.y, 0)
					lineY += math.Ceil(high.y-low.y) + 1
				}
				block := b.all[begin:]
				low, high := bounds(block)
				width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
				if b.setting.MergeType == 2 && nextY > 0 && nextY+height > variant.HeightLimit {
					nextX, nextY, columnWidth = nextX+columnWidth+gap, 0, 0
				}
				x, y := nextX, nextY
				if variant.Compact {
					// 配送行内部没有悬空的物理端口，无须额外预留直连模式的竖直寻路柱。
					contour, err := b.connectedBlockShape(ctx, &transportRow{}, block)
					if err != nil {
						return err
					}
					x, y, err = packing.place(ctx, contour, low, high, nextX, nextY, variant.HeightLimit)
					if err != nil {
						return err
					}
				}
				shift(block, x-low.x, y-low.y, 0)
				b.previewCheckpoint("layout")
				b.flyLayoutBlocks = append(b.flyLayoutBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
				nextY = math.Max(nextY, y+height+gap)
				columnWidth = math.Max(columnWidth, x-nextX+width)
				if req.SurfaceLayout != nil {
					b.surfaceBlocks = append(b.surfaceBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
				}
				if len(b.all) > blueprint.MaxBuildingCount {
					return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配送机器和端点超过20万建筑，请拆分目标"}
				}
			}
			if len(recipe.Results) > 1 {
				b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 的各产物已从机器端过滤并独立分带；剩余副产物仍需持续取走，避免设备自身满仓停产", recipe.ID))
			}
		}
		if len(b.all) > zoneBegin {
			_, high := bounds(b.all[zoneBegin:])
			zoneX = math.Ceil(high.x) + 1 + gap
		}
		b.zoneRanges = append(b.zoneRanges, zoneRange{zone: zoneIndex, begin: zoneBegin, end: len(b.all)})
	}
	if len(zones) > 1 {
		b.warnings = append(b.warnings, fmt.Sprintf("多目标保留%d个分区；共享中间产物只生成一份，配送边按规范逐边分配；外部输入和副产物接口见物流诊断", len(zones)))
	}
	b.zones = zones
	if err := b.prepareFlyLeftovers(ctx, req.Mode); err != nil {
		return err
	}
	b.previewCheckpoint("routing")
	return nil
}
