package copyconflict

// 文件职责：释放被 V2 安装覆盖目标文件的旧 installed file 归属记录。
// 使用：copyfiles.CopyPlannedFiles 在用户选择覆盖且复制完成后调用 ReleaseRecords。
// 边界：只删除 packageKey 不同的文件记录，不删除物理文件、不删除组件记录、不提交事务边界。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// ReleaseRecords 释放被覆盖目标文件的其它包归属记录。
// 职责：在用户选择覆盖后删除同目标路径的外部 installed file 记录。
// 使用：copyfiles.CopyPlannedFiles 完成所有文件复制后调用，避免提前释放导致失败时难以恢复。
// 边界：只删除 packageKey 不同的文件记录，不删除物理文件或组件记录。
func ReleaseRecords(db *gorm.DB, file installplan.File) error {
	return db.
		Where("lower(target_path) IN ? AND package_key <> ?", installpath.ValuesForQuery(file.TargetPath), file.PackageKey).
		Delete(&modDependenceModel.ModInstalledFile{}).
		Error
}
