package headless

// 文件职责：汇总无头服务器环境状态中的阻塞问题。
// 使用：InspectHeadlessEnvironment 和未配置状态构造流程生成前端可读 Issues。
// 边界：只表达本地可检测问题，不诊断联机协议、存档或远端连接质量。

import (
	"fmt"
	"strings"

	"DysonSphereBox/helper/modDependence/headless/contract"
)

// unconfiguredHeadlessIssues 构造未配置游戏目录时的问题列表。
// 职责：为设置页引导用户选择 DSPGAME.exe 提供基础问题文案。
// 使用：BuildUnconfiguredHeadlessEnvironmentStatus 调用。
// 边界：只处理未配置状态，不访问游戏目录。
func unconfiguredHeadlessIssues(reason string) []string {
	issues := []string{"未检测到 DSPGAME.exe，请先在设置页选择《戴森球计划》游戏目录。"}
	if strings.TrimSpace(reason) != "" {
		issues = append(issues, reason)
	}
	return issues
}

// collectHeadlessIssues 汇总当前开服环境的阻塞问题。
// 职责：根据 BepInEx、Nebula、steam_appid 和端口状态生成问题列表。
// 使用：InspectHeadlessEnvironment 设置 Status.Issues 和 Ready 时调用。
// 边界：只表达本地可检测问题，不诊断联机协议、存档或远端连接质量。
func collectHeadlessIssues(status contract.HeadlessEnvironmentStatus) []string {
	issues := make([]string, 0)
	if !status.BepInExStatus.Installed {
		issues = append(issues, "BepInEx 前置框架未就绪")
	}
	if !status.NebulaReady {
		issues = append(issues, "Nebula 无头服务器依赖未安装完整")
	}
	if !status.SteamAppIDValid {
		issues = append(issues, "steam_appid.txt 缺失或内容不是 1366540")
	}
	if _, err := normalizeHeadlessArchive(status.ServerArchive); err != nil {
		issues = append(issues, err.Error())
	}
	if status.ProcessRunning && !status.PortListening {
		issues = append(issues, fmt.Sprintf("DSPGAME 进程运行中，但端口 %d 未监听", status.HostPort))
	}
	if !status.ProcessRunning && status.PortListening {
		issues = append(issues, fmt.Sprintf("端口 %d 已被其它进程占用", status.HostPort))
	}

	return issues
}
