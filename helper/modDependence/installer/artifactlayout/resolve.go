package artifactlayout

// 文件职责：把 artifact 包内相对路径解析为游戏目录下的目标相对路径。
// 使用：installer 父包在安装规划时调用 ResolveTargetRelativePath。
// 边界：只处理路径归一化与相对路径拼接，不访问数据库、不读写文件、不决定安装冲突策略。

import "path/filepath"

// ResolveTargetRelativePath 将包内文件路径解析为游戏目录下的目标相对路径。
// 职责：兼容 BepInEx、plugins、patchers、config 以及普通裸文件布局。
// 使用：安装规划遍历解压文件时调用。
// 边界：返回的是相对路径；调用方仍需与游戏目录拼接并做越界校验。
func ResolveTargetRelativePath(relativePath string, artifact ArtifactDescriptor) string {
	normalizedRelativePath := NormalizeEntryPath(relativePath)
	root, rest, hasRoot, fromBepInEx := SplitInstallRoot(normalizedRelativePath)
	if hasRoot {
		if fromBepInEx {
			return filepath.FromSlash(normalizedRelativePath)
		}
		switch root {
		case "plugins", "patchers":
			return ResolveScopedBepInExRelativePath(root, rest, artifact)
		case "config":
			return filepath.Join("BepInEx", "config", filepath.FromSlash(rest))
		}
	}

	return filepath.Join("BepInEx", "plugins", InstallDirName(artifact), filepath.FromSlash(normalizedRelativePath))
}

// ResolveScopedBepInExRelativePath 将 plugins 或 patchers 裸路径归入组件目录。
// 职责：为未显式带 BepInEx 前缀的 plugins/patchers 条目补上组件作用域。
// 使用：安装规划和历史迁移修复裸露 DLL 时调用。
// 边界：若 rest 已带组件目录前缀则保持原结构，避免重复包一层目录。
func ResolveScopedBepInExRelativePath(root string, rest string, artifact ArtifactDescriptor) string {
	installDir := InstallDirName(artifact)
	if rest == "" {
		return filepath.Join("BepInEx", root, installDir)
	}
	if HasArtifactDirectoryPrefix(rest, artifact) {
		return filepath.Join("BepInEx", root, filepath.FromSlash(rest))
	}
	return filepath.Join("BepInEx", root, installDir, filepath.FromSlash(rest))
}
