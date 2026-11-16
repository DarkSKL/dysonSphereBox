package build

// 文件职责：分派普通/超密度配送全场布局，保留内部 mode5 的独立布局入口。
// 使用：配送模式在物料图和设备参数核验后调用；边界：公开 mode2 不退回旧逐行配送布局。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
	mode2common "DysonSphereBox/helper/blueprint/build/planning/distributor"
	"DysonSphereBox/helper/quant"
)

// flyLayoutVariant 职责：复用配送规划的空间策略字段；使用：内部mode5单次装配；边界：纯参数规则由 planning/distributor 所有。
type flyLayoutVariant = mode2common.LayoutVariant

// assembleFlyLayout 职责：把配送闭合单元独立装箱并保留分区与产线映射；使用：每个有限候选；边界：整组基线仅为可行性对照，单行位置以全体带/箱体/臂/电杆轮廓检查。
func (b *builder) assembleFlyLayout(ctx context.Context, req Request, dataset *quant.Dataset, variant flyLayoutVariant) error {
	b.flyLayoutHeight = variant.HeightLimit
	zones := planZones(req.Plan)
	rank := make(map[int]int, len(b.flowGraph.ProductionOrder))
	for index, owner := range b.flowGraph.ProductionOrder {
		rank[owner] = index
	}
	const gap = 2.
	zoneX, completed := 0., 0
	for zoneIndex := range zones {
		zone := &zones[zoneIndex]
		sort.SliceStable(zone.ProductionIndices, func(i, j int) bool { return rank[zone.ProductionIndices[i]] < rank[zone.ProductionIndices[j]] })
		zoneBegin := len(b.all)
		nextX, nextY, columnWidth := zoneX, 0., 0.
		packing := &packingIndex{cells: map[[3]int][]int{}}
		for _, owner := range zone.ProductionIndices {
			if err := b.checkGeneration("配送机器行规划"); err != nil {
				return err
			}
			production := req.Plan.Productions[owner]
			recipe, err := datasetRecipeForProduction(dataset, production)
			if err != nil {
				return err
			}
			counts, err := layoutLineCounts(production, recipe, b.setting, dataset.ByID[production.MachineID].Speed, req.Density)
			if err != nil {
				return err
			}
			b.owner = owner
			completed++
			reportProgress(ctx, "production", "正在紧凑排布"+dataset.ByID[production.MainItemID].Name+"配送产线", completed, len(req.Plan.Productions))
			perGroup := 1
			if variant.GroupRows && req.SurfaceLayout == nil {
				perGroup = len(counts)
			}
			for groupStart := 0; groupStart < len(counts); groupStart += perGroup {
				begin, lineY := len(b.all), 0.
				for index := groupStart; index < min(groupStart+perGroup, len(counts)); index++ {
					row, err := b.buildLine(production, recipe, counts[index], req.Mode, 0, true)
					if err != nil {
						return err
					}
					low, high := bounds(row.all)
					shift(row.all, -low.x, lineY-low.y, 0)
					lineY += math.Ceil(high.y-low.y) + 1
				}
				block := b.all[begin:]
				low, high := bounds(block)
				width, height := math.Ceil(high.x-low.x)+1, math.Ceil(high.y-low.y)+1
				if b.setting.MergeType == 2 && nextY > 0 && nextY+height > variant.HeightLimit {
					nextX, nextY, columnWidth = nextX+columnWidth+gap, 0, 0
				}
				x, y := nextX, nextY
				if variant.Compact {
					// 配送行内部没有悬空的物理端口，无须额外预留直连模式的竖直寻路柱。
					contour, err := b.connectedBlockShape(ctx, &transportRow{}, block)
					if err != nil {
						return err
					}
					x, y, err = packing.place(ctx, contour, low, high, nextX, nextY, variant.HeightLimit)
					if err != nil {
						return err
					}
				}
				shift(block, x-low.x, y-low.y, 0)
				b.previewCheckpoint("layout")
				b.flyLayoutBlocks = append(b.flyLayoutBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
				nextY = math.Max(nextY, y+height+gap)
				columnWidth = math.Max(columnWidth, x-nextX+width)
				if req.SurfaceLayout != nil {
					b.surfaceBlocks = append(b.surfaceBlocks, surfaceBlockRange{begin: begin, end: len(b.all)})
				}
				if len(b.all) > blueprint.MaxBuildingCount {
					return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "配送机器和端点超过20万建筑，请拆分目标"}
				}
			}
			if len(recipe.Results) > 1 {
				b.warnings = append(b.warnings, fmt.Sprintf("配方 %d 的各产物已从机器端过滤并独立分带；剩余副产物仍需持续取走，避免设备自身满仓停产", recipe.ID))
			}
		}
		if len(b.all) > zoneBegin {
			_, high := bounds(b.all[zoneBegin:])
			zoneX = math.Ceil(high.x) + 1 + gap
		}
		b.zoneRanges = append(b.zoneRanges, zoneRange{zone: zoneIndex, begin: zoneBegin, end: len(b.all)})
	}
	if len(zones) > 1 {
		b.warnings = append(b.warnings, fmt.Sprintf("多目标保留%d个分区；共享中间产物只生成一份，配送边按规范逐边分配；外部输入和副产物接口见物流诊断", len(zones)))
	}
	b.zones = zones
	if err := b.prepareFlyLeftovers(ctx, req.Mode); err != nil {
		return err
	}
	b.previewCheckpoint("routing")
	return nil
}
