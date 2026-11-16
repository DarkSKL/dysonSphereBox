package build

// 文件职责：分别计算生产容量证明、最低运输档位、指定档位容量证明与原生工作量。
// 使用：生产工作流逐项规则工具；边界：最低档位计算只推导数值下界，不执行布局候选；明确档位校验不自动升档。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// ValidateInputProductionCapacity 职责：核验每个生产项的真实设备能力、物料端点和本地回流能力；使用：独立生产容量规则步骤；边界：不改机器数，不把净流端点当本地回收。
func ValidateInputProductionCapacity(ctx context.Context, production *ProductionInputFacts) (*ProductionCapacityValidation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if production == nil || production.fingerprint == "" || production.facts == nil || production.facts.dataset == nil || production.policy == nil || production.source == nil || production.source.fingerprint == "" {
		return nil, inputStageError("生产容量核验前缺少完整生产事实")
	}
	dataset := production.facts.dataset
	reportProgress(ctx, "capacity", "正在核验物料与设备端口容量", 0, 0)
	observations := make([]productionCapacityObservation, 0, len(production.request.Plan.Productions))
	for _, p := range production.request.Plan.Productions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateProductionCapacity(p, dataset, production.request.Setting); err != nil {
			return nil, inputStageCause(ctx, err)
		}
		recipe, err := datasetRecipeForProduction(dataset, p)
		if err != nil {
			return nil, inputStageCause(ctx, err)
		}
		recycle := recipeHasMachineRecycle(recipe)
		if !production.policy.supportsMachineRecycle && recycle {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 需要本地毛流回收，但当前装配策略尚未实现；净流物流端点不能代替实际回收链", p.RecipeID)}
		}
		observations = append(observations, productionCapacityObservation{RecipeID: p.RecipeID, MachineID: p.MachineID, MachineCount: p.MachineCount, ExactMachineCount: p.ExactMachineCount, InputKinds: len(p.Inputs), OutputKinds: len(p.Outputs), LocalRecycle: recycle})
	}
	identity, err := makeInputStageIdentity(ctx, "production-capacity-validation", observations, production.fingerprint)
	if err != nil {
		return nil, err
	}
	return &ProductionCapacityValidation{inputStageIdentity: identity, productionID: production.fingerprint, observations: observations}, nil
}

// PlanMinimumInputTransport 职责：计算满足单机端口和带毛流的最低运输档位；使用：生产容量规则后；边界：只生成实际档位计划，不运行布局、档位候选或签发后续容量证明。
func PlanMinimumInputTransport(ctx context.Context, production *ProductionInputFacts, capacity *ProductionCapacityValidation) (*InputTransportPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if production == nil || production.fingerprint == "" || production.facts == nil || production.facts.dataset == nil || capacity == nil || capacity.fingerprint == "" || capacity.productionID != production.fingerprint {
		return nil, inputStageError("最低运输档位缺少同版本生产容量证明")
	}
	setting, err := upgradeProductionTransport(ctx, production.request.Plan, production.facts.dataset, production.request.Setting)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	for {
		err = inputBoundaryCapacity(production.request, setting)
		if err == nil {
			break
		}
		belt, _ := transportUpgradeNeeds(err)
		if !belt || setting.BeltLv >= 3 {
			return nil, inputStageCause(ctx, err)
		}
		setting.BeltLv++
		setting, err = normalizeSetting(setting)
		if err != nil {
			return nil, inputStageCause(ctx, err)
		}
	}
	request, err := cloneInputRequest(production.request)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	request.Setting = setting
	identity, err := makeInputStageIdentity(ctx, "minimum-transport-plan", inputRequestIdentity(request), production.fingerprint, capacity.fingerprint)
	if err != nil {
		return nil, err
	}
	identity.bytes += production.bytes + capacity.bytes
	return &InputTransportPlan{inputStageIdentity: identity, request: request, dataset: production.facts.dataset, policy: production.policy, production: production, capacity: capacity}, nil
}

// validInputTransportPlan 职责：核对档位计划具有真实来源；使用：容量与工作量阶段；边界：不代替执行容量规则。
func validInputTransportPlan(plan *InputTransportPlan) bool {
	if plan == nil || plan.fingerprint == "" || plan.dataset == nil || plan.policy == nil {
		return false
	}
	if plan.base != nil {
		return plan.base.fingerprint != "" && plan.base.dataset == plan.dataset && plan.base.graph != nil
	}
	return plan.production != nil && plan.production.fingerprint != "" && plan.capacity != nil && plan.capacity.fingerprint != "" && plan.capacity.productionID == plan.production.fingerprint
}

// ValidateInputTransportCapacity 职责：独立校验一个确定档位的分拣器及传送带容量并保存真实拆行结果；使用：初次或运输再准备工作流；边界：不升档、不重载事实、不重建守恒图。
func ValidateInputTransportCapacity(ctx context.Context, plan *InputTransportPlan) (*TransportCapacityValidation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validInputTransportPlan(plan) {
		return nil, inputStageError("运输容量规则缺少完整明确档位计划")
	}
	if err := inputBoundaryCapacity(plan.request, plan.request.Setting); err != nil {
		return nil, inputStageCause(ctx, err)
	}
	observations := make([]transportCapacityObservation, 0, len(plan.request.Plan.Productions))
	for _, p := range plan.request.Plan.Productions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		recipe, err := datasetRecipeForProduction(plan.dataset, p)
		if err != nil {
			return nil, inputStageCause(ctx, err)
		}
		if err := validateMachineInserterCapacity(p, recipe, plan.request.Setting); err != nil {
			return nil, inputStageCause(ctx, err)
		}
		counts, err := lineCounts(p, recipe, plan.request.Setting, plan.dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return nil, inputStageCause(ctx, err)
		}
		observations = append(observations, transportCapacityObservation{RecipeID: p.RecipeID, MachineID: p.MachineID, Counts: append([]int(nil), counts...)})
	}
	identity, err := makeInputStageIdentity(ctx, "transport-capacity-validation", observations, plan.fingerprint)
	if err != nil {
		return nil, err
	}
	return &TransportCapacityValidation{inputStageIdentity: identity, planID: plan.fingerprint, observations: observations}, nil
}

// EstimateInputWorkload 职责：按明确档位估计机器与中转缓存模块工作量；使用：原生选择前的独立预算步骤；边界：不签发容量证明，不替代真实几何或改变产量。
func EstimateInputWorkload(ctx context.Context, plan *InputTransportPlan) (*InputWorkloadFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !validInputTransportPlan(plan) {
		return nil, inputStageError("工作量估计缺少完整明确档位计划")
	}
	workload, err := denseNativeWorkload(ctx, plan.request, plan.dataset)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "native-workload-facts", workload, plan.fingerprint)
	if err != nil {
		return nil, err
	}
	return &InputWorkloadFacts{inputStageIdentity: identity, planID: plan.fingerprint, workload: workload}, nil
}
