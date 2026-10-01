package uninstall

// 文件职责：清理 V2 安装器删除文件后遗留的空目录。
// 使用：卸载事务完成或会话回滚删除文件后调用 CleanupEmptyManagedDirs 与 CleanupEmptyInstallParentDirs。
// 边界：只删除空目录，不删除 root 本身、不删除包含文件的目录、不处理数据库记录。

import (
	"os"
	"path/filepath"
	"strings"
)

// CleanupEmptyManagedDirs 清理 BepInEx 下安装器可能留下的空目录。
// 职责：递归清理 plugins、patchers、config、core、monomod 下的空子目录。
// 使用：卸载事务完成后调用。
// 边界：不会删除 root 本身，也不会删除包含文件的目录。
func CleanupEmptyManagedDirs(root string) {
	for _, child := range []string{"plugins", "patchers", "config", "core", "monomod"} {
		cleanupEmptyDirs(filepath.Join(root, child), filepath.Join(root, child))
	}
}

// CleanupEmptyInstallParentDirs 从文件父目录向上清理空安装目录。
// 职责：复制冲突回滚或会话清理后移除空的组件目录。
// 使用：文件删除或回滚后按单个文件路径调用。
// 边界：遇到 plugins、patchers、config、BepInEx 或非空目录即停止。
func CleanupEmptyInstallParentDirs(filePath string) {
	if strings.TrimSpace(filePath) == "" {
		return
	}
	dir := filepath.Dir(filePath)
	for {
		base := strings.ToLower(filepath.Base(dir))
		if base == "plugins" || base == "patchers" || base == "config" || base == "core" || base == "monomod" || base == "bepinex" || dir == filepath.Dir(dir) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

// cleanupEmptyDirs 递归判断并清理空子目录。
// 职责：为 CleanupEmptyManagedDirs 提供深度优先目录空值判断。
// 使用：只在安装器管理目录下调用。
// 边界：root 自身永远不返回可删除，读取失败时按非空处理。
func cleanupEmptyDirs(root string, current string) bool {
	entries, err := os.ReadDir(current)
	if err != nil {
		return false
	}
	empty := true
	for _, entry := range entries {
		if !entry.IsDir() {
			empty = false
			continue
		}
		childPath := filepath.Join(current, entry.Name())
		if cleanupEmptyDirs(root, childPath) {
			_ = os.Remove(childPath)
		} else {
			empty = false
		}
	}
	return empty && current != root
}
