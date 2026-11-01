package build

// 文件职责：将超大矩形或连接闭合的产线块转换为原生多 Area 蓝图。
// 使用：完整矩形无法装入单纬带时优先按安全设施切面拆分，失败后按闭合产线块装箱。
// 边界：不切开单个设施；跨纬带共享总线仍需专用重连，游戏 Physics 与已有世界实体不在本文件内。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// preparePackedSurfaceEncoding 职责：校验块闭合性并整体平移、建立朝主区域收敛的父链；使用：单矩形容量回退；边界：任何遗漏或跨块连接拒绝，不返回半份蓝图。
// 逻辑流：1. 核对块归属和连接 => 2. 安全余量装箱 => 3. 建立连续纬带 Area => 4. 平移两端并登记区域。
func (b *builder) preparePackedSurfaceEncoding(ctx context.Context, layout *SurfaceLayout) (*surfaceEncoding, error) {
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "产线块装箱已取消或超时"}
	}
	sizes := make([]surfaceBlockSize, len(b.surfaceBlocks))
	minima := make([]vector, len(sizes))
	blockOf := make(map[*placed]int, len(b.all))
	for i, block := range b.surfaceBlocks {
		if block.begin < 0 || block.end > len(b.all) || block.end <= block.begin {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "球面产线块范围无效"}
		}
		minimum, maximum := bounds(b.all[block.begin:block.end])
		minima[i] = minimum
		sizes[i] = surfaceBlockSize{width: int(math.Ceil(maximum.x-minimum.x)) + 1, height: int(math.Ceil(maximum.y-minimum.y)) + 1}
		for _, p := range b.all[block.begin:block.end] {
			if _, exists := blockOf[p]; exists {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "球面产线块重复包含建筑"}
			}
			blockOf[p] = i
		}
	}
	if len(blockOf) != len(b.all) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "存在跨产线公共设施，须先划分其纬带连接，不能直接拆开蓝图"}
	}
	for p, block := range blockOf {
		for _, other := range []*placed{p.input, p.output} {
			if other == nil {
				continue
			}
			if otherBlock, exists := blockOf[other]; !exists || otherBlock != block {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "存在跨产线连接，不能用整体平移截断物流路径"}
			}
		}
	}
	packLayout := *layout
	if packLayout.Mode == "autoTropic" {
		packLayout.ExpandAdjacent = true
	}
	gap := max(2, int(math.Ceil(b.setting.LocalStationHitRadius)))
	// 仅矩形容量无法证明块内机器在该纬度不会因环向收缩重叠，装箱前按实际建造盒过滤落点。
	cache := make(map[[2]int]bool)
	var geometryError error
	accept := func(index int, band surfacePackedBand, y int) bool {
		if ctx.Err() != nil {
			geometryError = &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "产线块装箱已取消或超时"}
			return false
		}
		if geometryError != nil {
			return false
		}
		row := band.low + y
		if layout.Mode == "explicitTropic" && !layout.ExpandAdjacent && row < layout.LatitudeGridIndex {
			return false
		}
		key := [2]int{index, row}
		if valid, exists := cache[key]; exists {
			return valid
		}
		block := b.surfaceBlocks[index]
		records := make([]blueprint.Building, 0, block.end-block.begin)
		for _, p := range b.all[block.begin:block.end] {
			if surfaceIsBelt(p.record.ItemID) || p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
				continue
			}
			r := p.record
			r.Index, r.AreaIndex = int32(len(records)), 0
			r.LocalOffset = blueprint.Point{X: float32(p.a.x - minima[index].x), Y: float32(p.a.y - minima[index].y), Z: float32(p.a.z)}
			records = append(records, r)
		}
		probe := &surfaceEncoding{areas: []blueprint.Area{{ParentIndex: -1, AreaSegments: int16(band.segments)}}, plan: &SurfacePlan{GridSegment: packingSegment(layout), AnchorLatitudeGrid: row}}
		err := probe.validateStaticColliders(ctx, records)
		if err != nil && !surfaceAnchorRetryable(err) {
			geometryError = err
		}
		cache[key] = err == nil
		return err == nil
	}
	stations := make([][]blueprint.Building, len(sizes))
	for i, block := range b.surfaceBlocks {
		for _, p := range b.all[block.begin:block.end] {
			if !surfaceIsStation(p.record.ItemID) {
				continue
			}
			r := p.record
			r.LocalOffset = blueprint.Point{X: float32(p.a.x - minima[i].x), Y: float32(p.a.y - minima[i].y), Z: float32(p.a.z)}
			stations[i] = append(stations[i], r)
		}
	}
	var acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool
	hasStations := false
	for _, block := range stations {
		hasStations = hasStations || len(block) > 0
	}
	if hasStations {
		acceptPlacement = func(candidate surfacePackPlacement, previous []surfacePackPlacement) bool {
			if geometryError != nil {
				return false
			}
			valid, err := surfacePackedStationsFit(ctx, &packLayout, stations, candidate, previous)
			if err != nil {
				geometryError = err
			}
			return valid
		}
	}
	packing, err := planSurfaceBlocksChecked(ctx, &packLayout, sizes, gap, accept, acceptPlacement)
	if geometryError != nil {
		return nil, geometryError
	}
	if err != nil {
		return nil, err
	}
	encoding, bands, err := newPackedSurfaceEncoding(layout, packing)
	if err != nil {
		return nil, err
	}
	anchor := encoding.plan.AnchorLatitudeGrid
	bandIndex := make(map[surfacePackedBand]int, len(bands))
	for i, band := range bands {
		bandIndex[band] = i
	}
	for i, block := range b.surfaceBlocks {
		packed := packing.blocks[i]
		area := bandIndex[packing.bands[packed.band]]
		shift(b.all[block.begin:block.end], float64(packed.x)-minima[i].x, float64(packing.bands[packed.band].low-anchor+packed.y)-minima[i].y, 0)
		encoding.areas[area].Width = max(encoding.areas[area].Width, int16(packed.x+sizes[i].width))
		encoding.plan.Bands[area].BuildingCount += block.end - block.begin
		for _, p := range b.all[block.begin:block.end] {
			encoding.byBuilding[p] = area
		}
	}
	for i, area := range encoding.areas {
		if !candidateFitsRing(encoding.plan.Bands[i].MinLatitudeGrid, 0, int(area.Height)-1, int(area.Width), packing.segment) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("区域 %d 超出原生纬带容量", i)}
		}
		encoding.plan.LayoutWidth = max(encoding.plan.LayoutWidth, int(area.Width))
	}
	encoding.plan.LayoutHeight = encoding.plan.Bands[len(bands)-1].MaxLatitudeGrid - encoding.plan.Bands[0].MinLatitudeGrid + 1
	b.warnings = append(b.warnings, fmt.Sprintf("完整矩形不满足球面容量或几何约束，已将 %d 个完整产线块重排到 %d 个原生纬带区域；请按建议纬度放置", len(sizes), len(bands)))
	return encoding, nil
}

// packingSegment 职责：为未经过请求归一化的内部测试补齐分段；使用：块几何探测；边界：支持范围由生产入口校验。
func packingSegment(layout *SurfaceLayout) int {
	if layout.GridSegment == 0 {
		return defaultSurfaceGridSegment
	}
	return layout.GridSegment
}
