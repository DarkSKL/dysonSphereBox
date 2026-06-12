package artifact

// 文件职责：编排旧官方整合包单个依赖 artifact 的安装流程。
// 使用：legacy/install.InstallModDependencyPackage 遍历依赖列表时调用 InstallDependency。
// 边界：只处理旧接口中的 ModDependencyDTO zip 包，不处理 Thunderstore 依赖解析、V2 冲突恢复或安装会话。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/contract"
	legacyCleanup "DysonSphereBox/helper/modDependence/installer/legacy/cleanup"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"fmt"
	"os"
)

// InstallDependency 安装单个旧官方整合包依赖。
// 职责：把依赖 zip 下载或复用缓存后解压到 BepInEx/plugins 下的独立目录。
// 使用：旧整合包安装入口按依赖列表逐项调用。
// 边界：依赖名和下载地址不能为空；目标目录必须位于 pluginRoot 内，失败时返回错误交给入口终止安装。
// 逻辑流：1. 校验依赖元数据 => 2. 下载或复用 zip 缓存 => 3. 解压到缓冲目录 => 4. 重建插件目录 => 5. 返回安装结果。
func InstallDependency(bufferDir string, pluginRoot string, dependency modDependenceResponse.ModDependencyDTO) (contract.DependencyInstallResult, error) {
	name := legacyValue.String(dependency.ModName)
	if name == "" {
		return contract.DependencyInstallResult{}, fmt.Errorf("dependency %d modName is required", dependency.ID)
	}
	if legacyValue.String(dependency.Url) == "" {
		return contract.DependencyInstallResult{}, fmt.Errorf("dependency %s download url is required", name)
	}

	zipPath, usedCache, err := downloadDependencyZip(bufferDir, dependency)
	if err != nil {
		return contract.DependencyInstallResult{}, err
	}

	extractPath, err := extractDependencyZip(bufferDir, zipPath, dependency)
	if err != nil {
		return contract.DependencyInstallResult{}, err
	}

	pluginDir := legacyCleanup.DependencyPluginDir(pluginRoot, name)
	if !fileops.IsSubPath(pluginRoot, pluginDir) {
		return contract.DependencyInstallResult{}, fmt.Errorf("plugin path escapes plugin root: %s", pluginDir)
	}
	if err := os.RemoveAll(pluginDir); err != nil {
		return contract.DependencyInstallResult{}, err
	}
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return contract.DependencyInstallResult{}, err
	}
	if err := fileops.CopyDirectoryContents(extractPath, pluginDir); err != nil {
		return contract.DependencyInstallResult{}, err
	}

	return contract.DependencyInstallResult{
		ID:        dependency.ID,
		Name:      name,
		Version:   legacyValue.String(dependency.ModVersion),
		PluginDir: pluginDir,
		ZipPath:   zipPath,
		Installed: true,
		Message:   dependencyInstallMessage(usedCache),
	}, nil
}
