package generator

// 文件职责：注册逐项输入准备工具，声明真实读写与证据依赖。
// 使用：主registry追加inputToolSpecs；边界：工具不调用引擎或旧整段Prepare，输入规则不授予工厂实体的最终验证权限。

import (
	core "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// inputToolSpecs 职责：绑定独立inputTools方法和固定阶段键；使用：生产registry初始化；边界：同一验证/预算方法用不同不可变receiver区分初次与运输候选，不能动态切换权限。
func inputToolSpecs(a artifacts, implementation string) []workflow.ToolSpec {
	stages := a.inputStages
	initial := &inputTools{keys: a, plan: stages.minimumTransport, capacity: stages.transportCapacity, workload: stages.workload}
	retry := &inputTools{keys: a, plan: stages.retryPlan, capacity: stages.retryCapacity, workload: stages.retryWorkload}
	one := func(key workflow.Artifact) []workflow.Artifact { return []workflow.Artifact{key} }
	spec := func(id, method string, receiver *inputTools, reads, writes []workflow.Artifact) workflow.ToolSpec {
		return workflow.ToolSpec{ID: id, Method: method, Receiver: receiver, Implementation: implementation, Reads: reads, Requires: reads, Writes: writes, Provides: writes, FailureCodes: solutionFailures, Role: workflow.Transform}
	}
	result := []workflow.ToolSpec{
		spec("input.normalize@1", "NormalizeRequest", initial, one(a.request.Ref()), one(stages.normalized.Ref())),
		spec("input.load-data@1", "LoadDataset", initial, one(stages.normalized.Ref()), one(stages.dataset.Ref())),
		spec("input.validate-source@1", "ValidateSource", initial, []workflow.Artifact{stages.normalized.Ref(), stages.dataset.Ref()}, one(stages.source.Ref())),
		spec("input.production-rates@1", "NormalizeProductions", initial, []workflow.Artifact{stages.normalized.Ref(), stages.dataset.Ref(), stages.source.Ref()}, one(stages.production.Ref())),
		spec("input.build-flow@1", "BuildFlow", initial, one(stages.production.Ref()), one(stages.flow.Ref())),
		spec("input.validate-production@1", "ValidateProduction", initial, one(stages.production.Ref()), one(stages.productionCapacity.Ref())),
		spec("input.minimum-transport@1", "MinimumTransport", initial, []workflow.Artifact{stages.production.Ref(), stages.productionCapacity.Ref()}, one(stages.minimumTransport.Ref())),
		spec("input.validate-transport@1", "ValidateTransport", initial, one(stages.minimumTransport.Ref()), one(stages.transportCapacity.Ref())),
		spec("input.estimate-workload@1", "EstimateWorkload", initial, one(stages.minimumTransport.Ref()), one(stages.workload.Ref())),
		spec("input.compose-production@1", "ComposeProduction", initial, []workflow.Artifact{stages.flow.Ref(), stages.productionCapacity.Ref(), stages.minimumTransport.Ref(), stages.transportCapacity.Ref(), stages.workload.Ref()}, one(a.base.Ref())),
		spec("input.apply-transport@1", "ApplyTransport", retry, []workflow.Artifact{a.base.Ref(), a.transportSpecs.Ref()}, one(stages.retryPlan.Ref())),
		spec("input.validate-transport-candidate@1", "ValidateTransport", retry, one(stages.retryPlan.Ref()), one(stages.retryCapacity.Ref())),
		spec("input.estimate-transport-workload@1", "EstimateWorkload", retry, one(stages.retryPlan.Ref()), one(stages.retryWorkload.Ref())),
		spec("input.compose-transport@1", "ComposeTransport", retry, []workflow.Artifact{a.base.Ref(), stages.retryPlan.Ref(), stages.retryCapacity.Ref(), stages.retryWorkload.Ref()}, one(a.transport.Ref())),
	}
	// 初次阶段在最后一个真实消费者完成后消费私有产物；版本墓碑保留依赖，禁止后续读取已释放值。
	result[0].Defaults = defaults(core.DefaultLayoutShapePolicy())
	result[0].Validate = workflow.ArgumentValidator(core.LayoutShapePolicy.Validate)
	result[0].Consumes = one(a.request.Ref())
	result[3].Consumes = append([]workflow.Artifact(nil), result[3].Requires...)
	result[6].Consumes = one(stages.production.Ref())
	result[9].Consumes = append([]workflow.Artifact(nil), result[9].Requires...)
	result[13].Consumes = []workflow.Artifact{stages.retryPlan.Ref(), stages.retryCapacity.Ref(), stages.retryWorkload.Ref()}
	return result
}
