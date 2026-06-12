package install

// 文件职责：记录旧官方 Mod 整合包安装与卸载审计日志。
// 使用：service/modDependence 的旧官方整合包安装和卸载流程成功或失败后调用。
// 边界：只记录系统日志，不执行依赖安装、不回滚文件、不保存 GORM 模型。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// RecordPackageInstallSuccess 记录 Mod 整合包安装成功事件。
// 职责：把旧官方整合包安装成功或跳过结果写入系统审计日志。
// 使用：InstallPackage 成功返回后调用。
// 边界：result 为空时直接返回；不负责安装依赖或保存 GORM 模型。
func RecordPackageInstallSuccess(result *contract.PackageInstallResult) {
	if result == nil {
		return
	}

	resultText := "已安装"
	if result.Skipped {
		resultText = "已跳过"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "installer",
		Event:     "installPackage",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "Mod 整合包状态：未安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"packageId":       result.PackageID,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
			"pluginRoot":      result.PluginRoot,
			"message":         result.Message,
		},
	})
}

// RecordPackageInstallFailure 记录 Mod 整合包安装失败事件。
// 职责：把旧官方整合包安装失败写入系统审计日志。
// 使用：InstallPackage 捕获错误后调用。
// 边界：只记录失败，不回滚文件；回滚由安装 helper 流程负责。
func RecordPackageInstallFailure(pack modDependenceResponse.ModDependencyPackageDTO, err error) {
	packageName := ""
	if pack.PackageName != nil {
		packageName = *pack.PackageName
	}

	systemHelper.RecordFailure("modDependence", "installer", "installPackage", "Mod 整合包安装失败", err, map[string]interface{}{
		"packageId":   pack.ID,
		"packageName": packageName,
	})
}

// RecordPackageUninstallSuccess 记录 Mod 整合包卸载成功事件。
// 职责：把旧官方整合包卸载成功结果写入系统审计日志。
// 使用：UninstallPackage 成功返回后调用。
// 边界：result 为空时直接返回；不负责执行卸载或删除数据库记录。
func RecordPackageUninstallSuccess(result *contract.PackageInstallResult) {
	if result == nil {
		return
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "installer",
		Event:     "uninstallPackage",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "Mod 整合包状态：已安装 -> 已卸载",
		Metadata: map[string]interface{}{
			"packageId":       result.PackageID,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
		},
	})
}

// RecordPackageUninstallFailure 记录 Mod 整合包卸载失败事件。
// 职责：把旧官方整合包卸载失败写入系统审计日志。
// 使用：UninstallPackage 捕获错误后调用。
// 边界：只记录失败，不修改文件或安装状态。
func RecordPackageUninstallFailure(packageID uint, err error) {
	systemHelper.RecordFailure("modDependence", "installer", "uninstallPackage", "Mod 整合包卸载失败", err, map[string]interface{}{
		"packageId": packageID,
	})
}
