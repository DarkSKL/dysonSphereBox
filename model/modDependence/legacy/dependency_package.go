package legacy

// 文件职责：定义旧官方源 Mod 整合包及依赖关系持久化模型。
// 使用：由 model/modDependence 注册入口聚合给 core 自动迁移，并由传统整合包安装/卸载 helper 直接通过 legacy 子包 GORM 读写。
// 边界：只描述 legacy 关注点的整合包表结构，不包含 Thunderstore 统一包、远端 DTO 或安装流程。

import (
	"time"

	"gorm.io/datatypes"
)

// ModDependencyPackage 保存官方源整合包及其依赖关系。
// 职责：描述旧官方源整合包及其依赖多对多关系的本地表结构。
// 使用：传统整合包安装、卸载和旧市场状态同步流程通过 GORM 读写该模型。
// 边界：仅描述持久化表结构，远端响应 DTO 位于 apiDefinition/modDependence。
type ModDependencyPackage struct {
	// ID 职责：保存历史整合包本地主键；使用：GORM 维护旧整合包表；边界：统一来源新流程不以该字段作为身份。
	ID uint `json:"id" form:"id" gorm:"primaryKey;comment:主键ID;column:id;"`
	// PackageName 职责：保存旧官方整合包名称；使用：旧市场展示和传统安装结果；边界：指针为空时调用方需兜底。
	PackageName *string `json:"packageName" form:"packageName" gorm:"comment:整合包名称;column:package_name;" binding:"required"`
	// PackageVersion 职责：保存旧官方整合包版本；使用：传统安装记录和展示；边界：不在模型层解析语义化版本。
	PackageVersion *string `json:"packageVersion" form:"packageVersion" gorm:"comment:整合包版本号;column:package_version;" binding:"required"`
	// PackageDescription 职责：保存旧官方整合包说明；使用：旧市场详情和诊断；边界：不做文本清洗。
	PackageDescription *string `json:"packageDescription" form:"packageDescription" gorm:"comment:整合包描述;column:package_description;type:text;"`
	// PackageSize 职责：保存旧官方整合包大小；使用：旧市场展示；边界：不作为下载完整性校验依据。
	PackageSize *int64 `json:"packageSize" form:"packageSize" gorm:"comment:整合包大小,单位byte;column:package_size;default:0;"`
	// DownloadCount 职责：保存旧官方下载次数；使用：旧市场热度展示；边界：不代表本地下载次数。
	DownloadCount *int64 `json:"downloadCount" form:"downloadCount" gorm:"comment:整合包下载数量;column:download_count;default:0;"`
	// DependencyCount 职责：保存旧官方依赖数量；使用：旧市场摘要；边界：实际关联以 Dependencies 关系为准。
	DependencyCount *int64 `json:"dependencyCount" form:"dependencyCount" gorm:"comment:整合包内依赖数量;column:dependency_count;default:0;"`
	// GameVersions 职责：保存旧官方适配游戏版本 JSON；使用：旧市场展示；边界：模型层不解析具体结构。
	GameVersions datatypes.JSON `json:"gameVersions" form:"gameVersions" gorm:"comment:整合包适配的游戏版本数组;column:game_versions;" swaggertype:"array,object"`
	// Tags 职责：保存旧官方标签 JSON；使用：旧市场展示；边界：模型层不解析标签语义。
	Tags datatypes.JSON `json:"tags" form:"tags" gorm:"comment:整合包标签数组;column:tags;" swaggertype:"array,string"`
	// Source 职责：保存旧官方来源文本；使用：旧市场展示和兼容查询；边界：统一来源来源值由新 DTO 维护。
	Source *string `json:"source" form:"source" gorm:"comment:整合包来源;column:source;"`
	// Url 职责：保存旧官方整合包下载地址；使用：传统安装器下载安装包；边界：下载前仍需由安装器校验可用性。
	Url *string `json:"url" form:"url" gorm:"comment:整合包下载链接;column:url;"`
	// CoverUrl 职责：保存旧官方封面地址；使用：旧市场卡片展示；边界：图片可用性由前端处理。
	CoverUrl *string `json:"coverUrl" form:"coverUrl" gorm:"comment:整合包封面图地址;column:cover_url;"`
	// Author 职责：保存旧官方作者文本；使用：旧市场展示和搜索；边界：为空不影响安装记录。
	Author *string `json:"author" form:"author" gorm:"comment:整合包作者;column:author;"`
	// Status 职责：保存旧官方包状态码；使用：兼容草稿/发布/下架状态；边界：模型层不解释状态迁移。
	Status *int `json:"status" form:"status" gorm:"comment:整合包状态:1草稿,2发布,3下架;column:status;default:1;"`
	// Trustworthiness 职责：保存旧官方可信度评分；使用：旧市场展示；边界：不作为本地安全校验依据。
	Trustworthiness *int64 `json:"trustworthiness" form:"trustworthiness" gorm:"comment:整合包可信度评分(1-5星);column:trustworthiness;"`
	// UpdateTime 职责：保存旧官方更新时间；使用：旧市场排序或诊断；边界：不代表本地安装时间。
	UpdateTime *time.Time `json:"updateTime" form:"updateTime" gorm:"comment:整合包最后更新时间;column:update_time;"`
	// Config 职责：保存旧官方整合包配置文本；使用：传统安装兼容和诊断；边界：模型层不解析配置内容。
	Config *string `json:"config" form:"config" gorm:"comment:整合包全局配置文本;column:config;type:text;"`
	// Ex 职责：保存旧官方整合包扩展字段；使用：兼容历史接口和诊断；边界：不参与安装决策。
	Ex *string `json:"ex" form:"ex" gorm:"comment:整合包扩展信息;column:ex;type:text;"`
	// Dependencies 职责：保存旧官方整合包与依赖的多对多关系；使用：传统安装器安装依赖和卸载共享判断；边界：不保存统一来源 V2 组件关系。
	Dependencies []ModDependency `json:"dependencies" form:"dependencies" gorm:"many2many:mod_dependency_package_relations;" binding:"-"`
}

// TableName 返回 ModDependencyPackage 对应的历史整合包表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModDependencyPackage) TableName() string {
	return "mod_dependency_packages"
}
