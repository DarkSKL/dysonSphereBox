package legacy

// 文件职责：定义旧官方源单个 Mod 依赖持久化模型。
// 使用：由 model/modDependence 注册入口聚合给 core 自动迁移，并由传统整合包 helper 直接通过 legacy 子包 GORM 读写。
// 边界：只描述 legacy 关注点的依赖表结构，不包含 Thunderstore 统一包、远端 DTO 或安装流程。

import (
	"time"

	"gorm.io/datatypes"
)

// ModDependency 保存官方源返回的单个 Mod 依赖持久化结构。
// 职责：描述旧官方源单个 Mod 依赖在本地 SQLite 中的表结构。
// 使用：传统整合包安装和旧市场缓存兼容流程通过 GORM 读写该模型。
// 边界：这是历史官方整合包模型，不承载 Thunderstore 统一包的安装状态。
type ModDependency struct {
	// ID 职责：保存历史依赖记录本地主键；使用：GORM 维护旧依赖表；边界：旧流程可用 ID，统一来源新流程不用它做身份。
	ID uint `json:"id" form:"id" gorm:"primaryKey;comment:主键ID;column:id;"`
	// ModName 职责：保存旧官方依赖名称；使用：传统整合包展示和安装目录命名；边界：指针为空时调用方需兜底。
	ModName *string `json:"modName" form:"modName" gorm:"comment:Mod的名称;column:mod_name;" binding:"required"`
	// ModVersion 职责：保存旧官方依赖版本；使用：传统安装结果和展示；边界：不在模型层解析语义化版本。
	ModVersion *string `json:"modVersion" form:"modVersion" gorm:"comment:Mod的版本号;column:mod_version;" binding:"required"`
	// ModDescription 职责：保存旧官方依赖说明；使用：旧市场详情和诊断；边界：不做文本清洗。
	ModDescription *string `json:"modDescription" form:"modDescription" gorm:"comment:Mod的详细描述;column:mod_description;type:text;"`
	// GameVersions 职责：保存旧官方依赖适配游戏版本 JSON；使用：旧市场展示；边界：模型层不解析具体版本结构。
	GameVersions datatypes.JSON `json:"gameVersions" form:"gameVersions" gorm:"comment:Mod适配的游戏版本数组;column:game_versions;" swaggertype:"array,object" binding:"required"`
	// UpdateTime 职责：保存旧官方依赖更新时间；使用：旧市场排序或诊断；边界：不代表本地安装时间。
	UpdateTime *time.Time `json:"updateTime" form:"updateTime" gorm:"comment:Mod的最后更新时间;column:update_time;"`
	// Source 职责：保存旧官方依赖来源文本；使用：旧市场展示和兼容查询；边界：统一来源来源值由新 DTO 维护。
	Source *string `json:"source" form:"source" gorm:"comment:Mod的提供来源;column:source;" binding:"required"`
	// Url 职责：保存旧官方依赖下载地址；使用：传统安装器下载依赖；边界：下载前仍需由安装器校验可用性。
	Url *string `json:"url" form:"url" gorm:"comment:Mod的下载链接;column:url;" binding:"required"`
	// Trustworthiness 职责：保存旧官方可信度评分；使用：旧市场展示；边界：不作为本地安全校验依据。
	Trustworthiness *int64 `json:"trustworthiness" form:"trustworthiness" gorm:"comment:Mod的可信度评分(1-5星);column:trustworthiness;" binding:"required"`
	// Config 职责：保存旧官方依赖配置文本；使用：传统安装兼容和诊断；边界：模型层不解析配置内容。
	Config *string `json:"config" form:"config" gorm:"comment:Mod的特殊配置文本;column:config;type:text;"`
	// Ex 职责：保存旧官方依赖扩展字段；使用：兼容历史接口和诊断；边界：不参与安装决策。
	Ex *string `json:"ex" form:"ex" gorm:"comment:Mod的扩展信息;column:ex;"`
}

// TableName 返回 ModDependency 对应的历史依赖表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModDependency) TableName() string {
	return "mod_dependencies"
}
