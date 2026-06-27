package runtime

// 文件职责：维护 Overlay 附进程内部的窗口贴附同步循环。
// 使用：由 --overlay 模式的 Wails 生命周期启动和停止，用浏览器窗口作为第一版贴附目标。
// 边界：不启动附进程、不暴露 Wails 服务、不渲染 Vue 内容；只管理附进程自身窗口与目标窗口的贴附状态。

import (
	"context"
	"sync"
	"time"

	overlayConstant "DysonSphereBox/constant/overlay"
	overlayWindow "DysonSphereBox/helper/overlay/window"
)

// Overlay HUD 尺寸常量组定义原型悬浮窗的原生窗口占用范围。
// 使用：同步循环把附进程窗口贴到目标窗口左侧中段，容纳工具栏、间隔和展开栏。
// 边界：尺寸只覆盖当前展开栏原型，未来多面板 HUD 应改为按前端内容或配置计算。
const (
	overlaySyncInterval     = 30 * time.Millisecond
	overlayToolbarWidth     = 108
	overlayToolPanelGap     = 36
	overlayDrawerWidth      = 620
	overlayExpandedHeight   = 360
	overlayVerticalRatio    = 0.30
	overlayWindowMarginTop  = 24
	overlayWindowMarginDown = 24
)

// Controller 管理 Overlay 附进程内部的窗口同步状态。
// 使用：OverlayApp 在 startup 时创建并启动，shutdown 时停止。
// 边界：控制器只在附进程内使用；主进程生命周期管理由 service/overlay 负责。
type Controller struct {
	// 职责：保护控制器运行状态和最新快照。
	// 使用：Start、Stop 与 Snapshot 并发访问时加锁。
	// 边界：不跨进程同步。
	mu sync.Mutex
	// 职责：保存取消同步循环的函数。
	// 使用：Stop 调用以结束后台 goroutine。
	// 边界：为空表示未运行。
	cancel context.CancelFunc
	// 职责：保存当前 Overlay 窗口句柄。
	// 使用：同步循环设置穿透、移动和显示隐藏。
	// 边界：窗口重建时会重新查找。
	overlayHwnd uintptr
	// 职责：保存最近一次命中的目标窗口快照。
	// 使用：Snapshot 提供给前端展示当前贴附状态。
	// 边界：这是缓存快照，不保证窗口仍然存在。
	lastTarget *overlayWindow.Snapshot
	// 职责：记录 Overlay 当前是否可点击。
	// 使用：SetInteractive 切换鼠标穿透样式。
	// 边界：只影响 Windows 扩展样式，不改变 Vue 的 pointer-events。
	interactive bool
	// 职责：记录前端是否正在拖动 Overlay 工具栏。
	// 使用：拖动期间暂停层级刷新，避免 SetWindowPos 与前端 WindowSetPosition 互相打架。
	// 边界：只影响当前附进程同步循环，不持久化拖动结果。
	dragging bool
}

// NewController 创建 Overlay 附进程控制器。
// 使用：--overlay 模式启动时构造一次。
// 边界：创建时不启动循环，必须显式调用 Start。
func NewController() *Controller {
	return &Controller{interactive: true}
}

