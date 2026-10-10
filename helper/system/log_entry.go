package system

// 文件职责：定义系统日志写入入口使用的 LogEntry 契约。
// 使用：service/system、helper/system 和各业务审计 helper 构造 LogEntry 后交给 RecordLog。
// 边界：只描述日志内容，不直接决定是否写入，也不访问数据库。

// LogEntry 描述一次需要持久化的重要操作。
// 职责：承载系统日志入库所需的维度、正文、扩展信息和错误详情。
// 使用：服务层或 helper 层把业务语义转换成 LogEntry，再交给 RecordLog 统一入库。
// 边界：只描述日志内容，不直接决定是否写入；是否记录由调用方业务流程判断。
type LogEntry struct {
	// Action 职责：保存行为描述；使用：日志列表；边界：不猜测根因。
	Action string
	// Level 职责：保存严重程度；使用：筛选和展示；边界：只能使用 logging 枚举。
	Level string
	// Source 职责：保存真实记录位置；使用：区分前后端和运行时；边界：不表示触发者。
	Source string
	// Reason 职责：保存已知触发原因；使用：错误详情；边界：未知原因明确标记。
	Reason string
	// Namespace 职责：保存域.模块.事件；使用：筛选和聚合；边界：必须是稳定标识。
	Namespace string
	// Data 职责：保存结构化事实；使用：差错；边界：由采集器脱敏。
	Data map[string]interface{}
	// Handler 职责：保存操作发起者；使用：区分 User/System/Unknown；边界：子步骤继承。
	Handler string
	// OperationID 职责：保存操作关联号；使用：还原时间线；边界：未知时为空。
	OperationID string
	// SessionID 职责：保存运行会话号；使用：关联启动；边界：采集器可自动补齐。
	SessionID string
	// Error 职责：接收原始错误文本；使用：转入 data.error；边界：不成为外部日志字段。
	Error string
}
