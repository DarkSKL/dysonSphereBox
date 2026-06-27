//go:build windows

package window

// 文件职责：封装 Overlay 窗口置顶、透明、穿透、显示隐藏等 Windows 样式操作。
// 使用：由 Overlay 附进程控制器在窗口创建后和同步循环中调用。
// 边界：只调整指定窗口句柄，不查找目标进程，不决定悬浮内容。

// Overlay 样式常量组维护 SetWindowLongPtr 和 SetWindowPos 需要的 Win32 值。
// 使用：ApplyOverlayStyles、MoveTopMostNoActivate、ShowWindow 和 HideWindow 调用。
// 边界：这些值只适用于 Windows 桌面窗口。
const (
	wsExLayered      = 0x00080000
	wsExTransparent  = 0x00000020
	wsExToolWindow   = 0x00000080
	wsExNoActivate   = 0x08000000
	hwndTop          = 0
	hwndNoTopMost    = ^uintptr(1)
	swpNoSize        = 0x0001
	swpNoMove        = 0x0002
	swpNoActivate    = 0x0010
	swpShowWindow    = 0x0040
	swpNoOwnerZOrder = 0x0200
	swHide           = 0
	swShowNoActivate = 4
	gwHwndPrev       = 3
)

// gwlExStyleIndex 保存 GWL_EXSTYLE 的 uintptr 补码形式。
// 使用：GetWindowLongPtrW 和 SetWindowLongPtrW 读取/写入扩展窗口样式。
// 边界：Win32 常量原值是 -20，不能在 Go 中直接转换为 uintptr 常量。
var gwlExStyleIndex = ^uintptr(19)

// gwlpHwndParentIndex 保存 GWLP_HWNDPARENT 的 uintptr 补码形式。
// 使用：将 Overlay popup 设置为目标窗口的 owner，使其跟随目标窗口所在的 Z-order 分组。
// 边界：Win32 常量原值是 -8，不能在 Go 中直接转换为 uintptr 常量。
var gwlpHwndParentIndex = ^uintptr(7)

// Overlay 样式 API 过程句柄组延迟绑定窗口样式函数。
// 使用：本文件内部调整 Wails Overlay 顶层窗口。
// 边界：调用失败不 panic，由上层下一轮同步重试。
var (
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procShowWindow        = user32.NewProc("ShowWindow")
	procGetWindow         = user32.NewProc("GetWindow")
)

// ApplyOverlayStyles 为悬浮窗应用透明、工具窗口、置顶和可选鼠标穿透样式。
// 使用：Overlay 附进程找到自身窗口后调用，交互状态变化时可再次调用。
// 边界：不移动窗口、不显示窗口；样式失败时留给下一轮同步重试。
func ApplyOverlayStyles(hwnd uintptr, clickThrough bool) {
	if hwnd == 0 {
		return
	}
	style := getWindowExStyle(hwnd)
	style |= wsExLayered | wsExToolWindow | wsExNoActivate
	if clickThrough {
		style |= wsExTransparent
	} else {
		style &^= wsExTransparent
	}
	setWindowExStyle(hwnd, style)
	procSetWindowPos.Call(hwnd, hwndNoTopMost, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpNoOwnerZOrder)
}

// MoveWithTargetLayerNoActivate 将悬浮窗移动到目标窗口同层级的正上方但不抢焦点。
// 使用：Overlay 同步循环用目标浏览器窗口矩形驱动自身窗口，并让其他覆盖目标的窗口也覆盖 Overlay。
// 边界：宽高非正数时跳过，避免创建不可见或异常窗口。
func MoveWithTargetLayerNoActivate(hwnd uintptr, targetHwnd uintptr, rect Rect) {
	if hwnd == 0 || targetHwnd == 0 || rect.Width() <= 0 || rect.Height() <= 0 {
		return
	}
	setWindowOwner(hwnd, targetHwnd)
	procSetWindowPos.Call(
		hwnd,
		targetLayerInsertAfter(targetHwnd),
		uintptr(rect.Left),
		uintptr(rect.Top),
		uintptr(rect.Width()),
		uintptr(rect.Height()),
		swpNoActivate|swpShowWindow|swpNoOwnerZOrder,
	)
}

// RefreshTargetLayerNoActivate 刷新悬浮窗在目标窗口 Z-order 分组内的位置但不移动窗口。
// 使用：同步循环在矩形未变化时也周期性修正悬浮窗与目标窗口的相对层级。
// 边界：只让 Overlay 跟随目标窗口层级，不提升为全系统置顶。
func RefreshTargetLayerNoActivate(hwnd uintptr, targetHwnd uintptr) {
	if hwnd == 0 || targetHwnd == 0 {
		return
	}
	setWindowOwner(hwnd, targetHwnd)
	procSetWindowPos.Call(hwnd, targetLayerInsertAfter(targetHwnd), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|swpNoOwnerZOrder)
}

// ShowWindowNoActivate 显示悬浮窗且不抢占焦点。
// 使用：目标浏览器窗口可见时由同步循环调用。
// 边界：显示后仍可能被独占全屏或其他置顶窗口遮挡。
func ShowWindowNoActivate(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swShowNoActivate)
}

// HideWindow 隐藏悬浮窗。
// 使用：目标窗口不存在或最小化时由同步循环调用。
// 边界：只隐藏窗口，不停止附进程。
func HideWindow(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procShowWindow.Call(hwnd, swHide)
}

// 获取扩展窗口样式。
// 职责：包装 GetWindowLongPtrW；使用：ApplyOverlayStyles 合并样式；边界：失败时返回 0。
func getWindowExStyle(hwnd uintptr) uintptr {
	ret, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyleIndex)
	return ret
}

// 设置扩展窗口样式。
// 职责：包装 SetWindowLongPtrW；使用：ApplyOverlayStyles 写入样式；边界：失败时由后续同步重试。
func setWindowExStyle(hwnd uintptr, style uintptr) {
	procSetWindowLongPtrW.Call(hwnd, gwlExStyleIndex, style)
}

// 设置 Overlay popup 的 owner 窗口。
// 职责：让 Overlay 进入目标窗口的 Z-order 分组；使用：移动和刷新层级前调用；边界：目标窗口销毁后下一轮会重新设置或隐藏。
func setWindowOwner(hwnd uintptr, ownerHwnd uintptr) {
	procSetWindowLongPtrW.Call(hwnd, gwlpHwndParentIndex, ownerHwnd)
}

// 计算目标同层级插入点。
// 职责：找到目标窗口上方的相邻窗口，把 Overlay 插入在其后，从而位于目标上方且低于覆盖目标的窗口；使用：移动和刷新层级时调用；边界：目标已在顶部时回退 HWND_TOP。
func targetLayerInsertAfter(targetHwnd uintptr) uintptr {
	previous, _, _ := procGetWindow.Call(targetHwnd, uintptr(gwHwndPrev))
	if previous != 0 {
		return previous
	}
	return uintptr(hwndTop)
}
