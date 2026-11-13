//go:build linux || darwin

package r2modman

// 文件职责：为非 Windows 回归环境提供 Profile 操作文件锁。
// 使用：辅助包在 Linux/macOS 测试时调用；边界：与 Windows 一样只约束盒子自身，不覆盖外部管理器。

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// 职责：取得跨进程排他锁；使用：修改和恢复入口；边界：失败立即返回，进程退出自动释放锁。
func acquireProfileLock(root string) (func(), error) {
	path, err := safeProfilePath(root, "_state/dyson-sphere-box/operation.lock")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("另一盒子操作正在管理此 Profile: %w", err)
	}
	return func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }, nil
}
