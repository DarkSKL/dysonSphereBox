package install

// 文件职责：安装、跳过或修复当前游戏目录中的 BepInEx 前置框架。
// 使用：Mod 安装流程、BepInEx 服务和无头服务器准备流程在需要确保前置框架可用时调用。
// 边界：只处理 BepInEx 包下载、解压、复制和 Doorstop 修复；不记录审计日志、不管理 Mod 市场或前端状态。

import (
	bepinexArchive "DysonSphereBox/helper/modDependence/bepinex/archive"
	bepinexConfig "DysonSphereBox/helper/modDependence/bepinex/config"
	"DysonSphereBox/helper/modDependence/bepinex/contract"
	"DysonSphereBox/helper/modDependence/bepinex/remote"
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/gamepath"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallBepInEx 确保当前游戏目录具备可用的 BepInEx 注入环境。
// 职责：在配置的游戏目录中安装、跳过或修复 BepInEx 前置框架。
// 使用：Mod 安装和开服准备前调用，服务层负责记录审计日志。
// 边界：已安装时只校验或修复 Doorstop 配置；下载、解压或复制失败时返回错误，不写审计日志。
// 逻辑流：1. 读取并校验游戏目录 => 2. 检测现有 BepInEx 状态 => 3. 已安装则修复/确认 Doorstop 后返回 => 4. 解析并下载最新包 => 5. 解压并复制 BepInExPack => 6. 再次检测并返回结果。
func InstallBepInEx() (*contract.BepInExInstallResult, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	status := bepinexStatus.CheckBepInExStatus(gameDir)
	if status.CoreExists && status.PreloaderExists && status.WinHTTPExists && status.DoorstopExists && !status.DoorstopValid {
		if err := bepinexConfig.EnsureDoorstopConfig(gameDir); err != nil {
			return nil, err
		}
		status = bepinexStatus.CheckBepInExStatus(gameDir)
		return &contract.BepInExInstallResult{GameDir: gameDir, Installed: false, Skipped: true, Message: "BepInEx 已安装，已修复 Doorstop 配置", Status: status}, nil
	}
	if status.Installed {
		if err := bepinexConfig.EnsureDoorstopConfig(gameDir); err != nil {
			return nil, err
		}
		status = bepinexStatus.CheckBepInExStatus(gameDir)
		return &contract.BepInExInstallResult{GameDir: gameDir, Installed: false, Skipped: true, Message: "BepInEx 已安装，已确认 Doorstop 配置", Status: status}, nil
	}
	pkg, err := remote.ResolveLatestBepInExPackage()
	if err != nil {
		return nil, err
	}
	bufferDir, err := remote.BufferDir()
	if err != nil {
		return nil, err
	}
	zipPath, err := remote.DownloadPackage(bufferDir, pkg)
	if err != nil {
		return nil, err
	}
	extractPath, err := bepinexArchive.ExtractPackage(bufferDir, zipPath, pkg)
	if err != nil {
		return nil, err
	}
	packRoot := filepath.Join(extractPath, "BepInExPack")
	if info, err := os.Stat(packRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("BepInEx package structure is invalid: missing BepInExPack")
	}
	if err := fileops.CopyDirectoryContents(packRoot, gameDir); err != nil {
		return nil, err
	}
	if err := bepinexConfig.EnsureDoorstopConfig(gameDir); err != nil {
		return nil, err
	}
	nextStatus := bepinexStatus.CheckBepInExStatus(gameDir)
	if !nextStatus.Installed {
		return nil, fmt.Errorf("BepInEx install incomplete, missing: %s", strings.Join(nextStatus.MissingFiles, ", "))
	}
	return &contract.BepInExInstallResult{GameDir: gameDir, BufferDir: bufferDir, Installed: true, Skipped: false, Message: "BepInEx 安装完成", Package: pkg, ZipPath: zipPath, ExtractPath: extractPath, Status: nextStatus}, nil
}
