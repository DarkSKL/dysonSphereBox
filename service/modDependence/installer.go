package modDependence

// 文件职责：提供 Mod 包安装、卸载、取消、冲突继续和进度恢复的 Wails 服务。
// 使用：Mod 市场页和已安装列表通过生成绑定触发远端包安装工作流。
// 边界：服务层负责入参转发和审计日志；下载、解压、复制、回滚和数据库记录由 helper 子包负责。

import (
	ModDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installAudit "DysonSphereBox/helper/modDependence/audit/install"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Installer "DysonSphereBox/helper/modDependence/installer/v2"
	v2Cancel "DysonSphereBox/helper/modDependence/installer/v2/cancel"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	v2ProgressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	v2ProgressSnapshot "DysonSphereBox/helper/modDependence/installer/v2/progress/snapshot"
	v2Query "DysonSphereBox/helper/modDependence/installer/v2/query"
	v2UninstallEntry "DysonSphereBox/helper/modDependence/installer/v2/uninstall/entry"
)

// InstallerService 提供前端可调用的 Mod 包安装能力。
// 职责：编排统一来源 Mod 包的安装、卸载、取消和进度恢复服务。
// 使用：Mod 市场卡片点击安装、卸载、继续冲突或恢复进度时由前端调用。
// 边界：服务层只做 Wails 入参与 helper 转发，下载、解压、文件删除和数据库写入由 helper 承接。
type InstallerService struct {
}

// InstallRemotePackage 安装统一来源 Mod 包。
// 职责：触发远端包安装并记录成功或失败日志。
// 使用：Mod 页来源列表选择安装时调用；支持 Thunderstore 与手动导入来源。
// 边界：冲突和取消错误由 helper 保持特殊状态，服务层不把它们记录为普通失败。
// 逻辑流：1. 调用远端安装 helper => 2. 判断取消/冲突/失败分支 => 3. 记录安装成功或失败日志 => 4. 返回安装结果给前端。
func (service *InstallerService) InstallRemotePackage(pack ModDependenceResponse.RemoteModPackageDTO) (*contract.PackageInstallResult, error) {
	result, err := v2Installer.InstallRemoteModPackage(pack)
	if err != nil {
		if !v2InstallError.IsCancelled(err) && !v2InstallError.IsConflict(err) {
			installAudit.RecordRemotePackageInstallFailure(pack, err)
		}
		return nil, err
	}

	installAudit.RecordRemotePackageInstallSuccess(result)
	return result, nil
}

// ContinueRemotePackageInstall 按用户选择继续处理 Mod 包安装冲突。
// 职责：把前端冲突决策传回 V2 安装器继续执行。
// 使用：前端卡片显示修复按钮后，用户选择覆盖、跳过或失败时调用。
// 边界：只处理处于 conflict 状态的安装包；非法 action 由 helper 返回错误。
func (service *InstallerService) ContinueRemotePackageInstall(packageKey string, action string) (*contract.PackageInstallResult, error) {
	result, err := v2Installer.ContinueRemoteModPackageInstall(packageKey, contract.InstallConflictAction(action))
	if err != nil {
		return nil, err
	}

	installAudit.RecordRemotePackageInstallSuccess(result)
	return result, nil
}

// CancelRemotePackageInstall 取消当前进程中正在执行的 Mod 包安装。
// 职责：请求取消正在运行的 V2 安装上下文。
// 使用：Mod 卡片下载进度不再变化或用户想停止安装时调用。
// 边界：只能取消当前进程内注册的安装任务；已完成或未注册任务由 helper 返回对应结果。
func (service *InstallerService) CancelRemotePackageInstall(packageKey string) error {
	return v2Cancel.CancelRemotePackageInstall(packageKey)
}

// UninstallRemotePackage 按统一 packageKey 卸载 Mod 包。
// 职责：触发 V2 包卸载并维护卸载进度与审计日志。
// 使用：前端对 V2 已安装卡片点击卸载时调用。
// 边界：V2 卸载按安装文件清单清理，避免 Thunderstore patcher 或手动导入文件残留。
// 逻辑流：1. 调用卸载 helper => 2. 失败时标记进度和审计日志 => 3. 成功时记录卸载结果 => 4. 返回卸载明细。
func (service *InstallerService) UninstallRemotePackage(packageKey string) (*contract.PackageInstallResult, error) {
	result, err := v2UninstallEntry.UninstallRemoteModPackage(packageKey)
	if err != nil {
		v2ProgressEvent.MarkFailure(packageKey, "卸载失败", err)
		installAudit.RecordRemotePackageUninstallFailure(packageKey, err)
		return nil, err
	}

	installAudit.RecordRemotePackageUninstallSuccess(result)
	return result, nil
}

// ListInstalledPackageKeys 返回 V2 已安装包 key。
// 职责：提供统一来源包的本地安装状态索引。
// 使用：前端按 key 映射不同来源的安装状态，避免数字 ID 冲突。
// 边界：只读取本地安装记录，不访问远端市场，不包含失败或冲突中的包。
func (service *InstallerService) ListInstalledPackageKeys() ([]string, error) {
	return v2Query.ListInstalledPackageKeys()
}

// ListInstallProgressSnapshots 返回当前进程内和本地表中的 Mod 安装进度快照。
// 职责：恢复安装中、失败和待处理冲突的卡片状态。
// 使用：前端页面刷新后恢复正在安装或失败的卡片状态。
// 边界：只返回可恢复状态，不主动重启安装任务。
func (service *InstallerService) ListInstallProgressSnapshots() ([]contract.InstallProgressSnapshot, error) {
	return v2ProgressSnapshot.List()
}
