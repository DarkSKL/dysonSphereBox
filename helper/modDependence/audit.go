package modDependence

import (
	systemHelper "DysonSphereBox/helper/system"
	ModDependenceResponse "DysonSphereBox/model/modDependence/response"
	systemModel "DysonSphereBox/model/system"
)

// RecordPackageInstallSuccess 记录 Mod 整合包安装成功事件。
func RecordPackageInstallSuccess(result *PackageInstallResult) {
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
func RecordPackageInstallFailure(pack ModDependenceResponse.ModDependencyPackageDTO, err error) {
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
func RecordPackageUninstallSuccess(result *PackageInstallResult) {
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
func RecordPackageUninstallFailure(packageID uint, err error) {
	systemHelper.RecordFailure("modDependence", "installer", "uninstallPackage", "Mod 整合包卸载失败", err, map[string]interface{}{
		"packageId": packageID,
	})
}

// RecordBepInExFailure 记录 BepInEx 操作失败事件。
func RecordBepInExFailure(event string, content string, err error) {
	systemHelper.RecordFailure("modDependence", "bepInEx", event, content, err, nil)
}

// RecordBepInExUninstallSuccess 记录 BepInEx 卸载成功事件。
func RecordBepInExUninstallSuccess(result *BepInExUninstallResult) {
	if result == nil {
		return
	}

	resultText := "已移除"
	if result.Skipped {
		resultText = "已跳过"
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "bepInEx",
		Event:     "uninstall",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "BepInEx 状态：已安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"gameDir":        result.GameDir,
			"removed":        result.Removed,
			"skipped":        result.Skipped,
			"removedCount":   len(result.RemovedPaths),
			"clearedRecords": result.ClearedRecords,
			"message":        result.Message,
		},
	})
}

// RecordBepInExInstallResult 记录 BepInEx 安装或更新结果。
func RecordBepInExInstallResult(event string, result *BepInExInstallResult) {
	if result == nil {
		return
	}

	resultText := "已安装"
	if result.Skipped {
		resultText = "已跳过"
	}

	packageVersion := ""
	if result.Package != nil {
		packageVersion = result.Package.Version
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "bepInEx",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeInstallation,
		Content:   "BepInEx 状态：未安装 -> " + resultText,
		Metadata: map[string]interface{}{
			"gameDir":   result.GameDir,
			"bufferDir": result.BufferDir,
			"version":   packageVersion,
			"message":   result.Message,
		},
	})
}
