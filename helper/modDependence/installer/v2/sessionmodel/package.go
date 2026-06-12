package sessionmodel

// 文件职责：封装 V2 主包安装会话快照与 installed package 模型之间的转换。
// 使用：覆盖安装前捕获旧主包状态，失败清理后恢复旧主包记录。
// 边界：只做字段映射，不访问数据库、不处理文件系统、不修改安装进度；InstallManifest 的重置由恢复调用方明确触发。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/datatypes"
)

// PackageToSnapshot 将 installed package 模型转为会话快照。
// 职责：复制恢复历史主包所需的持久化字段。
// 使用：覆盖安装前捕获旧包状态时调用。
// 边界：不复制数据库自增 ID，不修改 InstallManifest。
func PackageToSnapshot(record installedModel.ModInstalledPackage) *sessionstate.PackageSnapshot {
	return &sessionstate.PackageSnapshot{
		Source:          record.Source,
		SourcePackageID: record.SourcePackageID,
		Namespace:       record.Namespace,
		Name:            record.Name,
		DisplayName:     record.DisplayName,
		Version:         record.Version,
		Description:     record.Description,
		Author:          record.Author,
		DownloadURL:     record.DownloadURL,
		PackageURL:      record.PackageURL,
		CoverURL:        record.CoverURL,
		ArtifactKind:    record.ArtifactKind,
		LocalPath:       record.LocalPath,
		DownloadCount:   record.DownloadCount,
		DependencyCount: record.DependencyCount,
		PackageSize:     record.PackageSize,
		Tags:            record.Tags,
		GameVersions:    record.GameVersions,
		Trustworthiness: record.Trustworthiness,
		InstallState:    record.InstallState,
		InstallStage:    record.InstallStage,
		InstallStatus:   record.InstallStatus,
		InstallProgress: record.InstallProgress,
		ErrorMessage:    record.ErrorMessage,
		CurrentSpeed:    record.CurrentSpeed,
		Raw:             record.Raw,
		InstalledAt:     record.InstalledAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

// SnapshotToPackage 将会话主包快照恢复为 installed package 模型。
// 职责：把失败清理时保存的主包字段转换回可 upsert 的 GORM 模型。
// 使用：恢复覆盖安装前主包状态时调用。
// 边界：InstallManifest 会重置为空对象，避免恢复旧会话污染下一次安装。
func SnapshotToPackage(packageKey string, snapshot sessionstate.PackageSnapshot) installedModel.ModInstalledPackage {
	return installedModel.ModInstalledPackage{
		PackageKey:      packageKey,
		Source:          snapshot.Source,
		SourcePackageID: snapshot.SourcePackageID,
		Namespace:       snapshot.Namespace,
		Name:            snapshot.Name,
		DisplayName:     snapshot.DisplayName,
		Version:         snapshot.Version,
		Description:     snapshot.Description,
		Author:          snapshot.Author,
		DownloadURL:     snapshot.DownloadURL,
		PackageURL:      snapshot.PackageURL,
		CoverURL:        snapshot.CoverURL,
		ArtifactKind:    snapshot.ArtifactKind,
		LocalPath:       snapshot.LocalPath,
		DownloadCount:   snapshot.DownloadCount,
		DependencyCount: snapshot.DependencyCount,
		PackageSize:     snapshot.PackageSize,
		Tags:            snapshot.Tags,
		GameVersions:    snapshot.GameVersions,
		Trustworthiness: snapshot.Trustworthiness,
		InstallManifest: datatypes.JSON([]byte("{}")),
		InstallState:    snapshot.InstallState,
		InstallStage:    snapshot.InstallStage,
		InstallStatus:   snapshot.InstallStatus,
		InstallProgress: snapshot.InstallProgress,
		ErrorMessage:    snapshot.ErrorMessage,
		CurrentSpeed:    snapshot.CurrentSpeed,
		Raw:             snapshot.Raw,
		InstalledAt:     snapshot.InstalledAt,
		UpdatedAt:       snapshot.UpdatedAt,
	}
}
