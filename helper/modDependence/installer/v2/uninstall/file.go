package uninstall

// 文件职责：删除卸载计划中的受管文件。
// 使用：卸载计划确认目标文件未被共享后调用 RemoveManagedFile。
// 边界：只删除单个文件，不删除目录、不校验数据库状态、不记录审计日志。

import "os"

// RemoveManagedFile 删除安装器管理的单个文件。
// 职责：在文件存在时执行删除，并把不存在视为已清理。
// 使用：卸载计划确认文件未共享后调用。
// 边界：不会删除目录；路径为空或文件不存在时返回 nil。
func RemoveManagedFile(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Remove(path)
}
