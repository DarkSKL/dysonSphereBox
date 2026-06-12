package status

// 文件职责：检测 BepInEx 注入环境状态并支持用户选择路径的即时状态加载。
// 使用：服务层、开服环境检测、安装器和卸载流程在需要读取 BepInEx 当前状态时调用。
// 边界：只读取文件和配置，不创建、修复或删除任何本地内容。

import (
	bepinexConfig "DysonSphereBox/helper/modDependence/bepinex/config"
	"DysonSphereBox/helper/modDependence/bepinex/contract"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/utils"
	"DysonSphereBox/utils/dysonsphere"
	"fmt"
	"path/filepath"
)

// CheckBepInExStatus 检查指定游戏目录下 BepInEx 注入文件是否齐全。
// 职责：检测 BepInEx 核心 DLL、Doorstop 注入文件和配置有效性。
// 使用：服务状态刷新、安装前跳过判断、无头服务器环境检测都会调用。
// 边界：只读取文件和配置，不会创建、修复或删除任何本地内容。
// 逻辑流：1. 归一化游戏目录 => 2. 检查关键文件是否存在 => 3. 校验 Doorstop 配置 => 4. 汇总缺失项和本地版本。
func CheckBepInExStatus(gameDir string) contract.BepInExStatus {
	gameDir = utils.CleanPath(gameDir)
	bepInExDir := filepath.Join(gameDir, "BepInEx")
	status := contract.BepInExStatus{GameDir: gameDir, BepInExDir: bepInExDir, MissingFiles: make([]string, 0)}
	requiredFiles := []struct {
		relativePath string
		present      *bool
	}{{filepath.Join("BepInEx", "core", "BepInEx.dll"), &status.CoreExists}, {filepath.Join("BepInEx", "core", "BepInEx.Preloader.dll"), &status.PreloaderExists}, {"winhttp.dll", &status.WinHTTPExists}, {"doorstop_config.ini", &status.DoorstopExists}}
	for _, requiredFile := range requiredFiles {
		exists := utils.FileExists(filepath.Join(gameDir, requiredFile.relativePath))
		*requiredFile.present = exists
		if !exists {
			status.MissingFiles = append(status.MissingFiles, requiredFile.relativePath)
		}
	}
	status.DoorstopValid = bepinexConfig.IsDoorstopConfigValid(gameDir)
	if status.DoorstopExists && !status.DoorstopValid {
		status.MissingFiles = append(status.MissingFiles, "doorstop_config.ini 配置未启用或目标程序集错误")
	}
	status.Installed = len(status.MissingFiles) == 0
	status.InstalledVersion = bepinexConfig.ReadBepInExVersion(gameDir)
	return status
}

// SearchAndLoadBepInEx 根据用户选择路径定位游戏目录并返回 BepInEx 状态。
// 职责：把用户选择的文件或目录归一化为游戏目录并执行 BepInEx 状态检测。
// 使用：设置页手动选择目录后立即检测，不依赖已保存配置。
// 边界：路径无效时返回错误而不是写入配置，也不会自动安装 BepInEx。
func SearchAndLoadBepInEx(selectedPath string) (*contract.BepInExStatus, error) {
	gameDir := dysonsphere.NormalizeDir(selectedPath)
	if gameDir == "" {
		return nil, fmt.Errorf("selected directory does not contain %s", dysonsphere.ExeName)
	}
	if err := gamepath.AssertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}
	status := CheckBepInExStatus(gameDir)
	return &status, nil
}
