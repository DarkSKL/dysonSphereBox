package artifactfetch

// 文件职责：解析 V2 artifact 的本地或远端可安装路径。
// 使用：artifact 安装规划在需要获取 zip 或本地目录路径时调用 ResolvePath。
// 边界：只决定路径来源并委托下载，不解压 zip、不复制游戏文件、不保存 installed 模型。

import (
	"context"
	"fmt"

	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"

	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
)

// ResolvePath 解析 artifact 的可安装路径。
// 职责：本地 artifact 直接返回本地路径，远端 artifact 下载到缓存后返回 zip 路径。
// 使用：artifactinstall.InstallFiles 在处理文件前调用。
// 边界：本地路径只校验非空；路径是否可遍历由后续规划阶段处理。
// 逻辑流：1. 判断本地或远端 artifact => 2. 本地路径直接返回 => 3. 远端委托下载缓存流程 => 4. 返回后续规划可读取路径。
func ResolvePath(ctx context.Context, bufferDir string, artifact v2Artifact.Artifact, progressStart int, progressEnd int) (string, error) {
	switch artifact.ArtifactKind {
	case artifactConstant.ArtifactKindLocalZip, artifactConstant.ArtifactKindLocalDir:
		if err := cancel.Check(ctx); err != nil {
			return "", err
		}
		if artifact.LocalPath == "" {
			return "", fmt.Errorf("local path is required for %s", artifact.ComponentKey)
		}
		progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("%s 已使用本地文件", artifact.DisplayName), progressEnd)
		return artifact.LocalPath, nil
	default:
		if artifact.DownloadURL == "" {
			return "", fmt.Errorf("download url is required for %s", artifact.ComponentKey)
		}
		return downloadRemoteArtifact(ctx, bufferDir, artifact, progressStart, progressEnd)
	}
}
