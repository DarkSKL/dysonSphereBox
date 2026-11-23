package industryplan

// 文件职责：补齐依赖存档矿源与规划偏好的量化默认值；使用：世界校验后、首次量化前；边界：显式配方、外供和已有权重始终优先。

import (
	"context"
	"maps"
	"slices"

	"DysonSphereBox/helper/quant"
)

const energyRecipeWeight = 10

// compareOilCracking 比较净余油可回用于制氢的一种原版配方，仅接受全量化物料与制造能耗同时改善的候选。
func compareOilCracking(ctx context.Context, req Request, world World, base quant.Result) (Request, quant.Result, bool) {
	if req.keepOilRecipe || req.Preference != "energy" || world.RecipeUnlocked == nil || !world.RecipeUnlocked(58) ||
		req.Quant.Scheme.ItemRecipeChoices["氢"] > 0 || !slices.ContainsFunc(base.Leftovers, func(r quant.ItemRate) bool { return r.ItemID == 1114 && r.RatePerMinute > 0 }) {
		return req, base, false
	}
	dataset, err := quant.LoadVanillaDataset(req.Quant.Dataset)
	if err != nil {
		return req, base, false
	}
	choice := slices.IndexFunc(dataset.Choices["氢"], func(i int) bool { return dataset.Recipes[i].ID == 58 })
	if choice < 0 {
		return req, base, false
	}
	trial := req
	trial.Quant.Scheme.ItemRecipeChoices = maps.Clone(req.Quant.Scheme.ItemRecipeChoices)
	if trial.Quant.Scheme.ItemRecipeChoices == nil {
		trial.Quant.Scheme.ItemRecipeChoices = map[string]int{}
	}
	trial.Quant.Scheme.ItemRecipeChoices["氢"] = choice + 1
	next, err := quant.Solve(ctx, trial.Quant)
	if err != nil || next.PowerMW >= base.PowerMW || next.MachineCount > base.MachineCount ||
		!energyCovered(base.RawInputs, next.RawInputs) || !energyCovered(base.Leftovers, next.Leftovers) ||
		!slices.ContainsFunc(next.Productions, func(p quant.Production) bool { return p.RecipeID == 58 }) {
		return req, base, false
	}
	return trial, next, true
}

// planningDefaults 职责：优先使用可核验硅矿并接通能耗偏好的配方成本；使用：规划入口；边界：只追加硅石，实际产能与自动回退仍由资源调查负责，不修改请求引用。
func planningDefaults(req Request, planets map[int64]Planet, canResolve bool) Request {
	if req.Preference == "energy" && req.Quant.Scheme.CostWeight.Power == 0 {
		req.Quant.Scheme.CostWeight.Power = energyRecipeWeight
	}
	if !enabled(req.EnableMining, true) || req.Quant.Scheme.ItemRecipeChoices["硅石"] != 0 || req.Quant.Settings.MineralizeList["硅石"] || slices.Contains(req.Quant.Settings.DirectResourceItemIDs, 1003) {
		return req
	}
	for _, planet := range planets {
		if !planet.Buildable {
			continue
		}
		for _, resource := range planet.Resources {
			if resource.ItemID != 1003 || resource.Kind != "solid" || resource.RemainingItems != nil && *resource.RemainingItems <= 0 {
				continue
			}
			known := resource.Source != "candidate" && resource.Count > 0
			candidate := resource.Source == "candidate" && !planet.ResourcesKnown && !planet.resourcesChecked && canResolve
			if known || candidate {
				req.Quant.Settings.DirectResourceItemIDs = append(slices.Clone(req.Quant.Settings.DirectResourceItemIDs), 1003)
				return req
			}
		}
	}
	return req
}
