package artifactresolve

// 文件职责：解析远端包及其依赖为 V2 安装器内部 artifact 列表。
// 使用：安装入口在准备好 BepInEx 后调用，用于决定后续下载、解压和复制顺序。
// 边界：只做依赖解析和 DTO 转换，不下载文件、不写数据库、不做文件冲突判断。
// 逻辑流：1. 判断整合包策略 => 2. 读取已安装组件快照 => 3. 递归解析依赖且依赖先于主包，复用实际安装版本和传递依赖 => 4. 返回待安装队列。

import (
	"context"
	"fmt"
	"strings"

	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"DysonSphereBox/helper/modDependence/installer/v2/installstore"
	marketplaceHelper "DysonSphereBox/helper/modDependence/marketplace"
)

// ResolveInstallArtifacts 根据包来源解析需要安装的 artifact 队列。
// 职责：普通 Mod 依赖使用最新 active 版本，Modpacks 保留清单锁定版本，并确保依赖先安装。
// 使用：InstallRemoteModPackageWithOptions 在安装执行前调用。
// 边界：普通 Mod 复用本地依赖；Modpacks 仅复用完全匹配的版本；未知来源返回错误。
func ResolveInstallArtifacts(pack modDependenceResponse.RemoteModPackageDTO) ([]v2Artifact.Artifact, error) {
	return ResolveInstallArtifactsWithContext(context.Background(), pack)
}

// ResolveInstallArtifactsWithContext 解析可取消的主包和依赖队列。
// 职责：将安装取消上下文传递到递归解析和市场请求；使用：安装准备阶段调用；边界：不写文件或数据库，取消返回统一安装错误。
func ResolveInstallArtifactsWithContext(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO) ([]v2Artifact.Artifact, error) {
	if err := cancel.Check(ctx); err != nil {
		return nil, err
	}
	switch pack.Source {
	case sourceConstant.ModSourceThunderstore:
		modpack := hasModpackCategory(pack.Tags)
		installed, err := installstore.InstalledThunderstoreComponents()
		if err != nil {
			return nil, err
		}
		visited := map[string]string{}
		namespace, name := pack.Namespace, pack.Name
		if namespace == "" || name == "" {
			namespace, name = dependencyIdentityFromKey(pack.Key)
		}
		visited[modIdentity(namespace, name)] = pack.Key
		artifacts := make([]v2Artifact.Artifact, 0)
		for _, dependency := range pack.Dependencies {
			if err := appendThunderstoreDependencyArtifacts(ctx, &artifacts, visited, installed, pack.Key, dependency, modpack); err != nil {
				return nil, err
			}
		}
		root := v2Artifact.FromPackage(pack.Key, pack)
		if existing, ok := installed[modIdentity(namespace, name)]; ok && existing.ComponentKey == root.ComponentKey {
			root.ReuseExisting = true
		}
		artifacts = append(artifacts, root)
		return artifacts, nil
	case sourceConstant.ModSourceManual:
		return []v2Artifact.Artifact{v2Artifact.FromPackage(pack.Key, pack)}, nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", pack.Source)
	}
}

