package v2

// 文件职责：安装 V2 远端包准备阶段解析出的 artifact 队列。
// 使用：InstallRemoteModPackageWithOptions 在准备完成后调用，逐个执行 installRemoteArtifact。
// 边界：只处理 artifact 队列错误分支；主包完成状态由 install_complete.go 负责。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"context"
)

// installRemoteArtifacts 安装全部 artifact 并返回组件结果。
// 职责：按解析顺序分配进度区间，处理可恢复冲突持久化和失败清理。
// 使用：InstallRemoteModPackageWithOptions 在 prepareRemotePackageInstall 成功后调用。
// 边界：冲突错误原样返回给服务层继续展示；非冲突错误触发会话清理后返回。
func installRemoteArtifacts(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO, options contract.InstallRemotePackageOptions, preparation *installPreparation) ([]contract.DependencyInstallResult, error) {
	results := make([]contract.DependencyInstallResult, 0, len(preparation.artifacts))
	for index, artifact := range preparation.artifacts {
		if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
			return nil, err
		}
		artifactStart := 12 + index*78/len(preparation.artifacts)
		artifactEnd := 12 + (index+1)*78/len(preparation.artifacts)
		result, err := installRemoteArtifact(ctx, preparation.bufferDir, preparation.gameDir, pack, artifact, artifactStart, artifactEnd, options)
		if err != nil {
			if isInstallConflict(err) {
				if conflictErr := conflictstate.PersistConflict(pack, err, options); conflictErr != nil {
					return nil, conflictErr
				}
				return nil, err
			}
			return nil, cleanupEntry.CleanupFailed(pack.Key, err)
		}
		results = append(results, result)
	}
	return results, nil
}
