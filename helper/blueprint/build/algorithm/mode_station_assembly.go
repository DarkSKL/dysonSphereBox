package build

// 文件职责：把物流塔布局计划装配成闭合机器行和塔组。
// 使用：物流塔模式 assembly 阶段调用；边界：不决定依赖顺序、机器分组策略或最终记录收尾。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
)

// assembleStationLayout 职责：按布局计划创建并放置物流塔产线块；使用：物流塔模式装配阶段；边界：建筑顺序和空间块记录供统一 finish 使用，错误时不返回部分蓝图。
func assembleStationLayout(ctx context.Context, builder *builder, request Request, dataset *quant.Dataset, input contract.StationLayoutInput, plan contract.StationLayoutPlan, config contract.StationConfig) ([]surfaceBlockRange, error) {
	const blockGap = 2.
	const zoneGap = blockGap
	zoneX, productionNumber := 0., 0
	stationBlocks := []surfaceBlockRange{}
	for zoneIndex, zone := range plan.Zones {
		zoneBegin := len(builder.all)
		nextX, nextY, columnWidth := zoneX, 0., 0.
		for _, work := range zone.Productions {
			productionNumber++
			production := input.Plan.Productions[work.Index]
			recipe := input.Productions[work.Index].Recipe
			productionMessage := "正在铺设" + dataset.ByID[production.MainItemID].Name + "产线"
			for groupStart := 0; groupStart < len(work.Counts); groupStart += work.PerGroup {
				reportProgress(ctx, "production", productionMessage, productionNumber, plan.ProductionCount)
				groupEnd := min(groupStart+work.PerGroup, len(work.Counts))
				begin := len(builder.all)
				builder.owner = work.Index
				lines := []*line{}
				lineY := 0.
				for i := groupStart; i < groupEnd; i++ {
					line, err := builder.buildLine(production, recipe, work.Counts[i], config.Mode, 0, true)
					if err != nil {
						return nil, err
					}
					minimum, maximum := bounds(line.all)
					// 外围端口在塔接线前保持整数格；局部机器和分拣器仍保留原始小数姿态。
					shift(line.all, -math.Floor(minimum.x), lineY-math.Floor(minimum.y), 0)
					builder.previewCheckpoint("assembly")
					lineY += math.Ceil(maximum.y-minimum.y) + 1
					lines = append(lines, line)
				}
				if err := builder.coatSupply(lines); err != nil {
					return nil, err
				}
				reportProgress(ctx, "stations", "正在连接物流站与传送带", productionNumber, plan.ProductionCount)
				if err := builder.attachStations(ctx, lines, config, begin); err != nil {
					return nil, err
				}
				block := builder.all[begin:]
				stationBlocks = append(stationBlocks, surfaceBlockRange{begin: begin, end: len(builder.all)})
				minimum, maximum := bounds(block)
				width := math.Ceil(maximum.x-minimum.x) + 1
				height := math.Ceil(maximum.y-minimum.y) + 1
				if builder.setting.MergeType == 2 && nextY > 0 && nextY+height > builder.setting.BlockWidthLimit {
					nextX += columnWidth + blockGap
					nextY, columnWidth = 0, 0
				}
				nextY += stationBlockClearance(builder.all[:begin], block, nextX-minimum.x, nextY-minimum.y)
				shift(block, nextX-minimum.x, nextY-minimum.y, 0)
				builder.previewCheckpoint("layout")
				nextY += height + blockGap
				columnWidth = math.Max(columnWidth, width)
				if request.SurfaceLayout != nil {
					builder.surfaceBlocks = append(builder.surfaceBlocks, surfaceBlockRange{begin: begin, end: len(builder.all)})
				}
				if len(builder.all) > blueprint.MaxBuildingCount {
					return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "建筑超过 20 万，请拆分生成"}
				}
			}
			if len(recipe.Results) > 1 {
				builder.warnings = append(builder.warnings, fmt.Sprintf("配方 %d 各产物已从机器端过滤并独立分带；剩余副产物仍需持续取走，避免设备自身满仓停产", production.RecipeID))
			}
		}
		if len(builder.all) > zoneBegin {
			_, maximum := bounds(builder.all[zoneBegin:])
			zoneX = math.Ceil(maximum.x) + 1 + zoneGap
		}
		builder.zoneRanges = append(builder.zoneRanges, zoneRange{zone: zoneIndex, begin: zoneBegin, end: len(builder.all)})
	}
	if len(plan.Zones) > 1 {
		builder.warnings = append(builder.warnings, fmt.Sprintf("多目标已按 %d 个分区排开：共用中间产物只生成一份，并按规范供需边配置各区物流；原料、目标及剩余副产物的外部接口见物流诊断", len(plan.Zones)))
	}
	builder.zones = make([]Zone, 0, len(plan.Zones))
	for _, zone := range plan.Zones {
		builder.zones = append(builder.zones, zoneFromLayout(zone.Zone))
	}
	if err := ctx.Err(); err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔装配已取消或超时"}
	}
	return stationBlocks, nil
}
