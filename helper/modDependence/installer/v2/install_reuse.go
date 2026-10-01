package v2

// 文件职责：跳过已成功安装的同版本 Thunderstore 主包。
// 使用：安装入口在创建会话之前调用。
// 边界：手动导入可重复安装；跳过不会覆盖现有配置或启动远端解析。

import (
	"path/filepath"

	modDTO "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	"DysonSphereBox/helper/modDependence/gamepath"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
)

// reuseInstalledRoot 构造同版本主包的跳过结果。
// 职责：避免重复下载与文件覆盖；使用：ResetSession 安装入口；边界：只信任稳定主包记录，不修复用户移除的文件。
func reuseInstalledRoot(pack modDTO.RemoteModPackageDTO) (*contract.PackageInstallResult, error) {
	if pack.Source != sourceConstant.ModSourceThunderstore {
		return nil, nil
	}
	previous, err := installmanifest.CapturePreviousPackage(pack.Key)
	if err != nil || previous == nil {
		return nil, err
	}
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	progressEvent.Package(pack.Key, "done", "同版本已安装，跳过重复安装", 100)
	return &contract.PackageInstallResult{PackageKey: pack.Key, PackageName: pack.DisplayName, GameDir: gameDir,
		PluginRoot: filepath.Join(gameDir, "BepInEx", "plugins"), Installed: true, Skipped: true,
		Message: "同版本已安装，跳过重复安装", BepInExStatus: bepinexStatus.CheckBepInExStatus(gameDir)}, nil
}
