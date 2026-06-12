package bepinex

// 文件职责：构造 BepInEx 前置框架安装、更新和卸载审计日志。
// 使用：由 service/modDependence 的 BepInExService 在 helper 返回结果或错误后调用。
// 边界：只写系统日志，不执行文件修改、网络请求、安装状态迁移或服务层错误处理。

import (
	bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// RecordBepInExFailure 记录 BepInEx 操作失败事件。
// 职责：统一写入 BepInEx 安装、更新或卸载失败日志。
// 使用：service/modDependence 在 BepInEx 服务方法捕获错误后调用。
// 边界：只记录日志，不吞掉错误，不修改安装文件或数据库状态。
func RecordBepInExFailure(event string, content string, err error) {
	systemHelper.RecordFailure("modDependence", "bepInEx", event, content, err, nil)
}

// RecordBepInExUninstallSuccess 记录 BepInEx 卸载成功事件。
// 职责：把卸载结果转换为系统审计日志。
// 使用：BepInEx 卸载服务成功返回后调用。
// 边界：result 为空时直接返回；不负责执行卸载或清理文件。
func RecordBepInExUninstallSuccess(result *bepinexContract.BepInExUninstallResult) {
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
// 职责：把安装、更新或跳过结果转换为系统审计日志。
// 使用：BepInEx 安装和更新服务成功或跳过后调用。
// 边界：result 为空时直接返回；不负责选择版本、下载或解压 BepInEx。
func RecordBepInExInstallResult(event string, result *bepinexContract.BepInExInstallResult) {
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
