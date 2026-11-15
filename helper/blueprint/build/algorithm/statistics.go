package build

// 文件职责：连接最终建筑记录与独立分析模块；使用：唯一获选结果发布前；边界：不在搜索候选中重复求统计，不读取客户端派生报告。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/factory/statistics"
	"DysonSphereBox/helper/quant"
	"context"
	"fmt"
)

// attachFactoryStatistics 职责：复用量化能力和最终实体坐标生成报告；使用：所有运输/密度模式统一出口；边界：绑定不完整立即报错，不静默少计制造设备。
func attachFactoryStatistics(ctx context.Context, plan quant.Result, result *Result) error {
	report, err := quant.BuildStatistics(plan)
	if err != nil {
		return err
	}
	records := result.statisticsRecords
	if len(records) != result.BuildingCount || result.LogisticsSummary == nil {
		return fmt.Errorf("生成结果缺少统计所需的最终记录")
	}
	productions := make([]int, len(records))
	for i := range productions {
		productions[i] = -1
	}
	for _, binding := range result.Bindings {
		for _, index := range binding.MachineIndices {
			if index < 0 || index >= len(records) || productions[index] >= 0 {
				return fmt.Errorf("制造设备统计绑定无效")
			}
			productions[index] = binding.ProductionIndex
		}
	}
	entities := make([]statistics.Entity, 0, len(records))
	fixed := make([]blueprint.Building, 0, len(records)/4)
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		entities = append(entities, statistics.Entity{Index: int(record.Index), ItemID: int(record.ItemID), ProductionIndex: productions[i], AreaIndex: int(record.AreaIndex),
			X: float64(record.LocalOffset.X), Y: float64(record.LocalOffset.Y), Z: float64(record.LocalOffset.Z)})
		if !surfaceIsBelt(record.ItemID) && !(record.ItemID >= 2011 && record.ItemID <= 2014) {
			fixed = append(fixed, record)
		}
	}
	m := result.LogisticsSummary.Metrics
	space := statistics.Space{BuildingFootprintAreaGrid2: finalRecordFootprintArea(fixed), EnvelopeAreaGrid2: m.EnvelopeArea, OccupiedAreaGrid2: m.FootprintArea, BeltCount: m.BeltCount, TotalBeltLengthMeters: m.TotalBeltLengthMeters, AreaCount: result.Areas}
	if m.EnvelopeArea > 0 {
		space.Utilization = m.FootprintArea / m.EnvelopeArea
	}
	if err := statistics.EnrichBlueprint(ctx, report, entities, space); err != nil {
		return err
	}
	result.Statistics = report
	result.statisticsRecords = nil
	return nil
}
