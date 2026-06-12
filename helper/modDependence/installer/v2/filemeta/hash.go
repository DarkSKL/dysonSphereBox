package filemeta

// 文件职责：提供 V2 安装器文件元信息读取能力。
// 使用：安装规划和冲突检测在需要计算文件哈希与大小时调用 SHA256。
// 边界：只读取文件内容，不修改文件、不访问数据库、不决定冲突处理策略。

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// SHA256 计算指定文件的 SHA256 和大小。
// 职责：为安装记录、冲突检测和复制计划生成稳定文件指纹。
// 使用：规划 artifact 文件和检查目标文件冲突时调用。
// 边界：只读取文件内容，不修改文件；打开或读取失败时返回错误。
func SHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
