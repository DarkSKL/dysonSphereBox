package fileops

// 文件职责：提供 zip 缓存校验与单条目安全解压能力。
// 使用：BepInEx 和 Mod 安装器在复用下载缓存、遍历 zip 包并写入目标目录时调用。
// 边界：不负责打开完整安装流程、不发送进度事件、不校验包内容是否符合业务安装规范。

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// IsValidZipFile 判断路径是否指向可打开的 zip 文件。
// 职责：快速验证本地文件是否具备基本 zip 结构。
// 使用：下载安装缓存复用前调用，避免使用损坏的 partial 或旧缓存文件。
// 边界：只验证 zip 结构可打开，不校验包内容是否符合 BepInEx 或 Mod 安装规范。
func IsValidZipFile(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()

	return true
}

// ExtractZipFile 将单个 zip 条目安全解压到目标目录。
// 职责：把单个 zip 条目写入目标目录并防止路径穿越。
// 使用：BepInEx 和 Mod 安装器遍历 zip 包时逐条调用，统一处理路径穿越与目录项创建。
// 边界：只处理单个条目，不负责打开或关闭 zip 读取器，也不发送进度事件。
// 逻辑流：1. 归一化条目名 => 2. 拼接并校验目标路径 => 3. 目录项创建目录 => 4. 文件项复制字节到目标文件。
func ExtractZipFile(file *zip.File, destinationDir string) error {
	entryName := NormalizeZipEntryName(file.Name)
	if entryName == "" {
		return nil
	}
	targetPath := filepath.Join(destinationDir, filepath.FromSlash(strings.TrimSuffix(entryName, "/")))
	if !IsSubPath(destinationDir, targetPath) {
		return fmt.Errorf("zip entry escapes destination: %s", file.Name)
	}

	if IsZipDirectoryEntry(file, entryName) {
		if err := RemoveBlockingFile(targetPath); err != nil {
			return err
		}
		return os.MkdirAll(targetPath, file.Mode())
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

// NormalizeZipEntryName 清洗 zip 条目名中的分隔符和绝对路径前缀。
// 职责：把 zip 条目名归一化为后续路径拼接可用的相对形式。
// 使用：ExtractZipFile 写入目标路径前调用，保证后续路径拼接使用相对条目。
// 边界：不解析 .. 片段，路径越界由 IsSubPath 在最终目标路径上判断。
func NormalizeZipEntryName(entryName string) string {
	entryName = strings.ReplaceAll(entryName, "\\", "/")
	entryName = strings.TrimLeft(entryName, "/")
	return entryName
}

// IsZipDirectoryEntry 判断 zip 条目是否表示目录。
// 职责：统一识别 zip 条目中的目录项表达。
// 使用：ExtractZipFile 区分目录创建和文件写入流程。
// 边界：兼容 FileInfo 与名称尾部斜杠两种目录表达，不校验目录内容。
func IsZipDirectoryEntry(file *zip.File, entryName string) bool {
	return file.FileInfo().IsDir() || strings.HasSuffix(entryName, "/")
}
