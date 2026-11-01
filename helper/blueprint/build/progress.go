package build

// 文件职责：提供纯引擎的真实阶段通知；使用：生成调用方按需通过上下文订阅；边界：不依赖桌面运行时，不估算百分比。

import (
	"context"
	"sync/atomic"
)

// Progress 职责：描述生成阶段或蓝图预览分块；使用：Wails 事件和内部观察者；边界：序号仅保证单请求内有序，不代表完成百分比。
type Progress struct {
	// ProtocolVersion 职责：标识事件载荷版本；使用：前端兼容分支；边界：只针对本事件契约。
	ProtocolVersion int `json:"protocolVersion"`
	// Kind 职责：标识 phase、building-batch 或 preview-complete；使用：前端分派；边界：未知值必须忽略。
	Kind string `json:"kind"`
	// Sequence 职责：提供请求内严格递增序号；使用：事件排序和缺失检测；边界：不是建筑索引。
	Sequence uint64 `json:"sequence"`
	// Stage 职责：标识执行阶段；使用：事件分类；边界：不代表任务成功。
	Stage string `json:"stage"`
	// Message 职责：提供中文阶段描述；使用：状态栏展示；边界：不包含计数。
	Message string `json:"message"`
	// Current 职责：标记当前处理项；使用：和 Total 展示真实序号；边界：无计数阶段为零。
	Current int `json:"current"`
	// Total 职责：标记本阶段项数；使用：描述产线或连接数量；边界：不是整个生成流程的工作量。
	Total int `json:"total"`
	// CoordinateSystem 职责：说明预览建筑坐标语义；使用：动画定位建筑；边界：阶段事件不携带坐标系。
	CoordinateSystem string `json:"coordinateSystem,omitempty"`
	// PreviewBatch 职责：承载一个模块的最终建筑记录分块；使用：增量预览渲染；边界：仅在严格编码成功后发布。
	PreviewBatch *BuildingBatch `json:"previewBatch,omitempty"`
	// BuildingCount 职责：保存预览所含建筑数；使用：完整事件收尾及数量核对；边界：不表示有效放置证明。
	BuildingCount int `json:"buildingCount,omitempty"`
	// BatchCount 职责：保存本请求预览分块总数；使用：收尾核对；边界：不替代 Sequence。
	BatchCount int `json:"batchCount,omitempty"`
}

// progressKey 职责：隔离上下文观察者键；使用：WithProgress 与通知函数；边界：不保存全局订阅。
type progressKey struct{}

// progressObserver 职责：保存单请求的同步观察者和事件序号；使用：各阶段统一发布；边界：不跨请求共享。
type progressObserver struct {
	// callback 职责：接收已编号事件；使用：WithProgress 注册；边界：必须快速返回且不修改引擎状态。
	callback func(Progress)
	// sequence 职责：分配单请求递增序号；使用：发布阶段和预览分块；边界：只在同一观察者内有效。
	sequence atomic.Uint64
}

// WithProgress 职责：为单次生成附加阶段观察者；使用：服务或测试包装父上下文；边界：回调同步执行，应快速返回且不可修改引擎状态。
func WithProgress(ctx context.Context, observer func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, &progressObserver{callback: observer})
}

// reportProgress 职责：发布实际到达的工作阶段；使用：生成检查点；边界：取消后不再通知，未订阅时不影响生成。
func reportProgress(ctx context.Context, stage, message string, current, total int) {
	publishProgress(ctx, Progress{Kind: ProgressKindPhase, Stage: stage, Message: message, Current: current, Total: total})
}

// publishProgress 职责：统一补齐协议版本和序号后同步通知观察者；使用：阶段与预览事件；边界：取消或未订阅时静默跳过。
func publishProgress(ctx context.Context, progress Progress) {
	if ctx.Err() != nil {
		return
	}
	observer, ok := ctx.Value(progressKey{}).(*progressObserver)
	if !ok || observer == nil || observer.callback == nil {
		return
	}
	progress.ProtocolVersion = ProgressProtocolVersion
	progress.Sequence = observer.sequence.Add(1)
	observer.callback(progress)
}

// hasProgressObserver 职责：判断当前调用是否能送达阶段事件；使用：跳过无监听者的可选预览解码；边界：不触发回调。
func hasProgressObserver(ctx context.Context) bool {
	observer, ok := ctx.Value(progressKey{}).(*progressObserver)
	return ok && observer != nil && observer.callback != nil
}
