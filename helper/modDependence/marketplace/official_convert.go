package marketplace

// 文件职责：将官方市场旧 DTO 转换为统一远端包和依赖 DTO。
// 使用：official.go 获取官方响应后调用，用于向服务层返回统一市场数据形状。
// 边界：只转换字段和统一 key，不校验下载地址可达性、不解析压缩包、不写安装记录。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	artifactConstant "DysonSphereBox/constant/modDependence/artifact"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"fmt"
	"strconv"
)

// officialPackageToRemote 将旧官方整合包 DTO 转换为统一远端包 DTO。
// 职责：映射包元数据、下载地址、依赖列表和统一 key。
// 使用：listOfficialRemotePackages 遍历官方接口结果时调用。
// 边界：只转换数据形状，不校验下载 URL 是否可达，不保存数据库。
func officialPackageToRemote(pack modDependenceResponse.ModDependencyPackageDTO) modDependenceResponse.RemoteModPackageDTO {
	version := stringValue(pack.PackageVersion)
	key := fmt.Sprintf("official:%d:%s", pack.ID, sanitizeKeySegment(version, "version"))
	dependencies := make([]modDependenceResponse.RemoteModDependencyDTO, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		dependencies = append(dependencies, officialDependencyToRemote(dependency))
	}

	return modDependenceResponse.RemoteModPackageDTO{
		Key:                 key,
		LegacyID:            pack.ID,
		Source:              sourceConstant.ModSourceOfficial,
		SourcePackageID:     strconv.FormatUint(uint64(pack.ID), 10),
		Name:                stringValue(pack.PackageName),
		DisplayName:         stringValue(pack.PackageName),
		Version:             version,
		Description:         stringValue(pack.PackageDescription),
		Author:              stringValue(pack.Author),
		DownloadURL:         resolveOfficialRemoteURL(stringValue(pack.Url)),
		PackageURL:          resolveOfficialRemoteURL(stringValue(pack.Url)),
		CoverURL:            stringValue(pack.CoverUrl),
		DownloadCount:       int64Value(pack.DownloadCount),
		DependencyCount:     int64Value(pack.DependencyCount),
		PackageSize:         int64Value(pack.PackageSize),
		GameVersions:        pack.GameVersions,
		Trustworthiness:     int64Value(pack.Trustworthiness),
		UpdateTime:          stringValue(pack.UpdateTime),
		ArtifactKind:        artifactConstant.ArtifactKindRemoteZip,
		Dependencies:        dependencies,
		RawDependencySource: "official",
	}
}

// officialDependencyToRemote 将旧官方依赖 DTO 转换为统一依赖 DTO。
// 职责：映射依赖元数据、下载地址和统一 key。
// 使用：officialPackageToRemote 转换依赖列表时调用。
// 边界：不解析压缩包结构，不判断依赖是否已经安装。
func officialDependencyToRemote(dependency modDependenceResponse.ModDependencyDTO) modDependenceResponse.RemoteModDependencyDTO {
	version := stringValue(dependency.ModVersion)
	key := fmt.Sprintf("official:%d:%s", dependency.ID, sanitizeKeySegment(version, "version"))
	return modDependenceResponse.RemoteModDependencyDTO{
		Key:                key,
		Source:             sourceConstant.ModSourceOfficial,
		SourceDependencyID: strconv.FormatUint(uint64(dependency.ID), 10),
		Name:               stringValue(dependency.ModName),
		DisplayName:        stringValue(dependency.ModName),
		Version:            version,
		Description:        stringValue(dependency.ModDescription),
		DownloadURL:        resolveOfficialRemoteURL(stringValue(dependency.Url)),
		PackageURL:         resolveOfficialRemoteURL(stringValue(dependency.Url)),
		ArtifactKind:       artifactConstant.ArtifactKindRemoteZip,
		Raw:                strconv.FormatUint(uint64(dependency.ID), 10),
	}
}
