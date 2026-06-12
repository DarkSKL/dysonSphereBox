package artifact

// 文件职责：将远端包和依赖 DTO 转换为 V2 安装器内部 artifact。
// 使用：artifactresolve 在解析官方包、Thunderstore 包和手动导入包时调用。
// 边界：只复制和兜底展示字段，不校验下载地址可达性、不解析包内文件布局。

import modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"

// FromDependency 将远端依赖 DTO 转为内部 artifact。
// 职责：统一依赖展示名、下载地址、本地路径和 artifact 类型。
// 使用：官方包依赖和 Thunderstore 依赖转换时调用。
// 边界：不校验下载地址可用性，下载阶段负责失败返回。
func FromDependency(packageKey string, dependency modDependenceResponse.RemoteModDependencyDTO) Artifact {
	displayName := dependency.DisplayName
	if displayName == "" {
		displayName = dependency.Name
	}
	return Artifact{
		PackageKey:       packageKey,
		ParentPackageKey: packageKey,
		ComponentKey:     dependency.Key,
		Source:           dependency.Source,
		Name:             dependency.Name,
		DisplayName:      displayName,
		Version:          dependency.Version,
		DownloadURL:      dependency.DownloadURL,
		LocalPath:        dependency.LocalPath,
		ArtifactKind:     dependency.ArtifactKind,
	}
}

// FromPackage 将远端主包 DTO 转为内部 artifact。
// 职责：把主包自身纳入 artifact 安装队列。
// 使用：Thunderstore 主包、手动导入包或依赖包转换时调用。
// 边界：只复制 DTO 字段，不解析包内文件布局。
func FromPackage(packageKey string, pack modDependenceResponse.RemoteModPackageDTO) Artifact {
	return Artifact{
		PackageKey:       packageKey,
		ParentPackageKey: packageKey,
		ComponentKey:     pack.Key,
		Source:           pack.Source,
		Name:             pack.Name,
		DisplayName:      pack.DisplayName,
		Version:          pack.Version,
		DownloadURL:      pack.DownloadURL,
		LocalPath:        pack.LocalPath,
		ArtifactKind:     pack.ArtifactKind,
	}
}
