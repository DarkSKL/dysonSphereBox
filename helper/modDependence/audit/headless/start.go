package headless

// 文件职责：记录无头服务器启动动作结果审计日志。
// 使用：HeadlessService.StartEnvironment 完成启动尝试后调用。
// 边界：只依据结果生成日志，不负责启动进程、不刷新状态、不修复环境。

import (
	"fmt"

	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// RecordHeadlessStartResult 记录无头服务器启动动作结果。
// 职责：把启动、跳过或启动异常状态写入系统审计日志。
// 使用：StartEnvironment 服务完成启动尝试后调用。
// 边界：result 为空时直接返回；不负责启动进程，只依据结果生成日志。
func RecordHeadlessStartResult(result *headlessContract.HeadlessStartResult) {
	if result == nil {
		return
	}

	level := systemModel.LogLevelInfo
	if !result.Started && !result.Skipped {
		level = systemModel.LogLevelWarn
	}
	content := result.Message
	if result.Started {
		content = fmt.Sprintf("已启动 DSPGAME.exe 无头服务器进程 PID %d，目标监听端口 :%d", result.ProcessID, result.Status.HostPort)
	} else if result.Skipped && result.ProcessID > 0 {
		content = fmt.Sprintf("DSPGAME.exe 无头服务器已在运行，PID %d，监听端口 :%d", result.ProcessID, result.Status.HostPort)
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     "start",
		Level:     level,
		Type:      systemModel.LogTypeConfiguration,
		Content:   content,
		Metadata: map[string]interface{}{
			"gameDir":          result.GameDir,
			"started":          result.Started,
			"skipped":          result.Skipped,
			"processId":        result.ProcessID,
			"ready":            result.Status.Ready,
			"issues":           result.Status.Issues,
			"hostPort":         result.Status.HostPort,
			"serverArchive":    result.Status.ServerArchive,
			"startCommandText": result.StartCommandText,
		},
	})
}
