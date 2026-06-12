package buffer

// 文件职责：解析 V2 安装器下载、解压、备份和回滚所需的缓存根目录。
// 使用：v2 主安装流程、复制回滚和中断清理流程在需要缓存路径时调用 Dir。
// 边界：只返回路径不创建目录；不下载、不解压、不复制文件，也不写数据库。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"fmt"
	"os"
	"path/filepath"
)

// Dir 返回 V2 安装器缓存根目录。
// 职责：基于当前可执行文件目录构造下载、解压、备份和回滚缓存路径。
// 使用：远端安装入口、复制回滚和中断清理需要缓存根目录时调用。
// 边界：只返回路径不创建目录；无法解析可执行文件路径时返回错误。
func Dir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), fileops.BufferRootDirName, "modDependenciesV2"), nil
}
