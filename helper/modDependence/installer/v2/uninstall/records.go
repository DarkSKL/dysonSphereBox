package uninstall

// 文件职责：在主包卸载时释放或移交其文件记录。
// 使用：卸载事务在删除包组件关系前调用，保留共享依赖的可卸载文件清单。
// 边界：只改数据库归属，不移动或删除物理文件，不修改组件元数据。

import (
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// ReleasePackageFileRecords 清理当前包独占记录并移交共享组件文件。
// 职责：防止卸载原安装包后，共享文件仍存在但清单丢失。
// 使用：正常卸载事务在删除 package-component 关系前调用。
// 边界：必须使用调用方事务；按组件引用移交，目标路径被其它组件管理时仅删除当前记录。
// 逻辑流：1. 查询当前包文件 => 2. 查找仍引用组件的其它包 => 3. 移交共享文件归属或删除独占记录。
func ReleasePackageFileRecords(tx *gorm.DB, packageKey string) error {
	var records []installedModel.ModInstalledFile
	if err := tx.Where("package_key = ?", packageKey).Find(&records).Error; err != nil {
		return err
	}
	for _, record := range records {
		var relation installedModel.ModInstalledPackageComponent
		result := tx.Where("package_key <> ? AND component_key = ?", packageKey, record.ComponentKey).
			Order("package_key ASC").Take(&relation)
		if result.Error == gorm.ErrRecordNotFound {
			if err := tx.Delete(&record).Error; err != nil {
				return err
			}
			continue
		}
		if result.Error != nil {
			return result.Error
		}
		if err := tx.Model(&record).Update("package_key", relation.PackageKey).Error; err != nil {
			return err
		}
	}
	return nil
}
