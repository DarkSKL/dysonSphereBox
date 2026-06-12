package fileops

// 文件职责：提供文件操作前的路径边界判断与目录冲突清理。
// 使用：安装器删除、解压、清理或迁移文件前调用，避免路径穿越和目录创建冲突。
// 边界：只做路径关系判断和同名普通文件删除，不决定目标路径是否属于某个安装策略。

import (
	"os"
	"path/filepath"
	"strings"
)

// RemoveBlockingFile 删除会阻塞目录创建的同名普通文件。
// 职责：在需要创建目录前清除同路径普通文件，避免目录创建失败。
// 使用：解压 zip 目录项前调用，确保目标路径可以安全创建目录。
// 边界：目录会被保留，路径不存在视为成功，不负责校验目标是否位于允许根目录内。
func RemoveBlockingFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil
	}
	return os.Remove(path)
}

// IsSubPath 判断目标路径是否位于指定根目录内。
// 职责：校验目标路径是否仍处于允许的根目录范围内。
// 使用：安装器删除、解压、清理或迁移文件前调用，避免路径穿越写出游戏目录或缓存目录。
// 边界：只做路径关系判断，不检查文件是否真实存在。
func IsSubPath(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)

	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return relativePath == "." || (!strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) && relativePath != "..")
}
