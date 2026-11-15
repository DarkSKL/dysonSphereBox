package build

// 文件职责：让塔候选枚举与实际装配共用同一组有效几何参数。
// 使用：普通及超密度塔在试装前稳定去重；边界：只合并完整参数相同的计划，未证明相同的容量分组、排序和塔口布局继续独立执行。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/quant"
)

// stationCandidateGeometry 职责：保存黑盒构造器真正消费的布局参数；使用：规划键与装配赋值共用；边界：高度索引保守区分，未试装前不假设两种高度相同。
type stationCandidateGeometry struct {
	// order/span/refine 职责：保存机器行及全场搜索策略；使用：赋值给候选；边界：跨度相同也不合并不同排序或微调。
	order  int
	span   float64
	refine bool
	// street/rotation/groups 职责：保存有效街道计划；使用：街道装配；边界：无合资格街道时旋转和分组均不消费。
	street   bool
	rotation int
	groups   int
	// latitude 职责：保存取整、限高后的纬向预算；使用：装箱和最终范围核验；边界：保留17格窄候选规则。
	latitude float64
	// materialOrder/consumerFirst 职责：保存物料相邻排序及供剂回调选择；使用：大厂分支；边界：消费者优先与枢纽优先不能互换。
	materialOrder bool
	consumerFirst bool
	// linearTowers/heightChoice 职责：保留塔阵列列数策略和试装高度选择；使用：边界构造及装配；边界：不能仅以机器数或限高相同省略这些差异。
	linearTowers bool
	heightChoice int
}

// stationGeometry 职责：计算塔黑盒实际参数而不装配实体；使用：候选去重与构造共用；边界：variant必须是上下文修饰解码后的0至6，未知配方沿用原错误。
func (b *builder) stationGeometry(ctx context.Context, req Request, dataset *quant.Dataset, variant int) (stationCandidateGeometry, error) {
	geometry := stationCandidateGeometry{order: variant % 4, span: []float64{18, 24, 32, 24}[variant%4], linearTowers: variant%2 == 1}
	if variant >= 2 {
		geometry.heightChoice = 1
	}
	needsRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return geometry, err
	}
	if needsRecycle {
		geometry.span = []float64{8, 8, 8, 24}[variant%4]
		geometry.refine = variant%2 == 1
	}
	if b.setting.EnableIntermediateBuffer {
		geometry.span = 32
	}
	streetSupported := false
	// 资格检查会读取generationCtx；用当前候选上下文，避免地面轨等修饰被父builder状态遮蔽。
	probe := &builder{setting: b.setting, adapter: b.adapter, generationCtx: ctx}
	for _, production := range req.Plan.Productions {
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return geometry, err
		}
		streetSupported = streetSupported || probe.denseStationStreetEligible(production, recipe)
	}
	geometry.street = variant%2 == 1 && streetSupported
	geometry.rotation = (variant / 2) % 2
	if variant >= 4 {
		choice := variant - 4
		scale := math.Max(1, math.Sqrt(float64(req.Plan.MachineCount)/96))
		geometry.latitude = math.Ceil([]float64{24, 32, 40}[choice]*scale/4) * 4
		geometry.latitude = denseLatitudeHeightLimit(ctx, geometry.latitude)
		zone, _ := ctx.Value(densityLatitudeKey{}).(string)
		selection, selected := densityLatitudeSelectionFor(zone)
		expand, _ := ctx.Value(denseAdjacentLatitudeKey{}).(bool)
		if selected && selection.tier >= 3 && choice == 0 && !expand {
			geometry.latitude = math.Min(geometry.latitude, 17)
		}
		geometry.order = 3
		geometry.span = []float64{12, 18, 32}[choice]
		compacted, _ := ctx.Value(denseSpatialCompactKey{}).(bool)
		if scale > 1 && !compacted {
			geometry.span = 32
		}
		geometry.groups = []int{0, 2, -1}[choice]
		geometry.street, geometry.rotation = streetSupported, 0
		geometry.materialOrder, geometry.consumerFirst = true, choice == 1
	}
	if span, _ := ctx.Value(denseSpatialHolesKey{}).(float64); span > 0 {
		geometry.span = 32
	}
	if !geometry.street {
		geometry.rotation, geometry.groups = 0, 0
	}
	return geometry, nil
}

// stationCandidateIdentity 职责：组合有效几何和剩余全部上下文修饰；使用：稳定去重；边界：化工、炼油、原生占地、压缩、地面轨等修饰均不能丢失。
type stationCandidateIdentity struct {
	modifiers int
	geometry  stationCandidateGeometry
}

// stationCandidateVariants 职责：先剥离无效修饰，再按实际执行参数保留首个计划；使用：普通与密铺塔派发前；边界：不试装、不缓存失败、不按实体数量或面积剪枝。
func (b *builder) stationCandidateVariants(ctx context.Context, req Request, dataset *quant.Dataset, scope string, variants []int) ([]int, error) {
	normalized, inactive := b.normalizeStationVariants(ctx, req, variants)
	keys := make(map[int]stationCandidateIdentity, len(normalized))
	for _, variant := range normalized {
		candidateCtx, base := stationBlackboxContext(ctx, req, variant)
		geometry, err := b.stationGeometry(candidateCtx, req, dataset, base)
		if err != nil {
			return nil, err
		}
		keys[variant] = stationCandidateIdentity{modifiers: variant - base, geometry: geometry}
	}
	unique := candidates.StableUnique(normalized, func(variant int) stationCandidateIdentity { return keys[variant] })
	recordCandidatePlans(ctx, scope, len(variants), len(unique), inactive)
	return unique, nil
}
