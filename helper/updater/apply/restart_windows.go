package apply

// 文件职责：启动隐藏的 Windows 更新进程并确认它已接管退出等待。
// 使用：系统更新服务在 Wails 关闭前调用 StartAfterExit。
// 边界：不退出主程序；交接失败时终止助手并返回错误，当前程序继续运行。

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// restartScript 职责：保存退出后安装、清理与重启脚本；使用：通过编码参数交给系统 PowerShell；边界：脚本只操作本轮更新的明确路径。
//
//go:embed restart.ps1
var restartScript string

// StartAfterExit 职责：交接更新并等待助手确认已取得旧进程句柄；使用：关闭钩子调用；边界：成功只表示等待已就绪，旧进程未退出时不会替换文件。
// 逻辑流：1. 检查暂存文件 => 2. 编码本地路径并隐藏启动助手 => 3. 等待就绪或错误 => 4. 超时终止助手，交由界面保留当前应用。
func (prepared *PreparedUpdate) StartAfterExit(parentPID int) error {
	staged := filepath.Join(prepared.directory, "update.exe")
	if _, err := os.Stat(staged); err != nil {
		return fmt.Errorf("更新暂存文件不可用: %w", err)
	}
	ready := filepath.Join(prepared.directory, "ready")
	_ = os.Remove(ready)
	plan, err := json.Marshal(map[string]interface{}{
		"target": prepared.target, "directory": prepared.directory, "parentPID": parentPID,
	})
	if err != nil {
		return err
	}
	script := "$plan = ConvertFrom-Json '" + strings.ReplaceAll(string(plan), "'", "''") + "'\n" + restartScript
	encoded := utf16.Encode([]rune(script))
	data := make([]byte, len(encoded)*2)
	for index, value := range encoded {
		binary.LittleEndian.PutUint16(data[index*2:], value)
	}
	command := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	var output bytes.Buffer
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		return fmt.Errorf("启动更新助手: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("更新助手未完成交接: %v %s", err, strings.TrimSpace(output.String()))
		case <-ticker.C:
			if _, err := os.Stat(ready); err == nil {
				return nil
			}
		case <-timeout.C:
			_ = command.Process.Kill()
			<-done
			return fmt.Errorf("更新助手启动超时，应用尚未退出，请重试: %s", strings.TrimSpace(output.String()))
		}
	}
}
