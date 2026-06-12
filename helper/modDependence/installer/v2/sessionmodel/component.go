package sessionmodel

// 文件职责：封装 V2 组件安装会话快照与 installed component 模型之间的转换。
// 使用：组件写入前捕获旧组件记录，失败清理事务中恢复旧组件基础记录。
// 边界：只做字段映射，不恢复 package-component 关系、不访问数据库、不处理文件系统。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// ComponentToSnapshot 将 installed component 模型转为会话快照。
// 职责：保存组件覆盖前可恢复字段。
// 使用：组件写入前捕获旧组件记录时调用。
// 边界：不保存 package-component 关系，关系恢复由清理事务另行处理。
func ComponentToSnapshot(record installedModel.ModInstalledComponent) sessionstate.ComponentSnapshot {
	return sessionstate.ComponentSnapshot{
		ComponentKey:       record.ComponentKey,
		Source:             record.Source,
		SourceDependencyID: record.SourceDependencyID,
		Namespace:          record.Namespace,
		Name:               record.Name,
		DisplayName:        record.DisplayName,
		Version:            record.Version,
		Description:        record.Description,
		DownloadURL:        record.DownloadURL,
		PackageURL:         record.PackageURL,
		ArtifactKind:       record.ArtifactKind,
		LocalPath:          record.LocalPath,
		Raw:                record.Raw,
		UpdatedAt:          record.UpdatedAt,
	}
}

// SnapshotToComponent 将会话组件快照恢复为 installed component 模型。
// 职责：把 manifest 中保存的组件字段转换为可 upsert 的 GORM 模型。
// 使用：失败清理事务中恢复旧组件记录时调用。
// 边界：不恢复 package-component 关系，只恢复组件基础记录。
func SnapshotToComponent(snapshot sessionstate.ComponentSnapshot) installedModel.ModInstalledComponent {
	return installedModel.ModInstalledComponent{
		ComponentKey:       snapshot.ComponentKey,
		Source:             snapshot.Source,
		SourceDependencyID: snapshot.SourceDependencyID,
		Namespace:          snapshot.Namespace,
		Name:               snapshot.Name,
		DisplayName:        snapshot.DisplayName,
		Version:            snapshot.Version,
		Description:        snapshot.Description,
		DownloadURL:        snapshot.DownloadURL,
		PackageURL:         snapshot.PackageURL,
		ArtifactKind:       snapshot.ArtifactKind,
		LocalPath:          snapshot.LocalPath,
		Raw:                snapshot.Raw,
		UpdatedAt:          snapshot.UpdatedAt,
	}
}
