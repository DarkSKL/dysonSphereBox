package system

// 文件职责：定义系统日志分页查询请求和分页结果 DTO。
// 使用：LogService.ListLogs 接收前端查询并返回 LogListResult。
// 边界：只描述查询和结果形状，不直接访问数据库、不创建日志、不删除日志。

import systemModel "DysonSphereBox/model/system"

// LogQuery 描述系统日志分页查询条件。
// 职责：承载日志页筛选条件和分页参数。
// 使用：LogService.ListLogs 接收前端查询时使用。
// 边界：空筛选项表示不过滤；分页参数会由 NormalizeLogPage 归一化。
type LogQuery struct {
	// Namespace 职责：保存日志命名空间筛选值；使用：日志页按业务域过滤；边界：为空时不过滤。
	Namespace string `json:"namespace"`
	// Module 职责：保存模块筛选值；使用：日志页按业务模块过滤；边界：为空时不过滤。
	Module string `json:"module"`
	// Event 职责：保存事件筛选值；使用：日志页按稳定事件名过滤；边界：为空时不过滤。
	Event string `json:"event"`
	// Level 职责：保存日志等级筛选值；使用：日志页按 info/warn/error 等等级过滤；边界：为 0 时不过滤。
	Level systemModel.LogLevel `json:"level"`
	// Type 职责：保存日志类型筛选值；使用：日志页按操作、配置、安装等类型过滤；边界：为 0 时不过滤。
	Type systemModel.LogType `json:"type"`
	// Page 职责：保存从 1 开始的页码；使用：LogService.ListLogs 计算 Offset；边界：小于 1 时使用默认值。
	Page int `json:"page"`
	// PageSize 职责：保存每页数量；使用：LogService.ListLogs 设置 Limit；边界：小于 1 使用默认值，超过上限时会被裁剪。
	PageSize int `json:"pageSize"`
}

// LogListResult 表示系统日志分页查询结果。
// 职责：承载日志列表、总数和实际分页参数。
// 使用：LogService.ListLogs 返回给日志页。
// 边界：Items 可为空；Total 是筛选后的总数，不代表数据库全部日志数量。
type LogListResult struct {
	// Items 职责：保存当前页日志 DTO；使用：前端表格渲染；边界：空切片表示当前页无数据。
	Items []LogDTO `json:"items"`
	// Total 职责：保存符合筛选条件的日志总数；使用：前端分页器计算页数；边界：不受当前页大小限制。
	Total int64 `json:"total"`
	// Page 职责：保存实际查询页码；使用：前端同步分页状态；边界：已经过 NormalizeLogPage 归一化。
	Page int `json:"page"`
	// PageSize 职责：保存实际查询分页大小；使用：前端同步分页大小；边界：已经过 NormalizeLogPage 裁剪。
	PageSize int `json:"pageSize"`
}

// CreateLogRequest 描述前端主动创建日志的请求。
// 职责：承载前端补充审计日志所需字段。
// 使用：LogService.CreateLog 接收前端无法由后端感知的重要交互。
// 边界：业务关键流程仍应由后端 helper 自动记录，前端不应替代后端埋点。
type CreateLogRequest struct {
	// Namespace 职责：保存日志所属业务域；使用：RecordLog 定位日志范围；边界：为空时由 helper 使用默认值。
	Namespace string `json:"namespace"`
	// Module 职责：保存业务域下的具体模块；使用：RecordLog 定位日志模块；边界：为空时由 helper 使用默认值。
	Module string `json:"module"`
	// Event 职责：保存可稳定检索的事件名；使用：RecordLog 写入事件维度；边界：为空时由 helper 使用默认值。
	Event string `json:"event"`
	// Level 职责：保存日志重要等级；使用：RecordLog 写入等级维度；边界：为 0 时按普通信息记录。
	Level systemModel.LogLevel `json:"level"`
	// Type 职责：保存日志业务分类；使用：RecordLog 写入类型维度；边界：为 0 时按普通操作记录。
	Type systemModel.LogType `json:"type"`
	// Content 职责：保存面向人阅读的日志摘要；使用：日志列表展示；边界：结构化信息应放入 Metadata。
	Content string `json:"content"`
	// Metadata 职责：保存结构化扩展信息；使用：日志详情和诊断；边界：必须可 JSON 序列化。
	Metadata map[string]interface{} `json:"metadata"`
	// Error 职责：保存失败详情；使用：失败日志详情展示；边界：成功日志保持为空。
	Error string `json:"error"`
}
