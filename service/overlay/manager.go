package overlay

// 文件职责：维护主进程对 Overlay 附进程的生命周期管理。
// 使用：OverlayService 调用 Manager 启动、停止和查询同一个 exe 的 --overlay 模式。
// 边界：这里只管理进程生命周期，不控制附进程内部窗口贴附循环和 Vue 渲染状态。

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
)

// Manager 管理 Overlay 附进程的启动、停止与状态刷新。
// 使用：主进程内作为全局单例供 OverlayService 调用。
// 边界：不跨程序实例共享状态；主程序重启后需要重新启动 Overlay。
type Manager struct {
	// 职责：保护 cmd 与 pid 状态。
	// 使用：Start、Stop、Status 并发调用时加锁。
	// 边界：不用于跨进程锁。
	mu sync.Mutex
	// 职责：保存当前启动的 Overlay 命令对象。
	// 使用：Stop 终止附进程，Status 检查退出状态。
	// 边界：进程退出后会被清空。
	cmd *exec.Cmd
}

// OverlayManager 是主进程 Overlay 生命周期管理单例。
// 使用：OverlayService 和 App.shutdown 调用。
// 边界：只在当前主进程内有效。
var OverlayManager = &Manager{}

// Start 启动 Overlay 附进程。
// 使用：主界面点击“启动悬浮窗案例”时调用。
// 边界：已运行时直接返回当前状态；启动失败返回错误给前端。
func (manager *Manager) Start() (OverlayProcessStatusDTO, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.refreshLocked()
	if manager.cmd != nil && manager.cmd.Process != nil {
		return manager.statusLocked(), nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return OverlayProcessStatusDTO{}, fmt.Errorf("resolve executable path: %w", err)
	}
	cmd := exec.Command(exePath, "--overlay")
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	if err := cmd.Start(); err != nil {
		return OverlayProcessStatusDTO{}, fmt.Errorf("start overlay process: %w", err)
	}
	manager.cmd = cmd
	go manager.waitForExit(cmd)
	return manager.statusLocked(), nil
}

// Stop 停止 Overlay 附进程。
// 使用：主界面点击“停止悬浮窗案例”或主进程退出时调用。
// 边界：未运行时返回空状态；终止失败返回错误但会尽量刷新状态。
func (manager *Manager) Stop() (OverlayProcessStatusDTO, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.refreshLocked()
	if manager.cmd == nil || manager.cmd.Process == nil {
		return manager.statusLocked(), nil
	}
	err := manager.cmd.Process.Kill()
	manager.cmd = nil
	if err != nil {
		return manager.statusLocked(), fmt.Errorf("stop overlay process: %w", err)
	}
	return manager.statusLocked(), nil
}

// Status 返回 Overlay 附进程状态。
// 使用：前端轮询或页面加载时展示当前运行状态。
// 边界：状态来自主进程记录；该方法会尝试刷新已经退出的子进程。
func (manager *Manager) Status() OverlayProcessStatusDTO {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.refreshLocked()
	return manager.statusLocked()
}

// 刷新已退出的附进程状态。
// 职责：用 ProcessState 判断子进程是否已退出；使用：状态查询和启停前调用；边界：不阻塞等待正在运行的进程。
func (manager *Manager) refreshLocked() {
	if manager.cmd == nil {
		return
	}
	if manager.cmd.ProcessState != nil && manager.cmd.ProcessState.Exited() {
		manager.cmd = nil
		return
	}
}

// 等待附进程退出并清理主进程记录。
// 职责：后台回收子进程并避免 Status 长期显示已退出进程；使用：Start 成功后启动；边界：只清理当前仍匹配的 cmd，避免误删新启动实例。
func (manager *Manager) waitForExit(cmd *exec.Cmd) {
	_ = cmd.Wait()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.cmd == cmd {
		manager.cmd = nil
	}
}

// 构造当前状态 DTO。
// 职责：把命令对象转换为前端状态；使用：Start、Stop、Status 返回；边界：不主动检查系统进程表。
func (manager *Manager) statusLocked() OverlayProcessStatusDTO {
	if manager.cmd == nil || manager.cmd.Process == nil {
		return OverlayProcessStatusDTO{Running: false}
	}
	return OverlayProcessStatusDTO{
		Running: true,
		PID:     manager.cmd.Process.Pid,
	}
}
