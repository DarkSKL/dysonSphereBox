package build

// 文件职责：将物流塔具名布局意图解析成实际装配规格，并按完整策略稳定去重。
// 使用：普通与密铺塔枚举共用；边界：不试装、不调度、不编码，不保留整数变体兼容入口。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/quant"
)

// resolveStationPlan 职责：按请求事实解析一个具名布局的有效几何；使用：候选枚举唯一解析入口；边界：只读配方与父上下文，不改变布局的拓扑修饰或高度选择。
func (b *builder) resolveStationPlan(ctx context.Context, req Request, plan stationCandidatePlan, recipes []quant.Recipe, recycle bool) StationCandidateSpec {
	spec := plan.spec
	ctx = spec.algorithmContext(ctx, req)
	if recycle {
		spec.Refine = plan.refineWithRecycle
		if !spec.MaterialOrder {
			spec.MaxSpan = plan.recycleSpan
		}
	}
	if b.setting.EnableIntermediateBuffer && !spec.MaterialOrder {
		spec.MaxSpan = 32
	}
	// 街道资格依赖当前策略上下文；共用请求配方，但不能复用其它候选的资格结果。
	probe := &builder{setting: b.setting, policy: b.policy, generationCtx: ctx}
	streetSupported := false
	for index, recipe := range recipes {
		streetSupported = streetSupported || probe.denseStationStreetEligible(req.Plan.Productions[index], recipe)
	}
	spec.Street = spec.Street && streetSupported
	if spec.MaterialOrder {
		scale := math.Max(1, math.Sqrt(float64(req.Plan.MachineCount)/96))
		spec.LatitudeSpan = math.Ceil(plan.latitudeBase*scale/4) * 4
		spec.LatitudeSpan = denseLatitudeHeightLimit(ctx, spec.LatitudeSpan)
		zone, _ := ctx.Value(densityLatitudeKey{}).(string)
		selection, selected := densityLatitudeSelectionFor(zone)
		expand, _ := ctx.Value(denseAdjacentLatitudeKey{}).(bool)
		if selected && selection.tier >= 3 && plan.latitudeBase == 24 && !expand {
			spec.LatitudeSpan = math.Min(spec.LatitudeSpan, 17)
		}
		compacted, _ := ctx.Value(denseSpatialCompactKey{}).(bool)
		if scale > 1 && !compacted {
			spec.MaxSpan = 32
		}
	}
	if span, _ := ctx.Value(denseSpatialHolesKey{}).(float64); span > 0 {
		spec.MaxSpan = 32
	}
	if plan.explicitRowSpan > 0 {
		spec.MaxSpan = plan.explicitRowSpan
	}
	if !spec.Street {
		spec.Rotation, spec.Groups = 0, 0
	}
	return spec
}

// stationCandidateSpecs 职责：规范化布局意图并保留首个完整有效规格；使用：普通与密铺候选枚举；边界：请求内配方只解析一次，不能按机器数、面积或诊断文字合并不同策略。
func (b *builder) stationCandidateSpecs(ctx context.Context, req Request, dataset *quant.Dataset, scope string, plans []stationCandidatePlan) ([]StationCandidateSpec, error) {
	normalized, inactive := b.normalizeStationPlans(ctx, req, plans)
	recipes := make([]quant.Recipe, len(req.Plan.Productions))
	recycle := false
	for index, production := range req.Plan.Productions {
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return nil, err
		}
		recipes[index] = recipe
		recycle = recycle || recipeHasMachineRecycle(recipe)
	}
	specs := make([]StationCandidateSpec, 0, len(normalized))
	for _, plan := range normalized {
		specs = append(specs, b.resolveStationPlan(ctx, req, plan, recipes, recycle))
	}
	unique := candidates.StableUnique(specs, func(spec StationCandidateSpec) StationCandidateSpec { return spec })
	recordCandidatePlans(ctx, scope, len(plans), len(unique), inactive)
	return unique, nil
}
