package build

// 文件职责：维护密铺塔真实候选的资格和参数枚举。
// 使用：具名候选核心读取有限计划；边界：不装配、不派发、不择优，不保留旧生成桥。

import (
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
	"context"
)

// enumerateDenseStationPlans 职责：按既有顺序扩展实际布局与拓扑策略；使用：具名候选核心；边界：不编解码编号、不装配、不择优。
func (b *builder) enumerateDenseStationPlans(ctx context.Context, req Request, dataset *quant.Dataset, input contract.StationLayoutInput) ([]stationCandidatePlan, int, error) {
	budget := 4
	for index, prepared := range input.Productions {
		if b.denseStationStreetEligible(input.Plan.Productions[index], prepared.Recipe) {
			budget = 6
			break
		}
	}
	fieldBudget := budget
	needsLocalRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return nil, 0, err
	}
	// 塔场路径尚未建立原生单元坐标；根球面候选即使完成平面接线，也会被最终编码明确拒绝。
	// 提前排除不具备目标坐标能力的候选，把同一总时限留给实际可编码的黑盒与后续单环搜索。
	if b.setting.EnableIntermediateBuffer || needsByproductPriority(req.Plan, b.flowGraph, dataset) || needsLocalRecycle || req.SurfaceLayout != nil && req.SurfaceLayout.polarRoot {
		fieldBudget = 0
	}
	includeMaterial := false
	narrowTier := false
	if req.SurfaceLayout != nil {
		selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
		narrowTier = ok && selection.tier >= 2 && selection.tier <= 5
	}
	// 窄纬中的中等规模厂也需要低纬向高度候选，避免仅按机器数量决定候选集合而回退到多塔转发。
	if req.Plan.MachineCount >= req.search.StationMaterialThreshold || narrowTier {
		includeMaterial = true
	}
	plans := stationRowPlans()
	material := stationMaterialPlans()
	if includeMaterial {
		plans = append(plans, material...)
	}
	special := material
	if !b.setting.EnableFlowMonitor {
		for index, production := range input.Productions {
			p, r := input.Plan.Productions[index], production.Recipe
			if p.MachineType == 4 && denseReferenceSource(p.MachineID) == 2317 && p.IncreasePoints == 0 && len(r.Items) == 3 && len(r.Results) == 1 && !recipeHasMachineRecycle(r) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv) {
				special = stationMaterialPlans()
				for i := range special {
					special[i].spec.ChemicalFour = true
				}
				plans = append(plans, special...)
				break
			}
		}
	}
	for index, production := range input.Productions {
		if b.denseRefineryEligible(input.Plan.Productions[index], production.Recipe) {
			special = stationMaterialPlans()
			for i := range special {
				// 炼油方案同时启用四端化工能力，保留混合工厂的既有策略。
				special[i].spec.RefineryCollinear, special[i].spec.ChemicalFour = true, true
			}
			plans = append(plans, special...)
			break
		}
	}
	// 共享原料使用独立候选；保留既有成功方案，按实际塔数、占地及线长比较完成终检的结果。
	if len(req.Plan.RawInputs) > 0 {
		for _, plan := range special {
			plan.spec.SharedInputs = true
			plans = append(plans, plan)
		}
	}
	if denseBalancedFrameEligible(ctx, req) && len(req.Plan.RawInputs) > 0 {
		balanced := plans[len(plans)-1]
		balanced.spec.BalancedFrame = true
		plans = append(plans, balanced)
		if req.Setting.EnableIntermediateBuffer {
			balanced.spec.GroundTracks = true
			plans = append(plans, balanced)
		}
	}
	// 共享出口保留最后一个特殊布局及基础物料布局，失败仍有独立容量基线。
	last := plans[len(plans)-1]
	last.spec.GroundTracks = false
	shared := stationMaterialPlans()
	for i := range shared {
		shared[i].spec.SharedInputs = len(req.Plan.RawInputs) > 0
	}
	sharedOutput := last
	sharedOutput.spec.SharedOutputs = true
	plans = append(plans, sharedOutput)
	if last != shared[0] {
		sharedOutput = shared[0]
		sharedOutput.spec.SharedOutputs = true
		plans = append(plans, sharedOutput)
	}
	if req.Density == densityDense {
		rows := stationRowPlans()
		for _, plan := range append([]stationCandidatePlan{rows[0], rows[2]}, shared...) {
			plan.spec.IndependentStations, plan.spec.SharedOutputs = true, true
			plans = append(plans, plan)
		}
		for _, plan := range shared {
			plan.spec.IndependentStations, plan.spec.Compact, plan.spec.SharedOutputs = true, true, true
			plans = append(plans, plan)
		}
		compactPlans := append([]stationCandidatePlan(nil), shared...)
		if last != shared[0] && last != shared[1] && last != shared[2] {
			compactPlans = append(compactPlans, last)
		}
		for _, plan := range compactPlans {
			plan.spec.Compact = true
			plans = append(plans, plan)
			plan.spec.SharedOutputs = true
			plans = append(plans, plan)
		}
		for _, plan := range shared {
			plan.spec.PairedOutputs = true
			plans = append(plans, plan)
			plan.spec.Compact, plan.spec.SharedOutputs = true, true
			plans = append(plans, plan)
		}
		for _, plan := range shared {
			plan.spec.ProducerSplits, plan.spec.Compact, plan.spec.SplitterModel = true, true, 39
			plans = append(plans, plan)
			plan.spec.SharedOutputs = true
			plans = append(plans, plan)
		}
		// 短行与真实接角仍各自保留，共用最长物料街道基线。
		for _, span := range []float64{20, 18} {
			eligible, err := b.denseStationHoleEligible(req, dataset, span)
			if err != nil {
				return nil, 0, err
			}
			if eligible {
				plan := shared[2]
				plan.spec.HoleSpan, plan.spec.BentRows, plan.spec.SharedOutputs = span, span == 18, true
				plans = append(plans, plan)
			}
		}
	}
	return plans, fieldBudget, nil
}
