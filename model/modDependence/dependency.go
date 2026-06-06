package modDependence

import (
	"time"

	"gorm.io/datatypes"
)

// Mod依赖信息 结构体  ModDependency
type ModDependency struct {
	ID              uint           `json:"id" form:"id" gorm:"primaryKey;comment:主键ID;column:id;"`
	ModName         *string        `json:"modName" form:"modName" gorm:"comment:Mod的名称;column:mod_name;" binding:"required"`                                                 //Mod名称
	ModVersion      *string        `json:"modVersion" form:"modVersion" gorm:"comment:Mod的版本号;column:mod_version;" binding:"required"`                                       //Mod版本
	ModDescription  *string        `json:"modDescription" form:"modDescription" gorm:"comment:Mod的详细描述;column:mod_description;type:text;"`                                   //Mod描述
	GameVersions    datatypes.JSON `json:"gameVersions" form:"gameVersions" gorm:"comment:Mod适配的游戏版本数组;column:game_versions;" swaggertype:"array,object" binding:"required"` //适配游戏版本
	UpdateTime      *time.Time     `json:"updateTime" form:"updateTime" gorm:"comment:Mod的最后更新时间;column:update_time;"`                                                       //更新时间
	Source          *string        `json:"source" form:"source" gorm:"comment:Mod的提供来源;column:source;" binding:"required"`                                                   //来源
	Url             *string        `json:"url" form:"url" gorm:"comment:Mod的下载链接;column:url;" binding:"required"`                                                            //下载地址
	Trustworthiness *int64         `json:"trustworthiness" form:"trustworthiness" gorm:"comment:Mod的可信度评分(1-5星);column:trustworthiness;" binding:"required"`                 //可信度
	Config          *string        `json:"config" form:"config" gorm:"comment:Mod的特殊配置文本;column:config;type:text;"`                                                          //配置
	Ex              *string        `json:"ex" form:"ex" gorm:"comment:Mod的扩展信息;column:ex;"`                                                                                  //扩展字段
}

// TableName Mod依赖信息 ModDependency自定义表名 mod_dependencies
func (ModDependency) TableName() string {
	return "mod_dependencies"
}
