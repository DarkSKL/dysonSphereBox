package system

// 文件职责：构造系统配置变更相关的审计日志。
// 使用：由 service/system 配置服务在保存配置成功或失败后调用。
// 边界：只封装日志内容和 metadata，不执行配置写入、不校验配置值、不暴露敏感配置内容。

import systemModel "DysonSphereBox/constant/logging"

// RecordSetConfigSuccess 记录配置保存成功事件。
// 职责：写入配置保存成功的系统审计日志。
// 使用：ConfigurationService.SetConfig 在配置成功保存并完成必要运行时刷新后调用。
// 边界：不保存配置值，避免日志泄露敏感内容。
func RecordSetConfigSuccess(group string, key string) {
	RecordLogSilently(LogEntry{
		Namespace: "system.configuration." + "setConfig",
		Source:    "backend.configuration",
		Reason:    "配置保存与必要的运行时刷新均已完成",
		Handler:   "User",
		Level:     systemModel.Info,
		Action:    "系统配置已更新",
		Data: map[string]interface{}{
			"group": group,
			"key":   key,
		},
	})
}

// RecordSetConfigFailure 记录配置保存失败事件。
// 职责：写入配置保存失败的系统审计日志。
// 使用：ConfigurationService.SetConfig 在配置校验或 ConfigUnit.Set 失败后调用。
// 边界：只记录 group/key 和错误文本，不重试保存。
func RecordSetConfigFailure(group string, key string, err error) {
	RecordLogSilently(LogEntry{
		Namespace: "system.configuration." + "setConfig",
		Source:    "backend.configuration",
		Reason:    "配置校验或持久化返回错误，未推断根因",
		Handler:   "User",
		Level:     systemModel.Error,
		Action:    "系统配置更新失败",
		Data: map[string]interface{}{
			"group": group,
			"key":   key,
		},
		Error: errorText(err),
	})
}
