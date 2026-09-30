package fileops

// 文件职责：提供目录树复制与单文件复制能力。
// 使用：BepInEx 安装和 Mod 包安装器在写入、备份或回滚文件时调用。
// 边界：会创建目标父目录并覆盖目标文件，不记录文件归属、不计算哈希、不发送进度事件。

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

// CopyDirectoryContents 将源目录内的全部内容复制到目标目录。
// 职责：递归复制源目录子项到目标目录，并保留文件权限。
// 使用：BepInEx 安装和旧版 Mod 安装在解压后把目录树写入游戏目录或插件目录。
// 边界：不清理目标中已有的额外文件；调用方必须先确认目标目录安全。
// 逻辑流：1. 遍历源目录 => 2. 跳过根目录自身 => 3. 创建目标目录 => 4. 按原文件权限复制文件。
func CopyDirectoryContents(sourceDir string, destinationDir string) error {
	return filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		destinationPath := filepath.Join(destinationDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o755)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		return CopyFile(sourcePath, destinationPath, info.Mode())
	})
}

// CopyFile 复制单个文件并保留调用方指定的文件权限。
// 职责：提供无取消上下文的单文件复制入口。
// 使用：安装回滚、备份和普通目录复制在无需取消上下文时调用。
// 边界：会覆盖目标文件，不负责计算哈希或记录文件归属。
func CopyFile(sourcePath string, destinationPath string, mode os.FileMode) error {
	return CopyFileWithContext(context.Background(), sourcePath, destinationPath, mode)
}

// CopyFileWithContext 复制单个文件并在读取过程中响应取消上下文。
// 职责：复制单个文件并把上下文取消转换为统一取消错误。
// 使用：新版安装器复制较大文件时调用，允许用户取消后尽快停止写入。
// 边界：取消时返回 ErrInstallCancelled；已经写入的部分文件由上层安装会话清理。
func CopyFileWithContext(ctx context.Context, sourcePath string, destinationPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, newContextReader(ctx, source))
	return err
}
