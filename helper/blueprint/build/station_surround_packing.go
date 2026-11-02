package build

// 文件职责：把闭合物流塔组作为不可拆单元进行确定性偏矩形装箱。
// 使用：mode3/4 局部环绕与接驳完成后比较有限候选；边界：每个候选均须完整球面、端口、碰撞、供电和编码终检，不复用失败摘要。

import (
	"context"
	"errors"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// stationLayoutSnapshot 职责：保存闭合塔组的装配库存与实体状态；使用：隔离 finish 的平移、删塔和参数写入；边界：领域连接仍引用同一实体，不对共享流图做深复制。
type stationLayoutSnapshot struct {
	// original 职责：保存候选开始前的装配字段；使用：恢复库存与区间；边界：每次恢复复制可变切片和编码器。
	original builder
	// entities 职责：保存全部原实体记录；使用：恢复被删塔、坐标及参数；边界：次端和原连接一并保留。
	entities []placed
}

// newStationLayoutSnapshot 职责：保留塔候选的完整原实体；使用：有限列高搜索前；边界：参数与原始尾部独立保存，不只记录坐标。
func newStationLayoutSnapshot(b *builder) stationLayoutSnapshot {
	snapshot := stationLayoutSnapshot{original: *b, entities: make([]placed, len(b.all))}
	snapshot.original.all = append([]*placed(nil), b.all...)
	for i, entity := range b.all {
		snapshot.entities[i] = *entity
		snapshot.entities[i].record.Parameters = append([]int32(nil), entity.record.Parameters...)
		snapshot.entities[i].record.RawTail = append([]byte(nil), entity.record.RawTail...)
	}
	return snapshot
}

// restore 职责：重建下一候选的独立工作视图；使用：每候选前和搜索退出时；边界：取消后仍恢复原库存，已返回结果的摘要与表面计划不再被修改。
func (snapshot stationLayoutSnapshot) restore(b *builder) {
	*b = snapshot.original
	b.all = append([]*placed(nil), snapshot.original.all...)
	b.zoneRanges = append([]zoneRange(nil), snapshot.original.zoneRanges...)
	b.surfaceBlocks = append([]surfaceBlockRange(nil), snapshot.original.surfaceBlocks...)
	b.flyLayoutBlocks = append([]surfaceBlockRange(nil), snapshot.original.flyLayoutBlocks...)
	b.warnings = append([]string(nil), snapshot.original.warnings...)
	for i, entity := range b.all {
		*entity = snapshot.entities[i]
		entity.record.Parameters = append([]int32(nil), snapshot.entities[i].record.Parameters...)
		entity.record.RawTail = append([]byte(nil), snapshot.entities[i].record.RawTail...)
	}
	if original := snapshot.original.surfaceBusEncoding; original != nil {
		encoding := *original
		encoding.areas = append([]blueprint.Area(nil), original.areas...)
		encoding.latitudeOffsets = append([]int(nil), original.latitudeOffsets...)
		encoding.longitudeScales = append([]float64(nil), original.longitudeScales...)
		encoding.anchorCandidates = append([]int(nil), original.anchorCandidates...)
		if original.byBuilding != nil {
			encoding.byBuilding = make(map[*placed]int, len(original.byBuilding))
			for entity, area := range original.byBuilding {
				encoding.byBuilding[entity] = area
			}
		}
		if original.plan != nil {
			plan := *original.plan
			plan.Bands = append([]SurfaceBandPlan(nil), original.plan.Bands...)
			encoding.plan = &plan
		}
		b.surfaceBusEncoding = &encoding
	}
}

// finishSurroundStationLayouts 职责：比较真实塔组占地推导的有限装箱候选；使用：塔模式最终出口；边界：只有完整 finish 成功结果参与形状比较，取消不返回旧候选。
func (b *builder) finishSurroundStationLayouts(ctx context.Context, req Request, dataset *quant.Dataset, blocks []surfaceBlockRange) (Result, error) {
	snapshot := newStationLayoutSnapshot(b)
	defer snapshot.restore(b)
	area := 0.
	for _, block := range blocks {
		low, high := bounds(b.all[block.begin:block.end])
		area += (math.Ceil(high.x-low.x) + 3) * (math.Ceil(high.y-low.y) + 3)
	}
	heights := rectangularHeightCandidates(area, b.setting.BlockWidthLimit)
	warnings := append([]string(nil), b.warnings...)
	var best *Result
	var candidateErrors []error
	var searchCandidates []SearchCandidateDiagnostic
	rejected := 0
	for index, height := range heights {
		if ctx.Err() != nil {
			return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔装箱候选已取消或超时"}
		}
		snapshot.restore(b)
		if len(b.all) != len(snapshot.entities) {
			return Result{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔候选改变了闭合塔组的建筑数量"}
		}
		b.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
		b.warnings = append([]string(nil), warnings...)
		parameters := fmt.Sprintf("heightLimit=%g", height)
		if err := b.packSurroundStationBlocks(ctx, blocks, height); err != nil {
			searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("station-layout", index, parameters, "packing", err), b.logistics)
			candidateErrors, rejected = append(candidateErrors, err), rejected+1
			continue
		}
		for _, link := range b.stationLinks {
			previous := link.head
			for _, belt := range link.path {
				a, c := previous.a, belt.a
				b.logistics.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
				previous = belt
			}
		}
		result, err := b.finish(ctx, req, dataset)
		if err != nil {
			searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("station-layout", index, parameters, "final-validation", err), b.logistics)
			candidateErrors, rejected = append(candidateErrors, fmt.Errorf("物流塔列高 %.0f 候选：%w", height, err)), rejected+1
			continue
		}
		searchCandidates = appendSearchCandidateDiagnostics(searchCandidates, newSearchCandidateDiagnostic("station-layout", index, parameters, "final-validation", nil), result.LogisticsSummary)
		if best == nil || betterTransportCandidate(result.LogisticsSummary.Metrics, best.LogisticsSummary.Metrics) {
			best = &result
		}
	}
	if ctx.Err() != nil {
		return Result{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔装箱候选已取消或超时"}
	}
	if best == nil {
		if len(candidateErrors) == 0 {
			return Result{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔装箱没有通过完整终检的可行候选"}
		}
		return Result{}, errors.Join(candidateErrors...)
	}
	best.LogisticsSummary.Metrics.CandidateBudget = len(heights)
	best.LogisticsSummary.Metrics.EvaluatedCandidates = len(heights)
	best.LogisticsSummary.Metrics.RejectedCandidates = rejected
	best.LogisticsSummary.SearchCandidates = searchCandidates
	best.LogisticsSummary.Checks = append(best.LogisticsSummary.Checks, LogisticsCheck{RuleID: "station-surround-packing", Status: "passed", Message: fmt.Sprintf("中央塔组机器行分布南北两侧，闭合塔组按真实占地比较 %d 个有限装箱候选（拒绝 %d）；所有可行候选通过完整终检，按面积与过长惩罚选择，不是数学全局最优", len(heights), rejected)})
	return *best, nil
}

// packSurroundStationBlocks 职责：整体平移闭合塔组并补齐真实塔型间距；使用：有限列高候选；边界：不拆带、不旋转整塔组、不依赖目标分区强行拉开共用产线。
func (b *builder) packSurroundStationBlocks(ctx context.Context, blocks []surfaceBlockRange, heightLimit float64) error {
	x, y, columnWidth := 0., 0., 0.
	for _, block := range blocks {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔闭合块装箱已取消或超时"}
		}
		buildings := b.all[block.begin:block.end]
		low, high := bounds(buildings)
		width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
		if y > 0 && y+height > heightLimit {
			x, y, columnWidth = x+columnWidth+2, 0, 0
		}
		clearance := stationBlockClearance(b.all[:block.begin], buildings, x-low.x, y-low.y)
		if y > 0 && y+clearance+height > heightLimit {
			x, y, columnWidth = x+columnWidth+2, 0, 0
			clearance = stationBlockClearance(b.all[:block.begin], buildings, x-low.x, -low.y)
		}
		y += clearance
		shift(buildings, x-low.x, y-low.y, 0)
		y += height + 2
		columnWidth = math.Max(columnWidth, width)
	}
	return nil
}
