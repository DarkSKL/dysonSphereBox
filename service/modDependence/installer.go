package modDependence

import (
	modDependenceHelper "DysonSphereBox/helper/modDependence"
	ModDependenceResponse "DysonSphereBox/model/modDependence/response"
)

// InstallerService 提供前端可调用的 Mod 整合包安装能力。
// 用途：Mod 市场卡片点击安装或卸载时调用；安装完成后同步本地 SQLite 安装记录。
// 边界：服务层只做 Wails 入参与 helper 转发，下载、解压、文件删除和数据库写入由 helper 承接。
type InstallerService struct {
}

// InstallRemotePackage 安装 Thunderstore Mod 包。
// 使用：Mod 页 V2 来源列表选择安装时调用；支持国内源、Thunderstore 与手动导入模型。
func (service *InstallerService) InstallRemotePackage(pack ModDependenceResponse.RemoteModPackageDTO) (*modDependenceHelper.PackageInstallResult, error) {
	result, err := modDependenceHelper.InstallRemoteModPackage(pack)
	if err != nil {
		if !modDependenceHelper.IsInstallCancelledError(err) && !modDependenceHelper.IsInstallConflictError(err) {
			modDependenceHelper.RecordRemotePackageInstallFailure(pack, err)
		}
		return nil, err
	}

	modDependenceHelper.RecordRemotePackageInstallSuccess(result)
	return result, nil
}

// ContinueRemotePackageInstall 按用户选择继续处理 Thunderstore 安装冲突。
// 使用：前端卡片显示修复按钮后，用户选择覆盖、跳过或失败时调用。
func (service *InstallerService) ContinueRemotePackageInstall(packageKey string, action string) (*modDependenceHelper.PackageInstallResult, error) {
	result, err := modDependenceHelper.ContinueRemoteModPackageInstall(packageKey, modDependenceHelper.InstallConflictAction(action))
	if err != nil {
		return nil, err
	}

	modDependenceHelper.RecordRemotePackageInstallSuccess(result)
	return result, nil
}

// CancelRemotePackageInstall 取消当前进程中正在执行的 Thunderstore Mod 包安装。
// 使用：Mod 卡片下载进度不再变化或用户想停止安装时调用。
func (service *InstallerService) CancelRemotePackageInstall(packageKey string) error {
	return modDependenceHelper.CancelRemoteModPackageInstall(packageKey)
}

// UninstallRemotePackage 按统一 packageKey 卸载 Mod 包。
// 边界：V2 卸载按安装文件清单清理，避免 Thunderstore patcher 或手动导入文件残留。
func (service *InstallerService) UninstallRemotePackage(packageKey string) (*modDependenceHelper.PackageInstallResult, error) {
	result, err := modDependenceHelper.UninstallRemoteModPackage(packageKey)
	if err != nil {
		modDependenceHelper.MarkPackageProgressFailure(packageKey, "卸载失败", err)
		modDependenceHelper.RecordRemotePackageUninstallFailure(packageKey, err)
		return nil, err
	}

	modDependenceHelper.RecordRemotePackageUninstallSuccess(result)
	return result, nil
}

// ListInstalledPackageKeys 返回 V2 已安装包 key。
// 使用：前端按 key 映射不同来源的安装状态，避免数字 ID 冲突。
func (service *InstallerService) ListInstalledPackageKeys() ([]string, error) {
	return modDependenceHelper.ListInstalledPackageKeys()
}

// ListInstallProgressSnapshots 返回当前进程内和本地表中的 Mod 安装进度快照。
// 使用：前端页面刷新后恢复正在安装或失败的卡片状态。
func (service *InstallerService) ListInstallProgressSnapshots() ([]modDependenceHelper.InstallProgressSnapshot, error) {
	return modDependenceHelper.ListInstallProgressSnapshots()
}

// InstallPackage 安装一个远端市场返回的整合包。
// 使用：前端把选中的 ModDependencyPackageDTO 原样传入；安装器会先确保 BepInEx 可用。
// 返回：包含游戏目录、插件根目录、依赖安装明细和 BepInEx 状态。
func (service *InstallerService) InstallPackage(pack ModDependenceResponse.ModDependencyPackageDTO) (*modDependenceHelper.PackageInstallResult, error) {
	result, err := modDependenceHelper.InstallModDependencyPackage(pack)
	if err != nil {
		modDependenceHelper.RecordPackageInstallFailure(pack, err)
		return nil, err
	}

	modDependenceHelper.RecordPackageInstallSuccess(result)
	return result, nil
}

// UninstallPackage 卸载指定 ID 的本地整合包记录和可安全移除的插件目录。
// 边界：若依赖仍被其它已安装整合包引用，则保留该依赖插件目录。
func (service *InstallerService) UninstallPackage(packageID uint) (*modDependenceHelper.PackageInstallResult, error) {
	result, err := modDependenceHelper.UninstallModDependencyPackage(packageID)
	if err != nil {
		modDependenceHelper.RecordPackageUninstallFailure(packageID, err)
		return nil, err
	}

	modDependenceHelper.RecordPackageUninstallSuccess(result)
	return result, nil
}

// ListInstalledPackageIds 返回本地已安装整合包 ID。
// 使用：前端刷新市场列表后调用，用于把远端卡片映射成本地安装状态。
func (service *InstallerService) ListInstalledPackageIds() ([]uint, error) {
	return modDependenceHelper.ListInstalledPackageIds()
}
