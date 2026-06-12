package uninstall

// 文件职责：生成 V2 卸载流程所需的数据库文件计划。
// 使用：正常卸载流程和失败/取消会话清理流程在删除文件前调用 FindPackageFiles 与 BuildPlan。
// 边界：只查询数据库并标记共享状态，不删除文件、不删除主包记录、不发送进度事件。

import (
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// PlannedFile 表示卸载前规划出的目标文件。
// 职责：保存文件记录及其是否仍被其它包共享。
// 使用：上层卸载流程决定删除或保留文件时使用。
// 边界：内部计划类型，不暴露给前端；共享标记只代表规划时刻。
type PlannedFile struct {
	// Record 职责：保存数据库中的已安装文件记录；使用：定位目标文件和归属组件；边界：必须来自 installed 表。
	Record modDependenceModel.ModInstalledFile
	// Shared 职责：标记文件是否仍被其它包引用；使用：卸载时决定跳过删除；边界：只代表规划时刻。
	Shared bool
}

// FindPackageFiles 查找主包卸载时需要处理的文件清单。
// 职责：读取 packageKey 直接归属文件，并兼容通过组件关联归属的历史依赖文件。
// 使用：正常卸载和会话清理构建卸载/清理计划前调用。
// 边界：兼容早期依赖文件误写到依赖 package_key 的记录，通过组件关联补找这些历史残留。
func FindPackageFiles(db *gorm.DB, packageKey string) ([]modDependenceModel.ModInstalledFile, error) {
	fileRecords := make([]modDependenceModel.ModInstalledFile, 0)
	if err := db.
		Where("package_key = ?", packageKey).
		Or("component_key IN (?)", db.
			Model(&modDependenceModel.ModInstalledPackageComponent{}).
			Select("component_key").
			Where("package_key = ?", packageKey),
		).
		Order("target_path ASC").
		Find(&fileRecords).Error; err != nil {
		return nil, err
	}
	return fileRecords, nil
}

// BuildPlan 基于当前安装记录生成卸载文件计划。
// 职责：把数据库文件记录转换为带共享标记的卸载计划。
// 使用：正常卸载流程在真正删除文件前调用。
// 边界：卸载执行阶段只按数据库快照删除文件，不在事务内反复查询共享关系，避免 SQLite 写锁卡住进度。
func BuildPlan(db *gorm.DB, packageKey string, records []modDependenceModel.ModInstalledFile) ([]PlannedFile, error) {
	plan := make([]PlannedFile, 0, len(records))
	for _, record := range records {
		shared, err := IsFileShared(db, packageKey, record)
		if err != nil {
			return nil, err
		}
		plan = append(plan, PlannedFile{Record: record, Shared: shared})
	}
	return plan, nil
}
