package uninstall

// 文件职责：判断卸载文件或组件是否仍被其它安装包共享。
// 使用：BuildPlan 和卸载数据库事务在删除文件记录或组件记录前调用。
// 边界：只做数据库关系判断，不删除文件、不清理目录、不更新主包状态。

import (
	"strings"

	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// DeleteLegacyComponentFileRecords 清理早期安装器写错归属的依赖文件记录。
// 职责：删除当前包独占组件的历史文件记录，修复早期 packageKey 归属不一致问题。
// 使用：正常卸载的数据库事务内调用。
// 边界：若组件仍被其它主包引用，只保留记录与文件，避免破坏共享依赖。
func DeleteLegacyComponentFileRecords(tx *gorm.DB, packageKey string) error {
	componentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key = ?", packageKey)
	sharedComponentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key <> ?", packageKey)
	return tx.
		Where("component_key IN (?)", componentKeys).
		Where("component_key NOT IN (?)", sharedComponentKeys).
		Delete(&modDependenceModel.ModInstalledFile{}).
		Error
}

// IsFileShared 判断目标文件是否仍被其它安装包引用。
// 职责：根据组件关联和目标路径判断文件是否被其它包共享。
// 使用：BuildPlan 为每个文件记录生成 Shared 标记时调用。
// 边界：历史记录归属异常时优先按 componentKey 判断共享关系，再按目标路径兜底。
func IsFileShared(tx *gorm.DB, packageKey string, record modDependenceModel.ModInstalledFile) (bool, error) {
	var relatedPackageCount int64
	if strings.TrimSpace(record.ComponentKey) != "" {
		if err := tx.Model(&modDependenceModel.ModInstalledPackageComponent{}).
			Where("package_key <> ? AND component_key = ?", packageKey, record.ComponentKey).
			Count(&relatedPackageCount).Error; err != nil {
			return false, err
		}
		if relatedPackageCount > 0 {
			return true, nil
		}
	}

	var sharedFileCount int64
	componentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key = ?", packageKey)
	if err := tx.Model(&modDependenceModel.ModInstalledFile{}).
		Where("lower(target_path) IN ?", installpath.ValuesForQuery(record.TargetPath)).
		Where("NOT (package_key = ? OR component_key IN (?))", packageKey, componentKeys).
		Count(&sharedFileCount).Error; err != nil {
		return false, err
	}
	return sharedFileCount > 0, nil
}

// DeleteUnreferencedComponents 删除未被任何包引用的组件记录。
// 职责：保持 installed component 表不累积孤立组件。
// 使用：卸载包事务删除 package-component 关系后调用。
// 边界：只删除组件表记录，不删除文件；文件删除已在事务外按计划完成。
func DeleteUnreferencedComponents(tx *gorm.DB) error {
	return tx.
		Where("component_key NOT IN (SELECT DISTINCT component_key FROM mod_installed_package_components)").
		Delete(&modDependenceModel.ModInstalledComponent{}).
		Error
}
