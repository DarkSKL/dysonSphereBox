package installed

// 文件职责：定义 V2 安装组件或依赖的持久化模型。
// 使用：安装器写入组件记录，卸载流程通过 ModInstalledComponent 判断组件共享和展示明细。
// 边界：只保存组件元数据，不保存主包状态、不保存具体文件清单、不执行业务判断。

import (
	"time"

	"gorm.io/datatypes"
)

// ModInstalledComponent 记录 V2 安装包内的组件或依赖。
// 职责：保存统一来源安装组件或依赖的元数据。
// 使用：安装器写入依赖组件记录，卸载时据此判断同一组件是否被多个包共享。
// 边界：不保存具体文件清单，文件归属由 ModInstalledFile 记录。
type ModInstalledComponent struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护组件记录；边界：业务身份必须使用 ComponentKey。
	ID uint `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	// ComponentKey 职责：保存跨来源组件唯一标识；使用：文件归属、共享判断和依赖关联；边界：不能为空且必须稳定。
	ComponentKey string `json:"componentKey" gorm:"uniqueIndex;size:255;not null;comment:跨来源组件唯一标识;column:component_key;"`
	// Source 职责：保存组件来源；使用：查询和展示 thunderstore/manual 来源；边界：只存来源文本。
	Source string `json:"source" gorm:"index;size:32;not null;comment:组件来源;column:source;"`
	// SourceDependencyID 职责：保存来源侧依赖标识；使用：回溯远端依赖数据；边界：为空时仍以 ComponentKey 为准。
	SourceDependencyID string `json:"sourceDependencyId" gorm:"size:255;comment:来源侧依赖标识;column:source_dependency_id;"`
	// Namespace 职责：保存来源命名空间；使用：Thunderstore 展示和依赖诊断；边界：manual 来源可能为空。
	Namespace string `json:"namespace" gorm:"size:128;comment:来源命名空间;column:namespace;"`
	// Name 职责：保存来源侧机器友好组件名；使用：调试和搜索；边界：前端展示优先使用 DisplayName。
	Name string `json:"name" gorm:"size:255;comment:组件名称;column:name;"`
	// DisplayName 职责：保存用户可读组件名；使用：安装结果、冲突提示和卸载明细展示；边界：为空时调用方可回退 Name。
	DisplayName string `json:"displayName" gorm:"size:255;comment:显示名称;column:display_name;"`
	// Version 职责：保存组件版本文本；使用：冲突提示和卸载明细展示；边界：不在模型层解析语义化版本。
	Version string `json:"version" gorm:"size:128;comment:组件版本;column:version;"`
	// Description 职责：保存组件说明；使用：诊断和展示扩展；边界：不做 Markdown 或 HTML 清洗。
	Description string `json:"description" gorm:"type:text;comment:说明;column:description;"`
	// DownloadURL 职责：保存组件下载地址；使用：诊断和重装扩展；边界：本地导入组件可能为空。
	DownloadURL string `json:"downloadUrl" gorm:"type:text;comment:下载地址;column:download_url;"`
	// PackageURL 职责：保存来源页面地址；使用：前端跳转或诊断；边界：不参与卸载。
	PackageURL string `json:"packageUrl" gorm:"type:text;comment:来源页面地址;column:package_url;"`
	// ArtifactKind 职责：保存组件产物类型；使用：诊断安装来源；边界：不驱动卸载文件删除。
	ArtifactKind string `json:"artifactKind" gorm:"size:32;comment:产物类型;column:artifact_kind;"`
	// LocalPath 职责：保存本地产物路径；使用：手动导入诊断；边界：远端组件通常为空。
	LocalPath string `json:"localPath" gorm:"type:text;comment:本地产物路径;column:local_path;"`
	// Raw 职责：保存来源原始依赖 DTO JSON；使用：诊断和恢复上下文；边界：不作为文件归属判断依据。
	Raw datatypes.JSON `json:"raw" gorm:"comment:来源原始信息;column:raw;"`
	// UpdatedAt 职责：保存组件记录更新时间；使用：GORM 更新和诊断排序；边界：不代表远端组件更新时间。
	UpdatedAt time.Time `json:"updatedAt" gorm:"comment:更新时间;column:updated_at;"`
}

// TableName 指定 V2 安装组件表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModInstalledComponent) TableName() string {
	return "mod_installed_components"
}
