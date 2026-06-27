package window

// 文件职责：定义悬浮窗窗口查找与定位流程共享的数据结构。
// 使用：由 Windows API 封装、Overlay 附进程控制器和前端 DTO 转换共同使用。
// 边界：这里只描述内存快照形状，不执行平台 API 调用，不保存长期运行状态。

// Rect 描述 Windows 桌面坐标系中的窗口矩形。
// 使用：Overlay 同步循环按该矩形移动和缩放悬浮窗口。
// 边界：坐标单位来自系统 API，未在这里做 DPI 换算或显示器裁剪。
type Rect struct {
	// Left 表示窗口左边界的屏幕坐标。
	// 使用：传给 SetWindowPos 的 x 坐标。
	// 边界：当窗口跨屏或位于负坐标显示器时可能为负数。
	Left int
	// Top 表示窗口上边界的屏幕坐标。
	// 使用：传给 SetWindowPos 的 y 坐标。
	// 边界：当窗口跨屏或位于负坐标显示器时可能为负数。
	Top int
	// Right 表示窗口右边界的屏幕坐标。
	// 使用：与 Left 相减得到窗口宽度。
	// 边界：不单独保证大于 Left，调用方需要校验。
	Right int
	// Bottom 表示窗口下边界的屏幕坐标。
	// 使用：与 Top 相减得到窗口高度。
	// 边界：不单独保证大于 Top，调用方需要校验。
	Bottom int
}

// Width 返回窗口矩形宽度。
// 使用：Overlay 同步循环传给 SetWindowPos。
// 边界：异常矩形返回非正数，调用方应跳过移动。
func (rect Rect) Width() int {
	return rect.Right - rect.Left
}

// Height 返回窗口矩形高度。
// 使用：Overlay 同步循环传给 SetWindowPos。
// 边界：异常矩形返回非正数，调用方应跳过移动。
func (rect Rect) Height() int {
	return rect.Bottom - rect.Top
}

// Equal 判断两个窗口矩形是否完全一致。
// 使用：同步循环用于减少重复 SetWindowPos 调用。
// 边界：只比较坐标值，不判断窗口句柄或显示器归属。
func (rect Rect) Equal(other Rect) bool {
	return rect.Left == other.Left && rect.Top == other.Top && rect.Right == other.Right && rect.Bottom == other.Bottom
}

// Snapshot 描述一次顶层窗口识别结果。
// 使用：服务层和 Overlay 控制器用它展示目标窗口状态、执行贴附定位。
// 边界：这是瞬时快照，窗口可能在返回后立即关闭或重建。
type Snapshot struct {
	// Hwnd 保存窗口句柄。
	// 使用：传给 Windows API 读取状态或调整位置。
	// 边界：句柄只在当前系统会话中有效，不应持久化。
	Hwnd uintptr
	// PID 保存窗口所属进程 ID。
	// 使用：前端展示和进程匹配诊断。
	// 边界：PID 可能被系统复用，不能作为长期身份。
	PID uint32
	// Title 保存窗口标题。
	// 使用：前端调试页展示用户可识别的目标窗口。
	// 边界：浏览器标题会频繁变化，不应用作唯一匹配条件。
	Title string
	// ProcessName 保存进程文件名。
	// 使用：识别第一版浏览器监听目标，后续可替换为 DSPGAME.exe。
	// 边界：同名进程仍需结合路径或窗口标题进一步验证。
	ProcessName string
	// ExePath 保存进程完整可执行文件路径。
	// 使用：辅助排查匹配到的浏览器实例。
	// 边界：权限不足时可能为空字符串。
	ExePath string
	// Visible 表示窗口当前是否可见。
	// 使用：控制 Overlay 显示隐藏。
	// 边界：可见不代表未被其他窗口遮挡。
	Visible bool
	// Minimized 表示窗口是否最小化。
	// 使用：目标最小化时隐藏 Overlay。
	// 边界：最小化状态来自瞬时 API 查询。
	Minimized bool
	// Foreground 表示窗口是否为当前前台窗口。
	// 使用：浏览器被其他窗口覆盖时隐藏 Overlay，避免悬浮窗漂浮在非目标窗口上方。
	// 边界：只比较顶层窗口句柄，不代表窗口内容完全无遮挡。
	Foreground bool
	// Rect 保存窗口矩形。
	// 使用：Overlay 贴附目标窗口。
	// 边界：DWM 扩展边框失败时回退为普通窗口矩形。
	Rect Rect
	// AttachedRect 保存 Overlay 实际应该吸附的窗口矩形。
	// 使用：前端调试显示和后端确认吸附位置是否按目标窗口计算。
	// 边界：只在同步循环计算后赋值，普通窗口查找可能为空矩形。
	AttachedRect Rect
}

// WindowCandidate 描述一个可贴附目标窗口候选项。
// 使用：服务层调试接口展示浏览器窗口枚举结果，帮助判断实际命中的 HWND 是否符合预期。
// 边界：候选项只在一次枚举中有效，不代表窗口后续仍存在或仍保持同一 Z-order。
type WindowCandidate struct {
	// Snapshot 保存候选窗口快照。
	// 使用：前端诊断列表展示进程、标题、矩形和前台状态。
	// 边界：这是值拷贝，不应作为长期窗口身份保存。
	Snapshot Snapshot
	// Score 保存候选窗口评分。
	// 使用：排序时优先选择前台窗口和大面积主窗口。
	// 边界：评分规则只服务当前 Overlay 原型，可随目标游戏切换调整。
	Score int64
}
