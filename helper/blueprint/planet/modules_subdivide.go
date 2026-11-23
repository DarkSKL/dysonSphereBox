package planet

// 文件职责：在已经选定的星球内试摆多原料制造块；使用：跨星制造装配；边界：保持原分工、整数设备和额定产耗。

import (
	"context"
	"fmt"
	"slices"

	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// subdivideWideRecipes 将多路进料或裂解回氢工区试分为八机模板，返回施工小块对应的原分工下标。
func subdivideWideRecipes(ctx context.Context, site industryplan.PlanetPlan) (industryplan.PlanetPlan, []int, error) {
	wide := func(p quant.Production) bool {
		return (len(p.Inputs) >= 7 || p.RecipeID == 58) && p.MachineCount > 8 && p.Capacity != nil
	}
	if !slices.ContainsFunc(site.Plan.Productions, wide) {
		return site, nil, nil
	}
	rows := make([]quant.Production, 0, len(site.Plan.Productions))
	parents := make([]int, 0, len(site.Plan.Productions))
	ids := make([]string, 0, len(site.BlockIDs))
	for i, p := range site.Plan.Productions {
		pieces := []quant.Production{p}
		if wide(p) {
			line, err := manufacturingMaterial(ctx, site.Plan, pieces, i)
			if err != nil {
				return site, nil, err
			}
			parts, err := industryplan.SplitProductions(ctx, line, 8)
			if err != nil {
				return site, nil, err
			}
			pieces = parts.Productions
		}
		for part, row := range pieces {
			rows = append(rows, row)
			parents = append(parents, i)
			if i < len(site.BlockIDs) {
				id := site.BlockIDs[i]
				if len(pieces) > 1 {
					id = fmt.Sprintf("%s/工区%d", id, part+1)
				}
				ids = append(ids, id)
			}
		}
	}
	site.Plan.Productions, site.BlockIDs = rows, ids
	graph, err := quant.BuildFlowGraphContext(ctx, site.Plan)
	if err != nil {
		return site, nil, err
	}
	site.Plan.FlowGraph = &graph
	return site, parents, nil
}
