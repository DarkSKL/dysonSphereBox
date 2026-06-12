package sessionstate

// 文件职责：定义安装会话的组件规划和清理日志临时结构。
// 使用：失败清理、目录清理和审计日志刷新时读写这些轻量状态。
// 边界：这些结构只服务当前安装会话，不持久化清理日志、不执行实际删除或日志写入。

// ComponentPlan 表示安装会话计划处理的组件目录。
// 职责：记录组件 key 与计划安装目录。
// 使用：失败清理时补充删除空目录和诊断日志。
// 边界：只代表计划，不代表组件已经成功安装。
type ComponentPlan struct {
	// ComponentKey 职责：记录计划组件 key；使用：共享判断和清理日志；边界：不能为空才可用于组件级清理。
	ComponentKey string `json:"componentKey"`
	// InstallDir 职责：记录计划组件安装目录名；使用：失败清理时生成目录候选；边界：只记录目录名，不含 BepInEx 根路径。
	InstallDir string `json:"installDir"`
}

// CleanupLogItem 表示一次安装会话清理日志条目。
// 职责：暂存清理对象、动作、原因和错误，最后批量写入审计日志。
// 使用：清理文件、目录、备份或数据库记录时收集。
// 边界：仅在当前清理流程内存活，不持久化到 manifest。
type CleanupLogItem struct {
	// PackageKey 职责：记录清理所属包；使用：写入系统日志 metadata；边界：不能为空。
	PackageKey string
	// Kind 职责：记录清理对象类型；使用：区分 file、dir、buffer、record 等；边界：只作日志维度。
	Kind string
	// Target 职责：记录清理目标路径或记录标识；使用：诊断清理行为；边界：可能为空。
	Target string
	// Action 职责：记录执行动作；使用：日志展示 remove、restore、skip 等；边界：只作日志文本。
	Action string
	// Reason 职责：记录动作原因；使用：解释跳过或删除依据；边界：不参与程序分支。
	Reason string
	// Err 职责：记录该项清理错误；使用：决定日志级别和错误文本；边界：不在日志刷新时重试。
	Err error
}
