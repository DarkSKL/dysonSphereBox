package build

// 文件职责：在单带容量组内部均衡细分过长机器行，允许全场搜索形成紧凑矩形。
// 使用：密铺各模式的空间候选；边界：不增加机器、不放大带宽或分拣器能力，不超过用户行上限。

import (
	"DysonSphereBox/helper/quant"
	"math"
)

// denseSpatialRowCounts 职责：将已核验容量组拆为指定跨度的子组；使用：原密铺候选均分，填洞候选可按研究站完整堆叠列优先填满；边界：每个原容量组独立守恒，层数仍来自真实端口能力。
func denseSpatialRowCounts(production quant.Production, recipe quant.Recipe, setting Setting, counts []int, maxSpan float64, wholeColumns bool) ([]int, error) {
	reference := referenceLowRecipe(production, recipe) && referenceSingleArmsSufficient(production, recipe, setting.InserterLv)
	return denseSpatialRowCountsForTemplate(production, recipe, setting, counts, maxSpan, wholeColumns, reference)
}

// denseSpatialRowCountsForTemplate 职责：按实际启用的轨道拓扑拆行；使用：低层参考轨与地面轨候选共用；边界：地面单排不能沿用参考双排的翻倍行预算。
func denseSpatialRowCountsForTemplate(production quant.Production, recipe quant.Recipe, setting Setting, counts []int, maxSpan float64, wholeColumns, reference bool) ([]int, error) {
	paired := densePairRecipe(production, recipe) && densePairPortsSupported(production, recipe, setting)
	pitch, err := denseLineSpacing(production.MachineType, setting.extraSpace(production.MachineType))
	if err != nil {
		return nil, err
	}
	if reference {
		pitch, err = referenceLineSpacing(production.MachineType, setting.extraSpace(production.MachineType))
		if err != nil {
			return nil, err
		}
	}
	columns := max(1, int(math.Floor(maxSpan/pitch))+1)
	result := []int{}
	for _, count := range counts {
		limit := columns
		if reference || paired {
			limit *= 2
		}
		if !reference && !paired && production.MachineType != 6 {
			if maximum := setting.maxCount(production.MachineType); maximum > 0 {
				limit = min(limit, maximum)
			}
			if setting.MaxMachineInALine > 0 {
				limit = min(limit, setting.MaxMachineInALine)
			}
			limit = max(1, limit)
		}
		if production.MachineType == 6 {
			ports := planMachineInserters(production, recipe, setting, count)
			if ports.capacityError != nil {
				return nil, ports.capacityError
			}
			limit = columns * max(1, ports.layers)
		}
		parts := (count + limit - 1) / limit
		for i := 0; i < parts; i++ {
			part := count / parts
			if i < count%parts {
				part++
			}
			if wholeColumns && production.MachineType == 6 {
				// 仅填洞候选按完整堆叠列优先装满，避免均分后每个短组都新增一根不满列。
				part = min(limit, count-i*limit)
			}
			result = append(result, part)
		}
	}
	return result, nil
}
