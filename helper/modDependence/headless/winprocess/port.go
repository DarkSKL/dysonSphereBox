package winprocess

// 文件职责：封装 Windows netstat 端口监听检测。
// 使用：由 headless 根包在环境状态刷新和设置保存校验时调用，用于判断 TCP/UDP 端口是否占用。
// 边界：只读取 netstat 当前输出，不预留端口；命令失败时按未监听处理。

import (
	"DysonSphereBox/helper/modDependence/headless/wincommand"
	"strconv"
	"strings"
)

// IsPortOccupied 判断目标端口是否被 TCP 或 UDP 占用。
// 职责：在保存开服设置前阻止用户选择已被监听的端口。
// 使用：headless.SaveHeadlessSettings 校验 HostPort 时调用。
// 边界：基于 netstat 当前输出，不能保证返回后端口仍保持空闲；命令失败时视为未占用。
func IsPortOccupied(port int) bool {
	return IsNetstatPortListening("tcp", port) || IsNetstatPortListening("udp", port)
}

// IsNetstatPortListening 检查指定协议端口是否监听。
// 职责：通过 netstat -ano -p 解析 TCP 或 UDP 监听状态。
// 使用：headless.InspectHeadlessEnvironment 生成端口状态，IsPortOccupied 复用该检测。
// 边界：命令失败时返回 false；UDP 只要出现端口即视为监听。
func IsNetstatPortListening(protocol string, port int) bool {
	output, err := wincommand.Output("netstat", "-ano", "-p", protocol)
	if err != nil {
		return false
	}

	portToken := ":" + strconv.Itoa(port)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		normalized := strings.ToUpper(strings.TrimSpace(line))
		if !strings.Contains(normalized, strings.ToUpper(protocol)) || !strings.Contains(normalized, portToken) {
			continue
		}
		if protocol == "tcp" {
			return strings.Contains(normalized, "LISTENING")
		}
		return true
	}

	return false
}
