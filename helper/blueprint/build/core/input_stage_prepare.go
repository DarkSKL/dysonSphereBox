package build

// 文件职责：把请求归一、事实载入、来源核对、生产配置复核和守恒图分别实现为真实阶段。
// 使用：生产工作流逐步调用并提交封闭产物；边界：不检查运输档位、不创建装配器、不补跑省略的前置规则。

import (
	"context"
	"encoding/json"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// NormalizeInputRequest 职责：独立化请求并规范设置、密度、球面参数和模式能力；使用：输入准备首阶段；边界：NaN设置保持GEN_E_LAYOUT，尚未信任Plan版本或能力。
func NormalizeInputRequest(ctx context.Context, request Request) (*NormalizedInputRequest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.search == nil {
		defaults := DefaultLayoutSearchPolicy()
		request.search = &defaults
	} else if err := request.search.Validate(); err != nil {
		return nil, err
	}
	if request.shape == nil {
		defaults := DefaultLayoutShapePolicy()
		request.shape = &defaults
	}
	setting, err := normalizeSetting(request.Setting)
	if err != nil {
		return nil, err
	}
	setting.RequestStationWarpers = setting.RequestStationWarpers && request.Mode == 4
	request.Setting = setting
	snapshot, err := cloneInputRequest(request)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	snapshot.Density = normalizeDensity(snapshot.Density)
	policy, err := newAssemblyPolicy(snapshot.Mode, snapshot.Density, snapshot.shape, snapshot.search)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	snapshot.SurfaceLayout, err = normalizeSurfaceLayout(snapshot.SurfaceLayout)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "request-normalization", inputRequestIdentity(snapshot))
	if err != nil {
		return nil, err
	}
	return &NormalizedInputRequest{inputStageIdentity: identity, request: snapshot, policy: policy}, nil
}

// RequestSnapshot 职责：返回规范请求的独立副本；使用：诊断与任务输入映射；边界：修改返回值不能污染阶段或取得后续校验证据。
func (request *NormalizedInputRequest) RequestSnapshot() (Request, error) {
	if request == nil || request.fingerprint == "" {
		return Request{}, inputStageError("缺少请求归一产物")
	}
	return cloneInputRequest(request.request)
}

// LoadInputDatasetFacts 职责：载入并独立复制设备及配方事实；使用：来源规则前的数据阶段；边界：不返回全局quant缓存，不核准Plan来源。
func LoadInputDatasetFacts(ctx context.Context, request *NormalizedInputRequest) (*InputDatasetFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || request.fingerprint == "" {
		return nil, inputStageError("载入数据事实前缺少规范请求")
	}
	dataset, err := quant.LoadDataset(request.request.Plan.Dataset)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	raw, err := json.Marshal(dataset)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	var copy quant.Dataset
	if err = json.Unmarshal(raw, &copy); err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "dataset-facts", &copy, request.fingerprint)
	if err != nil {
		return nil, err
	}
	identity.bytes += request.bytes
	return &InputDatasetFacts{inputStageIdentity: identity, source: request, dataset: &copy}, nil
}

// ValidateInputPlanSource 职责：核对PlanID、数据版本和非空制造集合并绑定真实来源；使用：独立校验工作流步骤；边界：不运行生产能力或流图检查。
func ValidateInputPlanSource(ctx context.Context, request *NormalizedInputRequest, facts *InputDatasetFacts) (*PlanSourceValidation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || request.fingerprint == "" || facts == nil || facts.fingerprint == "" || facts.dataset == nil || facts.source == nil || facts.source.fingerprint != request.fingerprint {
		return nil, inputStageError("计划来源核对缺少匹配请求与数据事实")
	}
	plan := request.request.Plan
	if plan.DataVersion != facts.dataset.Info.DataVersion || plan.PlanID == "" {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "数据集已变化，请重新量化"}
	}
	if len(plan.Productions) == 0 {
		return nil, &blueprint.Error{Code: "BP_E_EMPTY", Message: "当前方案没有可生成的制造产线"}
	}
	result := &PlanSourceValidation{requestID: request.fingerprint, datasetID: facts.fingerprint, planID: plan.PlanID, dataVersion: plan.DataVersion, productions: len(plan.Productions)}
	identity, err := makeInputStageIdentity(ctx, "plan-source-validation", struct {
		PlanID, DataVersion string
		Productions         int
	}{result.planID, result.dataVersion, result.productions}, request.fingerprint, facts.fingerprint)
	if err != nil {
		return nil, err
	}
	result.inputStageIdentity = identity
	return result, nil
}

// NormalizeInputProductions 职责：以独立原型事实重算配置满负荷能力及精确设备数；使用：来源证明后的生产复核步骤；边界：必须有同版本来源证据，不改原Plan或制造设备数量。
func NormalizeInputProductions(ctx context.Context, request *NormalizedInputRequest, facts *InputDatasetFacts, source *PlanSourceValidation) (*ProductionInputFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || request.fingerprint == "" || facts == nil || facts.fingerprint == "" || facts.dataset == nil || source == nil || source.fingerprint == "" || source.requestID != request.fingerprint || source.datasetID != facts.fingerprint || facts.source == nil || facts.source.fingerprint != request.fingerprint {
		return nil, inputStageError("生产配置复核缺少同版本来源证明")
	}
	snapshot, err := cloneInputRequest(request.request)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	snapshot.Plan, err = normalizeProductionRateBasis(ctx, snapshot.Plan, facts.dataset)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "production-rate-facts", inputRequestIdentity(snapshot), request.fingerprint, facts.fingerprint, source.fingerprint)
	if err != nil {
		return nil, err
	}
	identity.bytes += facts.bytes
	return &ProductionInputFacts{inputStageIdentity: identity, request: snapshot, facts: facts, source: source, policy: request.policy}, nil
}

// BuildInputFlowFacts 职责：从重算生产账建立守恒流图与强连通分量；使用：生产配置后的独立流图步骤；边界：不信任调用者FlowGraph，不检查运输档位或实体。
func BuildInputFlowFacts(ctx context.Context, production *ProductionInputFacts) (*InputFlowFacts, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if production == nil || production.fingerprint == "" || production.source == nil || production.source.fingerprint == "" {
		return nil, inputStageError("构建流图前缺少生产事实及来源证明")
	}
	reportProgress(ctx, "flow", "正在构建守恒物料图与强连通分量", 0, 0)
	graph, err := quant.BuildFlowGraphContext(ctx, production.request.Plan)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	recordPerformanceFlow(ctx, graph)
	raw, err := json.Marshal(graph)
	if err != nil {
		return nil, inputStageCause(ctx, err)
	}
	var copy quant.FlowGraph
	if err = json.Unmarshal(raw, &copy); err != nil {
		return nil, inputStageCause(ctx, err)
	}
	identity, err := makeInputStageIdentity(ctx, "flow-facts", &copy, production.fingerprint)
	if err != nil {
		return nil, err
	}
	identity.bytes += production.bytes
	return &InputFlowFacts{inputStageIdentity: identity, production: production, graph: &copy}, nil
}
