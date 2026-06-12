package sessionmodel

// 文件职责：封装 V2 文件记录快照与 installed file 模型之间的转换。
// 使用：复制文件前捕获覆盖前归属，失败清理事务中恢复旧文件记录。
// 边界：只做字段映射，不验证目标文件是否存在、不恢复物理文件、不访问数据库。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// FileRecordToSnapshot 将 installed file 模型转为会话文件记录快照。
// 职责：保存目标文件覆盖前的归属和哈希信息。
// 使用：复制文件前捕获可恢复的历史归属时调用。
// 边界：不保存数据库 ID；恢复时按 packageKey 与 targetPath 匹配。
func FileRecordToSnapshot(record installedModel.ModInstalledFile) sessionstate.FileRecordSnapshot {
	return sessionstate.FileRecordSnapshot{
		PackageKey:   record.PackageKey,
		ComponentKey: record.ComponentKey,
		SourcePath:   record.SourcePath,
		TargetPath:   record.TargetPath,
		RelativePath: record.RelativePath,
		Sha256:       record.Sha256,
		FileSize:     record.FileSize,
		CreatedAt:    record.CreatedAt,
	}
}

// SnapshotToFileRecord 将会话文件记录快照恢复为 installed file 模型。
// 职责：把覆盖前文件归属恢复为可写回数据库的 GORM 模型。
// 使用：失败清理事务中恢复旧文件记录时调用。
// 边界：不验证目标文件是否仍存在；文件系统恢复由清理文件流程负责。
func SnapshotToFileRecord(snapshot sessionstate.FileRecordSnapshot) installedModel.ModInstalledFile {
	return installedModel.ModInstalledFile{
		PackageKey:   snapshot.PackageKey,
		ComponentKey: snapshot.ComponentKey,
		SourcePath:   snapshot.SourcePath,
		TargetPath:   snapshot.TargetPath,
		RelativePath: snapshot.RelativePath,
		Sha256:       snapshot.Sha256,
		FileSize:     snapshot.FileSize,
		CreatedAt:    snapshot.CreatedAt,
	}
}
