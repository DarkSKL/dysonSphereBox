package generator

// 文件职责：定义输入工作流的独立强类型阶段键。
// 使用：主目录嵌入inputArtifacts，阶段工具提交同版本事实；边界：只复制封闭核心句柄，不暴露事实缓存，不在克隆或预算中运行规则。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// inputArtifacts 职责：持有初次输入及运输再准备的真实阶段产物键；使用：独立输入工具；边界：两种作用域的计划/证明/工作量分键，不能互相覆盖。
type inputArtifacts struct {
	// normalized/dataset/source 职责：绑定规范请求、独立事实和来源证明；使用：生产配置复核；边界：不包含完成布尔值。
	normalized workflow.ArtifactKey[*core.NormalizedInputRequest]
	dataset    workflow.ArtifactKey[*core.InputDatasetFacts]
	source     workflow.ArtifactKey[*core.PlanSourceValidation]
	// production/flow/productionCapacity 职责：绑定重算配置、守恒图和生产容量证明；使用：最低档位及最终组合；边界：只能读取当前工作区。
	production         workflow.ArtifactKey[*core.ProductionInputFacts]
	flow               workflow.ArtifactKey[*core.InputFlowFacts]
	productionCapacity workflow.ArtifactKey[*core.ProductionCapacityValidation]
	// minimumTransport/transportCapacity/workload 职责：绑定初次最低档位及相关证明；使用：写production.base；边界：不复用运输候选键。
	minimumTransport  workflow.ArtifactKey[*core.InputTransportPlan]
	transportCapacity workflow.ArtifactKey[*core.TransportCapacityValidation]
	workload          workflow.ArtifactKey[*core.InputWorkloadFacts]
	// retryPlan/retryCapacity/retryWorkload 职责：绑定一个明确运输候选；使用：写production.transport；边界：不重载数据或流图。
	retryPlan     workflow.ArtifactKey[*core.InputTransportPlan]
	retryCapacity workflow.ArtifactKey[*core.TransportCapacityValidation]
	retryWorkload workflow.ArtifactKey[*core.InputWorkloadFacts]
}

// newInputArtifacts 职责：注册全部封闭输入阶段的克隆与大小器；使用：主产物目录初始化；边界：调用核心Clone拒绝零值，不创建任务状态或实体。
func newInputArtifacts(all *[]workflow.Artifact) (inputArtifacts, error) {
	a := inputArtifacts{}
	var err error
	if a.normalized, err = addArtifact(all, "input.normalized", (*core.NormalizedInputRequest).Clone, (*core.NormalizedInputRequest).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.dataset, err = addArtifact(all, "input.dataset", (*core.InputDatasetFacts).Clone, (*core.InputDatasetFacts).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.source, err = addArtifact(all, "input.plan-source", (*core.PlanSourceValidation).Clone, (*core.PlanSourceValidation).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.production, err = addArtifact(all, "input.production", (*core.ProductionInputFacts).Clone, (*core.ProductionInputFacts).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.flow, err = addArtifact(all, "input.flow", (*core.InputFlowFacts).Clone, (*core.InputFlowFacts).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.productionCapacity, err = addArtifact(all, "input.production-capacity", (*core.ProductionCapacityValidation).Clone, (*core.ProductionCapacityValidation).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.minimumTransport, err = addArtifact(all, "input.minimum-transport", (*core.InputTransportPlan).Clone, (*core.InputTransportPlan).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.transportCapacity, err = addArtifact(all, "input.transport-capacity", (*core.TransportCapacityValidation).Clone, (*core.TransportCapacityValidation).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.workload, err = addArtifact(all, "input.workload", (*core.InputWorkloadFacts).Clone, (*core.InputWorkloadFacts).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.retryPlan, err = addArtifact(all, "input.transport.plan", (*core.InputTransportPlan).Clone, (*core.InputTransportPlan).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.retryCapacity, err = addArtifact(all, "input.transport.capacity", (*core.TransportCapacityValidation).Clone, (*core.TransportCapacityValidation).EstimatedBytes, false); err != nil {
		return a, err
	}
	if a.retryWorkload, err = addArtifact(all, "input.transport.workload", (*core.InputWorkloadFacts).Clone, (*core.InputWorkloadFacts).EstimatedBytes, false); err != nil {
		return a, err
	}
	return a, nil
}
