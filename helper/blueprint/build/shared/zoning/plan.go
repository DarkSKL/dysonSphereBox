package zoning

// 文件职责：按量化产线服务的目标创建确定性分区。
// 使用：直连、配送和物流塔布局共同调用；边界：不依赖具体模式、密度或建筑状态，不修改输入。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// Plan 职责：将单目标产线分配到目标分区、多目标产线归入共享区；使用：模式布局准备；边界：单目标兼容地吸收共享产线，无目标仍保留共享区。
func Plan(plan quant.Result) []contract.Zone {
	zones := []contract.Zone{}
	byTarget := map[int]int{}
	for _, target := range plan.Targets {
		if _, ok := byTarget[target.ItemID]; ok {
			continue
		}
		byTarget[target.ItemID] = len(zones)
		zones = append(zones, contract.Zone{Key: fmt.Sprintf("target-%d", target.ItemID), TargetItemID: target.ItemID, ProductionIndices: []int{}})
	}
	shared := contract.Zone{Key: "shared", ProductionIndices: []int{}}
	for i, production := range plan.Productions {
		if len(production.TargetItemIDs) == 1 {
			if index, ok := byTarget[production.TargetItemIDs[0]]; ok {
				zones[index].ProductionIndices = append(zones[index].ProductionIndices, i)
				continue
			}
		}
		shared.ProductionIndices = append(shared.ProductionIndices, i)
	}
	result := []contract.Zone{}
	if len(shared.ProductionIndices) > 0 && len(zones) > 1 {
		result = append(result, shared)
	} else if len(shared.ProductionIndices) > 0 && len(zones) == 1 {
		zones[0].ProductionIndices = append(zones[0].ProductionIndices, shared.ProductionIndices...)
	}
	for _, zone := range zones {
		if len(zone.ProductionIndices) > 0 {
			result = append(result, zone)
		}
	}
	if len(result) == 0 {
		result = append(result, contract.Zone{Key: "shared", ProductionIndices: shared.ProductionIndices})
	}
	return result
}
