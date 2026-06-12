package headless

// 文件职责：记录无头服务器设置保存结果审计日志。
// 使用：HeadlessService.SaveSettings 成功保存端口、存档和启动命令配置后调用。
// 边界：只写系统日志，不校验端口、不写 nebula.cfg、不修改启动命令。

import (
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// RecordHeadlessSettingsResult 记录无头服务器设置保存结果。
// 职责：把无头服务器端口、存档和启动命令设置写入系统审计日志。
// 使用：SaveSettings 服务成功保存配置后调用。
// 边界：result 为空时直接返回；不负责校验端口或写入配置文件。
func RecordHeadlessSettingsResult(result *headlessContract.HeadlessSettings) {
	if result == nil {
		return
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "settings",
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeConfiguration,
		Content:   "无头服务器设置已保存",
		Metadata: map[string]interface{}{
			"gameDir":          result.Status.GameDir,
			"hostPort":         result.HostPort,
			"serverArchive":    result.ServerArchive,
			"nebulaConfigPath": result.NebulaConfigPath,
			"startCommandText": result.Status.StartCommandText,
		},
	})
}
