//go:build !windows

package window

// 文件职责：提供非 Windows 平台的 Overlay 窗口 API 空实现。
// 使用：保证项目在非 Windows 平台仍可编译基础包。
// 边界：当前悬浮窗原型只支持 Windows，非 Windows 返回空结果。

// FindFirstProcessWindow 在非 Windows 平台不执行窗口查找。
// 使用：调用方可按 nil 判断当前平台不可用。
// 边界：不尝试跨平台兼容实现。
func FindFirstProcessWindow(processNames []string) *Snapshot {
	return nil
}

// ListProcessWindowCandidates 在非 Windows 平台不执行候选窗口枚举。
// 使用：调试接口可安全调用并得到空列表。
// 边界：不尝试读取系统窗口。
func ListProcessWindowCandidates(processNames []string) []WindowCandidate {
	return nil
}

// FindCurrentProcessWindow 在非 Windows 平台不查找自身窗口。
// 使用：Overlay 控制器据此跳过样式设置。
// 边界：返回 0 表示不可用。
func FindCurrentProcessWindow(titleKeyword string) uintptr {
	return 0
}

// ReadWindowRect 在非 Windows 平台返回零值矩形。
// 使用：满足调用方编译依赖。
// 边界：不提供真实窗口坐标。
func ReadWindowRect(hwnd uintptr) Rect {
	return Rect{}
}

// ApplyOverlayStyles 在非 Windows 平台不调整窗口样式。
// 使用：满足调用方编译依赖。
// 边界：无副作用。
func ApplyOverlayStyles(hwnd uintptr, clickThrough bool) {}

// MoveWithTargetLayerNoActivate 在非 Windows 平台不移动窗口。
// 使用：满足调用方编译依赖。
// 边界：无副作用。
func MoveWithTargetLayerNoActivate(hwnd uintptr, targetHwnd uintptr, rect Rect) {}

// RefreshTargetLayerNoActivate 在非 Windows 平台不刷新层级状态。
// 使用：满足调用方编译依赖。
// 边界：无副作用。
func RefreshTargetLayerNoActivate(hwnd uintptr, targetHwnd uintptr) {}

// ShowWindowNoActivate 在非 Windows 平台不显示窗口。
// 使用：满足调用方编译依赖。
// 边界：无副作用。
func ShowWindowNoActivate(hwnd uintptr) {}

// HideWindow 在非 Windows 平台不隐藏窗口。
// 使用：满足调用方编译依赖。
// 边界：无副作用。
func HideWindow(hwnd uintptr) {}
