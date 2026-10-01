package environment

// 文件职责：判断环境资源库 Mod 目录是否与模组市场安装器托管的插件目录重名。
// 使用：EnvironmentManagerService 在列表、首次纳管、启用、禁用和删除 Mod 前调用。
// 边界：只做目录名比较和只读统计，不查询数据库、不修改文件；托管目录集合由服务层从安装记录读取后传入。

import (
	"os"
	"path/filepath"
	"strings"
)

// ModLibraryDirName 从资源库相对路径中提取 Mod 目录名。
// 职责：把 plugins/<dir> 形式的相对路径转换为一级目录名。
// 使用：服务层把资源库目录名与安装器托管目录集合比较。
// 边界：空路径返回空字符串；不校验目录是否存在。
func ModLibraryDirName(libraryPath string) string {
	libraryPath = strings.TrimSpace(libraryPath)
	if libraryPath == "" {
		return ""
	}
	return filepath.Base(filepath.FromSlash(libraryPath))
}

// IsInstallerManagedDir 判断目录名是否属于安装器托管目录。
// 职责：按 Windows 大小写不敏感规则比较目录名。
// 使用：服务层判断 Mod 能否由环境管理写入或删除游戏侧文件。
// 边界：集合为空或目录名为空时返回 false。
func IsInstallerManagedDir(managedDirs map[string]struct{}, dirName string) bool {
	dirName = strings.ToLower(strings.TrimSpace(dirName))
	if dirName == "" || len(managedDirs) == 0 {
		return false
	}
	_, ok := managedDirs[dirName]
	return ok
}

// CountUnmanagedFirstLevelDirs 统计游戏侧未被安装器托管的一级 Mod 目录数量。
// 职责：为首次同步预览统计可由环境管理纳管的 Mod 目录。
// 使用：PreviewInitialSync 和概览构建时调用。
// 边界：目录不存在返回 0；普通文件和安装器托管目录不计入。
func CountUnmanagedFirstLevelDirs(root string, managedDirs map[string]struct{}) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && !IsInstallerManagedDir(managedDirs, entry.Name()) {
			count++
		}
	}
	return count
}
