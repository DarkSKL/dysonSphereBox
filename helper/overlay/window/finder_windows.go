//go:build windows

package window

// 文件职责：封装 Windows 顶层窗口枚举、进程匹配和窗口矩形读取。
// 使用：由 Overlay 原型服务查询浏览器窗口状态，也由附进程同步循环定位目标窗口。
// 边界：只封装平台 API，不负责启动附进程、记录业务日志或渲染前端内容。

import (
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows API 常量组维护窗口枚举和进程查询所需的稳定平台值。
// 使用：本文件内部调用 user32、dwmapi 和 kernel32 时传入。
// 边界：这些值只服务当前 Windows 实现，非 Windows 平台不编译本文件。
const (
	processQueryLimitedInformation = 0x1000
	dwmwaExtendedFrameBounds       = 9
	dwmwaCloaked                   = 14
	minTargetWindowWidth           = 320
	minTargetWindowHeight          = 220
)

// Windows API 过程句柄组延迟绑定需要调用的系统函数。
// 使用：FindFirstProcessWindow 与 ReadWindowRect 调用这些过程。
// 边界：过程调用失败时返回空结果或回退结果，不在 helper 层弹窗提示。
var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	dwmapi                       = windows.NewLazySystemDLL("dwmapi.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procIsIconic                 = user32.NewProc("IsIconic")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procDwmGetWindowAttribute    = dwmapi.NewProc("DwmGetWindowAttribute")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
)

// nativeRect 对齐 Win32 RECT 内存布局。
// 使用：接收 GetWindowRect 和 DwmGetWindowAttribute 的返回值。
// 边界：不暴露给 service 层，转换为 Rect 后再返回。
type nativeRect struct {
	// Left 表示原生 RECT 左边界。
	// 使用：转换为平台无关 Rect。
	// 边界：不在结构体层做合法性校验。
	Left int32
	// Top 表示原生 RECT 上边界。
	// 使用：转换为平台无关 Rect。
	// 边界：不在结构体层做合法性校验。
	Top int32
	// Right 表示原生 RECT 右边界。
	// 使用：转换为平台无关 Rect。
	// 边界：不在结构体层做合法性校验。
	Right int32
	// Bottom 表示原生 RECT 下边界。
	// 使用：转换为平台无关 Rect。
	// 边界：不在结构体层做合法性校验。
	Bottom int32
}

// FindFirstProcessWindow 查找第一个匹配进程名的可见顶层窗口。
// 使用：第一版 Overlay 原型传入浏览器进程名列表，用于验证贴附和跟随能力。
// 边界：未找到时返回 nil；多窗口时优先返回前台窗口，其次返回枚举到的第一个可见窗口。
// 逻辑流：1. 规范化目标进程名 => 2. 枚举可见顶层窗口 => 3. 读取 PID 与进程路径 => 4. 命中前台窗口则提前采用 => 5. 返回窗口快照。
func FindFirstProcessWindow(processNames []string) *Snapshot {
	candidates := ListProcessWindowCandidates(processNames)
	if len(candidates) == 0 {
		return nil
	}
	snapshot := candidates[0].Snapshot
	return &snapshot
}

// ListProcessWindowCandidates 枚举匹配进程名的可贴附窗口候选项。
// 使用：Overlay 调试页展示所有浏览器候选窗口，辅助判断偏移是否来自错误 HWND。
// 边界：只返回当前瞬时枚举结果，调用方不应持久化候选顺序。
// 逻辑流：1. 规范化目标进程名 => 2. 枚举顶层窗口并过滤不可贴附窗口 => 3. 读取进程和矩形信息 => 4. 计算评分 => 5. 按评分降序返回。
func ListProcessWindowCandidates(processNames []string) []WindowCandidate {
	allowed := normalizeProcessNames(processNames)
	if len(allowed) == 0 {
		return nil
	}
	foregroundHwnd, _, _ := procGetForegroundWindow.Call()
	candidates := make([]WindowCandidate, 0, 8)

	enumWindows(func(hwnd uintptr) bool {
		if !isCandidateTopLevelWindow(hwnd) {
			return true
		}
		title := getWindowTitle(hwnd)
		if strings.TrimSpace(title) == "" {
			return true
		}
		pid := getWindowPID(hwnd)
		if pid == 0 {
			return true
		}
		exePath := queryProcessImagePath(pid)
		processName := strings.ToLower(filepath.Base(exePath))
		if processName == "" {
			return true
		}
		if _, ok := allowed[processName]; !ok {
			return true
		}

		rect := ReadWindowRect(hwnd)
		if rect.Width() < minTargetWindowWidth || rect.Height() < minTargetWindowHeight {
			return true
		}

		snapshot := Snapshot{
			Hwnd:        hwnd,
			PID:         pid,
			Title:       title,
			ProcessName: processName,
			ExePath:     exePath,
			Visible:     true,
			Minimized:   isWindowMinimized(hwnd),
			Foreground:  hwnd == foregroundHwnd,
			Rect:        rect,
		}
		score := scoreCandidateWindow(&snapshot)
		if hwnd == foregroundHwnd {
			score += 1_000_000_000
		}
		candidates = append(candidates, WindowCandidate{
			Snapshot: snapshot,
			Score:    score,
		})
		return true
	})

	sort.SliceStable(candidates, func(leftIndex int, rightIndex int) bool {
		return candidates[leftIndex].Score > candidates[rightIndex].Score
	})
	return candidates
}

// FindCurrentProcessWindow 查找当前进程指定标题的顶层窗口。
// 使用：Overlay 附进程启动后定位自己的 Wails 窗口句柄，再设置穿透和跟随。
// 边界：标题匹配失败时返回 0，调用方应稍后重试。
func FindCurrentProcessWindow(titleKeyword string) uintptr {
	currentPID := uint32(windows.GetCurrentProcessId())
	keyword := strings.ToLower(strings.TrimSpace(titleKeyword))
	var matched uintptr
	enumWindows(func(hwnd uintptr) bool {
		if getWindowPID(hwnd) != currentPID {
			return true
		}
		title := strings.ToLower(getWindowTitle(hwnd))
		if keyword != "" && !strings.Contains(title, keyword) {
			return true
		}
		matched = hwnd
		return false
	})
	return matched
}

// ReadWindowRect 读取窗口真实显示矩形。
// 使用：Overlay 同步循环每轮读取目标窗口尺寸，优先采用 DWM 扩展边界减少边框偏差。
// 边界：DWM 失败时回退 GetWindowRect；全部失败时返回零值矩形。
func ReadWindowRect(hwnd uintptr) Rect {
	var raw nativeRect
	ret, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaExtendedFrameBounds),
		uintptr(unsafe.Pointer(&raw)),
		unsafe.Sizeof(raw),
	)
	if ret == 0 {
		return raw.toRect()
	}
	ret, _, _ = procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&raw)))
	if ret != 0 {
		return raw.toRect()
	}
	return Rect{}
}

