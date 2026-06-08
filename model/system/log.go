package system

import (
	"time"

	"gorm.io/datatypes"
)

// LogLevel 表示系统日志的重要等级。
// 使用：业务记录日志时按影响范围选择等级，查询端可按等级过滤重要操作。
type LogLevel int

const (
	// LogLevelDebug 表示调试信息，默认不用于关键业务审计。
	LogLevelDebug LogLevel = 10
	// LogLevelInfo 表示普通操作信息。
	LogLevelInfo LogLevel = 20
	// LogLevelWarn 表示需要关注但未阻断流程的异常状态。
	LogLevelWarn LogLevel = 30
	// LogLevelError 表示操作失败或需要用户介入的错误。
	LogLevelError LogLevel = 40
)

// LogType 表示系统日志的业务分类。
// 使用：与命名空间、模块、事件共同组成审计维度，便于后续做筛选与统计。
type LogType int

const (
	// LogTypeOperation 表示用户或系统触发的普通操作。
	LogTypeOperation LogType = 1
	// LogTypeConfiguration 表示配置读取、写入或刷新。
	LogTypeConfiguration LogType = 2
	// LogTypeInstallation 表示安装、卸载、更新等会修改本地文件或数据库的操作。
	LogTypeInstallation LogType = 3
	// LogTypeSecurity 表示权限、安全校验或可信度相关事件。
	LogTypeSecurity LogType = 4
)

// Log 保存系统重要操作日志。
// 边界：日志写入失败不应影响原业务流程；业务层应通过 helper 统一记录，避免直接拼表字段。
type Log struct {
	// ID 是日志主键。
	ID uint `json:"id" gorm:"primaryKey;autoIncrement;comment:主键ID;column:id;"`
	// Namespace 是日志命名空间，用于区分 system、modDependence、server 等业务域。
	Namespace string `json:"namespace" gorm:"type:varchar(80);not null;index:idx_logs_scope_time,priority:1;index:idx_logs_event,priority:1;comment:日志命名空间;column:namespace;"`
	// Module 是命名空间下的模块，例如 configuration、installer、process、port。
	Module string `json:"module" gorm:"type:varchar(80);not null;index:idx_logs_scope_time,priority:2;index:idx_logs_event,priority:2;comment:日志所属模块;column:module;"`
	// Event 是稳定事件名，例如 start、stop、openPort、setConfig。
	Event string `json:"event" gorm:"type:varchar(120);not null;index:idx_logs_event,priority:3;comment:日志事件名称;column:event;"`
	// Level 是日志等级，用于快速筛选普通信息、警告和错误。
	Level LogLevel `json:"level" gorm:"not null;default:20;index:idx_logs_level_time,priority:1;comment:日志等级;column:level;"`
	// Type 是日志类型，用于区分操作、配置、安装和安全等业务分类。
	Type LogType `json:"type" gorm:"not null;default:1;index:idx_logs_type_time,priority:1;comment:日志类型;column:type;"`
	// Content 是面向用户或维护者阅读的日志摘要。
	Content string `json:"content" gorm:"type:text;comment:日志正文;column:content;"`
	// Metadata 是结构化扩展信息，用于保存端口、PID、文件路径等不适合做固定列的细节。
	Metadata datatypes.JSON `json:"metadata" gorm:"comment:结构化扩展信息;column:metadata;"`
	// Error 是失败详情；成功事件保持为空。
	Error string `json:"error" gorm:"type:text;comment:错误详情;column:error;"`
	// DateTime 是日志发生时间。
	DateTime *time.Time `json:"dateTime" gorm:"not null;index:idx_logs_scope_time,priority:3;index:idx_logs_level_time,priority:2;index:idx_logs_type_time,priority:2;comment:日志发生时间;column:date_time;"`
}

// TableName 指定系统日志表名。
func (Log) TableName() string {
	return "system_logs"
}
