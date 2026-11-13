package r2modman

// 文件职责：在 Windows 上独占锁定单个 Profile 的盒子操作。
// 使用：写入和恢复持有锁直到完成；边界：锁不约束 r2modman，仍需清单快照复核，进程退出由系统释放。

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// 职责：取得跨进程文件锁并返回释放函数；使用：Profile 修改入口；边界：其他盒子实例持锁时立即报错，不删除持久锁文件。
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
	state := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, state); err != nil {
		file.Close()
		return nil, fmt.Errorf("另一盒子操作正在管理此 Profile: %w", err)
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, state); _ = file.Close() }, nil
}
