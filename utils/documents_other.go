//go:build !windows

package utils

// 文件职责：提供非 Windows 构建的文档目录候选。
// 使用：跨平台编译和路径测试；边界：不声称具有 Windows 已知文件夹语义。

import (
	"os"
	"path/filepath"
)

// DocumentsDirectory 职责：组合用户目录中的 Documents；使用：非 Windows 路径候选；边界：调用方仍需检查存在性。
func DocumentsDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents"), nil
}
