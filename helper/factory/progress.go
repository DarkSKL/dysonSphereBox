package factory

// 文件职责：将纯生成引擎阶段桥接到 Wails；使用：生成服务创建请求级观察者；边界：无桌面上下文时静默跳过，不写数据库。

import (
	factoryConstant "DysonSphereBox/constant/factory"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/blueprint/build"
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// generationProgress 职责：沿用引擎的单一事件契约；使用：Wails 事件载荷；边界：不再额外包裹任务身份或重复渲染操作。
type generationProgress = build.Progress

// GenerationContext 职责：为本次生成接入桌面阶段通知；使用：服务完成方案校验后调用；边界：空 ID 或非桌面调用保持纯引擎行为。
func GenerationContext(parent context.Context, taskID string) context.Context {
	runtimeContext := core.GLOBAL.RuntimeContext
	if taskID == "" || runtimeContext == nil {
		return parent
	}
	return generationProgressContext(parent, taskID, func(progress generationProgress) {
		runtime.EventsEmit(runtimeContext, factoryConstant.GenerationProgressEvent, progress)
	})
}

// generationProgressContext 职责：为引擎事件附加本次请求身份；使用：桌面桥接及离线契约测试；边界：不保存全局订阅，空ID或无发送器时不启用建筑预览。
func generationProgressContext(parent context.Context, taskID string, emit func(generationProgress)) context.Context {
	if taskID == "" || emit == nil {
		return parent
	}
	return build.WithProgress(parent, func(progress build.Progress) {
		progress.TaskID = taskID
		emit(progress)
	})
}
