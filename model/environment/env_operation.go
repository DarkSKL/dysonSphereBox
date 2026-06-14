package environment

// 文件职责：定义环境管理操作记录持久化模型。
// 使用：启用、禁用、导入、移动、重命名、删除等长任务记录计划、状态和结果。
// 边界：只保存操作审计结构，不承载系统日志查询维度，不直接执行业务回滚。

import (
	"time"

	"gorm.io/datatypes"
)

// EnvOperation 保存环境管理资源操作记录。
// 职责：记录一次环境资源操作的类型、状态、用户可读消息和结构化详情。
// 使用：长任务查询、启动恢复和冲突处理流程读取。
// 边界：系统级日志仍写入 system_logs；该表用于环境业务自己的可恢复操作证据。
type EnvOperation struct {
	// ID 职责：保存本地数据库自增主键；使用：前端查询操作详情；边界：不作为跨设备身份。
	ID uint `json:"id" gorm:"primaryKey;autoIncrement;comment:主键ID;column:id;"`
	// Operation 职责：保存操作类型；使用：区分 enable、disable、rename、import、delete、move；边界：模型层不校验状态机。
	Operation string `json:"operation" gorm:"index;size:64;not null;comment:操作类型;column:operation;"`
	// Kind 职责：保存操作资源类型；使用：按 Mod、蓝图、存档筛选操作；边界：只保存稳定文本。
	Kind string `json:"kind" gorm:"index;size:32;not null;comment:资源类型;column:kind;"`
	// ItemID 职责：保存关联资源 ID；使用：按资源追溯操作历史；边界：存档操作可保存扫描 ID 或为空。
	ItemID string `json:"itemId" gorm:"index;size:64;comment:资源ID;column:item_id;"`
	// Status 职责：保存操作状态；使用：前端展示 pending、running、success、failed 等状态；边界：状态迁移由服务层控制。
	Status string `json:"status" gorm:"index;size:32;not null;comment:操作状态;column:status;"`
	// Message 职责：保存用户可读操作说明；使用：操作列表和错误提示展示；边界：结构化详情应放入 DetailJSON。
	Message string `json:"message" gorm:"type:text;comment:操作说明;column:message;"`
	// DetailJSON 职责：保存操作计划、冲突、快照和结果等结构化详情；使用：冲突解决和启动恢复读取；边界：必须是可序列化 JSON。
	DetailJSON datatypes.JSON `json:"detailJson" gorm:"comment:操作详情;column:detail_json;"`
	// CreatedAt 职责：保存操作创建时间；使用：排序和恢复时定位最近操作；边界：不代表操作完成时间。
	CreatedAt time.Time `json:"createdAt" gorm:"comment:创建时间;column:created_at;"`
}

// TableName 指定环境操作记录表名。
// 职责：覆盖 GORM 默认表名，保持方案中的 environment_operations 命名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不访问数据库。
func (EnvOperation) TableName() string {
	return "environment_operations"
}
