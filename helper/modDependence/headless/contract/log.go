package contract

// 文件职责：定义开服控制台日志快照契约。
// 使用：日志读取 helper 构造后由 HeadlessService 返回。
// 边界：只承载日志尾部，不将日志内容推断为玩家连接或网络指标。

// HeadlessLogLine 职责：承载原始日志行与等级；使用：控制台搜索、筛选；边界：多行堆栈保留为独立行。
type HeadlessLogLine struct {
	// Level 职责：保存规范等级；使用：日志颜色与筛选；边界：未知文本为 info。
	Level string `json:"level"`
	// Content 职责：保留原始日志文本；使用：终端展示；边界：只渲染为文本。
	Content string `json:"content"`
}

// HeadlessLogSnapshot 职责：承载有限日志快照；使用：控制台定时刷新；边界：文件缺失是正常空态，I/O 失败由方法返回。
type HeadlessLogSnapshot struct {
	// Path 职责：记录固定日志路径；使用：排错与导出；边界：不能从前端指定任意文件。
	Path string `json:"path"`
	// Exists 职责：指示日志是否存在；使用：空态区分；边界：存在不保证本次启动产生。
	Exists bool `json:"exists"`
	// UpdatedAt 职责：记录文件修改时间；使用：底部时间；边界：不是采样时间。
	UpdatedAt string `json:"updatedAt"`
	// Truncated 职责：标识前部截断；使用：快照提示；边界：导出使用原始完整文件。
	Truncated bool `json:"truncated"`
	// Lines 职责：保存最多 300 行；使用：前端过滤；边界：不缓存整份日志。
	Lines []HeadlessLogLine `json:"lines"`
}
