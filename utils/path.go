package utils

// 文件职责：提供跨业务复用的路径文本清理、路径候选去重和文件存在判断工具。
// 使用：由 Steam 探测、快捷方式解析、游戏目录校验和安装 helper 在处理本地路径前调用。
// 边界：只做通用路径处理，不导入业务包、不访问数据库、不决定某个路径是否属于具体业务资源。

import (
	"os"
	"path/filepath"
	"strings"
)

// CleanPath 清理用户输入、环境变量和文件系统返回的路径字符串。
// 职责：统一路径文本的空白、引号、环境变量和分隔符规范化规则。
// 使用：路径候选收集、快捷方式解析和游戏目录校验前调用。
// 边界：空路径返回空字符串；非空路径会展开环境变量、去除引号并执行 filepath.Clean。
func CleanPath(path string) string {
	path = strings.Trim(strings.TrimSpace(os.ExpandEnv(path)), `"`)
	if path == "" {
		return ""
	}

	return filepath.Clean(path)
}

// AppendUniquePath 将规范化后的路径追加到列表且保持大小写不敏感去重。
// 职责：维护路径候选列表的规范化追加和去重规则。
// 使用：Steam、快捷方式和安装目录候选收集流程通过 seen 共享去重状态。
// 边界：空路径和重复路径会被跳过；不会检查路径是否真实存在。
func AppendUniquePath(paths *[]string, seen map[string]struct{}, path string) {
	path = CleanPath(path)
	if path == "" {
		return
	}

	key := strings.ToLower(path)
	if _, ok := seen[key]; ok {
		return
	}

	seen[key] = struct{}{}
	*paths = append(*paths, path)
}

// FileExists 判断指定路径是否存在且不是目录。
// 职责：提供通用文件存在性判断。
// 使用：游戏 exe 校验、安装文件校验和其它 helper 的轻量文件判断场景调用。
// 边界：无法访问、路径为空或目录路径都返回 false；不返回具体错误原因。
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
