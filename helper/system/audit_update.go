package system

// 文件职责：构造应用更新检查和自更新相关的审计日志。
// 使用：由 UpdateService 在远端检查、自更新替换完成或失败后调用。
// 边界：只记录更新流程结果，不重试网络请求、不下载更新包、不回滚替换文件。

import systemModel "DysonSphereBox/model/system"

// RecordUpdateCheckSuccess 记录应用更新检查成功事件。
// 职责：写入应用更新检查成功的审计日志。
// 使用：UpdateService.CheckAppUpdate 在远端版本检查完成后调用。
// 边界：result 仅作为 metadata 保存，不在日志层解释是否必须更新。
func RecordUpdateCheckSuccess(version string, result interface{}) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "update",
		Event:     "checkAppUpdate",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   "应用更新检查完成",
		Metadata: map[string]interface{}{
			"version": version,
			"result":  result,
		},
	})
}

// RecordUpdateCheckFailure 记录应用更新检查失败事件。
// 职责：写入应用更新检查失败的审计日志。
// 使用：UpdateService.CheckAppUpdate 捕获版本源或网络错误后调用。
// 边界：不重试网络请求，不修改当前版本状态。
func RecordUpdateCheckFailure(version string, err error) {
	RecordFailure("system", "update", "checkAppUpdate", "应用更新检查失败", err, map[string]interface{}{
		"version": version,
	})
}

// RecordUpdateSelfSuccess 记录应用自更新成功事件。
// 职责：写入应用自更新应用成功的审计日志。
// 使用：UpdateService.UpdateAppSelf 在更新 helper 成功替换目标文件后调用。
// 边界：不重启程序，只记录“重启后生效”的结果。
func RecordUpdateSelfSuccess(version string, result interface{}) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "update",
		Event:     "updateAppSelf",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   "应用自更新已应用，重启后生效",
		Metadata: map[string]interface{}{
			"version": version,
			"result":  result,
		},
	})
}

// RecordUpdateSelfFailure 记录应用自更新失败事件。
// 职责：写入应用自更新失败的审计日志。
// 使用：UpdateService.UpdateAppSelf 捕获下载、校验或替换失败后调用。
// 边界：不回滚文件，回滚策略由 updater helper 负责。
func RecordUpdateSelfFailure(version string, err error) {
	RecordFailure("system", "update", "updateAppSelf", "应用自更新失败", err, map[string]interface{}{
		"version": version,
	})
}
