package remote

// 文件职责：维护 BepInEx 安装缓存目录的解析规则。
// 使用：InstallBepInEx 下载或解压 Thunderstore BepInEx 包前调用，用于定位可复用缓存目录。
// 边界：只返回当前可执行文件旁的缓存路径，不负责创建目录、清理缓存或决定是否复用下载文件。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// BufferDirName 职责：定义 BepInEx 安装缓存子目录名。
	// 使用：BufferDir 拼接下载和解压缓存路径时使用。
	// 边界：该常量只描述缓存目录，不代表游戏目录中的 BepInEx 文件夹。
	BufferDirName = "bepInEx"
)

// BufferDir 返回 BepInEx 专用缓存目录。
// 职责：基于当前可执行文件目录构造 BepInEx 下载和解压缓存路径。
// 使用：安装流程把下载包和解压产物放入该目录下，避免污染游戏目录。
// 边界：无法解析当前可执行文件路径时返回错误，不创建目录。
func BufferDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), fileops.BufferRootDirName, BufferDirName), nil
}
