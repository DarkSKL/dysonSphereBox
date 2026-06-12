package plan

// 文件职责：提供历史迁移计划使用的 Windows 文件系统路径比较与 BepInEx 根解析。
// 使用：计划构建和排序时调用，用于确保目标路径在游戏目录内并避免重复迁移。
// 边界：只做字符串级路径清理和大小写无关比较，不解析符号链接或真实磁盘路径。

import (
	"path/filepath"
	"strings"
)

// resolveBepInExRootFromTarget 从目标文件路径中解析 BepInEx 目录。
// 职责：定位游戏目录下的 BepInEx 根，用于拼接修复后的相对路径。
// 使用：构造迁移计划时调用。
// 边界：路径中找不到 BepInEx 片段时返回空字符串。
func resolveBepInExRootFromTarget(targetPath string) string {
	cleanPath := filepath.Clean(targetPath)
	lowerPath := strings.ToLower(cleanPath)
	marker := string(filepath.Separator) + "bepinex"
	index := strings.LastIndex(lowerPath, marker)
	if index < 0 {
		if strings.EqualFold(cleanPath, "BepInEx") || strings.HasSuffix(lowerPath, marker) {
			return cleanPath
		}
		return ""
	}
	return cleanPath[:index+len(marker)]
}

// sameFilesystemPath 判断两个文件系统路径是否等价。
// 职责：统一清理路径并进行大小写无关比较。
// 使用：构造计划、排序和执行迁移时避免重复移动。
// 边界：按 Windows 语义比较大小写，不解析符号链接。
func sameFilesystemPath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
