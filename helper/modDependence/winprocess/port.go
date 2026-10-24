package winprocess

// 文件职责：封装 Windows netstat 端口监听检测与按进程 PID 的端口归属查询。
// 使用：headless 根包判断开服端口占用；gameport 探测游戏进程当前绑定的端口。
// 边界：只读取 netstat 当前输出，不预留端口；命令失败时按未监听处理。

import (
	"DysonSphereBox/utils/wincommand"
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

	return portListeningInOutput(string(output), protocol, port)
}

// portListeningInOutput 职责：精确匹配本地端点端口；使用：netstat 解析与回归验证；边界：忽略远端端口和非监听 TCP，兼容 IPv6。
func portListeningInOutput(output, protocol string, port int) bool {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || !strings.EqualFold(fields[0], protocol) {
			continue
		}
		local := fields[1]
		index := strings.LastIndex(local, ":")
		if index < 0 || local[index+1:] != strconv.Itoa(port) {
			continue
		}
		if strings.EqualFold(protocol, "udp") || (len(fields) >= 5 && strings.EqualFold(fields[3], "LISTENING")) {
			return true
		}
	}
	return false
}
