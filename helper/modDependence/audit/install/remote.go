package install

// 文件职责：记录统一来源 Mod 包安装与卸载审计日志。
// 使用：service/modDependence 的远端安装、继续安装和卸载流程成功或失败后调用。
// 边界：只记录系统日志，不执行文件修改、进度更新、安装会话恢复或数据库事务。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	systemModel "DysonSphereBox/constant/logging"
	"DysonSphereBox/core/diagnostics"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/diagnostic"
	systemHelper "DysonSphereBox/helper/system"
	"errors"
)

// RecordRemotePackageInstallSuccess 记录 Mod 包安装成功事件。
// 职责：把远端统一包安装成功结果写入系统审计日志。
// 使用：InstallRemotePackage 或 ContinueRemotePackageInstall 成功后调用。
// 边界：result 为空或跳过时不记录；不负责安装文件、更新进度或保存安装记录。
func RecordRemotePackageInstallSuccess(result *contract.PackageInstallResult) {
	if result == nil || result.Skipped {
		return
	}

	systemHelper.RecordLogSilently(diagnostic.Apply(systemHelper.LogEntry{
		Namespace: "modDependence.installer." + "installRemotePackage",
		Source:    "backend.installer",
		Reason:    "安装器完成文件写入及安装记录提交",
		Handler:   "User",
		Level:     systemModel.Info,
		Action:    "Mod 包已安装",
		Data: map[string]interface{}{
			"result":          "success",
			"packageKey":      result.PackageKey,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
			"pluginRoot":      result.PluginRoot,
			"message":         result.Message,
		},
	}, result.PackageKey))
}

// RecordRemotePackageInstallFailure 记录 Mod 包安装失败事件。
// 职责：把远端统一包安装失败及失败组件信息写入系统审计日志。
// 使用：InstallRemotePackage 捕获非取消、非冲突错误后调用。
// 边界：只记录失败，不改变安装会话；ArtifactInstallError 会被展开到 metadata。
func RecordRemotePackageInstallFailure(pack modDependenceResponse.RemoteModPackageDTO, err error) {
	metadata := map[string]interface{}{
		"packageKey":  pack.Key,
		"packageName": pack.DisplayName,
		"source":      pack.Source,
	}

	var artifactErr *contract.ArtifactInstallError
	if errors.As(err, &artifactErr) {
		metadata["failedStage"] = artifactErr.Stage
		metadata["failedComponentKey"] = artifactErr.ComponentKey
		metadata["failedComponentName"] = artifactErr.ComponentName
		metadata["failedComponentVersion"] = artifactErr.ComponentVersion
		metadata["failedDownloadURL"] = artifactErr.DownloadURL
	}

	metadata["result"], metadata["error"] = "failed", diagnostics.ErrorData(err)
	systemHelper.RecordLogSilently(diagnostic.Apply(systemHelper.LogEntry{Action: "Mod 包安装失败", Level: systemModel.Error, Source: "backend.installer", Reason: "安装器返回非取消、非冲突的错误", Namespace: "modDependence.installer.installRemotePackage", Handler: systemModel.User, Data: metadata}, pack.Key))
}

// RecordRemotePackageUninstallSuccess 记录 Mod 包卸载成功事件。
// 职责：把远端统一包卸载成功结果写入系统审计日志。
// 使用：UninstallRemotePackage 成功后调用。
// 边界：result 为空或跳过时不记录；不负责执行卸载或清理共享依赖。
func RecordRemotePackageUninstallSuccess(result *contract.PackageInstallResult) {
	if result == nil || result.Skipped {
		return
	}

	systemHelper.RecordLogSilently(diagnostic.Apply(systemHelper.LogEntry{
		Namespace: "modDependence.installer." + "uninstallRemotePackage",
		Source:    "backend.installer",
		Reason:    "卸载器完成清单处理及数据库提交",
		Handler:   "User",
		Level:     systemModel.Info,
		Action:    "Mod 包已卸载",
		Data: map[string]interface{}{
			"result":          "success",
			"packageKey":      result.PackageKey,
			"packageName":     result.PackageName,
			"dependencyCount": len(result.Dependencies),
			"gameDir":         result.GameDir,
			"message":         result.Message,
		},
	}, result.PackageKey))
}

// RecordRemotePackageUninstallFailure 记录 Mod 包卸载失败事件。
// 职责：把远端统一包卸载失败写入系统审计日志。
// 使用：UninstallRemotePackage 捕获错误后调用。
// 边界：只记录失败，不修改安装记录或文件状态。
func RecordRemotePackageUninstallFailure(packageKey string, err error) {
	RecordPackageFailure(packageKey, "uninstallRemotePackage", "Mod 包卸载失败", err)
}

// RecordPackageFailure 职责：记录关联包的失败；使用：继续安装和卸载失败；边界：保留真实事件名，不混淆安装和卸载。
func RecordPackageFailure(packageKey, event, action string, err error) {
	systemHelper.RecordLogSilently(diagnostic.Apply(systemHelper.LogEntry{Action: action, Level: systemModel.Error, Source: "backend.installer", Reason: "安装器操作返回错误", Namespace: "modDependence.installer." + event, Handler: systemModel.User, Data: map[string]interface{}{"packageKey": packageKey, "result": "failed", "error": diagnostics.ErrorData(err)}}, packageKey))
}
