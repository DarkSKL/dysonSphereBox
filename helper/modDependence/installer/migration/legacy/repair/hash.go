package repair

// 文件职责：计算历史迁移前文件的 SHA256 和大小。
// 使用：RepairScopedFile 移动文件前调用，用于确认磁盘文件仍是数据库记录中的受管理文件。
// 边界：只读取文件内容，不修改文件；打开或读取失败时返回错误。

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// fileSHA256 计算文件 SHA256 与大小。
// 职责：迁移前确认当前文件仍是数据库记录中的受管理文件。
// 使用：RepairScopedFile 移动文件前调用。
// 边界：只读取文件内容，不修改文件；打开或读取失败时返回错误。
func fileSHA256(path string) (string, int64, error) {
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
