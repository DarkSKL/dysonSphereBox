package overlay

// 文件职责：提供主前端可调用的 Overlay 原型控制服务。
// 使用：主应用调试页调用该服务启动/停止附进程、查询浏览器窗口检测结果。
// 边界：服务层只编排生命周期和 DTO，不直接调用底层 Win32 样式设置。

import (
	overlayConstant "DysonSphereBox/constant/overlay"
	overlayWindow "DysonSphereBox/helper/overlay/window"
)

// OverlayService 提供悬浮窗原型的 Wails 绑定方法。
// 使用：主界面用于验证“一个 exe + 附进程 + 浏览器窗口贴附”的第一版案例。
// 边界：第一版目标进程固定为浏览器，后续切换戴森球进程时替换匹配常量或增加配置。
type OverlayService struct{}

// FindBrowserWindow 查找第一版原型监听的浏览器窗口。
// 使用：主界面调试卡片刷新当前命中的浏览器进程、标题和矩形。
// 边界：未找到时返回 Found=false，不视为错误。
func (service *OverlayService) FindBrowserWindow() WindowSnapshotDTO {
	return NewWindowSnapshotDTO(overlayWindow.FindFirstProcessWindow(overlayConstant.TargetBrowserProcesses))
}

// ListBrowserWindowCandidates 列出第一版原型监听到的浏览器窗口候选项。
// 使用：主界面调试吸附偏移时查看后端实际枚举与评分。
// 边界：只返回诊断信息，不锁定目标窗口、不修改 Overlay 状态。
func (service *OverlayService) ListBrowserWindowCandidates() []WindowCandidateDTO {
	return NewWindowCandidateDTOs(overlayWindow.ListProcessWindowCandidates(overlayConstant.TargetBrowserProcesses))
}

// StartOverlay 启动同一个 exe 的 --overlay 附进程。
// 使用：主界面点击启动悬浮窗案例时调用。
// 边界：如果附进程已运行则返回现有 PID，不重复启动。
func (service *OverlayService) StartOverlay() (OverlayProcessStatusDTO, error) {
	return OverlayManager.Start()
}

// StopOverlay 停止当前主进程管理的 Overlay 附进程。
// 使用：主界面点击停止悬浮窗案例或退出应用时调用。
// 边界：未运行时返回 Running=false，不视为错误。
func (service *OverlayService) StopOverlay() (OverlayProcessStatusDTO, error) {
	return OverlayManager.Stop()
}

// GetOverlayStatus 查询 Overlay 附进程状态。
// 使用：主界面加载和定时刷新时调用。
// 边界：状态只覆盖当前主进程启动的附进程。
func (service *OverlayService) GetOverlayStatus() OverlayProcessStatusDTO {
	return OverlayManager.Status()
}
