package headless

// 文件职责：格式化无头服务器审计日志中的进程和协议状态文本。
// 使用：端口状态变迁日志构造 content 时调用。
// 边界：只生成展示文本，不重新探测进程、不判断端口状态真实性。

import (
	"fmt"

	headlessContract "DysonSphereBox/helper/modDependence/headless/contract"
)

// formatHeadlessProcessID 格式化无头服务器进程 ID。
// 职责：为端口日志提供稳定的 PID 展示文本。
// 使用：recordHeadlessPortTransition 构造日志 content 时调用。
// 边界：无有效 PID 时返回 `-`，不尝试重新探测进程。
func formatHeadlessProcessID(processID int) string {
	if processID <= 0 {
		return "-"
	}

	return fmt.Sprintf("%d", processID)
}

// formatHeadlessProtocolStatus 格式化无头服务器监听协议状态。
// 职责：把 TCP/UDP 布尔状态转换为用户可读文本。
// 使用：recordHeadlessPortTransition 构造日志 content 时调用。
// 边界：没有任一协议监听时返回“未监听”，不代表进程不存在。
func formatHeadlessProtocolStatus(status headlessContract.HeadlessEnvironmentStatus) string {
	if status.TCPListening && status.UDPListening {
		return "TCP/UDP"
	}
	if status.TCPListening {
		return "TCP"
	}
	if status.UDPListening {
		return "UDP"
	}

	return "未监听"
}
