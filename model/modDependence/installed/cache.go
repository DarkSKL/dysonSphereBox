package installed

// 文件职责：定义来源侧远端包列表缓存持久化模型。
// 使用：Thunderstore 等市场 helper 在全量接口读取失败或未过期时复用 ModRemotePackageCache。
// 边界：缓存不代表远端实时状态，不保存安装状态、不参与安装文件清理。

import (
	"time"

	"gorm.io/datatypes"
)

// ModRemotePackageCache 记录来源侧包列表缓存。
// 职责：保存远端市场包列表的来源缓存内容和过期时间。
// 使用：Thunderstore 等全量接口读取失败或未过期时复用最近一次结果。
// 边界：缓存不代表远端实时状态，不保存安装状态。
type ModRemotePackageCache struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护缓存记录；边界：缓存身份以 Source 和 CacheKey 唯一索引为准。
	ID uint `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	// Source 职责：保存远端来源；使用：按 thunderstore/manual 区分缓存；边界：不代表来源当前可用。
	Source string `json:"source" gorm:"uniqueIndex:idx_mod_remote_cache;size:32;not null;comment:来源;column:source;"`
	// CacheKey 职责：保存来源内缓存键；使用：区分包列表、详情或后续扩展缓存；边界：由 helper 定义，不在模型层解释。
	CacheKey string `json:"cacheKey" gorm:"uniqueIndex:idx_mod_remote_cache;size:255;not null;comment:缓存键;column:cache_key;"`
	// Content 职责：保存缓存 JSON 内容；使用：远端读取失败或未过期时复用；边界：不保证代表实时远端状态。
	Content datatypes.JSON `json:"content" gorm:"comment:缓存内容;column:content;"`
	// UpdatedAt 职责：保存缓存更新时间；使用：诊断和刷新判断；边界：不等同于远端包更新时间。
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
	// ExpiresAt 职责：保存缓存过期时间；使用：helper 判断是否可复用缓存；边界：过期缓存仍可作为远端失败兜底。
	ExpiresAt time.Time `json:"expiresAt" gorm:"comment:过期时间;column:expires_at;"`
}

// TableName 指定来源缓存表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModRemotePackageCache) TableName() string {
	return "mod_remote_package_cache"
}
