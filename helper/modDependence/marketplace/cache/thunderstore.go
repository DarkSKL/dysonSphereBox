package cache

// 文件职责：封装 Thunderstore 市场包索引的本地缓存读写。
// 使用：marketplace Thunderstore provider 在读取远端索引前后调用，用于减少全量索引请求和提供失败兜底。
// 边界：只读写市场缓存表，不查询远端市场、不转换 DTO、不修改安装记录或本地文件。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Thunderstore 缓存常量组定义 DSP 包索引缓存策略。
// 职责：集中保存缓存键和缓存有效期。
// 使用：ReadThunderstorePackages 与 WriteThunderstorePackages 查询和写入缓存表时引用。
// 边界：只影响市场包索引缓存，不影响安装记录、下载文件或前端临时进度。
const (
	// thunderstorePackageCacheKeyDSP 表示 DSP 包索引缓存键。
	// 职责：区分同一来源下不同游戏或索引的缓存内容。
	// 使用：ReadThunderstorePackages 与 WriteThunderstorePackages 查询缓存表时引用。
	// 边界：只用于 Thunderstore 戴森球计划索引，不作为安装包唯一标识。
	thunderstorePackageCacheKeyDSP = "dyson-sphere-program"
	// thunderstorePackageCacheTTL 表示 Thunderstore 包索引缓存有效期。
	// 职责：避免频繁请求 Thunderstore 全量索引。
	// 使用：WriteThunderstorePackages 计算 ExpiresAt 时引用。
	// 边界：过期后会尝试重新请求；请求失败时仍可由调用方回退旧缓存策略处理。
	thunderstorePackageCacheTTL = 6 * time.Hour
)

// requireDatabase 返回 marketplace 子包可用的数据库连接。
// 职责：在市场缓存读写前校验 core 全局数据库已初始化。
// 使用：Thunderstore 缓存读取和写入函数调用。
// 边界：只返回已有连接，不创建数据库，不执行迁移；未初始化时返回错误并让缓存逻辑降级。
func requireDatabase() (*gorm.DB, error) {
	if core.GLOBAL.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}
	return core.GLOBAL.DB, nil
}

// ReadThunderstorePackages 读取未过期的 Thunderstore 包索引缓存。
// 职责：从本地 SQLite 缓存表恢复 Thunderstore 原始包列表。
// 使用：thunderstoreindex.LoadPackages 在允许缓存或远端请求失败时调用。
// 边界：数据库不可用、记录不存在、缓存过期或 JSON 解析失败时返回 false。
func ReadThunderstorePackages() ([]thunderstoreAPI.Package, bool) {
	db, err := requireDatabase()
	if err != nil {
		return nil, false
	}

	var cache struct {
		// Content 职责：保存缓存表中的 Thunderstore 原始索引 JSON；使用：读取后反序列化为 Package 列表；边界：解析失败时缓存视为不可用。
		Content []byte
		// ExpiresAt 职责：保存缓存过期时间；使用：读取缓存时判断是否仍可直接复用；边界：过期后返回 false，由调用方决定是否请求远端。
		ExpiresAt time.Time
	}
	result := db.
		Table("mod_remote_package_cache").
		Select("content, expires_at").
		Where("source = ? AND cache_key = ?", string(sourceConstant.ModSourceThunderstore), thunderstorePackageCacheKeyDSP).
		Take(&cache)
	if result.Error != nil || time.Now().After(cache.ExpiresAt) {
		return nil, false
	}

	var packages []thunderstoreAPI.Package
	if err := json.Unmarshal(cache.Content, &packages); err != nil {
		return nil, false
	}
	return packages, true
}

// WriteThunderstorePackages 写入 Thunderstore 包索引缓存。
// 职责：把最新全量包索引序列化并保存过期时间。
// 使用：thunderstoreindex.LoadPackages 成功请求远端索引后调用。
// 边界：数据库不可用或序列化失败时静默跳过，不影响本次市场结果返回。
func WriteThunderstorePackages(packages []thunderstoreAPI.Package) {
	db, err := requireDatabase()
	if err != nil {
		return
	}

	content, err := json.Marshal(packages)
	if err != nil {
		return
	}

	now := time.Now()
	record := modDependenceModel.ModRemotePackageCache{
		Source:    string(sourceConstant.ModSourceThunderstore),
		CacheKey:  thunderstorePackageCacheKeyDSP,
		Content:   content,
		UpdatedAt: now,
		ExpiresAt: now.Add(thunderstorePackageCacheTTL),
	}
	_ = db.
		Where("source = ? AND cache_key = ?", record.Source, record.CacheKey).
		Assign(record).
		FirstOrCreate(&record).
		Error
}
