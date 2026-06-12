package uninstallpkg

// 文件职责：编排旧官方整合包的卸载入口。
// 使用：service/modDependence 的 InstallerService 在旧官方整合包已安装卡片点击卸载时调用。
// 边界：只处理 legacy 模型和 BepInEx/plugins 下的旧依赖目录，不处理 V2 文件清单卸载。

import (
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/installer/contract"
	legacyCleanup "DysonSphereBox/helper/modDependence/installer/legacy/cleanup"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"DysonSphereBox/helper/modDependence/installer/shared"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
	"fmt"
	"gorm.io/gorm"
	"path/filepath"
)

// UninstallModDependencyPackage 卸载本地 SQLite 中记录的整合包。
// 职责：移除旧官方整合包记录，并删除不再被其它整合包共享的插件目录。
// 使用：前端点击已安装卡片的卸载按钮时调用；会移除不再被其它整合包引用的插件目录。
// 边界：找不到记录时按已卸载处理，避免重复点击产生破坏性副作用。
// 逻辑流：1. 读取游戏目录与本地记录 => 2. 判断每个依赖是否共享 => 3. 删除可安全删除的插件目录 => 4. 清理关联表和孤立依赖 => 5. 返回卸载明细。
func UninstallModDependencyPackage(packageID uint) (*contract.PackageInstallResult, error) {
	if packageID == 0 {
		return nil, fmt.Errorf("package id is required")
	}
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	pluginRoot := filepath.Join(gameDir, "BepInEx", "plugins")
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	var pack modDependenceModel.ModDependencyPackage
	queryResult := db.Preload("Dependencies").Where("id = ?", packageID).Find(&pack)
	if queryResult.Error != nil {
		return nil, queryResult.Error
	}
	if queryResult.RowsAffected == 0 {
		return &contract.PackageInstallResult{PackageID: packageID, GameDir: gameDir, PluginRoot: pluginRoot, Uninstalled: true, Skipped: true, Message: "整合包未安装，已跳过卸载"}, nil
	}
	dependencyResults := make([]contract.DependencyInstallResult, 0, len(pack.Dependencies))
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, dependency := range pack.Dependencies {
			shared, err := legacyCleanup.DependencySharedByOtherPackages(tx, dependency.ID, packageID)
			if err != nil {
				return err
			}
			result := contract.DependencyInstallResult{ID: dependency.ID, Name: legacyValue.String(dependency.ModName), Version: legacyValue.String(dependency.ModVersion), Skipped: shared}
			result.PluginDir = legacyCleanup.DependencyPluginDir(pluginRoot, result.Name)
			if shared {
				result.Message = "仍被其它整合包引用，保留插件目录"
			} else {
				if err := legacyCleanup.RemoveDependencyPluginDir(pluginRoot, result.PluginDir); err != nil {
					return err
				}
				result.Removed = true
				result.Message = "插件目录已移除"
			}
			dependencyResults = append(dependencyResults, result)
		}
		if err := tx.Model(&pack).Association("Dependencies").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&pack).Error; err != nil {
			return err
		}
		for _, dependency := range pack.Dependencies {
			shared, err := legacyCleanup.DependencySharedByAnyPackage(tx, dependency.ID)
			if err != nil {
				return err
			}
			if !shared {
				if err := tx.Delete(&modDependenceModel.ModDependency{}, dependency.ID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &contract.PackageInstallResult{PackageID: packageID, PackageName: legacyValue.String(pack.PackageName), GameDir: gameDir, PluginRoot: pluginRoot, Uninstalled: true, Dependencies: dependencyResults, Message: "整合包卸载完成", BepInExStatus: bepinexStatus.CheckBepInExStatus(gameDir)}, nil
}
