package factory

// 文件职责：将纯生成引擎阶段桥接到 Wails；使用：生成服务创建请求级观察者；边界：无桌面上下文时静默跳过，不写数据库。

import (
	factoryConstant "DysonSphereBox/constant/factory"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/blueprint/build"
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// generationProgress 职责：关联请求与引擎阶段；使用：运行时事件载荷；边界：请求 ID 仅作消息隔离，不用于认证方案。
type generationProgress struct {
	// RequestID 职责：区分同方案的多次生成；使用：前端过滤旧消息；边界：仅回传本次请求的标识。
	RequestID string `json:"requestId"`
	// Progress 职责：携带真实阶段信息；使用：状态栏展示；边界：不表示整体百分比。
	build.Progress
}

// GenerationContext 职责：为本次生成接入桌面阶段通知；使用：服务完成方案校验后调用；边界：空 ID 或非桌面调用保持纯引擎行为。
func GenerationContext(parent context.Context, requestID string) context.Context {
	runtimeContext := core.GLOBAL.RuntimeContext
	if requestID == "" || runtimeContext == nil {
		return parent
	}
	return build.WithProgress(parent, func(progress build.Progress) {
		runtime.EventsEmit(runtimeContext, factoryConstant.GenerationProgressEvent, generationProgress{RequestID: requestID, Progress: progress})
	})
}
