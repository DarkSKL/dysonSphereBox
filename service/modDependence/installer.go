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

// InstallPackage 安装一个远端市场返回的整合包。
// 使用：前端把选中的 ModDependencyPackageDTO 原样传入；安装器会先确保 BepInEx 可用。
// 返回：包含游戏目录、插件根目录、依赖安装明细和 BepInEx 状态。
func (service *InstallerService) InstallPackage(pack ModDependenceResponse.ModDependencyPackageDTO) (*modDependenceHelper.PackageInstallResult, error) {
	return modDependenceHelper.InstallModDependencyPackage(pack)
}

// UninstallPackage 卸载指定 ID 的本地整合包记录和可安全移除的插件目录。
// 边界：若依赖仍被其它已安装整合包引用，则保留该依赖插件目录。
func (service *InstallerService) UninstallPackage(packageID uint) (*modDependenceHelper.PackageInstallResult, error) {
	return modDependenceHelper.UninstallModDependencyPackage(packageID)
}

// ListInstalledPackageIds 返回本地已安装整合包 ID。
// 使用：前端刷新市场列表后调用，用于把远端卡片映射成本地安装状态。
func (service *InstallerService) ListInstalledPackageIds() ([]uint, error) {
	return modDependenceHelper.ListInstalledPackageIds()
}
