package system

import systemModel "DysonSphereBox/model/system"

// RecordSetConfigSuccess 记录配置保存成功事件。
func RecordSetConfigSuccess(group string, key string) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "configuration",
		Event:     "setConfig",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeConfiguration,
		Content:   "系统配置已更新",
		Metadata: map[string]interface{}{
			"group": group,
			"key":   key,
		},
	})
}

// RecordSetConfigFailure 记录配置保存失败事件。
func RecordSetConfigFailure(group string, key string, err error) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "configuration",
		Event:     "setConfig",
		Level:     systemModel.LogLevelError,
		Type:      systemModel.LogTypeConfiguration,
		Content:   "系统配置更新失败",
		Metadata: map[string]interface{}{
			"group": group,
			"key":   key,
		},
		Error: errorText(err),
	})
}

// RecordGamePathSuccess 记录游戏目录加载或初始化成功事件。
func RecordGamePathSuccess(event string, content string, dirPath string, source string) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "environment",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeConfiguration,
		Content:   content,
		Metadata: map[string]interface{}{
			"dirPath": dirPath,
			"source":  source,
		},
	})
}

// RecordGamePathFailure 记录游戏目录加载、初始化或选择失败事件。
func RecordGamePathFailure(event string, content string, err error, dirPath string) {
	RecordLogSilently(LogEntry{
		Namespace: "system",
		Module:    "environment",
		Event:     event,
		Level:     systemModel.LogLevelError,
		Type:      systemModel.LogTypeConfiguration,
		Content:   content,
		Metadata: map[string]interface{}{
			"dirPath": dirPath,
		},
		Error: errorText(err),
	})
}

// RecordUpdateCheckSuccess 记录应用更新检查成功事件。
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
func RecordUpdateCheckFailure(version string, err error) {
	RecordFailure("system", "update", "checkAppUpdate", "应用更新检查失败", err, map[string]interface{}{
		"version": version,
	})
}

// RecordUpdateSelfSuccess 记录应用自更新成功事件。
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
func RecordUpdateSelfFailure(version string, err error) {
	RecordFailure("system", "update", "updateAppSelf", "应用自更新失败", err, map[string]interface{}{
		"version": version,
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}
