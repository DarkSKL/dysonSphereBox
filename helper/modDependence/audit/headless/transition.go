package headless

// 文件职责：记录无头服务器进程和端口状态变迁日志。
// 使用：RecordHeadlessStatusObservation 检测到进程或端口监听状态变化后调用。
// 边界：只记录审计日志，不启动/停止进程、不检查或修复端口占用。

import (
	"fmt"

	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
	systemHelper "DysonSphereBox/helper/system"
	systemModel "DysonSphereBox/model/system"
)

// recordHeadlessProcessTransition 记录无头服务器进程上线或下线日志。
// 职责：根据进程运行布尔值生成对应事件名、内容和元数据。
// 使用：RecordHeadlessStatusObservation 检测到 processRunning 变化时调用。
// 边界：只记录审计日志，不启动或停止进程。
func recordHeadlessProcessTransition(status headlessContract.HeadlessEnvironmentStatus, online bool, source string) {
	event := "processOffline"
	content := fmt.Sprintf("DSPGAME.exe 无头服务器进程已下线，目标监听端口 :%d", status.HostPort)
	if online {
		event = "processOnline"
		content = fmt.Sprintf("DSPGAME.exe 无头服务器进程已上线，PID %d，目标监听端口 :%d", status.ProcessID, status.HostPort)
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata: map[string]interface{}{
			"gameDir":   status.GameDir,
			"processId": status.ProcessID,
			"hostPort":  status.HostPort,
			"source":    source,
		},
	})
}

// recordHeadlessPortTransition 记录无头服务器端口开放或关闭日志。
// 职责：根据端口监听布尔值生成对应事件名、协议描述和元数据。
// 使用：RecordHeadlessStatusObservation 检测到 portListening 变化时调用。
// 边界：只记录审计日志，不检查或修复端口占用。
func recordHeadlessPortTransition(status headlessContract.HeadlessEnvironmentStatus, opened bool, source string) {
	event := "portClosed"
	content := fmt.Sprintf("无头服务器监听端口 :%d 已关闭，进程 PID %s，协议状态：%s", status.HostPort, formatHeadlessProcessID(status.ProcessID), formatHeadlessProtocolStatus(status))
	if opened {
		event = "portOpen"
		content = fmt.Sprintf("无头服务器监听端口 :%d 已开放，进程 PID %s，协议状态：%s", status.HostPort, formatHeadlessProcessID(status.ProcessID), formatHeadlessProtocolStatus(status))
	}

	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Namespace: "modDependence",
		Module:    "headless",
		Event:     event,
		Level:     systemModel.LogLevelInfo,
		Type:      systemModel.LogTypeOperation,
		Content:   content,
		Metadata: map[string]interface{}{
			"gameDir":      status.GameDir,
			"processId":    status.ProcessID,
			"hostPort":     status.HostPort,
			"tcpListening": status.TCPListening,
			"udpListening": status.UDPListening,
			"source":       source,
		},
	})
}
