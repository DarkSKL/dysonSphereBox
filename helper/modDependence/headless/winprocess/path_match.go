package winprocess

// 文件职责：封装 Windows 进程路径与无头启动参数的匹配规则。
// 使用：process.go 在遍历候选进程时调用，用于确认进程属于当前游戏目录。
// 边界：只做字符串级规范化和匹配，不解析符号链接、短路径或真实磁盘路径。

import (
	"path/filepath"
	"strings"
)

// isHeadlessProcessForGameDir 判断进程是否属于指定游戏目录的无头服务器。
// 职责：校验命令行包含无头服务器参数，并匹配可执行路径或命令行路径。
// 使用：FindDysonSphereHeadlessProcess 遍历进程列表时调用。
// 边界：命令行或可执行路径缺失时尽量保守匹配，避免误判普通游戏进程。
func isHeadlessProcessForGameDir(process windowsProcess, gameDir string, processName string, serverArgument string) bool {
	commandLine := strings.ToLower(process.CommandLine)
	if !strings.Contains(commandLine, strings.ToLower(serverArgument)) {
		return false
	}

	gameDir = filepath.Clean(gameDir)
	expectedExe := filepath.Clean(filepath.Join(gameDir, processName))
	if sameWindowsPath(process.ExecutablePath, expectedExe) {
		return true
	}

	normalizedCommandLine := normalizeWindowsPathForCompare(commandLine)
	return strings.Contains(normalizedCommandLine, normalizeWindowsPathForCompare(expectedExe)) ||
		strings.Contains(normalizedCommandLine, normalizeWindowsPathForCompare(gameDir))
}

// sameWindowsPath 比较两个 Windows 路径是否等价。
// 职责：统一大小写、斜杠和引号后比较路径。
// 使用：isHeadlessProcessForGameDir 校验进程可执行文件路径。
// 边界：只做字符串规范化，不解析短路径、符号链接或真实磁盘路径。
func sameWindowsPath(left string, right string) bool {
	return normalizeWindowsPathForCompare(left) == normalizeWindowsPathForCompare(right)
}

// normalizeWindowsPathForCompare 将 Windows 路径转换为可比较文本。
// 职责：去除引号、清理路径、统一反斜杠和小写。
// 使用：进程路径和命令行匹配时调用。
// 边界：空路径会被清理为当前目录语义，调用方需结合上下文判断。
func normalizeWindowsPathForCompare(path string) string {
	normalized := strings.Trim(strings.TrimSpace(path), `"`)
	normalized = filepath.Clean(normalized)
	normalized = strings.ReplaceAll(normalized, "/", `\`)
	return strings.ToLower(normalized)
}
