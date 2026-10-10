package system

// 文件职责：定义日志筛选、分页和健康契约。使用：LogService 和日志页面；边界：不直接访问数据库或运行时。

import (
	"DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
)

// LogQuery 职责：承载检索条件；使用：分页、时间线和导出；边界：多选 nil 表示不限、空数组表示无结果，同维度多选优先于旧单选字段。
type LogQuery struct {
	// Namespaces 职责：模块多选；使用：匹配任一完整值或子命名空间；边界：nil 兼容旧 Namespace，空数组不匹配任何记录。
	Namespaces []string `json:"namespaces"`
	// Levels 职责：级别多选；使用：匹配任一选中级别；边界：nil 兼容旧 Level，空数组不匹配任何记录。
	Levels []string `json:"levels"`
	// Handlers 职责：触发者多选；使用：匹配任一选中触发者；边界：nil 兼容旧 Handler，空数组不匹配任何记录。
	Handlers []string `json:"handlers"`
	// Namespace 职责：稳定命名空间前缀；使用：按模块筛选；边界：不接受 SQL。
	Namespace string `json:"namespace"`
	// Level 职责：严重等级；使用：筛选；边界：空不过滤。
	Level string `json:"level"`
	// Source 职责：记录位置；使用：筛选；边界：精确匹配。
	Source string `json:"source"`
	// Handler 职责：发起者；使用：筛选；边界：精确匹配。
	Handler string `json:"handler"`
	// Keyword 职责：关键词；使用：摘要和证据搜索；边界：字面量匹配。
	Keyword string `json:"keyword"`
	// SessionID 职责：会话号；使用：启动筛选；边界：空不过滤。
	SessionID string `json:"sessionId"`
	// OperationID 职责：关联操作；使用：时间线；边界：空不过滤。
	OperationID string `json:"operationId"`
	// From 职责：起始时间；使用：范围检索；边界：RFC3339Nano。
	From string `json:"from"`
	// To 职责：结束时间；使用：范围检索；边界：RFC3339Nano。
	To string `json:"to"`
	// Page 职责：页码；使用：分页；边界：从 1 开始。
	Page int `json:"page"`
	// PageSize 职责：条数；使用：分页；边界：最多 200。
	PageSize int `json:"pageSize"`
}

// LogListResult 职责：分页结果；使用：日志页；边界：索引异常返回文件读取结果及告警。
type LogListResult struct {
	// Items 职责：当前页；使用：列表；边界：可为空。
	Items []logging.Entry `json:"items"`
	// Total 职责：筛选后总数；使用：分页；边界：文件降级只统计保留文件。
	Total int64 `json:"total"`
	// Page 职责：实际页码；使用：分页；边界：已归一化。
	Page int `json:"page"`
	// PageSize 职责：实际大小；使用：分页；边界：已限流。
	PageSize int `json:"pageSize"`
	// Warning 职责：降级说明；使用：告警；边界：不能当作无错误。
	Warning string `json:"warning"`
}

// LogHealth 职责：采集与索引状态；使用：页面与诊断包；边界：溢出和落盘失败均显示。
type LogHealth struct {
	// File 职责：文件健康；使用：实际目录；边界：不受数据库影响。
	File diagnostics.Health `json:"file"`
	// IndexError 职责：索引错误；使用：告警；边界：文件仍可用。
	IndexError string `json:"indexError"`
	// Pending 职责：待入库数；使用：识别延迟；边界：不是未落盘数。
	Pending int `json:"pending"`
}
