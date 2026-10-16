package headless

// 文件职责：记录无头服务器环境准备结果审计日志。
// 使用：HeadlessService.PrepareEnvironment 完成 BepInEx 与基础文件准备后调用。
// 边界：只写系统日志，不创建基础文件、不安装 Nebula 依赖、不检测端口。

import (
	systemModel "DysonSphereBox/constant/logging"
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordHeadlessPrepareResult 记录无头服务器环境准备结果。
// 职责：把无头服务器环境准备结果写入系统审计日志。
// 使用：PrepareEnvironment 服务完成 BepInEx 与基础文件准备后调用。
// 边界：result 为空或环境已就绪且没有变更时不记录；不创建文件或启动进程。
func RecordHeadlessPrepareResult(result *headlessContract.HeadlessPrepareResult) {
	if result == nil || (!result.Changed && result.Status.Ready) {
		return
	}

	level := systemModel.Info
	if !result.Status.Ready {
		level = systemModel.Warn
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence.headless." + "prepare",
		Source:    "backend.headless",
		Reason:    "环境准备结束，实际就绪检查与问题见 data",
		Handler:   "User",
		Level:     level,
		Action:    result.Message,
		Data: map[string]interface{}{
			"gameDir":      result.GameDir,
			"changed":      result.Changed,
			"writtenPath":  result.WrittenPath,
			"ready":        result.Status.Ready,
			"issues":       result.Status.Issues,
			"hostPort":     result.Status.HostPort,
			"steamAppIdOk": result.Status.SteamAppIDValid,
			"nebulaReady":  result.Status.NebulaReady,
		},
	})
}
