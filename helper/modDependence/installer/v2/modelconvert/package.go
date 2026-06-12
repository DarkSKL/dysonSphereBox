package modelconvert

// 文件职责：把远端 Mod 包 DTO 转换为本地 installed package 持久化模型。
// 使用：V2 安装器在初始化安装状态或保存安装完成记录时调用 FromRemotePackage。
// 边界：只做字段映射和 JSON 序列化，不写数据库、不修改安装状态、不发送进度事件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
)

// FromRemotePackage 将远端包 DTO 转换为 installed package 模型。
// 职责：复制远端包基础字段，并序列化 tags、gameVersions 和 raw 字段。
// 使用：保存 running 状态和 installed 状态前调用，调用方再补充安装状态字段。
// 边界：不校验 packageKey 或来源合法性；JSON 序列化失败时返回错误交给上层终止安装。
func FromRemotePackage(pack modDependenceResponse.RemoteModPackageDTO) (modDependenceModel.ModInstalledPackage, error) {
	tags, err := json.Marshal(pack.Tags)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	gameVersions, err := json.Marshal(pack.GameVersions)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	now := time.Now()
	return modDependenceModel.ModInstalledPackage{
		PackageKey:      pack.Key,
		Source:          string(pack.Source),
		SourcePackageID: pack.SourcePackageID,
		Namespace:       pack.Namespace,
		Name:            pack.Name,
		DisplayName:     pack.DisplayName,
		Version:         pack.Version,
		Description:     pack.Description,
		Author:          pack.Author,
		DownloadURL:     pack.DownloadURL,
		PackageURL:      pack.PackageURL,
		CoverURL:        pack.CoverURL,
		ArtifactKind:    string(pack.ArtifactKind),
		LocalPath:       pack.LocalPath,
		DownloadCount:   pack.DownloadCount,
		DependencyCount: pack.DependencyCount,
		PackageSize:     pack.PackageSize,
		Tags:            datatypes.JSON(tags),
		GameVersions:    datatypes.JSON(gameVersions),
		Trustworthiness: pack.Trustworthiness,
		Raw:             datatypes.JSON(raw),
		InstalledAt:     now,
		UpdatedAt:       now,
	}, nil
}
