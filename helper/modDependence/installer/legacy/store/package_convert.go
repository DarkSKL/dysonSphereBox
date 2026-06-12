package store

// 文件职责：把旧官方整合包 DTO 转换为 legacy 持久化模型。
// 使用：SaveInstalledPackage 入库前调用 packageDTOToModel。
// 边界：只做字段复制、JSON 序列化和依赖 DTO 转换，不访问数据库、不安装文件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
)

// packageDTOToModel 将旧官方整合包 DTO 转为 legacy 持久化模型。
// 职责：复制可空字段、序列化 JSON 字段并转换依赖列表。
// 使用：SaveInstalledPackage 入库前调用。
// 边界：时间解析无法识别时按空时间处理；JSON 序列化失败会返回错误。
func packageDTOToModel(pack modDependenceResponse.ModDependencyPackageDTO) (modDependenceModel.ModDependencyPackage, []modDependenceModel.ModDependency, error) {
	updateTime, err := parseOptionalRemoteTime(pack.UpdateTime)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}
	gameVersions, err := jsonValue(pack.GameVersions)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}
	tags, err := jsonValue(pack.Tags)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}

	dependencies := make([]modDependenceModel.ModDependency, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		dependencyRecord, err := dependencyDTOToModel(dependency)
		if err != nil {
			return modDependenceModel.ModDependencyPackage{}, nil, err
		}
		dependencies = append(dependencies, dependencyRecord)
	}

	return modDependenceModel.ModDependencyPackage{
		ID:                 pack.ID,
		PackageName:        legacyValue.CloneString(pack.PackageName),
		PackageVersion:     legacyValue.CloneString(pack.PackageVersion),
		PackageDescription: legacyValue.CloneString(pack.PackageDescription),
		PackageSize:        legacyValue.CloneInt64(pack.PackageSize),
		DownloadCount:      legacyValue.CloneInt64(pack.DownloadCount),
		DependencyCount:    legacyValue.CloneInt64(pack.DependencyCount),
		GameVersions:       gameVersions,
		Tags:               tags,
		Source:             legacyValue.CloneString(pack.Source),
		Url:                legacyValue.CloneString(pack.Url),
		CoverUrl:           legacyValue.CloneString(pack.CoverUrl),
		Author:             legacyValue.CloneString(pack.Author),
		Status:             legacyValue.CloneInt(pack.Status),
		Trustworthiness:    legacyValue.CloneInt64(pack.Trustworthiness),
		UpdateTime:         updateTime,
		Config:             legacyValue.CloneString(pack.Config),
		Ex:                 legacyValue.CloneString(pack.Ex),
		Dependencies:       dependencies,
	}, dependencies, nil
}