// 枚举所有顶层窗口。
// 职责：把 Win32 回调包装为 Go 闭包；使用：窗口查找函数内部调用；边界：回调返回 false 时停止枚举。
func enumWindows(callback func(hwnd uintptr) bool) {
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if callback(hwnd) {
			return 1
		}
		return 0
	})
	procEnumWindows.Call(cb, 0)
}

// 规范化进程名列表。
// 职责：转小写并去重；使用：FindFirstProcessWindow 枚举前准备匹配表；边界：空项会被忽略。
func normalizeProcessNames(processNames []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(processNames))
	for _, processName := range processNames {
		normalized := strings.ToLower(strings.TrimSpace(processName))
		if normalized == "" {
			continue
		}
		allowed[normalized] = struct{}{}
	}
	return allowed
}

// 计算候选浏览器窗口得分。
// 职责：优先选择面积更大的主浏览器窗口，避免命中浏览器的小型浮层或代理窗口；使用：FindFirstProcessWindow 内部排序；边界：不比较窗口标题语义。
func scoreCandidateWindow(snapshot *Snapshot) int64 {
	if snapshot == nil {
		return 0
	}
	width := snapshot.Rect.Width()
	height := snapshot.Rect.Height()
	if width <= 0 || height <= 0 {
		return 0
	}
	return int64(width) * int64(height)
}

// 查询窗口可见性。
// 职责：包装 IsWindowVisible；使用：枚举窗口时过滤隐藏窗口；边界：可见不代表窗口位于最上层。
func isWindowVisible(hwnd uintptr) bool {
	ret, _, _ := procIsWindowVisible.Call(hwnd)
	return ret != 0
}

// 查询窗口最小化状态。
// 职责：包装 IsIconic；使用：目标最小化时隐藏 Overlay；边界：只返回当前瞬时状态。
func isWindowMinimized(hwnd uintptr) bool {
	ret, _, _ := procIsIconic.Call(hwnd)
	return ret != 0
}

// 判断窗口是否适合作为 Overlay 贴附目标。
// 职责：过滤隐藏、最小化、DWM cloaked 和工具窗口；使用：浏览器窗口枚举时减少后台残留窗口误命中；边界：不判断具体进程名。
func isCandidateTopLevelWindow(hwnd uintptr) bool {
	if !isWindowVisible(hwnd) || isWindowMinimized(hwnd) || isWindowCloaked(hwnd) {
		return false
	}
	exStyle := getWindowExStyle(hwnd)
	return exStyle&wsExToolWindow == 0
}

// 查询窗口是否被 DWM 隐藏。
// 职责：过滤浏览器关闭或虚拟桌面切换后残留但不可显示的窗口；使用：候选窗口判断；边界：DWM 调用失败时按未隐藏处理。
func isWindowCloaked(hwnd uintptr) bool {
	var cloaked uint32
	ret, _, _ := procDwmGetWindowAttribute.Call(
		hwnd,
		uintptr(dwmwaCloaked),
		uintptr(unsafe.Pointer(&cloaked)),
		unsafe.Sizeof(cloaked),
	)
	return ret == 0 && cloaked != 0
}

// 读取窗口所属进程 ID。
// 职责：包装 GetWindowThreadProcessId；使用：窗口枚举时关联进程路径；边界：失败时返回 0。
func getWindowPID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// 读取窗口标题。
// 职责：包装 GetWindowTextW；使用：前端展示和辅助过滤空窗口；边界：读取失败返回空字符串。
func getWindowTitle(hwnd uintptr) string {
	length, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return windows.UTF16ToString(buffer)
}

// 读取进程可执行文件路径。
// 职责：通过 OpenProcess 与 QueryFullProcessImageName 获取路径；使用：匹配浏览器进程并展示诊断信息；边界：权限不足时返回空字符串。
func queryProcessImagePath(pid uint32) string {
	handle, err := windows.OpenProcess(processQueryLimitedInformation, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(handle)

	buffer := make([]uint16, windows.MAX_PATH*4)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return ""
	}
	return windows.UTF16ToString(buffer[:size])
}

// 转换原生 RECT 为内部矩形。
// 职责：隔离 Win32 类型；使用：ReadWindowRect 读取成功后调用；边界：不裁剪负坐标。
func (rect nativeRect) toRect() Rect {
	return Rect{
		Left:   int(rect.Left),
		Top:    int(rect.Top),
		Right:  int(rect.Right),
		Bottom: int(rect.Bottom),
	}
}
