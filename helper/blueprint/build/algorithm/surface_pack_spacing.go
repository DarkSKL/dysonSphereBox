package build

// 文件职责：在球面产线块装箱候选中核对物流塔的真实三维距离。
// 使用：块重排在提交平移前复用最终 Area 父链与原版 pair 规则；边界：不放宽最终碰撞阈值，不把矩形 gap 当作米制塔距证明。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// newPackedSurfaceEncoding 职责：为已使用纬带生成包含空白中间带的统一 Area 父链；使用：真实落点探测与最终块编码；边界：不移动建筑，不省略游戏吸附所需的中间纬带。
func newPackedSurfaceEncoding(layout *SurfaceLayout, packing *surfacePacking) (*surfaceEncoding, []surfacePackedBand, error) {
	if len(packing.bands) == 0 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "连续纬带区域范围为空"}
	}
	allBands := surfaceLatitudeBands(packing.segment)
	first, last := -1, -1
	for i, band := range allBands {
		if band == packing.bands[0] {
			first = i
		}
		if band == packing.bands[len(packing.bands)-1] {
			last = i
		}
	}
	if first < 0 || last < first || last-first+1 > surfaceMaxAreas {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "连续纬带区域范围无效"}
	}
	bands := allBands[first : last+1]
	primary := 0
	for i, band := range bands {
		if band.segments > bands[primary].segments {
			primary = i
		}
	}
	anchor := bands[primary].low
	if layout.Mode == "explicitTropic" && !layout.ExpandAdjacent {
		anchor = layout.LatitudeGridIndex
	}
	encoding := &surfaceEncoding{
		packed: true, areas: make([]blueprint.Area, len(bands)), latitudeOffsets: make([]int, len(bands)),
		byBuilding: make(map[*placed]int), anchorCandidates: []int{anchor},
		plan: makeSurfacePlan(layout.Mode, packing.segment, packing.preferred, anchor, 0, 0),
	}
	encoding.plan.AnchorLatitudeGrid, encoding.plan.PrimaryAreaIndex = anchor, primary
	encoding.plan.Bands = make([]SurfaceBandPlan, len(bands))
	for i, band := range bands {
		origin := band.low
		if i == primary {
			origin = anchor
		}
		parent, offsetY := -1, 0
		if i != primary {
			parent = i - 1
			if i < primary {
				parent = i + 1
			}
			parentOrigin := bands[parent].low
			if parent == primary {
				parentOrigin = anchor
			}
			offsetY = origin - parentOrigin
		}
		encoding.areas[i] = blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(band.segments), AnchorLocalOffsetY: int16(offsetY), Width: 1, Height: int16(band.high-origin) + 1}
		encoding.latitudeOffsets[i] = origin - anchor
		encoding.plan.Bands[i] = SurfaceBandPlan{AreaIndex: i, MinLatitudeGrid: origin, MaxLatitudeGrid: band.high, LongitudeSegments: band.segments}
	}
	return encoding, bands, nil
}

// surfacePackedStationsFit 职责：独立映射当前候选与前序块的塔中心并执行原版 pair 间距；使用：有限经度/新行候选筛选；边界：只返回可行性，不提交实体，取消或未知坐标规则作为错误上抛。
func surfacePackedStationsFit(ctx context.Context, layout *SurfaceLayout, stations [][]blueprint.Building, candidate surfacePackPlacement, previous []surfacePackPlacement) (bool, error) {
	if ctx.Err() != nil {
		return false, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "球面物流塔落点检查已取消或超时"}
	}
	placements := append(append(make([]surfacePackPlacement, 0, len(previous)+1), previous...), candidate)
	lowest, highest := candidate.band, candidate.band
	for _, placement := range previous {
		if placement.band.low < lowest.low {
			lowest = placement.band
		}
		if placement.band.high > highest.high {
			highest = placement.band
		}
	}
	packing := &surfacePacking{segment: packingSegment(layout), preferred: layout.LatitudeGridIndex, bands: []surfacePackedBand{lowest, highest}}
	encoding, bands, err := newPackedSurfaceEncoding(layout, packing)
	if err != nil {
		return false, err
	}
	byBand := make(map[surfacePackedBand]int, len(bands))
	for i, band := range bands {
		byBand[band] = i
	}
	records := []blueprint.Building{}
	for _, placement := range placements {
		if placement.index < 0 || placement.index >= len(stations) {
			return false, fmt.Errorf("球面塔候选产线块索引越界")
		}
		area := byBand[placement.band]
		origin := encoding.plan.Bands[area].MinLatitudeGrid
		for _, source := range stations[placement.index] {
			r := source
			r.Index, r.AreaIndex = int32(len(records)), int8(area)
			r.LocalOffset.X = float32(float64(source.LocalOffset.X) + float64(placement.x))
			r.LocalOffset.Y = float32(float64(source.LocalOffset.Y) + float64(placement.band.low+placement.y-origin))
			records = append(records, r)
		}
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return false, err
	}
	left, _, err := surfaceStationConflict(ctx, records, positions)
	if err != nil {
		return false, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "球面物流塔落点检查已取消或超时"}
	}
	return left < 0, nil
}
