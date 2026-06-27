package overlay

// 文件职责：提供 Overlay 附进程运行态的 Wails 绑定服务。
// 使用：Overlay 前端页面调用该服务读取附进程控制器中的真实贴附快照。
// 边界：主进程注册空控制器实例仅用于生成绑定和安全兜底，不负责启动或同步窗口。

import overlayRuntime "DysonSphereBox/helper/overlay/runtime"

// OverlayRuntimeService 暴露 Overlay 附进程内部运行态查询能力。
// 使用：/overlay 页面读取真实贴附目标；主进程调试页不应依赖该服务判断附进程生命周期。
// 边界：controller 为空时返回 Found=false，避免主进程误读附进程内存状态。
type OverlayRuntimeService struct {
	// 职责：指向附进程内部的窗口同步控制器。
	// 使用：GetOverlayTargetWindow 读取最近一次贴附快照。
	// 边界：主进程注册时可以为空，不跨进程共享。
	controller *overlayRuntime.Controller
}

// NewOverlayRuntimeService 创建 Overlay 运行态服务。
// 使用：--overlay 模式启动时传入当前附进程控制器；主进程注册绑定时可使用零值结构。
// 边界：该构造不启动控制器，生命周期仍由 OverlayApp 管理。
func NewOverlayRuntimeService(controller *overlayRuntime.Controller) *OverlayRuntimeService {
	return &OverlayRuntimeService{controller: controller}
}

// GetOverlayTargetWindow 返回附进程最近一次真实贴附目标快照。
// 使用：Overlay HUD 页面轮询该方法，显示控制器实际移动窗口时使用的目标和吸附矩形。
// 边界：如果当前进程没有控制器或尚未命中目标，返回 Found=false。
func (service *OverlayRuntimeService) GetOverlayTargetWindow() WindowSnapshotDTO {
	if service == nil || service.controller == nil {
		return WindowSnapshotDTO{Found: false}
	}
	return NewWindowSnapshotDTO(service.controller.Snapshot())
}

// GetOverlayWindowRect 返回 Overlay 自身窗口的真实屏幕矩形。
// 使用：/overlay 页面拖动结束后用该矩形做吸边判断，避免 Wails 坐标与 Win32 坐标混用。
// 边界：controller 为空或窗口未创建时返回零值矩形。
func (service *OverlayRuntimeService) GetOverlayWindowRect() WindowRectDTO {
	if service == nil || service.controller == nil {
		return WindowRectDTO{}
	}
	rect := service.controller.OverlayRect()
	return WindowRectDTO{
		Left:   rect.Left,
		Top:    rect.Top,
		Right:  rect.Right,
		Bottom: rect.Bottom,
		Width:  rect.Width(),
		Height: rect.Height(),
	}
}

// SetOverlayDragging 设置 Overlay 前端是否正在拖动。
// 使用：/overlay 页面在拖动工具栏开始和结束时调用，避免后端层级刷新与前端移动窗口互相争抢。
// 边界：controller 为空时直接忽略，不返回错误阻断前端交互。
func (service *OverlayRuntimeService) SetOverlayDragging(dragging bool) {
	if service == nil || service.controller == nil {
		return
	}
	service.controller.SetDragging(dragging)
}
