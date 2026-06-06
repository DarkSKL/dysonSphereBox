package modDependence

import (
	"time"

	"gorm.io/datatypes"
)

// Mod依赖整合包 结构体 ModDependencyPackage
type ModDependencyPackage struct {
	ID                 uint            `json:"id" form:"id" gorm:"primaryKey;comment:主键ID;column:id;"`
	PackageName        *string         `json:"packageName" form:"packageName" gorm:"comment:整合包名称;column:package_name;" binding:"required"`                   //整合包名称
	PackageVersion     *string         `json:"packageVersion" form:"packageVersion" gorm:"comment:整合包版本号;column:package_version;" binding:"required"`         //整合包版本
	PackageDescription *string         `json:"packageDescription" form:"packageDescription" gorm:"comment:整合包描述;column:package_description;type:text;"`       //整合包描述
	PackageSize        *int64          `json:"packageSize" form:"packageSize" gorm:"comment:整合包大小,单位byte;column:package_size;default:0;"`                     //整合包大小
	DownloadCount      *int64          `json:"downloadCount" form:"downloadCount" gorm:"comment:整合包下载数量;column:download_count;default:0;"`                    //下载数量
	DependencyCount    *int64          `json:"dependencyCount" form:"dependencyCount" gorm:"comment:整合包内依赖数量;column:dependency_count;default:0;"`             //依赖数量
	GameVersions       datatypes.JSON  `json:"gameVersions" form:"gameVersions" gorm:"comment:整合包适配的游戏版本数组;column:game_versions;" swaggertype:"array,object"` //适配游戏版本
	Tags               datatypes.JSON  `json:"tags" form:"tags" gorm:"comment:整合包标签数组;column:tags;" swaggertype:"array,string"`                               //标签
	Source             *string         `json:"source" form:"source" gorm:"comment:整合包来源;column:source;"`                                                      //来源
	Url                *string         `json:"url" form:"url" gorm:"comment:整合包下载链接;column:url;"`                                                             //下载地址
	CoverUrl           *string         `json:"coverUrl" form:"coverUrl" gorm:"comment:整合包封面图地址;column:cover_url;"`                                            //封面图
	Author             *string         `json:"author" form:"author" gorm:"comment:整合包作者;column:author;"`                                                      //作者
	Status             *int            `json:"status" form:"status" gorm:"comment:整合包状态:1草稿,2发布,3下架;column:status;default:1;"`                                //状态
	Trustworthiness    *int64          `json:"trustworthiness" form:"trustworthiness" gorm:"comment:整合包可信度评分(1-5星);column:trustworthiness;"`                  //可信度
	UpdateTime         *time.Time      `json:"updateTime" form:"updateTime" gorm:"comment:整合包最后更新时间;column:update_time;"`                                     //更新时间
	Config             *string         `json:"config" form:"config" gorm:"comment:整合包全局配置文本;column:config;type:text;"`                                        //配置
	Ex                 *string         `json:"ex" form:"ex" gorm:"comment:整合包扩展信息;column:ex;type:text;"`                                                      //扩展字段
	Dependencies       []ModDependency `json:"dependencies" form:"dependencies" gorm:"many2many:mod_dependency_package_relations;" binding:"-"`               //包含的Mod依赖
}

// TableName Mod依赖整合包 ModDependencyPackage自定义表名 mod_dependency_packages
func (ModDependencyPackage) TableName() string {
	return "mod_dependency_packages"
}
