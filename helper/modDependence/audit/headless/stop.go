package headless

// 文件职责：记录无头服务器停止动作结果审计日志。
// 使用：HeadlessService.StopEnvironment 完成停止尝试后调用。
// 边界：只依据结果生成日志，不负责终止进程、不检查端口、不刷新状态。

import (
	"fmt"

	systemModel "DysonSphereBox/constant/logging"
	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordHeadlessStopResult 记录无头服务器停止动作结果。
// 职责：把停止、跳过或停止后仍运行状态写入系统审计日志。
// 使用：StopEnvironment 服务完成停止尝试后调用。
// 边界：result 为空时直接返回；不负责终止进程，只依据结果生成日志。
func RecordHeadlessStopResult(result *headlessContract.HeadlessStopResult) {
	if result == nil {
		return
	}

	level := systemModel.Info
	if !result.Stopped && !result.Skipped {
		level = systemModel.Warn
	}
	content := result.Message
	if result.Stopped {
		content = fmt.Sprintf("已停止 DSPGAME.exe 无头服务器进程 PID %d，监听端口 :%d 已释放", result.ProcessID, result.HostPort)
	} else if !result.Skipped && result.ProcessID > 0 {
		content = fmt.Sprintf("已发送停止命令到 DSPGAME.exe 进程 PID %d，但进程仍在运行，端口 :%d 状态需继续确认", result.ProcessID, result.HostPort)
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence.headless." + "stop",
		Source:    "backend.headless",
		Reason:    "停止请求已处理，进程检查结果见 data",
		Handler:   "User",
		Level:     level,
		Action:    content,
		Data: map[string]interface{}{
			"gameDir":   result.GameDir,
			"stopped":   result.Stopped,
			"skipped":   result.Skipped,
			"processId": result.ProcessID,
			"hostPort":  result.HostPort,
			"ready":     result.Status.Ready,
			"issues":    result.Status.Issues,
		},
	})
}
