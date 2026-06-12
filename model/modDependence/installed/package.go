package installed

// 文件职责：定义 V2 Thunderstore 安装主包持久化模型。
// 使用：model/modDependence 注册入口聚合给 core 自动迁移，service/helper 通过 ModInstalledPackage 读写主包状态。
// 边界：只描述主包表结构，不包含组件模型、文件清单模型、远端接口 DTO 或服务编排。

import (
	"time"

	"gorm.io/datatypes"
)

// ModInstalledPackage 记录 V2 Thunderstore 安装包。
// 职责：保存统一来源安装包的元数据、安装状态、进度和原始来源数据。
// 使用：V2 安装器写入安装记录，市场页读取安装状态，卸载流程按 PackageKey 定位主包。
// 边界：ID 只作为本地数据库主键，跨来源身份必须使用 PackageKey。
type ModInstalledPackage struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护记录身份；边界：业务跨来源身份必须使用 PackageKey。
	ID uint `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	// PackageKey 职责：保存跨来源安装包唯一标识；使用：安装、卸载、冲突恢复和前端状态映射定位主包；边界：不能为空且必须稳定。
	PackageKey string `json:"packageKey" gorm:"uniqueIndex;size:255;not null;comment:跨来源安装包唯一标识;column:package_key;"`
	// Source 职责：保存安装包来源；使用：查询和展示 official/thunderstore/manual 来源；边界：只存来源枚举文本，不验证来源可用性。
	Source string `json:"source" gorm:"index;size:32;not null;comment:安装包来源;column:source;"`
	// SourcePackageID 职责：保存来源侧包标识；使用：回溯远端数据和兼容旧官方 ID；边界：为空时仍以 PackageKey 为准。
	SourcePackageID string `json:"sourcePackageId" gorm:"size:255;comment:来源侧包标识;column:source_package_id;"`
	// Namespace 职责：保存来源命名空间；使用：Thunderstore 展示和回溯；边界：官方源可能为空。
	Namespace string `json:"namespace" gorm:"size:128;comment:来源命名空间;column:namespace;"`
	// Name 职责：保存来源侧机器友好包名；使用：调试、回溯和搜索；边界：前端展示优先使用 DisplayName。
	Name string `json:"name" gorm:"size:255;comment:来源包名称;column:name;"`
	// DisplayName 职责：保存用户可读包名；使用：前端卡片、进度和审计日志展示；边界：为空时调用方可回退 Name。
	DisplayName string `json:"displayName" gorm:"size:255;comment:显示名称;column:display_name;"`
	// Version 职责：保存已安装版本文本；使用：前端展示和卸载结果；边界：不在模型层解析语义化版本。
	Version string `json:"version" gorm:"size:128;comment:安装版本;column:version;"`
	// Description 职责：保存来源说明；使用：已安装列表或诊断展示；边界：不做 Markdown 或 HTML 清洗。
	Description string `json:"description" gorm:"type:text;comment:说明;column:description;"`
	// Author 职责：保存作者或团队；使用：前端展示和搜索扩展；边界：为空不影响安装记录有效性。
	Author string `json:"author" gorm:"size:255;comment:作者;column:author;"`
	// DownloadURL 职责：保存安装时使用的下载地址；使用：诊断和重新安装扩展；边界：本地导入包可能为空。
	DownloadURL string `json:"downloadUrl" gorm:"type:text;comment:下载地址;column:download_url;"`
	// PackageURL 职责：保存来源页面地址；使用：前端跳转或诊断；边界：不参与卸载和文件清理。
	PackageURL string `json:"packageUrl" gorm:"type:text;comment:来源页面地址;column:package_url;"`
	// CoverURL 职责：保存封面图地址；使用：前端已安装卡片展示；边界：图片可用性由前端兜底。
	CoverURL string `json:"coverUrl" gorm:"type:text;comment:封面地址;column:cover_url;"`
	// ArtifactKind 职责：保存安装产物类型；使用：诊断安装来源和恢复上下文；边界：不驱动卸载文件删除。
	ArtifactKind string `json:"artifactKind" gorm:"size:32;comment:产物类型;column:artifact_kind;"`
	// LocalPath 职责：保存本地产物路径；使用：手动导入诊断或后续重装扩展；边界：远端安装通常为空。
	LocalPath string `json:"localPath" gorm:"type:text;comment:本地产物路径;column:local_path;"`
	// DownloadCount 职责：保存来源下载次数快照；使用：前端展示和诊断；边界：不代表本地累计下载次数。
	DownloadCount int64 `json:"downloadCount" gorm:"comment:下载次数;column:download_count;"`
	// DependencyCount 职责：保存来源依赖数量快照；使用：前端摘要展示；边界：实际依赖关系以关联表为准。
	DependencyCount int64 `json:"dependencyCount" gorm:"comment:依赖数量;column:dependency_count;"`
	// PackageSize 职责：保存来源包大小快照；使用：前端展示或诊断；边界：不作为文件完整性校验依据。
	PackageSize int64 `json:"packageSize" gorm:"comment:包大小;column:package_size;"`
	// Tags 职责：保存来源标签 JSON；使用：前端展示和后续筛选扩展；边界：模型层不解析标签语义。
	Tags datatypes.JSON `json:"tags" gorm:"comment:标签;column:tags;"`
	// GameVersions 职责：保存来源适配游戏版本 JSON；使用：展示或兼容旧官方结构；边界：模型层不解析版本格式。
	GameVersions datatypes.JSON `json:"gameVersions" gorm:"comment:游戏版本;column:game_versions;"`
	// Trustworthiness 职责：保存来源可信度或评分快照；使用：展示和诊断；边界：不作为安全校验依据。
	Trustworthiness int64 `json:"trustworthiness" gorm:"comment:可信度;column:trustworthiness;"`
	// InstallManifest 职责：保存安装会话清单和恢复证据；使用：失败清理、冲突继续和启动恢复读取；边界：成功后也可保留用于诊断。
	InstallManifest datatypes.JSON `json:"installManifest" gorm:"comment:安装文件清单;column:install_manifest;"`
	// InstallState 职责：保存安装状态机当前状态；使用：进度恢复、冲突继续和已安装过滤；边界：状态迁移合法性由安装器控制。
	InstallState string `json:"installState" gorm:"index;size:32;comment:安装状态;column:install_state;"`
	// InstallStage 职责：保存当前安装阶段；使用：前端进度卡片展示和恢复；边界：只描述阶段名，不代表最终状态。
	InstallStage string `json:"installStage" gorm:"size:64;comment:安装阶段;column:install_stage;"`
	// InstallStatus 职责：保存当前安装状态文案；使用：前端进度卡片展示；边界：不用于程序分支判断。
	InstallStatus string `json:"installStatus" gorm:"type:text;comment:安装状态文案;column:install_status;"`
	// InstallProgress 职责：保存安装进度百分比；使用：前端恢复进度和状态展示；边界：由进度 helper 负责裁剪到合法范围。
	InstallProgress int `json:"installProgress" gorm:"comment:安装进度百分比;column:install_progress;"`
	// ErrorMessage 职责：保存安装失败或冲突错误文案；使用：前端失败卡片展示；边界：成功安装应为空。
	ErrorMessage string `json:"errorMessage" gorm:"type:text;comment:安装失败错误信息;column:error_message;"`
	// CurrentSpeed 职责：保存最近下载速度字节每秒；使用：前端下载进度展示；边界：非下载阶段可为 0。
	CurrentSpeed int64 `json:"currentSpeed" gorm:"comment:最近下载速度，单位字节每秒;column:current_speed;"`
	// Raw 职责：保存来源原始 DTO JSON；使用：冲突继续、恢复安装和诊断回放；边界：不直接暴露给用户编辑。
	Raw datatypes.JSON `json:"raw" gorm:"comment:来源原始信息;column:raw;"`
	// InstalledAt 职责：保存安装完成或记录创建时间；使用：前端排序和审计；边界：失败/冲突记录可能表示会话开始时间。
	InstalledAt time.Time `json:"installedAt" gorm:"comment:安装时间;column:installed_at;"`
	// UpdatedAt 职责：保存记录更新时间；使用：GORM 更新和前端排序扩展；边界：不代表远端包更新时间。
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定 V2 统一安装包表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModInstalledPackage) TableName() string {
	return "mod_installed_packages"
}
