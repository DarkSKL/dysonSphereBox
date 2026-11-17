package generator

// 文件职责：把每个输入事实或规则阶段单独接入受限工具。
// 使用：input.yaml的生产准备及明确运输候选准备；边界：入口只读取声明的键、调用一个细分核心并暂存，不调用PrepareInput、PrepareTransportCandidate或引擎。

import (
	"context"
	"fmt"

	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// inputTools 职责：保存只读产物键及固定档位阶段作用域；使用：同一容量/预算方法绑定两组键；边界：无当前任务状态，receiver创建后不修改。
type inputTools struct {
	// keys 职责：提供共同请求及最终输入键；使用：单次取数暂存；边界：不保存产物值。
	keys artifacts
	// plan/capacity/workload 职责：固定初次或运输候选的阶段键；使用：共用验证与预算方法；边界：不由工具参数动态挑选权限。
	plan     workflow.ArtifactKey[*core.InputTransportPlan]
	capacity workflow.ArtifactKey[*core.TransportCapacityValidation]
	workload workflow.ArtifactKey[*core.InputWorkloadFacts]
}

// NormalizeRequest 职责：调用唯一请求/设置/模式归一核心；使用：输入子流第一步；边界：NaN和非法Mode由此正式节点报告，克隆器不提前拦截。
func (t *inputTools) NormalizeRequest(ctx context.Context, s *workflow.StepContext, args core.LayoutShapePolicy) error {
	request, err := workflow.Read(s, t.keys.request)
	if err != nil {
		return err
	}
	request, err = core.ConfigureLayoutShape(request, args)
	if err != nil {
		return fmt.Errorf("形状策略配置无效: %w", err)
	}
	value, err := core.NormalizeInputRequest(ctx, request)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.normalized, value)
}

// LoadDataset 职责：单独载入并复制设备配方事实；使用：来源校验前；边界：不核对Plan或计算产能。
func (t *inputTools) LoadDataset(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	request, err := workflow.Read(s, t.keys.inputStages.normalized)
	if err != nil {
		return err
	}
	value, err := core.LoadInputDatasetFacts(ctx, request)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.dataset, value)
}

// ValidateSource 职责：独立核对请求Plan与数据来源版本；使用：生产复核前；边界：产物是封闭来源证明，不代表容量通过。
func (t *inputTools) ValidateSource(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	request, err := workflow.Read(s, t.keys.inputStages.normalized)
	if err != nil {
		return err
	}
	data, err := workflow.Read(s, t.keys.inputStages.dataset)
	if err != nil {
		return err
	}
	value, err := core.ValidateInputPlanSource(ctx, request, data)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.source, value)
}

// NormalizeProductions 职责：单独重算真实生产配置及精确台数；使用：已有来源证明后；边界：不能省略或跨版本使用来源证明。
func (t *inputTools) NormalizeProductions(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	request, err := workflow.Read(s, t.keys.inputStages.normalized)
	if err != nil {
		return err
	}
	data, err := workflow.Read(s, t.keys.inputStages.dataset)
	if err != nil {
		return err
	}
	source, err := workflow.Read(s, t.keys.inputStages.source)
	if err != nil {
		return err
	}
	value, err := core.NormalizeInputProductions(ctx, request, data, source)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.production, value)
}

// BuildFlow 职责：单独重建守恒物料图及强连通分量；使用：生产事实后；边界：不接受请求自带图冒充重建结果。
func (t *inputTools) BuildFlow(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	production, err := workflow.Read(s, t.keys.inputStages.production)
	if err != nil {
		return err
	}
	value, err := core.BuildInputFlowFacts(ctx, production)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.flow, value)
}

// ValidateProduction 职责：独立验证生产能力及物料/本地回流资格；使用：最低档位计算前；边界：不选择或改变运输设备。
func (t *inputTools) ValidateProduction(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	production, err := workflow.Read(s, t.keys.inputStages.production)
	if err != nil {
		return err
	}
	value, err := core.ValidateInputProductionCapacity(ctx, production)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.inputStages.productionCapacity, value)
}

