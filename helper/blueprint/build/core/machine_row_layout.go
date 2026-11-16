package build

// 文件职责：按实际启用的局部机器模板选择共用拆行能力。
// 使用：直连、塔和配送共同调用；边界：普通模板不会读取密铺排数，实际物理与运输终检保持独立。

import (
	"DysonSphereBox/helper/quant"
	"math"
)

// layoutSpatialRows 职责：使用实际获选局部模板的重复节距细分容量组；使用：直连、配送和塔候选；边界：各原单带组独立守恒，不改变产线总数或带速。
func (b *builder) layoutSpatialRows(p quant.Production, r quant.Recipe, counts []int, maxSpan float64) ([]int, error) {
	if b.policy != nil && b.policy.density == densityStandard {
		return singleRowCounts(p, r, b.setting, counts, maxSpan)
	}
	if b.generationCtx != nil && !referenceLowRecipe(p, r) {
		if shortSpan, _ := b.generationCtx.Value(denseSpatialHolesKey{}).(float64); shortSpan > 0 {
			// 低轨连续街道保留原容量组；独立机器行才细分，以免全厂短行化增加大量接头。
			maxSpan = math.Min(maxSpan, shortSpan)
		}
	}
	if b.referenceChemicalFourEligible(p, r) {
		pitch, err := referenceLineSpacing(p.MachineType, b.setting.extraSpace(p.MachineType))
		if err != nil {
			return nil, err
		}
		limit := 2 * max(1, int(math.Floor(maxSpan/pitch))+1)
		return splitCapacityGroups(counts, func(int) (int, error) { return limit, nil }, false)
	}
	if !b.denseRefineryEligible(p, r) {
		wholeColumns := false
		if b.generationCtx != nil {
			span, _ := b.generationCtx.Value(denseSpatialHolesKey{}).(float64)
			wholeColumns = span > 0
		}
		if b.generationCtx != nil {
			if ground, _ := b.generationCtx.Value(denseGroundTracksKey{}).(bool); ground {
				return denseSpatialRowCountsForTemplate(p, r, b.setting, counts, maxSpan, wholeColumns, false)
			}
		}
		return denseSpatialRowCounts(p, r, b.setting, counts, maxSpan, wholeColumns)
	}
	limit := max(1, int(math.Floor(maxSpan/(denseRefineryPitch+float64(b.setting.RefineryOtherSpace))))+1)
	return splitCapacityGroups(counts, func(int) (int, error) { return limit, nil }, false)
}
