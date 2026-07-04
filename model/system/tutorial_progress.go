package system

// 文件职责：定义首次使用引导教程阅读进度的数据库模型。
// 使用：由系统教程服务通过 GORM 查询、创建和更新每个页面每个教程版本的阅读状态。
// 边界：只描述持久化结构，不包含 Driver.js 步骤定义、前端页面选择器或服务编排逻辑。

import "time"

// TutorialProgress 保存页面教程的完成或跳过状态。
// 职责：描述 system_tutorial_progresses 表中 page_key 与 tutorial_version 维度下的用户阅读记录。
// 使用：TutorialService 查询当前版本是否应自动展示，并在完成或跳过时 upsert 写入。
// 边界：模型层不校验 page_key 是否存在，也不判断教程版本是否最新。
type TutorialProgress struct {
	// ID 是教程进度记录主键。
	// 职责：提供数据库自增标识。
	// 使用：GORM 创建、更新和删除记录时维护。
	// 边界：业务逻辑不应依赖该值判断教程状态。
	ID uint64 `json:"id" gorm:"primaryKey;autoIncrement;comment:主键ID;column:id;"`
	// PageKey 是教程所属页面或全局框架的稳定 key。
	// 职责：定位 global、home、mod 等教程组。
	// 使用：服务层按 page_key 与 tutorial_version 查询唯一进度。
	// 边界：不保存路由路径，路由到 key 的映射由前端教程模块维护。
	PageKey string `json:"pageKey" gorm:"type:varchar(80);not null;uniqueIndex:idx_tutorial_page_version,priority:1;comment:教程页面Key;column:page_key;"`
	// TutorialVersion 是教程内容版本。
	// 职责：支持教程内容升级后让旧用户重新看到新版教程。
	// 使用：前端 registry 提供当前版本，服务层与 page_key 一起作为唯一键。
	// 边界：版本号必须由服务层校验为正数，模型层不推断默认版本。
	TutorialVersion int `json:"tutorialVersion" gorm:"not null;uniqueIndex:idx_tutorial_page_version,priority:2;comment:教程版本;column:tutorial_version;"`
	// Status 是教程状态。
	// 职责：保存 completed 或 skipped。
	// 使用：服务层写入稳定枚举值，前端根据 DTO 的布尔字段判断是否自动启动。
	// 边界：模型层不校验枚举合法性，非法值由服务层拒绝。
	Status string `json:"status" gorm:"type:varchar(40);not null;index:idx_tutorial_status;comment:教程状态;column:status;"`
	// CompletedAt 是教程完成时间。
	// 职责：记录用户点击完成教程的时刻。
	// 使用：完成状态写入时填充；前端可用于后续展示阅读历史。
	// 边界：跳过状态下允许为空。
	CompletedAt *time.Time `json:"completedAt" gorm:"comment:完成时间;column:completed_at;"`
	// SkippedAt 是教程跳过时间。
	// 职责：记录用户关闭教程的时刻。
	// 使用：跳过状态写入时填充；前端后续同版本不再自动展示。
	// 边界：完成状态下允许为空。
	SkippedAt *time.Time `json:"skippedAt" gorm:"comment:跳过时间;column:skipped_at;"`
	// CreatedAt 是记录创建时间。
	// 职责：保留教程进度首次写入时刻。
	// 使用：由 GORM 创建记录时自动维护。
	// 边界：不作为教程是否展示的判断依据。
	CreatedAt time.Time `json:"createdAt" gorm:"comment:创建时间;column:created_at;"`
	// UpdatedAt 是记录更新时间。
	// 职责：保留教程状态最近一次变化时刻。
	// 使用：由 GORM 更新记录时自动维护。
	// 边界：不替代 completed_at 或 skipped_at 的业务语义。
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定教程进度表名。
// 职责：覆盖 GORM 默认表名，保持系统领域表名前缀一致。
// 使用：GORM AutoMigrate 与 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (TutorialProgress) TableName() string {
	return "system_tutorial_progresses"
}
