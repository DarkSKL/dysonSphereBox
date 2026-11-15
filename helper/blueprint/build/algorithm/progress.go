package build

// 文件职责：提供纯引擎的真实阶段通知；使用：生成调用方按需通过上下文订阅；边界：不依赖桌面运行时，不估算百分比。

import (
	"context"
	"sync"
	"sync/atomic"

	"DysonSphereBox/helper/blueprint/build/preview"
)

// Progress 职责：描述生成阶段或蓝图预览分块；使用：Wails 事件和内部观察者；边界：序号仅保证单请求内有序，不代表完成百分比。
type Progress struct {
	// CandidateSequence 职责：提供候选内连续序号；使用：分流校验；边界：不替代任务全局序号。
	CandidateSequence uint64 `json:"candidateSequence,omitempty"`
	// SnapshotRevision 职责：标识完整正式快照；使用：分块组装；边界：当前每任务提交一次获选快照。
	SnapshotRevision uint64 `json:"snapshotRevision,omitempty"`
	// ChunkIndex 职责：保存快照块下标；使用：原子组装；边界：零开始。
	ChunkIndex int `json:"chunkIndex,omitempty"`
	// ChunkCount 职责：保存快照总块数；使用：完整性校验；边界：不是预览差量批数。
	ChunkCount int `json:"chunkCount,omitempty"`
	// Candidates 职责：声明已启用候选组；使用：先铺开全部平台；边界：未启用回退组不提前创建。
	Candidates []CandidatePreviewPlan `json:"candidates,omitempty"`
	// CandidateID 职责：标识独立候选；使用：预览分流；边界：只在任务内有效。
	CandidateID string `json:"candidateId,omitempty"`
	// CandidateOrdinal 职责：保存候选稳定顺序；使用：平台排布；边界：不按完成顺序编号。
	CandidateOrdinal uint64 `json:"candidateOrdinal,omitempty"`
	// CandidateStatus 职责：保存候选局部终检状态；使用：独立淘汰；边界：可行不代表最终获选。
	CandidateStatus string `json:"candidateStatus,omitempty"`
	// WinnerCandidateID 职责：声明唯一获选候选；使用：正式结果对齐；边界：只有成功结束时有效。
	WinnerCandidateID string `json:"winnerCandidateId,omitempty"`
	// TaskID 职责：关联当前生成任务；使用：过滤迟到或旧任务事件；边界：不作为认证凭据或最终蓝图身份。
	TaskID string `json:"taskId,omitempty"`
	// ProtocolVersion 职责：标识事件载荷版本；使用：前端兼容分支；边界：只针对本事件契约。
	ProtocolVersion int `json:"protocolVersion"`
	// Kind 职责：标识阶段、旧最终分块或实时 preview-delta；使用：前端分派；边界：未知值必须忽略。
	Kind string `json:"kind"`
	// Sequence 职责：提供请求内严格递增序号；使用：事件排序和缺失检测；边界：不是建筑索引。
	Sequence uint64 `json:"sequence"`
	// Stage 职责：标识执行阶段；使用：事件分类；边界：不代表任务成功。
	Stage string `json:"stage"`
	// Message 职责：提供中文阶段描述；使用：状态栏展示；边界：不包含计数。
	Message string `json:"message"`
	// Detail 职责：保留阶段原始诊断；使用：界面展开或悬停查看候选失败原因；边界：不在状态栏铺开技术信息，不包含预览建筑数据。
	Detail string `json:"detail,omitempty"`
	// Current 职责：标记当前处理项；使用：和 Total 展示真实序号；边界：无计数阶段为零。
	Current int `json:"current"`
	// Total 职责：标记本阶段项数；使用：描述产线或连接数量；边界：不是整个生成流程的工作量。
	Total int `json:"total"`
	// CoordinateSystem 职责：说明预览建筑坐标语义；使用：动画定位建筑；边界：阶段事件不携带坐标系。
	CoordinateSystem string `json:"coordinateSystem,omitempty"`
	// PreviewBatch 职责：承载一个模块的最终建筑记录分块；使用：增量预览渲染；边界：仅在严格编码成功后发布，载荷使用实时预览协议版本。
	PreviewBatch *BuildingBatch `json:"previewBatch,omitempty"`
	// BuildingCount 职责：保存预览所含建筑数；使用：完整事件收尾及数量核对；边界：不表示有效放置证明。
	BuildingCount int `json:"buildingCount,omitempty"`
	// BatchCount 职责：保存本请求预览分块总数；使用：收尾核对；边界：不替代 Sequence。
	BatchCount int `json:"batchCount,omitempty"`
	// Operations 职责：提供实时对象变化；使用：preview-delta 事件；边界：不携带全量场景或蓝图串。
	Operations []preview.Operation `json:"operations,omitempty"`
	// PreviewAreas 职责：解释最终对象的 Area 坐标；使用：坐标系切换；边界：不是建筑全量快照。
	PreviewAreas []preview.Area `json:"areas,omitempty"`
	// PreviewPrimaryAreaIndex 职责：标识最终主区域；使用：增量投影；边界：临时布局为零。
	PreviewPrimaryAreaIndex int32 `json:"primaryAreaIndex,omitempty"`
}

// CandidatePreviewPlan 职责：在计算前声明完整候选平台身份；使用：候选组派发；边界：不包含未启用的条件回退组。
type CandidatePreviewPlan struct {
	ID      string `json:"id"`
	Ordinal uint64 `json:"ordinal"`
}

// progressKey 职责：隔离上下文观察者键；使用：WithProgress 与通知函数；边界：不保存全局订阅。
type progressKey struct{}

// generationTaskIDKey 职责：隔离本次生成的消息身份；使用：统一入口与事件发布；边界：复用父观察者也不会复用上次任务 ID。
type generationTaskIDKey struct{}

// progressObserver 职责：保存单请求的同步观察者和事件序号；使用：各阶段统一发布；边界：不跨请求共享。
type progressObserver struct {
	mu sync.Mutex
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
	recordPerformanceStage(ctx, stage, total)
	text, detail := progressPresentation(stage, message)
	publishProgress(ctx, Progress{Kind: ProgressKindPhase, Stage: stage, Message: text, Detail: detail, Current: current, Total: total})
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
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	if progress.ProtocolVersion == 0 {
		progress.ProtocolVersion = ProgressProtocolVersion
	}
	progress.TaskID, _ = ctx.Value(generationTaskIDKey{}).(string)
	progress.Sequence = observer.sequence.Add(1)
	observer.callback(progress)
}

// hasProgressObserver 职责：判断当前调用是否能送达阶段事件；使用：跳过无监听者的可选预览解码；边界：不触发回调。
func hasProgressObserver(ctx context.Context) bool {
	observer, ok := ctx.Value(progressKey{}).(*progressObserver)
	return ok && observer != nil && observer.callback != nil
}
