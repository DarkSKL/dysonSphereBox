package overlay

// 文件职责：定义悬浮窗服务暴露给前端的 DTO。
// 使用：主界面调试入口和 Overlay 页面通过 Wails 绑定读取这些结构。
// 边界：DTO 不直接暴露 Windows 句柄操作能力，句柄仅作为诊断数值返回。

import overlayWindow "DysonSphereBox/helper/overlay/window"

// WindowRectDTO 描述前端可展示的窗口矩形。
// 使用：前端状态卡片显示目标浏览器窗口位置和大小。
// 边界：只表达屏幕坐标，不负责 DPI 解释。
type WindowRectDTO struct {
	// Left 表示窗口左边界坐标。
	// 使用：前端展示。
	// 边界：可能为负数。
	Left int `json:"left"`
	// Top 表示窗口上边界坐标。
	// 使用：前端展示。
	// 边界：可能为负数。
	Top int `json:"top"`
	// Right 表示窗口右边界坐标。
	// 使用：前端展示。
	// 边界：只作为诊断信息。
	Right int `json:"right"`
	// Bottom 表示窗口下边界坐标。
	// 使用：前端展示。
	// 边界：只作为诊断信息。
	Bottom int `json:"bottom"`
	// Width 表示窗口宽度。
	// 使用：前端展示贴附尺寸。
	// 边界：异常窗口可能为非正数。
	Width int `json:"width"`
	// Height 表示窗口高度。
	// 使用：前端展示贴附尺寸。
	// 边界：异常窗口可能为非正数。
	Height int `json:"height"`
}

// WindowSnapshotDTO 描述前端可展示的目标窗口快照。
// 使用：调试页和 Overlay HUD 显示当前监听到的浏览器窗口。
// 边界：这是瞬时状态，不保证下一帧仍然有效。
type WindowSnapshotDTO struct {
	// Found 表示是否找到目标窗口。
	// 使用：前端决定显示命中状态或等待状态。
	// 边界：false 时其他字段为空或零值。
	Found bool `json:"found"`
	// Hwnd 表示窗口句柄数值。
	// 使用：仅用于调试诊断。
	// 边界：不要从前端持久化或回传作为长期身份。
	Hwnd uintptr `json:"hwnd"`
	// PID 表示进程 ID。
	// 使用：前端展示当前命中的浏览器进程。
	// 边界：PID 可被复用。
	PID uint32 `json:"pid"`
	// Title 表示窗口标题。
	// 使用：前端展示用户可识别的目标窗口。
	// 边界：浏览器标题会频繁变化。
	Title string `json:"title"`
	// ProcessName 表示进程文件名。
	// 使用：前端显示命中的浏览器类型。
	// 边界：第一版仅用于原型。
	ProcessName string `json:"processName"`
	// ExePath 表示进程完整路径。
	// 使用：前端调试时辅助判断是否命中预期进程。
	// 边界：权限不足时可能为空。
	ExePath string `json:"exePath"`
	// Visible 表示目标窗口是否可见。
	// 使用：前端展示状态。
	// 边界：可见不代表 Overlay 一定在最上层。
	Visible bool `json:"visible"`
	// Minimized 表示目标窗口是否最小化。
	// 使用：前端提示 Overlay 隐藏原因。
	// 边界：状态是瞬时值。
	Minimized bool `json:"minimized"`
	// Foreground 表示目标窗口是否处于前台。
	// 使用：前端展示 Overlay 是否应当显示。
	// 边界：只表示前台窗口匹配，不代表窗口完全无遮挡。
	Foreground bool `json:"foreground"`
	// Rect 表示目标窗口矩形。
	// 使用：前端展示贴附坐标。
	// 边界：未找到时为零值。
	Rect WindowRectDTO `json:"rect"`
	// AttachedRect 表示 Overlay 实际吸附的小窗口矩形。
	// 使用：前端调试吸附是否按目标窗口计算。
	// 边界：未计算吸附位置时为零值。
	AttachedRect WindowRectDTO `json:"attachedRect"`
}

