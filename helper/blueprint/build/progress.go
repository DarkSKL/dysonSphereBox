package build

import (
	"context"

	algorithm "DysonSphereBox/helper/blueprint/build/algorithm"
	"DysonSphereBox/helper/blueprint/build/preview"
)

// Progress 职责：公开有序生成事件；使用：任务订阅；边界：增量事件不携带完整蓝图串。
type Progress = algorithm.Progress

// PreviewModule 职责：公开预览模块信息；使用：兼容建筑分块协议；边界：不执行前端渲染。
type PreviewModule = algorithm.PreviewModule

// BuildingBatch 职责：公开建筑分块；使用：兼容已有订阅；边界：实时过程使用差量协议。
type BuildingBatch = algorithm.BuildingBatch

const (
	// ProgressProtocolVersion 职责：公开兼容协议版本；使用：事件校验；边界：事件可携带更高版本。
	ProgressProtocolVersion = algorithm.ProgressProtocolVersion
	// ProgressKindPhase 职责：标识阶段事件；使用：进度显示；边界：不代表场景变化。
	ProgressKindPhase = algorithm.ProgressKindPhase
	// ProgressKindBuildingBatch 职责：标识兼容建筑分块；使用：历史订阅；边界：不替代实时差量。
	ProgressKindBuildingBatch = algorithm.ProgressKindBuildingBatch
	// ProgressKindPreviewDelta 职责：标识对象差量；使用：增量预览订阅；边界：不携带失败原因解释。
	ProgressKindPreviewDelta = preview.Kind
	// ProgressKindPreviewComplete 职责：标识预览完成；使用：任务收尾；边界：以前置有序事件为基础。
	ProgressKindPreviewComplete = algorithm.ProgressKindPreviewComplete
	// PreviewCoordinateSystem 职责：公开最终预览坐标系；使用：解释区域坐标；边界：临时布局使用独立坐标系。
	PreviewCoordinateSystem = algorithm.PreviewCoordinateSystem
)

// WithProgress 职责：为生成任务注册事件观察者；使用：包装 Generate 的上下文；边界：回调应快速返回，不负责前端状态或动画。
func WithProgress(ctx context.Context, observer func(Progress)) context.Context {
	return algorithm.WithProgress(ctx, observer)
}
