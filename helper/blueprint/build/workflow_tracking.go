package build

// 文件职责：公开工作流后台跟踪的稳定门面；使用：服务层接线与查询；边界：不改变实体预览、取消或生成结果契约。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/generator"
	"DysonSphereBox/helper/blueprint/build/workflow"
)

// WithWorkflowTracking 职责：附加独立真实执行观察器；使用：生成服务；边界：回调错误只影响跟踪完整性。
func WithWorkflowTracking(ctx context.Context, observer func(workflow.ExecutionEvent) error) context.Context {
	return generator.WithWorkflowTracking(ctx, observer)
}

// WithWorkflowStart 职责：附加实际冻结图登记钩子；使用：生成生命周期；边界：仅影响本次上下文，不执行新任务。
func WithWorkflowStart(ctx context.Context, start func(workflow.PlanDescription) error) context.Context {
	return generator.WithWorkflowStart(ctx, start)
}

// WithWorkflowFinish 职责：登记真实收尾和引擎丢失计数回调；使用：生成服务；边界：早期失败的 trace 可为空，不包含领域实体。
func WithWorkflowFinish(ctx context.Context, finish func(*workflow.RunResult, error)) context.Context {
	return generator.WithWorkflowFinish(ctx, finish)
}

// DescribeWorkflow 职责：读取当前配置图；使用：工作流后台接口；边界：历史任务读取其绑定图，不调用此方法替代。
func DescribeWorkflow() (workflow.PlanDescription, error) { return generator.DescribeWorkflow() }
