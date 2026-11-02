package build

// 文件职责：共享闭合块的有界宽度推导、稳定排序和完整球面候选终检。
// 使用：塔组与配送组合块装箱；边界：各模式保留自己的宽度策略，任何候选均不得跳过最终硬规则。

import (
	"cmp"
	"context"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
)

// surfaceBlockWidthCandidates 职责：从真实完整块推导至多四种球面宽度；使用：塔组与配送组合块；边界：比例由模式决定，原容量与满纬圈退路始终保留。
func surfaceBlockWidthCandidates(layout *SurfaceLayout, sizes []surfaceBlockSize, gap int, fraction float64) []int {
	area, widest := 0, 0
	for _, size := range sizes {
		area += (size.width + gap) * (size.height + gap)
		widest = max(widest, size.width)
	}
	rules := newSurfacePackRules(gap)
	highest := 1
	for _, band := range surfaceLatitudeBands(packingSegment(layout)) {
		highest = max(highest, rules.usableHeight(band))
	}
	square := max(widest, int(math.Ceil(math.Sqrt(float64(area)))))
	capacity := max(square, int(math.Ceil(float64(area)/float64(highest))))
	result := make([]int, 0, 4)
	seen := map[int]bool{}
	for _, width := range []int{square, max(square, int(math.Ceil(float64(capacity)*fraction))), capacity, 0} {
		if !seen[width] {
			seen[width] = true
			result = append(result, width)
		}
	}
	return result
}

// planSurfaceBlocksHeightSorted 职责：优先给高块分配纬带并恢复原块身份；使用：闭合块的有限宽度候选；边界：零宽保留原顺序，回调不修改输入或建筑。
func planSurfaceBlocksHeightSorted(ctx context.Context, layout *SurfaceLayout, sizes []surfaceBlockSize, gap, width int, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool) (*surfacePacking, error) {
	if width == 0 {
		return planSurfaceBlocksWidthChecked(ctx, layout, sizes, gap, width, accept, acceptPlacement)
	}
	order := make([]int, len(sizes))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(sizes[b].height, sizes[a].height), cmp.Compare(sizes[b].width, sizes[a].width))
	})
	ordered := make([]surfaceBlockSize, len(sizes))
	for i, index := range order {
		ordered[i] = sizes[index]
	}
	var orderedAccept func(int, surfacePackedBand, int) bool
	if accept != nil {
		orderedAccept = func(index int, band surfacePackedBand, y int) bool { return accept(order[index], band, y) }
	}
	var orderedPlacement func(surfacePackPlacement, []surfacePackPlacement) bool
	if acceptPlacement != nil {
		orderedPlacement = func(candidate surfacePackPlacement, previous []surfacePackPlacement) bool {
			candidate.index = order[candidate.index]
			original := make([]surfacePackPlacement, len(previous))
			for i, placement := range previous {
				placement.index = order[placement.index]
				original[i] = placement
			}
			return acceptPlacement(candidate, original)
		}
	}
	packing, err := planSurfaceBlocksWidthChecked(ctx, layout, ordered, gap, width, orderedAccept, orderedPlacement)
	if err != nil {
		return nil, err
	}
	blocks := make([]surfacePackedBlock, len(sizes))
	for i, index := range order {
		blocks[index] = packing.blocks[i]
	}
	packing.blocks = blocks
	return packing, nil
}

// validatePackedSurfaceLayout 职责：用完整真实编号和双端坐标复核一个尚未提交的闭合块球面布局；使用：有限宽度比较；边界：不改 builder 或复用上个候选的检查标记，不把离线规则当游戏实测。
func (b *builder) validatePackedSurfaceLayout(ctx context.Context, layout *SurfaceLayout, packing *surfacePacking, sizes []surfaceBlockSize, blocks [][]blueprint.Building) (LayoutMetrics, error) {
	encoding, bands, err := newPackedSurfaceEncoding(layout, packing)
	if err != nil {
		return LayoutMetrics{}, err
	}
	byBand := make(map[surfacePackedBand]int, len(bands))
	for i, band := range bands {
		byBand[band] = i
	}
	records := make([]blueprint.Building, 0, len(b.all))
	for i, packed := range packing.blocks {
		if ctx.Err() != nil {
			return LayoutMetrics{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "闭合块球面候选记录已取消或超时"}
		}
		band := packing.bands[packed.band]
		area := byBand[band]
		block := b.surfaceBlocks[i]
		minimum, _ := bounds(b.all[block.begin:block.end])
		dx := float64(packed.x) - minimum.x
		dy := float64(band.low-encoding.plan.AnchorLatitudeGrid+packed.y) - minimum.y
		encoding.areas[area].Width = max(encoding.areas[area].Width, int16(packed.x+sizes[i].width))
		for j, source := range blocks[i] {
			building := b.all[block.begin+j]
			record := source
			record.AreaIndex = int8(area)
			record.LocalOffset = blueprint.Point{X: float32(building.a.x + dx), Y: float32(building.a.y + dy - float64(encoding.latitudeOffsets[area])), Z: float32(building.a.z)}
			record.LocalOffset2 = blueprint.Point{X: float32(building.b.x + dx), Y: float32(building.b.y + dy - float64(encoding.latitudeOffsets[area])), Z: float32(building.b.z)}
			records = append(records, record)
		}
	}
	if len(records) != len(b.all) {
		return LayoutMetrics{}, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "闭合块球面候选遗漏了建筑记录"}
	}
	if err := encoding.validateAnchorCandidates(ctx, records); err != nil {
		return LayoutMetrics{}, err
	}
	// 在独立实体库存中真实补电，避免候选预审先因边缘覆盖拒绝，也不能绕过新增塔后的完整碰撞审计。
	probe, err := b.newFinalPowerProbe(encoding, records)
	if err != nil {
		return LayoutMetrics{}, err
	}
	added, err := probe.repairFinalPower(ctx, encoding, records)
	if err != nil {
		return LayoutMetrics{}, err
	}
	if added > 0 {
		records, _, _, err = probe.records()
		if err != nil {
			return LayoutMetrics{}, err
		}
		if err := encoding.applyRecords(probe.all, records); err != nil {
			return LayoutMetrics{}, err
		}
	}
	if _, err := validateFinalPlanetRules(ctx, Request{Setting: b.setting}, encoding, records); err != nil {
		return LayoutMetrics{}, err
	}
	summary := LogisticsSummary{}
	for _, area := range encoding.areas {
		summary.Metrics.EnvelopeArea += float64(area.Width) * float64(area.Height)
	}
	updateLayoutShapeMetrics(&summary, encoding.areas)
	return summary.Metrics, nil
}