// WindowCandidateDTO 描述前端可展示的窗口候选诊断项。
// 使用：主调试页列出浏览器窗口枚举结果，确认 Overlay 实际会选中哪个窗口。
// 边界：只用于调试，不暴露窗口操作能力。
type WindowCandidateDTO struct {
	// Snapshot 表示候选窗口快照。
	// 使用：前端展示标题、进程、坐标和前台状态。
	// 边界：字段来自瞬时枚举。
	Snapshot WindowSnapshotDTO `json:"snapshot"`
	// Score 表示后端选择该候选时使用的评分。
	// 使用：前端按分值理解为什么某个窗口被优先命中。
	// 边界：评分规则可能随目标进程策略调整。
	Score int64 `json:"score"`
}

// OverlayProcessStatusDTO 描述主进程管理的附进程状态。
// 使用：主界面按钮展示 Overlay 是否已启动以及当前 PID。
// 边界：只表达主进程记录的状态，附进程可能在状态刷新前已经退出。
type OverlayProcessStatusDTO struct {
	// Running 表示主进程认为 Overlay 附进程正在运行。
	// 使用：前端切换启动/停止按钮。
	// 边界：调用状态接口时会尽量刷新一次进程退出状态。
	Running bool `json:"running"`
	// PID 表示 Overlay 附进程 PID。
	// 使用：前端调试展示。
	// 边界：未运行时为 0。
	PID int `json:"pid"`
}

// NewWindowSnapshotDTO 将 helper 层窗口快照转换为前端 DTO。
// 使用：OverlayService 和 OverlayApp 暴露窗口状态时调用。
// 边界：nil 快照转换为 Found=false，避免前端处理空对象。
func NewWindowSnapshotDTO(snapshot *overlayWindow.Snapshot) WindowSnapshotDTO {
	if snapshot == nil {
		return WindowSnapshotDTO{Found: false}
	}
	return WindowSnapshotDTO{
		Found:       true,
		Hwnd:        snapshot.Hwnd,
		PID:         snapshot.PID,
		Title:       snapshot.Title,
		ProcessName: snapshot.ProcessName,
		ExePath:     snapshot.ExePath,
		Visible:     snapshot.Visible,
		Minimized:   snapshot.Minimized,
		Foreground:  snapshot.Foreground,
		Rect: WindowRectDTO{
			Left:   snapshot.Rect.Left,
			Top:    snapshot.Rect.Top,
			Right:  snapshot.Rect.Right,
			Bottom: snapshot.Rect.Bottom,
			Width:  snapshot.Rect.Width(),
			Height: snapshot.Rect.Height(),
		},
		AttachedRect: WindowRectDTO{
			Left:   snapshot.AttachedRect.Left,
			Top:    snapshot.AttachedRect.Top,
			Right:  snapshot.AttachedRect.Right,
			Bottom: snapshot.AttachedRect.Bottom,
			Width:  snapshot.AttachedRect.Width(),
			Height: snapshot.AttachedRect.Height(),
		},
	}
}

// NewWindowCandidateDTOs 将 helper 候选项转换为前端 DTO 列表。
// 使用：OverlayService.ListBrowserWindowCandidates 调用，保持候选诊断与窗口查找共用同一数据形状。
// 边界：空列表返回空切片，避免前端判断 nil。
func NewWindowCandidateDTOs(candidates []overlayWindow.WindowCandidate) []WindowCandidateDTO {
	result := make([]WindowCandidateDTO, 0, len(candidates))
	for _, candidate := range candidates {
		snapshot := candidate.Snapshot
		result = append(result, WindowCandidateDTO{
			Snapshot: NewWindowSnapshotDTO(&snapshot),
			Score:    candidate.Score,
		})
	}
	return result
}
