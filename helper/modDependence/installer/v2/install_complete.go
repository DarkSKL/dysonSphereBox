package v2

// 文件职责：完成 V2 远端包安装并构造主包安装结果。
// 使用：InstallRemoteModPackageWithOptions 在全部 artifact 安装成功后调用。
// 边界：只保存主包完成状态和构造返回 DTO；失败时触发会话清理，不复制文件、不处理冲突。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/installstore"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"context"
	"path/filepath"
)

// completeRemotePackageInstall 保存主包完成状态并返回安装结果。
// 职责：检查最终取消状态、保存 installed 主包记录、发送完成进度并组装 PackageInstallResult。
// 使用：InstallRemoteModPackageWithOptions 在 artifact 队列全部成功后调用。
// 边界：SaveRemotePackage 失败时触发会话清理；不负责组件文件和组件记录保存。
func completeRemotePackageInstall(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO, preparation *installPreparation, results []contract.DependencyInstallResult) (*contract.PackageInstallResult, error) {
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return nil, err
	}
	progressEvent.Package(pack.Key, "record", "正在写入安装记录", 94)
	if err := installstore.SaveRemotePackage(pack); err != nil {
		return nil, cleanupEntry.CleanupFailed(pack.Key, err)
	}
	progressEvent.Package(pack.Key, "done", "安装完成", 100)
	return &contract.PackageInstallResult{
		PackageKey:    pack.Key,
		PackageName:   pack.DisplayName,
		GameDir:       preparation.gameDir,
		PluginRoot:    filepath.Join(preparation.gameDir, "BepInEx", "plugins"),
		Installed:     true,
		Message:       "Mod 包安装完成",
		Dependencies:  results,
		BepInExStatus: preparation.bepInExResult.Status,
	}, nil
}
