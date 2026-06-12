package store

// 文件职责：把旧官方依赖 DTO 转换为 legacy 依赖模型。
// 使用：packageDTOToModel 遍历依赖列表时调用 dependencyDTOToModel。
// 边界：只转换持久化字段，不校验下载地址可达性、不读取依赖 zip。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
)

// dependencyDTOToModel 将旧官方依赖 DTO 转为 legacy 依赖模型。
// 职责：保留依赖远端 ID、展示字段、来源、下载地址和扩展字段。
// 使用：packageDTOToModel 遍历依赖列表时调用。
// 边界：不校验下载地址可达性，文件安装阶段负责该校验。
func dependencyDTOToModel(dependency modDependenceResponse.ModDependencyDTO) (modDependenceModel.ModDependency, error) {
	updateTime, err := parseOptionalRemoteTime(dependency.UpdateTime)
	if err != nil {
		return modDependenceModel.ModDependency{}, err
	}
	gameVersions, err := jsonValue(dependency.GameVersions)
	if err != nil {
		return modDependenceModel.ModDependency{}, err
	}

	return modDependenceModel.ModDependency{
		ID:              dependency.ID,
		ModName:         legacyValue.CloneString(dependency.ModName),
		ModVersion:      legacyValue.CloneString(dependency.ModVersion),
		ModDescription:  legacyValue.CloneString(dependency.ModDescription),
		GameVersions:    gameVersions,
		UpdateTime:      updateTime,
		Source:          legacyValue.CloneString(dependency.Source),
		Url:             legacyValue.CloneString(dependency.Url),
		Trustworthiness: legacyValue.CloneInt64(dependency.Trustworthiness),
		Config:          legacyValue.CloneString(dependency.Config),
		Ex:              legacyValue.CloneString(dependency.Ex),
	}, nil
}