// Start 启动 Overlay 窗口同步循环。
// 使用：OverlayApp.startup 在 Wails runtime context 可用后调用。
// 边界：重复调用不会创建多个循环；目标窗口不存在时循环保持隐藏并持续重试。
func (controller *Controller) Start() {
	controller.mu.Lock()
	if controller.cancel != nil {
		controller.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	controller.cancel = cancel
	controller.mu.Unlock()

	go controller.syncLoop(ctx)
}

// Stop 停止 Overlay 窗口同步循环。
// 使用：OverlayApp.shutdown 或附进程退出前调用。
// 边界：只停止循环，不主动结束进程。
func (controller *Controller) Stop() {
	controller.mu.Lock()
	cancel := controller.cancel
	controller.cancel = nil
	controller.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// SetInteractive 切换 Overlay 是否接收鼠标交互。
// 使用：后续主进程通过 IPC 或前端按钮切换交互模式时调用。
// 边界：第一版默认保持不可交互；该方法只更新窗口样式，不负责渲染状态提示。
func (controller *Controller) SetInteractive(interactive bool) {
	controller.mu.Lock()
	controller.interactive = interactive
	hwnd := controller.overlayHwnd
	controller.mu.Unlock()
	if hwnd != 0 {
		overlayWindow.ApplyOverlayStyles(hwnd, !interactive)
	}
}

// SetDragging 标记 Overlay 前端是否正在拖动工具栏。
// 使用：OverlayRuntimeService 在 pointer down/up 时调用，用于暂停同步循环中的层级刷新。
// 边界：该状态只做抖动抑制，不负责计算最终吸边位置。
func (controller *Controller) SetDragging(dragging bool) {
	controller.mu.Lock()
	controller.dragging = dragging
	controller.mu.Unlock()
}

// Snapshot 返回最近一次目标窗口快照。
// 使用：Overlay 前端展示当前贴附到哪个浏览器窗口。
// 边界：未命中时返回 nil；返回值是浅拷贝，避免调用方修改内部状态。
func (controller *Controller) Snapshot() *overlayWindow.Snapshot {
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.lastTarget == nil {
		return nil
	}
	snapshot := *controller.lastTarget
	return &snapshot
}

// OverlayRect 返回 Overlay 自身窗口的真实屏幕矩形。
// 使用：前端原生拖动结束后读取真实 Win32/DWM 坐标，用于单侧吸边判断和落位。
// 边界：窗口尚未创建或句柄失效时返回零值矩形，不负责触发移动。
func (controller *Controller) OverlayRect() overlayWindow.Rect {
	hwnd := controller.ensureOverlayWindow()
	if hwnd == 0 {
		return overlayWindow.Rect{}
	}
	return overlayWindow.ReadWindowRect(hwnd)
}

// 后台同步 Overlay 与浏览器目标窗口。
// 职责：查找自身窗口、设置穿透样式、查找浏览器窗口、计算吸附矩形并刷新层级；使用：Start 内部启动；边界：移动和缩放交给 Overlay 前端通过 Wails runtime 执行，避免两个控制源争抢窗口。
// 逻辑流：1. 定位自身 Wails 窗口 => 2. 查找浏览器顶层窗口 => 3. 目标不存在或最小化时隐藏 => 4. 目标可见时计算吸附矩形并刷新层级 => 5. 周期性重试。
func (controller *Controller) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(overlaySyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hwnd := controller.ensureOverlayWindow()
			if hwnd == 0 {
				continue
			}

			target := overlayWindow.FindFirstProcessWindow(overlayConstant.TargetBrowserProcesses)
			controller.mu.Lock()
			controller.lastTarget = target
			interactive := controller.interactive
			dragging := controller.dragging
			controller.mu.Unlock()

			overlayWindow.ApplyOverlayStyles(hwnd, !interactive)
			if target == nil || target.Minimized || target.Rect.Width() <= 0 || target.Rect.Height() <= 0 {
				overlayWindow.HideWindow(hwnd)
				continue
			}

			overlayRect := calculateHUDRect(target.Rect)
			target.AttachedRect = overlayRect
			controller.mu.Lock()
			controller.lastTarget = target
			controller.mu.Unlock()
			if !dragging {
				overlayWindow.RefreshTargetLayerNoActivate(hwnd, target.Hwnd)
			}
			overlayWindow.ShowWindowNoActivate(hwnd)
		}
	}
}

// 计算 HUD 局部窗口矩形。
// 职责：将目标窗口矩形转换为左侧工具栏加展开栏的 Overlay 矩形；使用：同步循环避免透明窗口铺满目标内容；边界：目标窗口过小时保持固定宽度并尽量把高度约束在窗口内。
func calculateHUDRect(target overlayWindow.Rect) overlayWindow.Rect {
	width := overlayToolbarWidth + overlayToolPanelGap + overlayDrawerWidth
	height := overlayExpandedHeight
	maxHeight := target.Height() - overlayWindowMarginTop - overlayWindowMarginDown
	if maxHeight > 0 && maxHeight < height {
		height = maxHeight
	}

	left := target.Left
	top := target.Top + int(float64(target.Height())*overlayVerticalRatio)
	maxTop := target.Bottom - overlayWindowMarginDown - height
	minTop := target.Top + overlayWindowMarginTop
	if top > maxTop {
		top = maxTop
	}
	if top < minTop {
		top = minTop
	}
	return overlayWindow.Rect{
		Left:   left,
		Top:    top,
		Right:  left + width,
		Bottom: top + height,
	}
}

// 确认并缓存 Overlay 自身窗口句柄。
// 职责：按窗口标题查找当前进程的 Wails 顶层窗口；使用：同步循环每轮调用；边界：查找失败返回 0 并等待下一轮。
func (controller *Controller) ensureOverlayWindow() uintptr {
	controller.mu.Lock()
	hwnd := controller.overlayHwnd
	controller.mu.Unlock()
	if hwnd != 0 {
		return hwnd
	}

	hwnd = overlayWindow.FindCurrentProcessWindow("DysonSphereBoxOverlay")
	if hwnd == 0 {
		return 0
	}

	controller.mu.Lock()
	controller.overlayHwnd = hwnd
	interactive := controller.interactive
	controller.mu.Unlock()
	overlayWindow.ApplyOverlayStyles(hwnd, !interactive)
	return hwnd
}
