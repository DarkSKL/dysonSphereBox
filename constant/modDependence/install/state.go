package install

// 文件职责：维护 Mod 依赖安装流程的稳定状态常量。
// 使用：installer、model 查询和前端进度快照通过这些值识别安装中、失败、冲突和完成状态。
// 边界：只定义可持久化状态名，不执行安装流程、数据库查询或 Wails 事件推送；修改常量值必须同步数据迁移和前端状态映射。

// State 表示 Mod 安装记录和进度快照中的状态值。
// 职责：为安装状态机提供强类型字符串。
// 使用：安装器保存 installed package、恢复中断安装和前端进度快照时使用。
// 边界：该类型只表达状态名称，不验证状态迁移是否合法。
type State string

// 安装状态常量组定义安装器可持久化或可推送给前端的状态。
// 职责：集中维护 running、done、failed、installed、uninstalled、conflict 等稳定状态名。
// 使用：安装进度、数据库安装记录、冲突恢复和状态过滤共同引用。
// 边界：常量值已经参与数据库和前端协议，不能随意重命名。
const (
	// StateRunning 职责：表示安装任务正在执行；使用：开始安装、继续冲突后写入；边界：进程异常退出后会被恢复流程标记失败。
	StateRunning State = "running"
	// StateDone 职责：表示一次进度步骤完成；使用：进度落库前转换为 installed；边界：不直接作为持久最终态。
	StateDone State = "done"
	// StateFailed 职责：表示安装任务失败；使用：失败快照和失败安装记录过滤；边界：不会自动重试。
	StateFailed State = "failed"
	// StateInstalled 职责：表示安装任务已完成并落库；使用：已安装包查询和状态过滤；边界：不作为临时进度恢复状态。
	StateInstalled State = "installed"
	// StateUninstalled 职责：表示卸载任务已完成；使用：卸载运行时通知前端；边界：卸载后通常删除安装记录，不依赖该值恢复。
	StateUninstalled State = "uninstalled"
	// StateConflict 职责：表示安装暂停等待用户处理文件冲突；使用：冲突快照、继续安装入口和恢复过滤；边界：保留到用户选择覆盖、跳过或失败。
	StateConflict State = "conflict"
)
