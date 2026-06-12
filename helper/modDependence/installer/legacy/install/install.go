package install

// 文件职责：编排旧官方整合包的安装入口。
// 使用：service/modDependence 的 InstallerService 在兼容旧官方整合包来源时调用 InstallModDependencyPackage。
// 边界：只处理旧 ModDependencyPackageDTO；Thunderstore/V2 统一安装流程不经过本包。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	bepinexInstall "DysonSphereBox/helper/modDependence/bepinex/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	legacyArtifact "DysonSphereBox/helper/modDependence/installer/legacy/artifact"
	legacyCleanup "DysonSphereBox/helper/modDependence/installer/legacy/cleanup"
	legacyStore "DysonSphereBox/helper/modDependence/installer/legacy/store"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InstallModDependencyPackage 安装远端整合包 DTO 中声明的全部 Mod 依赖。
// 职责：确保 BepInEx 可用后下载、解压并写入旧官方整合包依赖记录。
// 使用：InstallerService 接收前端选中的市场卡片后调用；会先确保 BepInEx 可用。
// 边界：依赖必须包含可下载的 zip 地址，安装成功后才写入本地 SQLite 安装记录。
// 逻辑流：1. 校验整合包参数 => 2. 确保 BepInEx 与插件目录 => 3. 下载并复制依赖 => 4. 保存安装记录 => 5. 清理不再引用的旧依赖目录。
func InstallModDependencyPackage(pack modDependenceResponse.ModDependencyPackageDTO) (*contract.PackageInstallResult, error) {
	if pack.ID == 0 {
		return nil, fmt.Errorf("package id is required")
	}
	if len(pack.Dependencies) == 0 {
		return nil, fmt.Errorf("package has no dependencies to install")
	}
	bepInExResult, err := bepinexInstall.InstallBepInEx()
	if err != nil {
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		return nil, fmt.Errorf("BepInEx is not ready: %s", strings.Join(bepInExResult.Status.MissingFiles, ", "))
	}
	gameDir := bepInExResult.Status.GameDir
	pluginRoot := filepath.Join(gameDir, "BepInEx", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		return nil, err
	}
	staleDependencies, err := legacyCleanup.StalePackageDependencies(pack.ID, pack.Dependencies)
	if err != nil {
		return nil, err
	}
	bufferDir, err := legacyArtifact.BufferDir()
	if err != nil {
		return nil, err
	}
	dependencyResults := make([]contract.DependencyInstallResult, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		result, err := legacyArtifact.InstallDependency(bufferDir, pluginRoot, dependency)
		if err != nil {
			return nil, err
		}
		dependencyResults = append(dependencyResults, result)
	}
	if err := legacyStore.SaveInstalledPackage(pack); err != nil {
		return nil, err
	}
	if err := legacyCleanup.RemoveStaleDependencyPluginDirs(pluginRoot, staleDependencies); err != nil {
		return nil, err
	}
	return &contract.PackageInstallResult{PackageID: pack.ID, PackageName: legacyValue.String(pack.PackageName), GameDir: gameDir, PluginRoot: pluginRoot, Installed: true, Uninstalled: false, Skipped: false, Message: "整合包安装完成", Dependencies: dependencyResults, BepInExStatus: bepInExResult.Status}, nil
}
