package entry

// 文件职责：编排 V2 已安装远端 Mod 包的卸载流程。
// 使用：由 InstallerService.UninstallRemotePackage 在前端请求卸载 V2 包时调用。
// 边界：只按 installed 文件清单删除未共享文件并清理数据库记录，不处理安装、冲突继续或市场查询。

import (
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/operationguard"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"fmt"
	"gorm.io/gorm"
	"path/filepath"
	"strings"
)

// UninstallRemoteModPackage 按统一 packageKey 卸载 V2 安装包。
// 职责：按数据库文件清单删除未共享文件、清理安装记录并上报卸载进度。
// 使用：InstallerService.UninstallRemotePackage 在前端卸载 V2 包时调用。
// 边界：只删除由该 packageKey 记录且未被其它包共享的文件，避免误删用户手动文件。
// 逻辑流：1. 校验 packageKey 并读取本地记录 => 2. 构建文件卸载计划 => 3. 删除未共享文件 => 4. 事务清理数据库记录 => 5. 清理空目录并返回卸载结果。
func UninstallRemoteModPackage(packageKey string) (*contract.PackageInstallResult, error) {
	release, err := operationguard.Acquire()
	if err != nil {
		return nil, err
	}
	defer release()
	packageKey = strings.TrimSpace(packageKey)
	if packageKey == "" {
		return nil, fmt.Errorf("package key is required")
	}
	if err := cleanupEntry.CleanupInterrupted(packageKey, "卸载前恢复未完成安装"); err != nil {
		return nil, err
	}
	progressEvent.Uninstall(packageKey, "uninstall", "准备卸载", 5)
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	var installedPackage modDependenceModel.ModInstalledPackage
	result := db.Where("package_key = ?", packageKey).Take(&installedPackage)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			progressEvent.Uninstall(packageKey, "done", "未找到安装记录，已跳过卸载", 100)
			return &contract.PackageInstallResult{PackageKey: packageKey, GameDir: gameDir, PluginRoot: filepath.Join(gameDir, "BepInEx", "plugins"), Uninstalled: true, Skipped: true, Message: "Mod 包未安装，已跳过卸载", BepInExStatus: bepinexStatus.CheckBepInExStatus(gameDir)}, nil
		}
		return nil, result.Error
	}
	fileRecords, err := v2Uninstall.FindPackageFiles(db, packageKey)
	if err != nil {
		return nil, err
	}
	uninstallPlan, err := v2Uninstall.BuildPlan(db, packageKey, fileRecords)
	if err != nil {
		return nil, err
	}
	removed := make([]contract.DependencyInstallResult, 0, len(fileRecords))
	totalFiles := len(uninstallPlan)
	for index, plan := range uninstallPlan {
		progressEvent.Uninstall(packageKey, "uninstall", fmt.Sprintf("正在移除文件 %d/%d", index+1, totalFiles), 10+progresscalc.Percent(int64(index+1), int64(max(totalFiles, 1)))*70/100)
		if !plan.Shared {
			if err := v2Uninstall.RemoveManagedFile(plan.Record.TargetPath); err != nil {
				return nil, err
			}
		}
		removed = append(removed, contract.DependencyInstallResult{Key: plan.Record.ComponentKey, Name: filepath.Base(plan.Record.TargetPath), PluginDir: filepath.Dir(plan.Record.TargetPath), Removed: !plan.Shared, Skipped: plan.Shared, Message: v2Uninstall.FileMessage(plan.Shared)})
	}
	progressEvent.Uninstall(packageKey, "record", "正在清理安装记录", 88)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := v2Uninstall.ReleasePackageFileRecords(tx, packageKey); err != nil {
			return err
		}
		if err := v2Uninstall.DeleteLegacyComponentFileRecords(tx, packageKey); err != nil {
			return err
		}
		if err := tx.Where("package_key = ?", packageKey).Delete(&modDependenceModel.ModInstalledPackageComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&installedPackage).Error; err != nil {
			return err
		}
		return v2Uninstall.DeleteUnreferencedComponents(tx)
	}); err != nil {
		return nil, err
	}
	v2Uninstall.CleanupEmptyManagedDirs(filepath.Join(gameDir, "BepInEx"))
	progressEvent.Uninstall(packageKey, "done", "卸载完成", 100)
	return &contract.PackageInstallResult{PackageKey: packageKey, PackageName: installedPackage.DisplayName, GameDir: gameDir, PluginRoot: filepath.Join(gameDir, "BepInEx", "plugins"), Uninstalled: true, Message: "Mod 包卸载完成", Dependencies: removed, BepInExStatus: bepinexStatus.CheckBepInExStatus(gameDir)}, nil
}
