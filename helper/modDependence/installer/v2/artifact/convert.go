package artifact

// 文件职责：将远端包 DTO 转换为安装器内部 artifact。
// 使用：artifactresolve 在解析 Thunderstore 包和手动导入包时调用。
// 边界：只复制和兜底展示字段，不校验下载地址可达性、不解析包内文件布局。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
)

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
		Namespace:        pack.Namespace,
		Name:             pack.Name,
		DisplayName:      pack.DisplayName,
		Version:          pack.Version,
		DownloadURL:      pack.DownloadURL,
		LocalPath:        pack.LocalPath,
		ArtifactKind:     pack.ArtifactKind,
		Dependencies:     append([]modDependenceResponse.RemoteModDependencyDTO{}, pack.Dependencies...),
	}
}

// FromInstalledComponent 将本地组件记录转换为可复用 artifact。
// 职责：以组件表字段保留真实身份和版本，从 Raw 恢复已保存的依赖清单。
// 使用：installstore 查询本地安装组件时调用，调用方负责设置当前父包 key。
// 边界：Raw 缺失或无依赖字段时返回 nil 清单，由解析器回查该已安装版本；不读取数据库。
func FromInstalledComponent(component installedModel.ModInstalledComponent) Artifact {
	stored := Artifact{}
	if err := json.Unmarshal(component.Raw, &stored); err != nil {
		stored = Artifact{}
	}
	return Artifact{
		ComponentKey:  component.ComponentKey,
		Source:        sourceConstant.ModSource(component.Source),
		Namespace:     component.Namespace,
		Name:          component.Name,
		DisplayName:   component.DisplayName,
		Version:       component.Version,
		DownloadURL:   component.DownloadURL,
		LocalPath:     component.LocalPath,
		ArtifactKind:  artifactConstant.ArtifactKind(component.ArtifactKind),
		ReuseExisting: true,
		Dependencies:  stored.Dependencies,
	}
}
