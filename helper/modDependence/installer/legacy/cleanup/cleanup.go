package cleanup

// 文件职责：维护旧官方整合包依赖共享判断、孤立依赖清理和插件目录删除。
// 使用：旧整合包安装前清理过期依赖，卸载时判断依赖是否仍被其它整合包引用。
// 边界：只处理 legacy 模型和 BepInEx/plugins 下的依赖目录；不处理 V2 文件清单卸载。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/fileops"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"DysonSphereBox/helper/modDependence/installer/shared"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
	"fmt"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

// StalePackageDependencies 查找本次安装后不再属于该整合包且无人共享的旧依赖。
// 职责：为重复安装或升级旧整合包时的插件目录清理提供候选列表。
// 使用：legacy/install.InstallModDependencyPackage 保存新记录前调用，安装成功后再删除这些目录。
// 边界：找不到旧安装记录时返回空列表；共享依赖会被保留。
func StalePackageDependencies(packageID uint, nextDependencies []modDependenceResponse.ModDependencyDTO) ([]modDependenceModel.ModDependency, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}

	var installedPackage modDependenceModel.ModDependencyPackage
	queryResult := db.Preload("Dependencies").Where("id = ?", packageID).Find(&installedPackage)
	if queryResult.Error != nil {
		return nil, queryResult.Error
	}
	if queryResult.RowsAffected == 0 {
		return nil, nil
	}

	nextDependencyIDs := make(map[uint]struct{}, len(nextDependencies))
	for _, dependency := range nextDependencies {
		nextDependencyIDs[dependency.ID] = struct{}{}
	}

	staleDependencies := make([]modDependenceModel.ModDependency, 0)
	for _, dependency := range installedPackage.Dependencies {
		if _, exists := nextDependencyIDs[dependency.ID]; exists {
			continue
		}

		shared, err := DependencySharedByOtherPackages(db, dependency.ID, packageID)
		if err != nil {
			return nil, err
		}
		if !shared {
			staleDependencies = append(staleDependencies, dependency)
		}
	}

	return staleDependencies, nil
}

// RemoveStaleDependencyPluginDirs 删除不再需要的旧依赖插件目录。
// 职责：根据 StalePackageDependencies 的结果清理 BepInEx/plugins 下的目录。
// 使用：旧整合包重新安装且新记录保存成功后调用。
// 边界：任一目录删除失败会返回错误；不会修改数据库记录。
func RemoveStaleDependencyPluginDirs(pluginRoot string, dependencies []modDependenceModel.ModDependency) error {
	for _, dependency := range dependencies {
		pluginDir := DependencyPluginDir(pluginRoot, legacyValue.String(dependency.ModName))
		if err := RemoveDependencyPluginDir(pluginRoot, pluginDir); err != nil {
			return err
		}
	}

	return nil
}

// DeleteUnreferencedDependencies 删除无人引用的 legacy 依赖记录。
// 职责：保持旧官方整合包依赖表不累积孤立数据。
// 使用：保存整合包关联或卸载整合包事务内调用。
// 边界：必须传入调用方事务；不删除任何插件目录。
func DeleteUnreferencedDependencies(tx *gorm.DB) error {
	return tx.
		Where("id NOT IN (SELECT DISTINCT mod_dependency_id FROM mod_dependency_package_relations)").
		Delete(&modDependenceModel.ModDependency{}).
		Error
}

// DependencySharedByOtherPackages 判断依赖是否被其它旧整合包引用。
// 职责：为卸载或升级时是否删除插件目录提供数据库判断。
// 使用：卸载指定整合包或查找过期依赖时调用。
// 边界：只读取 legacy 关联表，不检查文件系统。
func DependencySharedByOtherPackages(db *gorm.DB, dependencyID uint, packageID uint) (bool, error) {
	var count int64
	err := db.Table("mod_dependency_package_relations").
		Where("mod_dependency_id = ? AND mod_dependency_package_id <> ?", dependencyID, packageID).
		Count(&count).Error
	return count > 0, err
}

// DependencySharedByAnyPackage 判断依赖是否仍被任意旧整合包引用。
// 职责：为事务中清理孤立依赖记录提供布尔判断。
// 使用：legacy/uninstallpkg.UninstallModDependencyPackage 清理关联后调用。
// 边界：只读取 legacy 关联表，不检查 V2 installed 模型。
func DependencySharedByAnyPackage(db *gorm.DB, dependencyID uint) (bool, error) {
	var count int64
	err := db.Table("mod_dependency_package_relations").
		Where("mod_dependency_id = ?", dependencyID).
		Count(&count).Error
	return count > 0, err
}

// RemoveDependencyPluginDir 删除单个旧依赖插件目录。
// 职责：在确认目录位于 pluginRoot 内后执行 RemoveAll。
// 使用：卸载或升级旧整合包清理依赖目录时调用。
// 边界：目录为空或越出 pluginRoot 时返回错误，不执行删除。
func RemoveDependencyPluginDir(pluginRoot string, pluginDir string) error {
	if pluginDir == "" || !fileops.IsSubPath(pluginRoot, pluginDir) {
		return fmt.Errorf("plugin path escapes plugin root: %s", pluginDir)
	}

	return os.RemoveAll(pluginDir)
}

// DependencyPluginDir 根据旧依赖名称生成插件目录路径。
// 职责：统一 legacy 依赖目录命名规则。
// 使用：安装、卸载和过期依赖清理时调用。
// 边界：只拼接路径，目录安全性由 RemoveDependencyPluginDir 或安装入口再次校验。
func DependencyPluginDir(pluginRoot string, name string) string {
	return filepath.Join(pluginRoot, shared.SanitizePathSegment(name, "mod"))
}
