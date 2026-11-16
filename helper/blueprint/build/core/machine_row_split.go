package build

// 文件职责：在每个已核验容量组内共用保守机器拆分算法。
// 使用：普通单排、密铺单排/双排、化工和炼油模板；边界：原组独立守恒，行上限由实际模板给出，不扩大运输能力。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"math"
)

func splitCapacityGroups(counts []int, limitFor func(int) (int, error), whole bool) ([]int, error) {
	result := make([]int, 0, len(counts))
	for _, count := range counts {
		if count < 1 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "容量组缺少真实机器"}
		}
		limit, err := limitFor(count)
		if err != nil {
			return nil, err
		}
		if limit < 1 {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "实际机器模板没有可用列容量"}
		}
		parts := (count + limit - 1) / limit
		for index := 0; index < parts; index++ {
			n := count / parts
			if index < count%parts {
				n++
			}
			if whole {
				n = min(limit, count-index*limit)
			}
			result = append(result, n)
		}
	}
	return result, nil
}

// singleRowCounts 职责：按普通单排真实机器中心间距拆行；使用：共同空间规划；边界：研究站按真实端口允许层数计算地面列，不借用密铺双排容量。
func singleRowCounts(p quant.Production, r quant.Recipe, setting Setting, counts []int, maxSpan float64) ([]int, error) {
	facts, err := loadAssemblyFacts()
	if err != nil {
		return nil, err
	}
	geometry, ok := facts.geometry[p.MachineType]
	if !ok {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "普通行缺少机器几何事实"}
	}
	pitch := machineSpacing(p.MachineType, geometry, setting.extraSpace(p.MachineType))
	columns := max(1, int(math.Floor(maxSpan/pitch))+1)
	return splitCapacityGroups(counts, func(count int) (int, error) {
		limit := columns
		if p.MachineType == 6 {
			ports := planMachineInserters(p, r, setting, count)
			if ports.capacityError != nil {
				return 0, ports.capacityError
			}
			return columns * max(1, ports.layers), nil
		}
		if value := setting.maxCount(p.MachineType); value > 0 {
			limit = min(limit, value)
		}
		if setting.MaxMachineInALine > 0 {
			limit = min(limit, setting.MaxMachineInALine)
		}
		return max(1, limit), nil
	}, false)
}
