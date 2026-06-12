package system

// 文件职责：定义系统日志的业务分类枚举。
// 使用：helper/system 构造日志记录和 service/system 查询日志时引用，用于业务维度筛选。
// 边界：只用于分类，不替代 Namespace、Module、Event 的具体语义。

// LogType 表示系统日志的业务分类。
// 职责：定义系统日志的业务维度分类。
// 使用：与命名空间、模块、事件共同组成审计维度，便于后续做筛选与统计。
// 边界：只用于分类，不替代 Namespace/Module/Event 的具体语义。
type LogType int

// 日志类型常量职责：定义当前系统日志支持的业务分类。
// 使用：helper 构造 LogEntry 和前端筛选类型时引用。
// 边界：新增类型需要同步前端展示文案和统计逻辑。
const (
	// LogTypeOperation 表示用户或系统触发的普通操作。
	// 职责：标记普通操作日志。
	// 使用：启动、停止、刷新、查询等非配置/安装类事件。
	// 边界：不用于安装、配置或安全分类事件。
	LogTypeOperation LogType = 1
	// LogTypeConfiguration 表示配置读取、写入或刷新。
	// 职责：标记配置相关日志。
	// 使用：配置保存、BaseURL 刷新、游戏路径初始化等场景。
	// 边界：不用于安装文件变更事件。
	LogTypeConfiguration LogType = 2
	// LogTypeInstallation 表示安装、卸载、更新等会修改本地文件或数据库的操作。
	// 职责：标记安装、卸载和更新类日志。
	// 使用：Mod 安装、BepInEx 安装、自更新等场景。
	// 边界：不用于纯状态查询。
	LogTypeInstallation LogType = 3
	// LogTypeSecurity 表示权限、安全校验或可信度相关事件。
	// 职责：标记安全、权限或可信度相关日志。
	// 使用：未来安全校验、来源可信度或权限异常事件。
	// 边界：当前不承载普通错误等级含义。
	LogTypeSecurity LogType = 4
)
