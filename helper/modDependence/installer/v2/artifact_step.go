package v2

// 文件职责：执行 V2 安装主流程中的单个 artifact 安装步骤。
// 使用：InstallRemoteModPackageWithOptions 遍历 artifact 队列时调用 installRemoteArtifact。
// 边界：只处理单个 artifact 的文件安装、记录保存和进度事件；不保存主包完成状态、不处理冲突持久化。

import (
	"context"
	"fmt"

	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/artifactinstall"
	v2InstallError "DysonSphereBox/helper/modDependence/installer/v2/installerror"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	"DysonSphereBox/helper/modDependence/installer/v2/installstore"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"DysonSphereBox/helper/modDependence/installer/v2/versionreplace"
)

// installRemoteArtifact 安装并记录单个远端 artifact。
// 职责：处理一个 artifact 的文件写入、测试 hook、组件记录保存和结果 DTO 构造。
// 使用：InstallRemoteModPackageWithOptions 的 artifact 循环中调用。
// 边界：冲突错误原样返回给主流程持久化；其它错误包装为 ArtifactInstallError 后由主流程清理会话。
// 逻辑流：1. 安装 artifact 文件 => 2. 执行测试 hook => 3. 检查取消 => 4. 保存组件和文件记录 => 5. 发送完成进度并返回结果。
func installRemoteArtifact(ctx context.Context, bufferDir string, gameDir string, pack modDependenceResponse.RemoteModPackageDTO, artifact v2Artifact.Artifact, artifactStart int, artifactEnd int, options contract.InstallRemotePackageOptions) (contract.DependencyInstallResult, error) {
	recordPercent := max(artifactStart, artifactEnd-3)
	progressEvent.Package(pack.Key, "install", fmt.Sprintf("正在处理 %s", artifact.DisplayName), artifactStart)
	if artifact.ReuseExisting {
		if err := installmanifest.AppendReusedArtifact(artifact.InstallRecordPackageKey(), artifact.ComponentKey); err != nil {
			return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("record", artifact, err)
		}
		if err := installstore.SaveArtifact(pack, artifact, nil); err != nil {
			return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("record", artifact, err)
		}
		progressEvent.Package(pack.Key, "install", fmt.Sprintf("复用已安装依赖 %s", artifact.DisplayName), artifactEnd)
		return contract.DependencyInstallResult{Key: artifact.ComponentKey, Name: artifact.DisplayName, Version: artifact.Version, Skipped: true, Message: "复用已安装组件"}, nil
	}
	fileRecords, zipPath, err := artifactinstall.InstallFiles(ctx, bufferDir, gameDir, artifact, artifactStart, recordPercent, options, installRemotePackageTestHooks.AfterPlannedFiles)
	if err != nil {
		if v2InstallError.IsConflict(err) {
			return contract.DependencyInstallResult{}, err
		}
		return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("install", artifact, err)
	}
	if installRemotePackageTestHooks.AfterArtifactFiles != nil {
		installRemotePackageTestHooks.AfterArtifactFiles(pack.Key, artifact.ComponentKey)
	}
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return contract.DependencyInstallResult{}, err
	}
	if len(fileRecords) == 0 {
		if err := installmanifest.AppendArtifact(artifact.InstallRecordPackageKey(), artifact.ComponentKey, nil); err != nil {
			return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("record", artifact, err)
		}
	}
	if err := versionreplace.Prepare(ctx, bufferDir, gameDir, artifact, fileRecords); err != nil {
		return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("replace", artifact, err)
	}
	progressEvent.Package(pack.Key, "record", fmt.Sprintf("正在记录 %s", artifact.DisplayName), recordPercent)
	if err := installstore.SaveArtifact(pack, artifact, fileRecords); err != nil {
		return contract.DependencyInstallResult{}, v2InstallError.WrapArtifact("record", artifact, err)
	}
	if installRemotePackageTestHooks.AfterSaveArtifact != nil {
		installRemotePackageTestHooks.AfterSaveArtifact(pack.Key, artifact.ComponentKey)
	}
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return contract.DependencyInstallResult{}, err
	}
	progressEvent.Package(pack.Key, "install", fmt.Sprintf("%s 安装完成", artifact.DisplayName), artifactEnd)
	return contract.DependencyInstallResult{Key: artifact.ComponentKey, Name: artifact.DisplayName, Version: artifact.Version, ZipPath: zipPath, Installed: true, Message: "组件安装完成"}, nil
}

// isInstallConflict 判断安装错误是否是可恢复冲突。
// 职责：把冲突判断封装在同包内，避免主流程直接依赖 installerror 子包。
// 使用：InstallRemoteModPackageWithOptions 收到 installRemoteArtifact 错误后调用。
// 边界：只做类型判断，不持久化冲突、不清理会话。
func isInstallConflict(err error) bool {
	return v2InstallError.IsConflict(err)
}
