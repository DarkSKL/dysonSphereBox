package apply

// 文件职责：保存已经校验、解压的更新文件，交给退出后的安装进程。
// 使用：PrepareRelease 调用 prepareExecutable；系统更新服务持有 PreparedUpdate。
// 边界：准备阶段不替换当前程序；失败只清理本次创建的暂存文件。

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// PreparedUpdate 职责：描述本轮暂存文件与替换目标；使用：服务退出时交接；边界：路径只由本地准备流程生成，不接受前端路径。
type PreparedUpdate struct {
	// target 职责：保存实际程序路径；使用：退出后替换与重启；边界：准备阶段只读取。
	target string
	// directory 职责：保存本轮独占暂存目录；使用：安装进程清理自身文件；边界：不包含用户资源。
	directory string
}

// prepareExecutable 职责：把已校验的可执行文件写入同盘暂存目录；使用：PrepareRelease 解压后调用；边界：写入、关闭或空文件错误均保留当前程序。
// 逻辑流：1. 检查现有程序 => 2. 创建独占目录并写入新版 => 3. 关闭文件 => 4. 返回交接信息，失败清理本轮文件。
func prepareExecutable(reader io.Reader, executablePath string) (*PreparedUpdate, error) {
	target, err := filepath.Abs(executablePath)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, fmt.Errorf("读取当前程序: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("当前程序不是普通文件: %s", target)
	}
	directory, err := os.MkdirTemp(filepath.Dir(target), ".dsbox-update-")
	if err != nil {
		return nil, fmt.Errorf("创建更新暂存目录: %w", err)
	}
	stagedPath := filepath.Join(directory, "update.exe")
	file, err := os.OpenFile(stagedPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		_ = os.Remove(directory)
		return nil, err
	}
	size, writeErr := io.Copy(file, reader)
	closeErr := file.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil && size == 0 {
		writeErr = fmt.Errorf("解压后的更新程序为空")
	}
	if writeErr != nil {
		_ = os.Remove(stagedPath)
		_ = os.Remove(directory)
		return nil, fmt.Errorf("准备更新程序: %w", writeErr)
	}
	return &PreparedUpdate{target: target, directory: directory}, nil
}

// Discard 职责：取消未交接的更新并清理本轮暂存；使用：助手启动失败后由服务调用；边界：不删除当前程序或用户文件，已交接后不得调用。
func (prepared *PreparedUpdate) Discard() error {
	for _, name := range []string{"update.exe", "ready"} {
		if err := os.Remove(filepath.Join(prepared.directory, name)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理更新暂存: %w", err)
		}
	}
	if err := os.Remove(prepared.directory); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理更新目录: %w", err)
	}
	return nil
}
