package generator

// 文件职责：提供独立于实体预览的工作流观察与冻结图旁路；使用：服务登记和主生成入口接线；边界：不读取实体、不解析日志、不另行执行工具。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/workflow"
)

// workflowTrackingKey 职责：隔离执行观察器上下文；使用：WithWorkflowTracking；边界：不覆盖算法预览订阅。
type workflowTrackingKey struct{}

// workflowStartKey 职责：隔离冻结图登记钩子；使用：实际计划装载后；边界：不从当前配置猜测历史图。
type workflowStartKey struct{}

// workflowFinishKey 职责：隔离真实任务收尾钩子；使用：引擎返回后；边界：不缓存任务结果或影响预览。
type workflowFinishKey struct{}

// HasWorkflowStart 职责：判断是否需要构造任务图副本；使用：主生成入口装载后；边界：未订阅时避免额外 Describe 成本，不影响执行观察器。
func HasWorkflowStart(ctx context.Context) bool {
	start, _ := ctx.Value(workflowStartKey{}).(func(workflow.PlanDescription) error)
	return start != nil
}

// WithWorkflowFinish 职责：登记真实生成收尾回调；使用：服务保存最终状态和观察丢失数；边界：回调应快速返回，trace 可为空，不可修改结果。
func WithWorkflowFinish(ctx context.Context, finish func(*workflow.RunResult, error)) context.Context {
	return context.WithValue(ctx, workflowFinishKey{}, finish)
}

// FinishWorkflowTracking 职责：传递真实返回错误及引擎观察丢失数；使用：主生成入口的延迟收尾；边界：启动失败 trace 为空，未注册时空转，不生成伪事件。
func FinishWorkflowTracking(ctx context.Context, trace *workflow.RunResult, err error) {
	finish, _ := ctx.Value(workflowFinishKey{}).(func(*workflow.RunResult, error))
	if finish != nil {
		finish(trace, err)
	}
}

// WithWorkflowTracking 职责：附加单次执行事件观察器；使用：生成服务包装上下文；边界：同步回调须快返，错误由引擎记录为观察丢失，不改变实体预览。
func WithWorkflowTracking(ctx context.Context, observer func(workflow.ExecutionEvent) error) context.Context {
	return context.WithValue(ctx, workflowTrackingKey{}, observer)
}

// WorkflowExecutionObserver 职责：取得本次旁路观察器；使用：主线 RunOptions.ObserveExecution 接线；边界：未注册返回 nil，不读取原预览观察器。
func WorkflowExecutionObserver(ctx context.Context) func(workflow.ExecutionEvent) error {
	observer, _ := ctx.Value(workflowTrackingKey{}).(func(workflow.ExecutionEvent) error)
	return observer
}

// WithWorkflowStart 职责：登记实际冻结图回调；使用：服务在 Generate 前绑定；边界：回调只应登记日志，不能修改计划或请求。
func WithWorkflowStart(ctx context.Context, start func(workflow.PlanDescription) error) context.Context {
	return context.WithValue(ctx, workflowStartKey{}, start)
}

// StartWorkflowTracking 职责：把实际装载计划交给服务登记；使用：主生成入口在 loadRuntime 后、首次执行前调用；边界：未注册安全空转，错误交给生命周期所有者决定。
func StartWorkflowTracking(ctx context.Context, graph workflow.PlanDescription) error {
	start, _ := ctx.Value(workflowStartKey{}).(func(workflow.PlanDescription) error)
	if start == nil {
		return nil
	}
	return start(graph)
}

// DescribeWorkflow 职责：描述当前配置的冻结生产图；使用：服务只读查询；边界：不执行任务，历史任务必须读取其 journal 绑定图。
func DescribeWorkflow() (workflow.PlanDescription, error) {
	r, err := loadRuntime()
	if err != nil {
		return workflow.PlanDescription{}, err
	}
	return r.plan.Describe(), nil
}
