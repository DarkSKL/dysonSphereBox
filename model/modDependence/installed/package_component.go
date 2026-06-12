package installed

// 文件职责：定义安装包与组件之间的轻量关联模型。
// 使用：安装器记录主包依赖关系，卸载时通过 ModInstalledPackageComponent 判断组件是否仍被其它主包引用。
// 边界：只保存 key 关联，不依赖 GORM 外键、不保存组件元数据、不保存文件清单。

import "time"

// ModInstalledPackageComponent 记录安装包与组件的轻量关联。
// 职责：保存安装包与组件之间的 key 关联。
// 使用：安装器记录主包依赖关系，卸载时判断组件是否仍被其它主包引用。
// 边界：不依赖 GORM 外键，不保存组件元数据或文件清单。
type ModInstalledPackageComponent struct {
	// ID 职责：保存本地数据库自增主键；使用：GORM 维护关联记录；边界：业务关联以 PackageKey 和 ComponentKey 唯一索引为准。
	ID uint `json:"id" gorm:"primaryKey;comment:本地主键ID;column:id;"`
	// PackageKey 职责：保存安装包唯一标识；使用：卸载时查找主包关联组件；边界：必须对应 ModInstalledPackage.PackageKey。
	PackageKey string `json:"packageKey" gorm:"uniqueIndex:idx_mod_installed_package_component;index;size:255;not null;comment:安装包唯一标识;column:package_key;"`
	// ComponentKey 职责：保存组件唯一标识；使用：共享组件判断和清理关联；边界：必须对应 ModInstalledComponent.ComponentKey。
	ComponentKey string `json:"componentKey" gorm:"uniqueIndex:idx_mod_installed_package_component;index;size:255;not null;comment:组件唯一标识;column:component_key;"`
	// CreatedAt 职责：保存关联创建时间；使用：诊断安装顺序；边界：不参与共享判断。
	CreatedAt time.Time `json:"createdAt" gorm:"comment:关联创建时间;column:created_at;"`
}

// TableName 指定 V2 安装包组件关联表名。
// 职责：覆盖 GORM 默认表名。
// 使用：GORM AutoMigrate 和 CRUD 时自动调用。
// 边界：只返回静态表名，不执行数据库操作。
func (ModInstalledPackageComponent) TableName() string {
	return "mod_installed_package_components"
}
