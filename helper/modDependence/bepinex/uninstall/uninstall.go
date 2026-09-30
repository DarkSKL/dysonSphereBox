package uninstall

// 文件职责：卸载游戏目录中的 BepInEx 注入文件。
// 使用：BepInEx 服务在用户明确卸载前置框架时调用。
// 边界：删除范围限定在游戏目录内的 BepInEx 注入目标；不清理 installed 安装记录，不记录审计日志。

import (
	"DysonSphereBox/helper/modDependence/bepinex/contract"
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/gamepath"
	"fmt"
	"os"
	"path/filepath"
)

// UninstallBepInEx 移除游戏目录下的 BepInEx 注入文件。
// 职责：删除 BepInEx 目录与 Doorstop 注入文件。
// 使用：设置页或服务层明确卸载 BepInEx 时调用。
// 边界：删除范围限定为游戏目录内的 BepInEx 目录和 Doorstop 注入文件；路径越界立即返回错误。
// 逻辑流：1. 获取并校验游戏目录 => 2. 记录卸载前状态 => 3. 删除允许范围内的 BepInEx 目标 => 4. 重新检测并返回卸载结果。
func UninstallBepInEx() (*contract.BepInExUninstallResult, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	if err := gamepath.AssertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}
	status := bepinexStatus.CheckBepInExStatus(gameDir)
	removeTargets := []string{filepath.Join(gameDir, "BepInEx"), filepath.Join(gameDir, "winhttp.dll"), filepath.Join(gameDir, "doorstop_config.ini"), filepath.Join(gameDir, ".doorstop_version"), filepath.Join(gameDir, "changelog.txt")}
	removedPaths := make([]string, 0, len(removeTargets))
	for _, targetPath := range removeTargets {
		if !fileops.IsSubPath(gameDir, targetPath) {
			return nil, fmt.Errorf("remove path escapes game directory: %s", targetPath)
		}
		if _, err := os.Stat(targetPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if err := os.RemoveAll(targetPath); err != nil {
			return nil, err
		}
		removedPaths = append(removedPaths, targetPath)
	}
	nextStatus := bepinexStatus.CheckBepInExStatus(gameDir)
	skipped := !status.Installed && len(removedPaths) == 0
	message := "BepInEx 与相关 Mod 依赖已卸载"
	if skipped {
		message = "未检测到可卸载的 BepInEx 文件"
	}
	return &contract.BepInExUninstallResult{GameDir: gameDir, Removed: !skipped, Skipped: skipped, Message: message, RemovedPaths: removedPaths, Status: nextStatus}, nil
}
