package sessionsnapshot

// 文件职责：合并和去重 V2 会话文件证据。
// 使用：sessioncleanup/files.CleanupFiles 在清理前合并 manifest 与数据库记录时调用。
// 边界：只处理内存切片，不读取数据库、不检查文件是否存在、不决定清理顺序。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// AppendRecordFallbackFiles 使用数据库记录补齐会话文件列表。
// 职责：当 manifest 缺少部分文件时，用 installed file 记录补齐清理目标。
// 使用：sessioncleanup/files.CleanupFiles 清理前合并 manifest 与数据库证据时调用。
// 边界：只补齐未出现的文件，不覆盖已有 manifest 文件信息。
func AppendRecordFallbackFiles(files []sessionstate.File, records []installedModel.ModInstalledFile) []sessionstate.File {
	result := make([]sessionstate.File, 0, len(files)+len(records))
	result = append(result, files...)
	for _, record := range records {
		exists := false
		for _, file := range result {
			if file.PackageKey == record.PackageKey && file.ComponentKey == record.ComponentKey && installpath.Equal(file.TargetPath, record.TargetPath) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		result = append(result, sessionstate.File{
			PackageKey:    record.PackageKey,
			ComponentKey:  record.ComponentKey,
			SourcePath:    record.SourcePath,
			TargetPath:    record.TargetPath,
			RelativePath:  record.RelativePath,
			Sha256:        record.Sha256,
			FileSize:      record.FileSize,
			TargetExisted: false,
		})
	}
	return result
}

// appendUniqueInstalledFileRecord 追加唯一 installed file 记录。
// 职责：按 packageKey 和 targetPath 去重合并数据库文件记录。
// 使用：InstalledFileRecords 合并多种查询来源时调用。
// 边界：只比较目标路径，不比较哈希或文件大小。
func appendUniqueInstalledFileRecord(records []installedModel.ModInstalledFile, next installedModel.ModInstalledFile) []installedModel.ModInstalledFile {
	for _, record := range records {
		if record.PackageKey == next.PackageKey && installpath.Equal(record.TargetPath, next.TargetPath) {
			return records
		}
	}
	return append(records, next)
}

// UniqueFiles 去重会话文件记录。
// 职责：按 packageKey、componentKey 和 targetPath 保留首个文件记录。
// 使用：sessioncleanup/files.CleanupFiles 反向清理文件前调用。
// 边界：不排序，不验证备份路径；调用方决定清理顺序。
func UniqueFiles(files []sessionstate.File) []sessionstate.File {
	result := make([]sessionstate.File, 0, len(files))
	for _, file := range files {
		exists := false
		for _, current := range result {
			if current.PackageKey == file.PackageKey && current.ComponentKey == file.ComponentKey && installpath.Equal(current.TargetPath, file.TargetPath) {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, file)
		}
	}
	return result
}
