package system

// 文件职责：定义系统日志写入入口使用的 LogEntry 契约。
// 使用：service/system、helper/system 和各业务审计 helper 构造 LogEntry 后交给 RecordLog。
// 边界：只描述日志内容，不直接决定是否写入，也不访问数据库。

import systemModel "DysonSphereBox/model/system"

// LogEntry 描述一次需要持久化的重要操作。
// 职责：承载系统日志入库所需的维度、正文、扩展信息和错误详情。
// 使用：服务层或 helper 层把业务语义转换成 LogEntry，再交给 RecordLog 统一入库。
// 边界：只描述日志内容，不直接决定是否写入；是否记录由调用方业务流程判断。
type LogEntry struct {
	// Namespace 职责：保存日志所属业务域；使用：日志查询和筛选；边界：为空时写入层使用默认值。
	Namespace string
	// Module 职责：保存业务域下的具体模块；使用：日志查询和筛选；边界：为空时写入层使用默认值。
	Module string
	// Event 职责：保存可稳定检索的事件名；使用：日志查询和统计；边界：为空时写入层使用默认值。
	Event string
	// Level 职责：保存日志重要等级；使用：前端过滤和展示；边界：为 0 时默认按普通信息处理。
	Level systemModel.LogLevel
	// Type 职责：保存日志业务分类；使用：前端过滤和统计；边界：为 0 时默认按普通操作处理。
	Type systemModel.LogType
	// Content 职责：保存面向人阅读的日志摘要；使用：日志列表主文案；边界：结构化信息应放入 Metadata。
	Content string
	// Metadata 职责：保存结构化扩展信息；使用：日志详情和排障；边界：必须能被 json.Marshal 序列化。
	Metadata map[string]interface{}
	// Error 职责：保存失败详情；使用：失败日志详情展示；边界：成功日志保持为空。
	Error string
}
