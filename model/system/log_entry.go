package system

// 文件职责：定义统一日志事件的 SQLite 持久化模型。使用：诊断索引和日志查询；边界：不负责生成日志、不保存外部 API DTO。

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// LogEntry 保存 schemaVersion=2 的统一日志事件。
// 职责：以 action/source/reason/namespace/data/handler 为核心保存可诊断事实。
// 使用：诊断采集器写入，日志服务查询。
// 边界：JSONL 是原始证据，本表是可重建索引；字段不表达未经验证的根因。
type LogEntry struct {
	// ID 职责：保存日志唯一标识；使用：查询和导出；边界：不作为业务去重键。
	ID string `json:"id" gorm:"primaryKey;type:varchar(40);comment:日志唯一标识;column:id"`
	// SchemaVersion 职责：标识日志结构版本；使用：迁移和解析；边界：当前写入固定为 2。
	SchemaVersion int `json:"schemaVersion" gorm:"not null;default:2;index;comment:日志结构版本;column:schema_version"`
	// Action 职责：保存行为描述；使用：列表展示；边界：不写推测性根因。
	Action string `json:"action" gorm:"type:varchar(240);not null;comment:行为描述;column:action"`
	// Level 职责：保存严重程度；使用：筛选和着色；边界：只接受约定字符串。
	Level string `json:"level" gorm:"type:varchar(16);not null;index:idx_log_entry_level_time,priority:1;comment:日志等级;column:level"`
	// Source 职责：保存实际产生记录的位置；使用：区分 frontend/backend/runtime；边界：不代替 handler。
	Source string `json:"source" gorm:"type:varchar(120);not null;index:idx_log_entry_source_time,priority:1;comment:记录来源;column:source"`
	// Reason 职责：保存已知触发原因；使用：诊断详情；边界：未知原因必须明确写明。
	Reason string `json:"reason" gorm:"type:text;not null;comment:触发原因;column:reason"`
	// Namespace 职责：保存域.模块.事件稳定标识；使用：分层筛选；边界：不保存动态文案。
	Namespace string `json:"namespace" gorm:"type:varchar(180);not null;index:idx_log_entry_namespace_time,priority:1;comment:命名空间;column:namespace"`
	// Data 职责：保存结构化诊断证据；使用：恢复结果、阶段、错误和资源信息；边界：内容已脱敏并限长。
	Data datatypes.JSON `json:"data" gorm:"not null;comment:结构化诊断数据;column:data"`
	// EventTime 职责：保存事件发生时间；使用：排序和时间线；边界：由采集器生成。
	EventTime time.Time `json:"time" gorm:"not null;index:idx_log_entry_level_time,priority:2;index:idx_log_entry_source_time,priority:2;index:idx_log_entry_namespace_time,priority:2;comment:事件时间;column:event_time"`
	// Handler 职责：保存整次操作发起者；使用：区分 User/System/Unknown；边界：子步骤继承。
	Handler string `json:"handler" gorm:"type:varchar(16);not null;index:idx_log_entry_handler_time,priority:1;comment:触发者;column:handler"`
	// SessionID 职责：保存盒子运行会话号；使用：关联一次启动；边界：历史迁移使用 legacy。
	SessionID string `json:"sessionId" gorm:"type:varchar(40);not null;index:idx_log_entry_session_time,priority:1;comment:运行会话;column:session_id"`
	// OperationID 职责：保存一次操作关联号；使用：聚合时间线；边界：无法关联时为空。
	OperationID string `json:"operationId" gorm:"type:varchar(40);index:idx_log_entry_operation_time,priority:1;comment:操作关联;column:operation_id"`
	// AppVersion 职责：保存盒子版本；使用：回归诊断；边界：未知版本为空。
	AppVersion string `json:"appVersion" gorm:"type:varchar(64);index;comment:应用版本;column:app_version"`
	// DeletedAt 职责：持久化用户删除标记；使用：列表、导出及文件回放排除已删除记录；边界：不改写原始 JSONL 证据。
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index;column:deleted_at"`
}

// TableName 指定统一日志索引表名。
// 职责：覆盖 GORM 默认表名；使用：迁移和 CRUD；边界：只返回静态表名。
func (LogEntry) TableName() string { return "diagnostic_log_entries" }
