package installpath

// 文件职责：提供 V2 安装目标路径的比较与 SQLite 查询候选值生成。
// 使用：冲突检测、文件清单写入、卸载和会话清理在比较 target_path 时调用。
// 边界：只处理路径字符串归一化，不访问文件系统、不校验路径越界、不读写数据库。

import (
	"path/filepath"
	"strings"
)

// Equal 判断两个安装目标路径是否指向同一条逻辑路径。
// 职责：统一 Windows 大小写不敏感和正反斜杠差异。
// 使用：冲突决策匹配和同一目标判断时调用。
// 边界：只做字符串级比较，不解析符号链接或真实文件系统路径。
func Equal(left string, right string) bool {
	return NormalizeForCompare(left) == NormalizeForCompare(right)
}

// NormalizeForCompare 将安装目标路径转成内存比较格式。
// 职责：把路径 clean 后转为 slash 并统一小写。
// 使用：需要判断前缀、相等或历史记录匹配时调用。
// 边界：不保证路径存在，也不判断路径是否位于游戏目录内。
func NormalizeForCompare(path string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
}

// ValuesForQuery 返回兼容历史路径写法的数据库匹配值。
// 职责：生成 clean、slash、backslash 三种小写候选，覆盖旧记录里的正反斜杠差异。
// 使用：SQLite 查询 lower(target_path) IN ? 时调用。
// 边界：SQLite 不理解 Windows 路径语义，因此本函数只提供候选值，不做 SQL 查询。
func ValuesForQuery(path string) []string {
	cleaned := strings.ToLower(filepath.Clean(path))
	slash := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	backslash := strings.ReplaceAll(slash, "/", "\\")
	values := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, value := range []string{cleaned, slash, backslash} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}
