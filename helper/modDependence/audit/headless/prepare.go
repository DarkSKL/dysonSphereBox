package headless

// 文件职责：记录无头服务器环境准备结果审计日志。
// 使用：HeadlessService.PrepareEnvironment 完成 BepInEx 与基础文件准备后调用。
// 边界：只写系统日志，不创建基础文件、不安装 Nebula 依赖、不检测端口。

import (
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// RecordHeadlessPrepareResult 记录无头服务器环境准备结果。
// 职责：把无头服务器环境准备结果写入系统审计日志。
// 使用：PrepareEnvironment 服务完成 BepInEx 与基础文件准备后调用。
// 边界：result 为空时直接返回；只记录准备结果，不创建文件或启动进程。
func RecordHeadlessPrepareResult(result *headlessContract.HeadlessPrepareResult) {
	if result == nil {
		return
	}

	level := systemModel.LogLevelInfo
	if !result.Status.Ready {
		level = systemModel.LogLevelWarn
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "prepare",
		Level:     level,
		Type:      systemModel.LogTypeConfiguration,
		Content:   result.Message,
		Metadata: map[string]interface{}{
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
