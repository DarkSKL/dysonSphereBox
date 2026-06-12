package cleanup

// 文件职责：清理 BepInEx 卸载时需要同步删除的旧版 Mod 依赖安装记录。
// 使用：bepinex/uninstall 在删除本地注入文件后调用。
// 边界：只处理旧版 legacy 依赖表，不删除新版 installed 表，也不触碰磁盘文件。

import (
	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
	"fmt"
	"gorm.io/gorm"
)

// ClearInstalledModDependencyRecords 清理旧版本地 Mod 依赖安装记录。
// 职责：在卸载 BepInEx 时同步删除依赖包、依赖项和关联表数据。
// 使用：uninstall.UninstallBepInEx 删除本地注入文件后调用，避免数据库仍显示旧 Mod 已安装。
// 边界：只覆盖旧版 mod_dependencies 与 mod_dependency_packages 表，不处理新版 installed 表。
// 逻辑流：1. 检查数据库可用性 => 2. 统计清理前记录数量 => 3. 事务删除关联和主表 => 4. 返回是否实际清理过记录。
func ClearInstalledModDependencyRecords() (bool, error) {
	if core.GLOBAL.DB == nil {
		return false, fmt.Errorf("database is not initialized")
	}

	var packageCount int64
	if err := core.GLOBAL.DB.Table("mod_dependency_packages").Count(&packageCount).Error; err != nil {
		return false, err
	}
	var dependencyCount int64
	if err := core.GLOBAL.DB.Table("mod_dependencies").Count(&dependencyCount).Error; err != nil {
		return false, err
	}

	err := core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM mod_dependency_package_relations").Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&modDependenceModel.ModDependencyPackage{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&modDependenceModel.ModDependency{}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return packageCount > 0 || dependencyCount > 0, nil
}
