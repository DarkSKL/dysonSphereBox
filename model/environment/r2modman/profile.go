package r2modman

// 文件职责：保存盒子已同步的 r2modman Profile 快照。
// 使用：环境领域统一注册迁移，Profile 同步与显式修改后持久化。
// 边界：只描述缓存表，不扫描外部目录，不替代 R2 的 mods.yml。

import (
	"gorm.io/datatypes"
	"time"
)

// ProfileSnapshot 职责：持久化每个 Profile 的资源快照；使用：资源页无感读取与下拉切换；边界：外部文件变化只在显式同步时刷新。
type ProfileSnapshot struct {
	// PathKey 职责：提供大小写归一的目录身份；使用：主键查询；边界：不作为文件写入路径。
	PathKey string `gorm:"primaryKey;type:text"`
	// Path 职责：保存已验证的规范化目录；使用：后续显式操作定位；边界：修改前仍重新校验。
	Path string `gorm:"type:text;not null"`
	// Name 职责：保存配置档显示名；使用：下拉列表；边界：只读快照值。
	Name string `gorm:"not null"`
	// SnapshotJSON 职责：保存包列表、数量和锁定状态；使用：不扫描文件的页面读取；边界：结构由 helper 契约解释。
	SnapshotJSON datatypes.JSON `gorm:"not null"`
	// SyncedAt 职责：记录最近同步时间；使用：诊断缓存时效；边界：不代表 R2 的安装时间。
	SyncedAt time.Time
}

// TableName 职责：提供稳定缓存表名；使用：GORM 迁移与查询；边界：不执行读写。
func (ProfileSnapshot) TableName() string { return "environment_r2modman_profiles" }
