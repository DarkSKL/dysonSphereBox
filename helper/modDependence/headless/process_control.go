package headless

// 文件职责：封装无头服务器环境、进程、网络或设置模块 headless_process_control。
// 使用：由 HeadlessService 在状态刷新、准备环境、启动停止或保存设置时调用。
// 边界：不负责 Mod 市场查询、通用安装器流程或前端页面状态管理。

import (
	"DysonSphereBox/helper/modDependence/headless/contract"
	"DysonSphereBox/helper/modDependence/headless/wincommand"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StartHeadlessEnvironment 启动 Nebula 无头服务器进程。
// 职责：在环境就绪且无头进程未运行时启动 DSPGAME.exe。
// 使用：开服页四宫格“启动”按钮调用，服务层负责记录审计日志。
// 边界：只在检测 ready 后启动，不负责下载 Nebula Mod；已运行时返回 skipped。
// 逻辑流：1. 检测当前环境 => 2. 已运行则返回跳过 => 3. 未就绪则返回阻塞状态 => 4. 执行隐藏窗口启动命令 => 5. 释放进程句柄 => 6. 延迟后重新检测状态。
func StartHeadlessEnvironment(gameDir string) (*contract.HeadlessStartResult, error) {
	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}

	if status.ProcessRunning {
		return &contract.HeadlessStartResult{
			GameDir:          status.GameDir,
			Started:          false,
			Skipped:          true,
			ProcessID:        status.ProcessID,
			Message:          "无头服务器已在运行",
			StartCommandText: status.StartCommandText,
			Status:           status,
		}, nil
	}

	if !status.Ready {
		return &contract.HeadlessStartResult{
			GameDir:          status.GameDir,
			Started:          false,
			Skipped:          true,
			Message:          "开服环境未就绪，暂不启动",
			StartCommandText: status.StartCommandText,
			Status:           status,
		}, nil
	}

	if len(status.StartCommand) == 0 {
		return nil, fmt.Errorf("headless start command is empty")
	}

	command := exec.Command(status.StartCommand[0], status.StartCommand[1:]...)
	command.Dir = status.GameDir
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := command.Start(); err != nil {
		return nil, err
	}

	processID := command.Process.Pid
	if err := command.Process.Release(); err != nil {
		return nil, err
	}

	time.Sleep(1200 * time.Millisecond)
	nextStatus, err := InspectHeadlessEnvironment(status.GameDir)
	if err != nil {
		return nil, err
	}
	if nextStatus.ProcessID == 0 {
		nextStatus.ProcessID = processID
	}

	return &contract.HeadlessStartResult{
		GameDir:          status.GameDir,
		Started:          true,
		Skipped:          false,
		ProcessID:        processID,
		Message:          "无头服务器启动命令已执行",
		StartCommandText: status.StartCommandText,
		Status:           nextStatus,
	}, nil
}

// StopHeadlessEnvironment 停止 Nebula 无头服务器进程。
// 职责：停止当前游戏目录下确认属于 Nebula 无头服务器的 DSPGAME.exe 进程。
// 使用：开服页四宫格“停止”按钮调用，服务层负责记录审计日志。
// 边界：只终止当前游戏目录下带有 -nebula-server 参数的进程，避免误关普通游戏；没有进程时返回 skipped。
// 逻辑流：1. 检测当前环境 => 2. 无进程则返回跳过 => 3. taskkill 终止进程树 => 4. 延迟后重新检测 => 5. 返回是否真正停止。
func StopHeadlessEnvironment(gameDir string) (*contract.HeadlessStopResult, error) {
	status, err := InspectHeadlessEnvironment(gameDir)
	if err != nil {
		return nil, err
	}

	if !status.ProcessRunning || status.ProcessID == 0 {
		return &contract.HeadlessStopResult{
			GameDir:   status.GameDir,
			Stopped:   false,
			Skipped:   true,
			ProcessID: 0,
			HostPort:  status.HostPort,
			Message:   "未找到正在运行的无头服务器",
			Status:    status,
		}, nil
	}

	output, err := wincommand.CombinedOutput("taskkill", "/PID", strconv.Itoa(status.ProcessID), "/T", "/F")
	if err != nil {
		return nil, fmt.Errorf("stop headless process %d: %w: %s", status.ProcessID, err, strings.TrimSpace(string(output)))
	}

	time.Sleep(1000 * time.Millisecond)
	nextStatus, err := InspectHeadlessEnvironment(status.GameDir)
	if err != nil {
		return nil, err
	}

	stopped := !nextStatus.ProcessRunning
	message := "无头服务器已停止"
	if !stopped {
		message = "停止命令已执行，但进程仍在运行"
	}

	return &contract.HeadlessStopResult{
		GameDir:   status.GameDir,
		Stopped:   stopped,
		Skipped:   false,
		ProcessID: status.ProcessID,
		HostPort:  status.HostPort,
		Message:   message,
		Status:    nextStatus,
	}, nil
}
