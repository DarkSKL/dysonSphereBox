package v2

// 文件职责：准备 V2 远端包安装的缓存目录和 artifact 队列。
// 使用：prepareRemotePackageInstall 在 BepInEx 就绪后调用，生成后续安装循环所需的输入。
// 边界：只解析和记录计划，不下载、不解压、不复制 artifact 文件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/artifactresolve"
	v2Buffer "DysonSphereBox/helper/modDependence/installer/v2/buffer"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"context"
)

// prepareArtifactsForRemoteInstall 解析安装缓存目录和 artifact 队列。
// 职责：获取 V2 缓存目录、解析主包与依赖 artifact，并把计划写入 manifest。
// 使用：prepareRemotePackageInstall 在 BepInEx 就绪后调用。
// 边界：只生成安装计划，不处理单个 artifact 文件内容或访问本项目服务器。
func prepareArtifactsForRemoteInstall(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO) (string, []v2Artifact.Artifact, error) {
	bufferDir, err := v2Buffer.Dir()
	if err != nil {
		return "", nil, err
	}
	artifacts, err := artifactresolve.ResolveInstallArtifactsWithContext(ctx, pack)
	if err != nil {
		return "", nil, err
	}
	if err := installmanifest.AppendPlannedArtifacts(pack.Key, artifacts); err != nil {
		return "", nil, err
	}
	progressEvent.Package(pack.Key, "prepare", "依赖解析完成", 12)
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return "", nil, err
	}
	return bufferDir, artifacts, nil
}
