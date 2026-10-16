package command

// 文件职责：构造无头服务器启动命令参数。
// 使用：headless 环境检测返回 StartCommand，启动流程执行 DSPGAME.exe 时调用 BuildStartCommand。
// 边界：不校验可执行文件存在性、不读取存档文件、不启动进程。

import (
	"path/filepath"
	"strings"
)

// BuildStartCommand 构造 DSPGAME.exe 无头服务器启动命令。
// 职责：组合游戏可执行文件、批处理参数、Unity 日志路径、Nebula 参数和存档参数。
// 使用：headless.InspectHeadlessEnvironment 生成状态和 StartHeadlessEnvironment 执行进程前调用。
// 边界：不校验可执行文件存在性；存档为空时使用 defaultSaveMode。
func BuildStartCommand(gameDir string, processName string, serverArgument string, defaultSaveMode string, serverArchive string) []string {
	startCommand := []string{
		filepath.Join(gameDir, processName),
		"-batchmode",
		"-nographics",
		// 显式指定控制台读取的日志文件；路径作为独立参数交给 exec.Command 处理空格。
		"-logFile",
		filepath.Join(gameDir, "output_log.txt"),
		serverArgument,
	}
	normalizedArchive := strings.TrimSpace(serverArchive)
	if normalizedArchive == "" {
		return append(startCommand, defaultSaveMode)
	}

	return append(startCommand, "-load", normalizedArchive)
}