// appendThunderstoreDependencyArtifacts 递归追加 Thunderstore 依赖 artifact。
// 职责：按 Owner-Name 去重并以深度优先顺序确保依赖先于使用方安装。
// 使用：ResolveInstallArtifacts 处理 Thunderstore 来源时调用。
// 边界：普通 Mod 未安装依赖取最新 active 版本，已安装依赖复用本地版本；Modpacks 使用清单版本；BepInEx、空 key 和循环依赖跳过。
func appendThunderstoreDependencyArtifacts(ctx context.Context, artifacts *[]v2Artifact.Artifact, visited map[string]string, installed map[string]v2Artifact.Artifact, parentPackageKey string, dependency modDependenceResponse.RemoteModDependencyDTO, modpack bool) error {
	if err := cancel.Check(ctx); err != nil {
		return err
	}
	if dependency.Key == "" || v2Artifact.IsBepInExDependencyKey(dependency.Key) {
		return nil
	}
	namespace := strings.TrimSpace(dependency.Namespace)
	name := strings.TrimSpace(dependency.Name)
	if namespace == "" || name == "" {
		namespace, name = dependencyIdentityFromKey(dependency.Key)
	}
	if namespace == "" || name == "" {
		return fmt.Errorf("invalid Thunderstore dependency: %s", dependency.Raw)
	}
	identity := modIdentity(namespace, name)
	if selected, ok := visited[identity]; ok {
		if modpack && selected != dependency.Key {
			return fmt.Errorf("整合包依赖要求同一模组的不同版本：%s / %s", selected, dependency.Key)
		}
		return nil
	}
	visited[identity] = dependency.Key
	if reused, ok := installed[identity]; ok && (!modpack || reused.ComponentKey == dependency.Key) {
		// 复用实际安装版本的依赖树，防止远端新版本的清单改变本地共享关系。
		if reused.Dependencies == nil {
			installedPackage, err := marketplaceHelper.ResolveRemotePackage(ctx, reused.ComponentKey)
			if err != nil {
				if cancelled := cancel.Check(ctx); cancelled != nil {
					return cancelled
				}
				return err
			}
			reused.Dependencies = installedPackage.Dependencies
		}
		for _, child := range reused.Dependencies {
			if err := appendThunderstoreDependencyArtifacts(ctx, artifacts, visited, installed, parentPackageKey, child, modpack); err != nil {
				return err
			}
		}
		reused.PackageKey = parentPackageKey
		reused.ParentPackageKey = parentPackageKey
		*artifacts = append(*artifacts, reused)
		return nil
	}

	var pack *modDependenceResponse.RemoteModPackageDTO
	var err error
	if modpack {
		pack, err = marketplaceHelper.ResolveRemotePackage(ctx, dependency.Key)
	} else {
		pack, err = marketplaceHelper.ResolveLatestRemotePackage(ctx, namespace, name)
	}
	if err != nil {
		if cancelled := cancel.Check(ctx); cancelled != nil {
			return cancelled
		}
		return err
	}
	for _, child := range pack.Dependencies {
		if err := appendThunderstoreDependencyArtifacts(ctx, artifacts, visited, installed, parentPackageKey, child, modpack); err != nil {
			return err
		}
	}
	*artifacts = append(*artifacts, v2Artifact.FromPackage(parentPackageKey, *pack))
	return nil
}

// hasModpackCategory 判断包是否属于 Thunderstore Modpacks 分类。
// 职责：决定依赖解析是否严格使用清单声明版本。
// 使用：ResolveInstallArtifacts 解析 Thunderstore 包时调用。
// 边界：分类匹配大小写不敏感；其它标签不影响普通 Mod 最新依赖策略。
func hasModpackCategory(tags []string) bool {
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), "Modpacks") {
			return true
		}
	}
	return false
}

// modIdentity 生成与依赖版本无关的 Thunderstore 包身份。
// 职责：按 Owner-Name 合并不同版本依赖。
// 使用：递归去重和已安装依赖过滤时调用。
// 边界：调用方必须提供非空 namespace 和 name。
func modIdentity(namespace string, name string) string {
	return strings.ToLower(strings.TrimSpace(namespace) + "-" + strings.TrimSpace(name))
}

// dependencyIdentityFromKey 从统一 Thunderstore key 提取 Owner-Name。
// 职责：兼容旧 installed 记录或不完整依赖 DTO 缺失独立命名字段的情况。
// 使用：依赖递归和已安装组件过滤在 Namespace/Name 为空时调用。
// 边界：只接受 thunderstore:owner:name:version 形式，格式不完整时返回空值。
func dependencyIdentityFromKey(key string) (string, string) {
	parts := strings.Split(strings.TrimSpace(key), ":")
	if len(parts) != 4 || !strings.EqualFold(parts[0], "thunderstore") || parts[1] == "" || parts[2] == "" {
		return "", ""
	}
	return parts[1], parts[2]
}
