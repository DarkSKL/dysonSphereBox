package build

// 文件职责：提供纯引擎的真实阶段通知；使用：生成调用方按需通过上下文订阅；边界：不依赖桌面运行时，不估算百分比。

import "context"

// Progress 职责：描述当前执行阶段；使用：生成器在实际工作前通知观察者；边界：序号表示当前项而非已完成比例。
type Progress struct {
	// Stage 职责：标识执行阶段；使用：事件分类；边界：不代表任务成功。
	Stage string `json:"stage"`
	// Message 职责：提供中文阶段描述；使用：状态栏展示；边界：不包含计数。
	Message string `json:"message"`
	// Current 职责：标记当前处理项；使用：和 Total 展示真实序号；边界：无计数阶段为零。
	Current int `json:"current"`
	// Total 职责：标记本阶段项数；使用：描述产线或连接数量；边界：不是整个生成流程的工作量。
	Total int `json:"total"`
}

// progressKey 职责：隔离上下文观察者键；使用：WithProgress 与通知函数；边界：不保存全局订阅。
type progressKey struct{}

// WithProgress 职责：为单次生成附加阶段观察者；使用：服务或测试包装父上下文；边界：回调同步执行，应快速返回且不可修改引擎状态。
func WithProgress(ctx context.Context, observer func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, observer)
}

// reportProgress 职责：发布实际到达的工作阶段；使用：生成检查点；边界：取消后不再通知，未订阅时不影响生成。
func reportProgress(ctx context.Context, stage, message string, current, total int) {
	if ctx.Err() != nil {
		return
	}
	if observer, ok := ctx.Value(progressKey{}).(func(Progress)); ok && observer != nil {
		observer(Progress{Stage: stage, Message: message, Current: current, Total: total})
	}
}
