package wincommand

// 文件职责：封装 Windows 命令的隐藏窗口执行与命令行格式化能力。
// 使用：headless、联机链路和游戏端口探测等需要调用系统命令的 helper。
// 边界：只负责隐藏窗口执行命令与格式化展示命令，不解析具体命令输出、不决定业务状态。

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// Output 在后台执行命令并返回 stdout。
// 职责：隐藏 Windows 控制台窗口并读取标准输出。
// 使用：进程、端口和网络状态检测命令调用。
// 边界：只返回 stdout；需要 stderr 诊断时使用 CombinedOutput。
func Output(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Output()
}

// OutputContext 在后台执行命令并返回 stdout，ctx 结束时终止进程。
// 职责：隐藏 Windows 控制台窗口并读取标准输出，支持超时。
// 使用：联机防火墙检测等不能无限等待的检测命令调用。
// 边界：只返回 stdout；超时返回 ctx 错误，由调用方决定兜底结果。
func OutputContext(ctx context.Context, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.Output()
}

// CombinedOutput 在后台执行命令并返回 stdout/stderr 合并输出。
// 职责：隐藏 Windows 控制台窗口并保留诊断输出。
// 使用：停止进程失败时保留 taskkill 输出用于错误信息。
// 边界：只封装命令执行，不解释命令输出。
func CombinedOutput(name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return command.CombinedOutput()
}

// Quote 将启动命令切片格式化为可读文本。
// 职责：为包含空格的参数加引号并拼接为命令行字符串。
// 使用：Headless 环境状态生成 StartCommandText 时调用。
// 边界：仅用于展示，不用于再次解析执行命令。
func Quote(parts []string) string {
	quotedParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.ContainsAny(part, " \t") {
			quotedParts = append(quotedParts, strconv.Quote(part))
			continue
		}
		quotedParts = append(quotedParts, part)
	}

	return strings.Join(quotedParts, " ")
}
