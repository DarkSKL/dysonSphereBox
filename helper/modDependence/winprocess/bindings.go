package winprocess

// 文件职责：按进程 PID 汇总 Windows TCP/UDP 本地端口绑定。
// 使用：gameport 判断游戏进程是否真的监听了 Nebula 主机端口，并据实际绑定推断协议。
// 边界：只读取 netstat 当前输出；TCP 只统计 LISTENING，UDP 统计已绑定端点；命令失败时返回错误。

import (
	"DysonSphereBox/utils/wincommand"
	"strconv"
	"strings"
)

// Binding 表示某个进程持有的一个本地端口绑定。
// 职责：记录协议与端口号，供上层判断可转发端口。
// 使用：ListBindings 返回给 gameport 的戴森球计划探测逻辑。
// 边界：只表示当前 netstat 快照，不保证调用后端口仍被占用。
type Binding struct {
	// Proto 职责：绑定协议；使用：映射为房间端口声明的 tcp / udp / both；边界：小写 tcp 或 udp。
	Proto string
	// Port 职责：本地端口号；使用：填充创建房间端口输入；边界：1–65535。
	Port int
}

// ListBindings 查询指定进程当前持有的 TCP/UDP 本地端口。
// 职责：一次性读取 netstat 快照并按 PID 过滤，去重协议与端口组合。
// 使用：gameport 探测游戏进程时调用；进程不存在或没有绑定端口时返回空切片。
// 边界：TCP 只保留 LISTENING，UDP 保留所有绑定端点；netstat 失败时返回错误。
func ListBindings(pid int) ([]Binding, error) {
	if pid <= 0 {
		return nil, nil
	}
	output, err := wincommand.Output("netstat", "-ano")
	if err != nil {
		return nil, err
	}

	return parseBindings(string(output), pid), nil
}

// parseBindings 从 netstat -ano 输出中筛选目标进程的绑定。
// 职责：解析 IPv4/IPv6 本地端点、协议和 PID，并去除重复项。
// 使用：ListBindings 与回归测试。
// 边界：忽略表头、非目标 PID、非监听 TCP 和非法端口。
func parseBindings(output string, pid int) []Binding {
	bindings := make([]Binding, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		protocol := strings.ToLower(fields[0])
		pidIndex := 0
		switch protocol {
		case "tcp":
			if len(fields) < 5 || !strings.EqualFold(fields[3], "LISTENING") {
				continue
			}
			pidIndex = 4
		case "udp":
			pidIndex = 3
		default:
			continue
		}
		owner, err := strconv.Atoi(fields[pidIndex])
		if err != nil || owner != pid {
			continue
		}
		port := localEndpointPort(fields[1])
		if port == 0 {
			continue
		}
		key := protocol + ":" + strconv.Itoa(port)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		bindings = append(bindings, Binding{Proto: protocol, Port: port})
	}

	return bindings
}

// localEndpointPort 提取本地端点字符串中的端口。
// 职责：兼容 0.0.0.0:8469、127.0.0.1:8469 与 [::]:8469。
// 使用：parseBindings 解析 netstat 行时调用。
// 边界：无冒号、非数字或超出端口范围时返回 0。
func localEndpointPort(endpoint string) int {
	index := strings.LastIndex(endpoint, ":")
	if index < 0 {
		return 0
	}
	port, err := strconv.Atoi(endpoint[index+1:])
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}

	return port
}