// MinimumTransport 职责：单独计算生产的最低运输档位计划；使用：生产容量证明后；边界：不签发确定档位容量证明或调度布局。
func (t *inputTools) MinimumTransport(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	production, err := workflow.Read(s, t.keys.inputStages.production)
	if err != nil {
		return err
	}
	capacity, err := workflow.Read(s, t.keys.inputStages.productionCapacity)
	if err != nil {
		return err
	}
	value, err := core.PlanMinimumInputTransport(ctx, production, capacity)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.plan, value)
}

// ValidateTransport 职责：校验receiver固定的一项明确档位容量；使用：初次或运输候选不同键组；边界：不自动升档，不读取另一作用域的计划或证明。
func (t *inputTools) ValidateTransport(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	plan, err := workflow.Read(s, t.plan)
	if err != nil {
		return err
	}
	value, err := core.ValidateInputTransportCapacity(ctx, plan)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.capacity, value)
}

// EstimateWorkload 职责：估计receiver固定档位的机器/缓存工作量；使用：原生预算前；边界：不代替容量规则或实体几何。
func (t *inputTools) EstimateWorkload(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	plan, err := workflow.Read(s, t.plan)
	if err != nil {
		return err
	}
	value, err := core.EstimateInputWorkload(ctx, plan)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.workload, value)
}

// ComposeProduction 职责：仅组合完整同版本阶段为生产基础输入；使用：初次输入子流末步；边界：不补跑缺失规则，不加载数据或重新构图。
func (t *inputTools) ComposeProduction(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	flow, err := workflow.Read(s, t.keys.inputStages.flow)
	if err != nil {
		return err
	}
	production, err := workflow.Read(s, t.keys.inputStages.productionCapacity)
	if err != nil {
		return err
	}
	plan, err := workflow.Read(s, t.plan)
	if err != nil {
		return err
	}
	capacity, err := workflow.Read(s, t.capacity)
	if err != nil {
		return err
	}
	workload, err := workflow.Read(s, t.workload)
	if err != nil {
		return err
	}
	value, err := core.ComposePreparedInput(ctx, flow, production, plan, capacity, workload)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.base, value)
}

// ApplyTransport 职责：只应用index指向的明确运输档位；使用：运输候选子流第一步；边界：越界不取其它规格，不重新准备整段输入。
func (t *inputTools) ApplyTransport(ctx context.Context, s *workflow.StepContext, args indexArgs) error {
	input, err := workflow.Read(s, t.keys.base)
	if err != nil {
		return err
	}
	specs, err := workflow.Read(s, t.keys.transportSpecs)
	if err != nil {
		return err
	}
	if args.Index < 0 || args.Index >= len(specs) {
		return fmt.Errorf("transport candidate index out of range")
	}
	value, err := core.ApplyTransportCandidate(ctx, input, specs[args.Index])
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.plan, value)
}

// ComposeTransport 职责：仅组合一个明确档位及独立容量/工作量为运输输入；使用：运输候选子流末步；边界：复用原事实和流图，不自动升档或运行布局。
func (t *inputTools) ComposeTransport(ctx context.Context, s *workflow.StepContext, _ emptyArgs) error {
	input, err := workflow.Read(s, t.keys.base)
	if err != nil {
		return err
	}
	plan, err := workflow.Read(s, t.plan)
	if err != nil {
		return err
	}
	capacity, err := workflow.Read(s, t.capacity)
	if err != nil {
		return err
	}
	workload, err := workflow.Read(s, t.workload)
	if err != nil {
		return err
	}
	value, err := core.ComposeTransportCandidate(ctx, input, plan, capacity, workload)
	if err != nil {
		return domainFailure(ctx, s, err)
	}
	return workflow.Stage(s, t.keys.transport, value)
}
