package artifactresolve

// 文件职责：解析远端包及其依赖为 V2 安装器内部 artifact 列表。
// 使用：安装入口在准备好 BepInEx 后调用，用于决定后续下载、解压和复制顺序。
// 边界：只做依赖解析和 DTO 转换，不下载文件、不写数据库、不做文件冲突判断。
// 逻辑流：1. 判断来源 => 2. 官方包转换依赖 DTO => 3. Thunderstore 递归展开依赖并追加主包 => 4. 手动来源仅安装主包。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	marketplaceHelper "DysonSphereBox/helper/modDependence/marketplace"
	"context"
	"fmt"
)

// ResolveInstallArtifacts 根据包来源解析需要安装的 artifact 队列。
// 职责：兼容官方依赖列表、Thunderstore 递归依赖和手动导入包。
// 使用：InstallRemoteModPackageWithOptions 在安装执行前调用。
// 边界：Thunderstore 会跳过 BepInEx 依赖；未知来源返回错误。
func ResolveInstallArtifacts(pack modDependenceResponse.RemoteModPackageDTO) ([]v2Artifact.Artifact, error) {
	switch pack.Source {
	case sourceConstant.ModSourceOfficial:
		artifacts := make([]v2Artifact.Artifact, 0, len(pack.Dependencies))
		for _, dependency := range pack.Dependencies {
			artifacts = append(artifacts, v2Artifact.FromDependency(pack.Key, dependency))
		}
		return artifacts, nil
	case sourceConstant.ModSourceThunderstore:
		visited := map[string]struct{}{}
		artifacts := make([]v2Artifact.Artifact, 0)
		for _, dependency := range pack.Dependencies {
			if err := appendThunderstoreDependencyArtifacts(&artifacts, visited, pack.Key, dependency.Key); err != nil {
				return nil, err
			}
		}
		artifacts = append(artifacts, v2Artifact.FromPackage(pack.Key, pack))
		return artifacts, nil
	case sourceConstant.ModSourceManual:
		return []v2Artifact.Artifact{v2Artifact.FromPackage(pack.Key, pack)}, nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", pack.Source)
	}
}

// appendThunderstoreDependencyArtifacts 递归追加 Thunderstore 依赖 artifact。
// 职责：按依赖树深度优先解析，确保依赖先于主包安装。
// 使用：ResolveInstallArtifacts 处理 Thunderstore 来源时调用。
// 边界：空 key、BepInEx 依赖和已访问依赖会跳过，避免重复安装和递归环。
func appendThunderstoreDependencyArtifacts(artifacts *[]v2Artifact.Artifact, visited map[string]struct{}, parentPackageKey string, key string) error {
	if key == "" {
		return nil
	}
	if v2Artifact.IsBepInExDependencyKey(key) {
		return nil
	}
	if _, ok := visited[key]; ok {
		return nil
	}
	visited[key] = struct{}{}

	pack, err := marketplaceHelper.ResolveRemotePackage(context.Background(), key)
	if err != nil {
		return err
	}
	for _, dependency := range pack.Dependencies {
		if err := appendThunderstoreDependencyArtifacts(artifacts, visited, parentPackageKey, dependency.Key); err != nil {
			return err
		}
	}
	*artifacts = append(*artifacts, v2Artifact.FromPackage(parentPackageKey, *pack))
	return nil
}
