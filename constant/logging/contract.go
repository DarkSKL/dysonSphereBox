package logging

// 文件职责：定义客户端各层共用的日志契约。使用：采集、落盘、查询和导出统一引用；边界：不执行存储或业务操作。

// 等级与触发者职责：固定跨层枚举；使用：日志写入和筛选；边界：Unknown 只用于无法确认的来源。
const (
	// ChangedEvent 职责：通知前端重新查询日志；使用：索引更新事件；边界：不携带可被前端当作权威列表的数据。
	ChangedEvent = "system:logsChanged"
	// Debug 职责：标识诊断细节；使用：阶段记录；边界：不表示操作成功。
	Debug = "Debug"
	// Info 职责：标识普通事实；使用：开始和完成记录；边界：结果另存 data.result。
	Info = "Info"
	// Warn 职责：标识需关注状态；使用：降级与冲突；边界：不等同于失败。
	Warn = "Warn"
	// Error 职责：标识失败；使用：异常记录；边界：不自动执行恢复。
	Error = "Error"
	// Fatal 职责：标识无法继续的故障；使用：启动失败；边界：是否退出由调用者决定。
	Fatal = "Fatal"
	// User 职责：标识用户发起；使用：用户操作及其子步骤；边界：不表示执行代码在前端。
	User = "User"
	// System 职责：标识系统发起；使用：启动、轮询和恢复；边界：不推断用户身份。
	System = "System"
	// Unknown 职责：标识无法还原的触发者；使用：历史迁移；边界：不得伪造为 User。
	Unknown = "Unknown"
)

// Entry 职责：统一客户端日志记录；使用：各层提交真实事实；边界：业务结果和错误放入 Data，身份与版本由采集端补齐。
type Entry struct {
	// ID 职责：唯一记录号；使用：合并与去重；边界：由采集端生成。
	ID string `json:"id"`
	// SchemaVersion 职责：格式版本；使用：兼容读写；边界：当前为 2。
	SchemaVersion int `json:"schemaVersion"`
	// Action 职责：中文行为描述；使用：列表摘要；边界：不是推测原因。
	Action string `json:"action"`
	// Level 职责：严重程度；使用：筛选与着色；边界：只能使用约定枚举。
	Level string `json:"level"`
	// Source 职责：实际记录位置；使用：区分 backend/frontend/runtime；边界：不代替触发者。
	Source string `json:"source"`
	// Reason 职责：已知触发原因；使用：诊断详情；边界：未知时明确说明，不猜根因。
	Reason string `json:"reason"`
	// Namespace 职责：域.模块.事件；使用：分层筛选；边界：稳定标识，不放路径或用户内容。
	Namespace string `json:"namespace"`
	// Data 职责：结构化证据；使用：结果、阶段、耗时、错误和资源标识；边界：统一脱敏和限长。
	Data map[string]interface{} `json:"data"`
	// Time 职责：带时区的发生时间；使用：排序；边界：RFC3339Nano，历史未知时间不伪造。
	Time string `json:"time"`
	// Handler 职责：整次操作的发起者；使用：User/System/Unknown；边界：子步骤继承发起者。
	Handler string `json:"handler"`
	// SessionID 职责：盒子运行会话号；使用：定位一次启动；边界：历史会话使用 legacy。
	SessionID string `json:"sessionId"`
	// OperationID 职责：关联一次操作；使用：时间线；边界：没有上下文时留空，不按时间猜测。
	OperationID string `json:"operationId"`
	// AppVersion 职责：采集时盒子版本；使用：版本回归诊断；边界：历史版本未知。
	AppVersion string `json:"appVersion"`
}
