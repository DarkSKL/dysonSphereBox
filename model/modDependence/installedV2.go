package modDependence

import (
	"time"

	"gorm.io/datatypes"
)

// ModInstalledPackage 记录 V2 统一来源安装包。
// 用途：以 PackageKey 承接国内源、Thunderstore 与手动导入的统一安装状态。
// 边界：ID 只作为本地数据库主键，跨来源身份必须使用 PackageKey。
type ModInstalledPackage struct {
	ID              uint           `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	PackageKey      string         `json:"packageKey" gorm:"uniqueIndex;size:255;not null;comment:跨来源安装包唯一标识;column:package_key;"`
	Source          string         `json:"source" gorm:"index;size:32;not null;comment:安装包来源;column:source;"`
	SourcePackageID string         `json:"sourcePackageId" gorm:"size:255;comment:来源侧包标识;column:source_package_id;"`
	Namespace       string         `json:"namespace" gorm:"size:128;comment:来源命名空间;column:namespace;"`
	Name            string         `json:"name" gorm:"size:255;comment:来源包名称;column:name;"`
	DisplayName     string         `json:"displayName" gorm:"size:255;comment:显示名称;column:display_name;"`
	Version         string         `json:"version" gorm:"size:128;comment:安装版本;column:version;"`
	Description     string         `json:"description" gorm:"type:text;comment:说明;column:description;"`
	Author          string         `json:"author" gorm:"size:255;comment:作者;column:author;"`
	DownloadURL     string         `json:"downloadUrl" gorm:"type:text;comment:下载地址;column:download_url;"`
	PackageURL      string         `json:"packageUrl" gorm:"type:text;comment:来源页面地址;column:package_url;"`
	CoverURL        string         `json:"coverUrl" gorm:"type:text;comment:封面地址;column:cover_url;"`
	ArtifactKind    string         `json:"artifactKind" gorm:"size:32;comment:产物类型;column:artifact_kind;"`
	LocalPath       string         `json:"localPath" gorm:"type:text;comment:本地产物路径;column:local_path;"`
	DownloadCount   int64          `json:"downloadCount" gorm:"comment:下载次数;column:download_count;"`
	DependencyCount int64          `json:"dependencyCount" gorm:"comment:依赖数量;column:dependency_count;"`
	PackageSize     int64          `json:"packageSize" gorm:"comment:包大小;column:package_size;"`
	Tags            datatypes.JSON `json:"tags" gorm:"comment:标签;column:tags;"`
	GameVersions    datatypes.JSON `json:"gameVersions" gorm:"comment:游戏版本;column:game_versions;"`
	Trustworthiness int64          `json:"trustworthiness" gorm:"comment:可信度;column:trustworthiness;"`
	InstallManifest datatypes.JSON `json:"installManifest" gorm:"comment:安装文件清单;column:install_manifest;"`
	InstallState    string         `json:"installState" gorm:"index;size:32;comment:安装状态;column:install_state;"`
	InstallStage    string         `json:"installStage" gorm:"size:64;comment:安装阶段;column:install_stage;"`
	InstallStatus   string         `json:"installStatus" gorm:"type:text;comment:安装状态文案;column:install_status;"`
	InstallProgress int            `json:"installProgress" gorm:"comment:安装进度百分比;column:install_progress;"`
	ErrorMessage    string         `json:"errorMessage" gorm:"type:text;comment:安装失败错误信息;column:error_message;"`
	CurrentSpeed    int64          `json:"currentSpeed" gorm:"comment:最近下载速度，单位字节每秒;column:current_speed;"`
	Raw             datatypes.JSON `json:"raw" gorm:"comment:来源原始信息;column:raw;"`
	InstalledAt     time.Time      `json:"installedAt" gorm:"comment:安装时间;column:installed_at;"`
	UpdatedAt       time.Time      `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定 V2 统一安装包表名。
func (ModInstalledPackage) TableName() string {
	return "mod_installed_packages"
}

// ModInstalledComponent 记录 V2 安装包内的组件或依赖。
// 用途：同一个依赖组件可被多个安装包引用，卸载时据此判断是否共享。
type ModInstalledComponent struct {
	ID                 uint           `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	ComponentKey       string         `json:"componentKey" gorm:"uniqueIndex;size:255;not null;comment:跨来源组件唯一标识;column:component_key;"`
	Source             string         `json:"source" gorm:"index;size:32;not null;comment:组件来源;column:source;"`
	SourceDependencyID string         `json:"sourceDependencyId" gorm:"size:255;comment:来源侧依赖标识;column:source_dependency_id;"`
	Namespace          string         `json:"namespace" gorm:"size:128;comment:来源命名空间;column:namespace;"`
	Name               string         `json:"name" gorm:"size:255;comment:组件名称;column:name;"`
	DisplayName        string         `json:"displayName" gorm:"size:255;comment:显示名称;column:display_name;"`
	Version            string         `json:"version" gorm:"size:128;comment:组件版本;column:version;"`
	Description        string         `json:"description" gorm:"type:text;comment:说明;column:description;"`
	DownloadURL        string         `json:"downloadUrl" gorm:"type:text;comment:下载地址;column:download_url;"`
	PackageURL         string         `json:"packageUrl" gorm:"type:text;comment:来源页面地址;column:package_url;"`
	ArtifactKind       string         `json:"artifactKind" gorm:"size:32;comment:产物类型;column:artifact_kind;"`
	LocalPath          string         `json:"localPath" gorm:"type:text;comment:本地产物路径;column:local_path;"`
	Raw                datatypes.JSON `json:"raw" gorm:"comment:来源原始信息;column:raw;"`
	UpdatedAt          time.Time      `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定 V2 安装组件表名。
func (ModInstalledComponent) TableName() string {
	return "mod_installed_components"
}

// ModInstalledPackageComponent 记录安装包与组件的轻量关联。
// 用途：用稳定的 packageKey/componentKey 关联来源模型，避免 SQLite 迁移时生成脆弱外键。
type ModInstalledPackageComponent struct {
	ID           uint      `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	PackageKey   string    `json:"packageKey" gorm:"uniqueIndex:idx_mod_installed_package_component;index;size:255;not null;comment:安装包唯一标识;column:package_key;"`
	ComponentKey string    `json:"componentKey" gorm:"uniqueIndex:idx_mod_installed_package_component;index;size:255;not null;comment:组件唯一标识;column:component_key;"`
	CreatedAt    time.Time `json:"createdAt" gorm:"comment:关联创建时间;column:created_at;"`
}

// TableName 指定 V2 安装包组件关联表名。
func (ModInstalledPackageComponent) TableName() string {
	return "mod_installed_package_components"
}

// ModInstalledFile 记录 V2 安装器写入的单个文件。
// 用途：卸载时按文件清单精准移除，支持 plugins、patchers、config 等不同落位。
type ModInstalledFile struct {
	ID           uint      `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	PackageKey   string    `json:"packageKey" gorm:"index;size:255;not null;comment:所属安装包;column:package_key;"`
	ComponentKey string    `json:"componentKey" gorm:"index;size:255;comment:所属组件;column:component_key;"`
	SourcePath   string    `json:"sourcePath" gorm:"type:text;comment:缓存或解压源文件;column:source_path;"`
	TargetPath   string    `json:"targetPath" gorm:"index;type:text;not null;comment:目标绝对路径;column:target_path;"`
	RelativePath string    `json:"relativePath" gorm:"type:text;not null;comment:相对 BepInEx 或游戏目录路径;column:relative_path;"`
	Sha256       string    `json:"sha256" gorm:"size:64;comment:目标文件校验值;column:sha256;"`
	FileSize     int64     `json:"fileSize" gorm:"comment:文件大小;column:file_size;"`
	CreatedAt    time.Time `json:"createdAt" gorm:"comment:记录时间;column:created_at;"`
}

// TableName 指定 V2 安装文件表名。
func (ModInstalledFile) TableName() string {
	return "mod_installed_files"
}

// ModRemotePackageCache 记录来源侧包列表缓存。
// 用途：Thunderstore 这类全量接口可按来源缓存，断网时仍可显示最近一次结果。
type ModRemotePackageCache struct {
	ID        uint           `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	Source    string         `json:"source" gorm:"uniqueIndex:idx_mod_remote_cache;size:32;not null;comment:来源;column:source;"`
	CacheKey  string         `json:"cacheKey" gorm:"uniqueIndex:idx_mod_remote_cache;size:255;not null;comment:缓存键;column:cache_key;"`
	Content   datatypes.JSON `json:"content" gorm:"comment:缓存内容;column:content;"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
	ExpiresAt time.Time      `json:"expiresAt" gorm:"comment:过期时间;column:expires_at;"`
}

// TableName 指定来源缓存表名。
func (ModRemotePackageCache) TableName() string {
	return "mod_remote_package_cache"
}
