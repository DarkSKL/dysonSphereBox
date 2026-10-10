package system

// 文件职责：定义历史系统日志数据库表模型。
// 使用：由 core 自动迁移并由 helper/system 写入、service/system 查询。
// 边界：不定义日志枚举、不包含远端接口 DTO、服务编排或文件系统操作。

import (
	"time"

	"gorm.io/datatypes"
)

// Log 保存迁移前的旧格式系统操作日志。
// 职责：描述 system_logs 表中的审计日志记录。
// 使用：helper/system 统一写入，LogService 分页查询并转换为 DTO。
// 边界：日志写入失败不应影响原业务流程；业务层应通过 helper 统一记录，避免直接拼表字段。
type Log struct {
	// ID 是日志主键。
	// 职责：提供日志记录的数据库自增标识。
	// 使用：GORM 创建和查询日志时自动维护。
	// 边界：不作为业务事件去重依据。
	ID uint `json:"id" gorm:"primaryKey;autoIncrement;comment:主键ID;column:id;"`
	// Namespace 是日志命名空间，用于区分 system、modDependence、server 等业务域。
	// 职责：标记日志所属顶层业务域。
	// 使用：日志查询按业务域筛选和分组时读取。
	// 边界：必须由日志 helper 归一化，模型层不推断默认值。
	Namespace string `json:"namespace" gorm:"type:varchar(80);not null;index:idx_logs_scope_time,priority:1;index:idx_logs_event,priority:1;comment:日志命名空间;column:namespace;"`
	// Module 是命名空间下的模块，例如 configuration、installer、process、port。
	// 职责：标记命名空间内的具体功能模块。
	// 使用：前端筛选配置、安装、进程等模块日志时读取。
	// 边界：只保存稳定模块名，不保存用户展示文案。
	Module string `json:"module" gorm:"type:varchar(80);not null;index:idx_logs_scope_time,priority:2;index:idx_logs_event,priority:2;comment:日志所属模块;column:module;"`
	// Event 是稳定事件名，例如 start、stop、openPort、setConfig。
	// 职责：标记一次可统计的业务事件。
	// 使用：审计列表和后续统计按事件名聚合。
	// 边界：事件语义由调用方定义，模型层不校验事件枚举。
	Event string `json:"event" gorm:"type:varchar(120);not null;index:idx_logs_event,priority:3;comment:日志事件名称;column:event;"`
	// Level 是日志等级，用于快速筛选普通信息、警告和错误。
	// 职责：保存日志严重程度。
	// 使用：日志页按 debug/info/warn/error 筛选。
	// 边界：非法等级由写入 helper 归一化，模型层只承载数值。
	Level LogLevel `json:"level" gorm:"not null;default:20;index:idx_logs_level_time,priority:1;comment:日志等级;column:level;"`
	// Type 是日志类型，用于区分操作、配置、安装和安全等业务分类。
	// 职责：保存日志业务分类。
	// 使用：前端按操作、配置、安装、安全分类查看日志。
	// 边界：不替代 Namespace、Module、Event 的细粒度语义。
	Type LogType `json:"type" gorm:"not null;default:1;index:idx_logs_type_time,priority:1;comment:日志类型;column:type;"`
	// Content 是面向用户或维护者阅读的日志摘要。
	// 职责：保存用户可读的日志正文。
	// 使用：日志列表主要文本和审计导出读取。
	// 边界：不承载结构化字段，结构化信息应放入 Metadata。
	Content string `json:"content" gorm:"type:text;comment:日志正文;column:content;"`
	// Metadata 是结构化扩展信息，用于保存端口、PID、文件路径等不适合做固定列的细节。
	// 职责：保存端口、PID、包 key、路径等扩展上下文。
	// 使用：详情弹窗、排查和未来统计读取。
	// 边界：必须是可序列化 JSON，不作为必填索引字段。
	Metadata datatypes.JSON `json:"metadata" gorm:"comment:结构化扩展信息;column:metadata;"`
	// Error 是失败详情；成功事件保持为空。
	// 职责：保存错误链或失败说明。
	// 使用：错误日志详情和排查时展示。
	// 边界：成功事件为空，业务错误恢复策略不在模型层表达。
	Error string `json:"error" gorm:"type:text;comment:错误详情;column:error;"`
	// DateTime 是日志发生时间。
	// 职责：记录日志产生时刻。
	// 使用：按时间排序、分页和范围筛选。
	// 边界：由写入 helper 统一填充，模型层不生成时间。
	DateTime *time.Time `json:"dateTime" gorm:"not null;index:idx_logs_scope_time,priority:3;index:idx_logs_level_time,priority:2;index:idx_logs_type_time,priority:2;comment:日志发生时间;column:date_time;"`
}

// TableName 指定系统日志表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (Log) TableName() string {
	return "system_logs"
}
