package build

// 文件职责：只组合已经完成的输入事实与同版本规则证明。
// 使用：生产输入和运输再准备的最终组合步骤；边界：不加载数据、不重建流图、不重算配置、容量或工作量，不补跑被省略规则。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ComposePreparedInput 职责：核对全部阶段版本并组合最终只读能力输入；使用：初次输入工作流末步；边界：缺少或跨版本证明直接失败，不在此执行被省略的规则。
func ComposePreparedInput(ctx context.Context, flow *InputFlowFacts, capacity *ProductionCapacityValidation, plan *InputTransportPlan, transport *TransportCapacityValidation, workload *InputWorkloadFacts) (*PreparedInput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if flow == nil || flow.fingerprint == "" || flow.production == nil || flow.production.fingerprint == "" || flow.graph == nil || capacity == nil || capacity.fingerprint == "" || capacity.productionID != flow.production.fingerprint || !validInputTransportPlan(plan) || plan.base != nil || plan.production == nil || plan.production.fingerprint != flow.production.fingerprint || plan.capacity == nil || plan.capacity.fingerprint != capacity.fingerprint {
		return nil, inputStageError("最终输入缺少同版本流图、生产容量证明和最低档位计划")
	}
	if err := validateInputCompositionProofs(plan, transport, workload); err != nil {
		return nil, err
	}
	result := &PreparedInput{request: plan.request, requestedSetting: newFinalizationCloner().buildSetting(plan.production.facts.source.request.Setting), dataset: plan.dataset, graph: flow.graph, policy: plan.policy,
		checks: append([]LogisticsCheck(nil), newLogisticsSummary(*flow.graph, plan.request.Setting).Checks...), nativeWorkload: workload.workload}
	if err := completePreparedInputIdentity(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

// validateInputCompositionProofs 职责：共享同一明确档位的容量和工作量版本核对；使用：初次及运输再准备组合；边界：不执行容量或预算计算。
func validateInputCompositionProofs(plan *InputTransportPlan, transport *TransportCapacityValidation, workload *InputWorkloadFacts) error {
	if plan == nil || plan.fingerprint == "" || transport == nil || transport.fingerprint == "" || transport.planID != plan.fingerprint || workload == nil || workload.fingerprint == "" || workload.planID != plan.fingerprint {
		return inputStageError("输入组合缺少同档位运输容量证明或工作量事实")
	}
	return nil
}

// ComposeTransportCandidate 职责：组合一个明确运输档位及其独立规则证据；使用：运输再准备末步；边界：只复用原数据集/流图，不运行整段PrepareInput或自动升档。
func ComposeTransportCandidate(ctx context.Context, input *PreparedInput, plan *InputTransportPlan, transport *TransportCapacityValidation, workload *InputWorkloadFacts) (*PreparedInput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input == nil || input.fingerprint == "" || input.dataset == nil || input.graph == nil || !validInputTransportPlan(plan) || plan.base == nil || plan.base.fingerprint != input.fingerprint || plan.base.dataset != input.dataset || plan.base.graph != input.graph {
		return nil, inputStageError("运输组合缺少同版本原输入与档位应用产物")
	}
	if err := validateInputCompositionProofs(plan, transport, workload); err != nil {
		return nil, err
	}
	result := *input
	result.request = plan.request
	result.warnings = append([]string(nil), input.warnings...)
	result.checks = append([]LogisticsCheck(nil), input.checks...)
	result.nativeWorkload = workload.workload
	if err := completePreparedInputIdentity(ctx, &result); err != nil {
		return nil, err
	}
	if input.geometry != (SurfaceCandidateSpec{}) {
		identity, err := makeInputStageIdentity(ctx, "transport-prepared-surface", struct {
			Request  string
			Geometry SurfaceCandidateSpec
		}{result.fingerprint, result.geometry}, input.fingerprint, plan.fingerprint)
		if err != nil {
			return nil, err
		}
		result.fingerprint = identity.fingerprint
	}
	return &result, nil
}

// completePreparedInputIdentity 职责：仅建立最终输入摘要及保守大小；使用：两个组合出口；边界：序列化不是再次执行业务校验，摘要区分实际策略和选区。
func completePreparedInputIdentity(ctx context.Context, input *PreparedInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// 所有输入都包含冻结策略和私有选区，派生输入还需区分具体候选几何。
	identity := struct {
		Request  any
		Geometry SurfaceCandidateSpec
	}{inputRequestIdentity(input.request), input.geometry}
	canonical, err := json.Marshal(identity)
	if err != nil {
		return inputStageCause(ctx, err)
	}
	data, err := json.Marshal(input.dataset)
	if err != nil {
		return inputStageCause(ctx, err)
	}
	graph, err := json.Marshal(input.graph)
	if err != nil {
		return inputStageCause(ctx, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	input.fingerprint = hex.EncodeToString(digest[:])
	input.bytes = int64(len(canonical)+len(data)+len(graph))*4 + 4096
	return nil
}

// Clone 职责：复制最终输入的封闭只读句柄；使用：工作流产物边界；边界：不向调用者暴露共享可写容器，不补跑任何规则。
func (input *PreparedInput) Clone() (*PreparedInput, error) {
	if input == nil || input.fingerprint == "" || input.dataset == nil || input.graph == nil {
		return nil, inputStageError("缺少最终输入产物")
	}
	result := *input
	return &result, nil
}
