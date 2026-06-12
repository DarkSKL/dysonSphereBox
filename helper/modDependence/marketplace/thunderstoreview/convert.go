package thunderstoreview

// 文件职责：封装 Thunderstore 原始包版本到统一市场 DTO 的转换规则。
// 使用：marketplace 根包在列表分页和按 key 解析包时调用，用于生成前端与安装器共同使用的 Remote DTO。
// 边界：只转换数据形状，不加载远端索引、不读写缓存、不判断列表分页和搜索关键字。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"fmt"
	"strings"
)

// PackageToRemote 将 Thunderstore 包版本转换为统一远端包 DTO。
// 职责：映射 Thunderstore 包、版本、依赖和下载元数据到前端统一结构。
// 使用：marketplace 列表过滤和 ResolveRemotePackage 匹配版本后调用。
// 边界：只转换调用方传入的数据，不判断包是否应展示，不下载文件。
func PackageToRemote(pkg thunderstoreAPI.Package, version thunderstoreAPI.Version, packages []thunderstoreAPI.Package) modDependenceResponse.RemoteModPackageDTO {
	dependencies := make([]modDependenceResponse.RemoteModDependencyDTO, 0, len(version.Dependencies))
	for _, dependency := range version.Dependencies {
		dependencies = append(dependencies, parseDependency(dependency, packages))
	}
	return modDependenceResponse.RemoteModPackageDTO{
		Key:                 PackageKey(pkg, version),
		Source:              sourceConstant.ModSourceThunderstore,
		SourcePackageID:     fullName(pkg),
		Namespace:           pkg.Owner,
		Name:                pkg.Name,
		DisplayName:         pkg.Name,
		Version:             version.VersionNumber,
		Description:         version.Description,
		Author:              pkg.Owner,
		DownloadURL:         version.DownloadURL,
		PackageURL:          pkg.PackageURL,
		CoverURL:            version.Icon,
		DownloadCount:       version.Downloads,
		DependencyCount:     int64(len(version.Dependencies) + 1),
		PackageSize:         version.FileSize,
		Tags:                append([]string(nil), pkg.Categories...),
		Trustworthiness:     pkg.RatingScore,
		UpdateTime:          version.DateCreated,
		Deprecated:          pkg.IsDeprecated,
		ArtifactKind:        artifactConstant.ArtifactKindRemoteZip,
		Dependencies:        dependencies,
		RawDependencySource: "thunderstore",
	}
}

// parseDependency 解析 Thunderstore 原始依赖声明。
// 职责：把 `namespace-name-version` 依赖字符串转换为统一依赖 DTO。
// 使用：PackageToRemote 和依赖链判断逻辑调用。
// 边界：格式异常时保留 Raw 并尽量填充名称，不中断市场列表展示。
func parseDependency(raw string, packages []thunderstoreAPI.Package) modDependenceResponse.RemoteModDependencyDTO {
	fullPackageName := ""
	version := ""
	for _, pkg := range packages {
		candidate := fullName(pkg)
		prefix := candidate + "-"
		if strings.HasPrefix(raw, prefix) && len(candidate) > len(fullPackageName) {
			fullPackageName = candidate
			version = strings.TrimPrefix(raw, prefix)
		}
	}
	if fullPackageName == "" {
		parts := strings.Split(raw, "-")
		if len(parts) >= 3 {
			fullPackageName = parts[0] + "-" + parts[1]
			version = strings.Join(parts[2:], "-")
		} else {
			fullPackageName = raw
		}
	}

	namespace, name := splitFullName(fullPackageName)
	key := fmt.Sprintf("thunderstore:%s:%s:%s", namespace, name, version)
	dependency := modDependenceResponse.RemoteModDependencyDTO{
		Key:                key,
		Source:             sourceConstant.ModSourceThunderstore,
		SourceDependencyID: fullPackageName,
		Namespace:          namespace,
		Name:               name,
		DisplayName:        name,
		Version:            version,
		ArtifactKind:       artifactConstant.ArtifactKindRemoteZip,
		Raw:                raw,
	}

	for _, pkg := range packages {
		if fullName(pkg) != fullPackageName {
			continue
		}
		for _, pkgVersion := range pkg.Versions {
			if pkgVersion.VersionNumber != version || !pkgVersion.IsActive {
				continue
			}
			dependency.Description = pkgVersion.Description
			dependency.DownloadURL = pkgVersion.DownloadURL
			dependency.PackageURL = pkg.PackageURL
			dependency.Deprecated = pkg.IsDeprecated
			return dependency
		}
	}

	return dependency
}
