package artifact

// 文件职责：将远端包 DTO 转换为安装器内部 artifact。
// 使用：artifactresolve 在解析 Thunderstore 包和手动导入包时调用。
// 边界：只复制和兜底展示字段，不校验下载地址可达性、不解析包内文件布局。

import modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"

// FromPackage 将远端主包 DTO 转为内部 artifact。
// 职责：把主包自身纳入 artifact 安装队列。
// 使用：Thunderstore 主包、依赖包或手动导入包转换时调用。
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
