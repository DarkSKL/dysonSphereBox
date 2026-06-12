package repair

// 文件职责：更新历史裸露文件迁移后的 installed 文件记录。
// 使用：普通迁移和阻塞目录迁移成功后调用，把修复后的绝对路径和相对路径写回数据库。
// 边界：只更新当前文件记录路径字段，不修改组件、包、manifest 或文件系统。

import (
	"DysonSphereBox/core"
	legacyPlan "DysonSphereBox/helper/modDependence/installer/migration/legacy/plan"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
)

// updateFileRecordPath 更新历史文件记录的修复后路径。
// 职责：把迁移成功后的绝对路径和相对路径写回 installed 文件记录。
// 使用：普通迁移和阻塞目录迁移成功后调用。
// 边界：只更新当前记录路径字段，不修改组件、包或 manifest。
func updateFileRecordPath(item legacyPlan.ScopedFileRepairPlan) error {
	return core.GLOBAL.DB.Model(&modDependenceModel.ModInstalledFile{}).
		Where("id = ?", item.Record.ID).
		Updates(map[string]interface{}{
			"target_path":   item.NextPath,
			"relative_path": item.NextRelativePath,
		}).Error
}
