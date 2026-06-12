package artifactlayout

// 文件职责：提供 artifact 包内路径归一化与根目录识别能力。
// 使用：安装规划、历史迁移和目录前缀判断在解析包内路径时调用。
// 边界：只处理字符串路径，不校验最终目标是否越界、不访问文件系统。

import (
	"path"
	"strings"
)

// NormalizeEntryPath 将 zip 或本地 artifact 内路径归一化为 slash 分隔的相对路径。
// 职责：统一 Windows 反斜杠、清理点路径并去除首尾斜杠。
// 使用：布局判断、迁移修复和安装布局校验前调用。
// 边界：不会检查路径是否越界；调用方仍需拒绝绝对路径或 .. 越界结果。
func NormalizeEntryPath(relativePath string) string {
	normalizedPath := strings.ReplaceAll(relativePath, "\\", "/")
	normalizedPath = path.Clean(normalizedPath)
	if normalizedPath == "." {
		return ""
	}
	return strings.Trim(normalizedPath, "/")
}

// SplitInstallRoot 识别 artifact 内路径是否声明了 BepInEx 目标根目录。
// 职责：判断 BepInEx 前缀、plugins、patchers、config 等特殊根。
// 使用：安装规划和历史路径迁移复用该判断。
// 边界：带 BepInEx 前缀时视为包作者显式声明完整目标路径，不再额外改写目录。
func SplitInstallRoot(relativePath string) (root string, rest string, hasRoot bool, fromBepInEx bool) {
	normalizedPath := NormalizeEntryPath(relativePath)
	lowerPath := strings.ToLower(normalizedPath)
	if strings.HasPrefix(lowerPath, "bepinex/") {
		root, rest = SplitFirstPathSegment(normalizedPath[len("bepinex/"):])
		return strings.ToLower(root), rest, true, true
	}

	root, rest = SplitFirstPathSegment(normalizedPath)
	switch strings.ToLower(root) {
	case "plugins", "patchers", "config":
		return strings.ToLower(root), rest, true, false
	default:
		return "", normalizedPath, false, false
	}
}

// SplitFirstPathSegment 拆分归一化路径的首段和剩余部分。
// 职责：为根目录识别和目录前缀判断提供稳定拆分。
// 使用：SplitInstallRoot 与 HasArtifactDirectoryPrefix 调用。
// 边界：空路径返回两个空字符串，不返回错误。
func SplitFirstPathSegment(relativePath string) (string, string) {
	relativePath = NormalizeEntryPath(relativePath)
	if relativePath == "" {
		return "", ""
	}
	segments := strings.SplitN(relativePath, "/", 2)
	if len(segments) == 1 {
		return segments[0], ""
	}
	return segments[0], segments[1]
}
