package logging

// 文件职责：记录 Thunderstore V2 安装会话清理过程中的内部审计日志。
// 使用：由安装会话恢复、取消安装清理和失败回滚流程在清理开始、单项处理、清理完成时调用。
// 边界：只记录清理过程日志，不执行文件删除、数据库事务或服务层安装成功/失败审计；日志失败由 helper/system 静默处理。
// 逻辑流：1. 清理入口记录开始摘要 => 2. 各清理步骤记录单项动作 => 3. 事务内日志延迟收集 => 4. 事务后刷新并记录完成结果。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	systemHelper "DysonSphereBox/helper/system"
	installedModel "DysonSphereBox/model/modDependence/installed"
	system "DysonSphereBox/model/system"
	"fmt"
)

// RecordStart 记录安装会话清理开始日志。
// 职责：输出包信息、清理状态和 manifest 规模，便于诊断中断恢复。
// 使用：sessioncleanup/entry.CleanupInterrupted 在执行文件清理前调用。
// 边界：只写审计日志，不影响清理流程；日志失败由底层静默处理。
func RecordStart(record installedModel.ModInstalledPackage, status string, manifest sessionstate.Manifest) {
	recordRemotePackageCleanupLog("cleanupRemotePackageInstallStart", system.LogLevelInfo, "开始清理未完成的 Thunderstore Mod 包安装", nil, map[string]interface{}{
		"packageKey":              record.PackageKey,
		"packageName":             record.DisplayName,
		"source":                  record.Source,
		"status":                  status,
		"installStage":            record.InstallStage,
		"installStatus":           record.InstallStatus,
		"installProgress":         record.InstallProgress,
		"startedAt":               manifest.StartedAt,
		"plannedComponentCount":   len(manifest.PlannedComponents),
		"plannedTargetDirCount":   len(manifest.PlannedTargetDirs),
		"installedComponentCount": len(manifest.InstalledComponents),
		"installedFileCount":      len(manifest.InstalledFiles),
	})
}

// RecordItem 记录单个清理动作日志。
// 职责：统一描述文件、目录、缓存、数据库记录等清理项的动作结果。
// 使用：清理流程每处理一个重要对象时调用。
// 边界：err 非空时记录错误级别；不会重试或改变清理决策。
func RecordItem(packageKey string, kind string, target string, action string, reason string, err error) {
	level := system.LogLevelInfo
	success := true
	if err != nil {
		level = system.LogLevelError
		success = false
	}
	content := fmt.Sprintf("Thunderstore Mod 包清理：%s %s", action, kind)
	recordRemotePackageCleanupLog("cleanupRemotePackageInstallItem", level, content, err, map[string]interface{}{
		"packageKey": packageKey,
		"kind":       kind,
		"target":     target,
		"action":     action,
		"reason":     reason,
		"success":    success,
	})
}

// FlushItems 在数据库写事务完成后再写系统日志。
// 职责：延迟刷新事务内收集的清理日志，避免 SQLite 写锁嵌套。
// 使用：sessioncleanup/files.CleanupFiles 事务结束后调用。
// 边界：只负责日志输出，不回滚事务；SQLite 同库事务内不能嵌套写 system_logs。
func FlushItems(items []sessionstate.CleanupLogItem) {
	for _, item := range items {
		RecordItem(item.PackageKey, item.Kind, item.Target, item.Action, item.Reason, item.Err)
	}
}

// RecordDone 记录安装会话清理完成或失败日志。
// 职责：汇总清理结果、manifest 规模和成功状态。
// 使用：sessioncleanup/entry.CleanupInterrupted 在每个退出分支调用。
// 边界：只记录日志，不覆盖安装状态；err 非空时记录失败事件。
func RecordDone(packageKey string, status string, manifest sessionstate.Manifest, err error) {
	level := system.LogLevelInfo
	event := "cleanupRemotePackageInstallDone"
	content := "Thunderstore Mod 包未完成安装清理完成"
	if err != nil {
		level = system.LogLevelError
		event = "cleanupRemotePackageInstallFailed"
		content = "Thunderstore Mod 包未完成安装清理失败"
	}
	recordRemotePackageCleanupLog(event, level, content, err, map[string]interface{}{
		"packageKey":              packageKey,
		"status":                  status,
		"plannedComponentCount":   len(manifest.PlannedComponents),
		"plannedTargetDirCount":   len(manifest.PlannedTargetDirs),
		"installedComponentCount": len(manifest.InstalledComponents),
		"installedFileCount":      len(manifest.InstalledFiles),
		"cleanedAt":               manifest.CleanedAt,
		"success":                 err == nil,
	})
}

// recordRemotePackageCleanupLog 记录远端安装会话清理过程日志。
// 职责：统一写入清理开始、单项清理和清理结果日志。
// 使用：安装会话恢复和清理流程在处理未完成安装记录时调用。
// 边界：只记录日志，不执行文件删除或数据库事务；err 为空时日志 Error 字段为空。
func recordRemotePackageCleanupLog(event string, level system.LogLevel, content string, err error, metadata map[string]interface{}) {
	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "installer",
		Event:     event,
		Level:     level,
		Type:      system.LogTypeInstallation,
		Content:   content,
		Metadata:  metadata,
		Error:     errorText,
	})
}
