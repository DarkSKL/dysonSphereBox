package directory

// 文件职责：判断目录下是否仍存在其它包管理的文件或共享组件。
// 使用：计划目录和组件目录删除前调用 hasExternalManagedFilesUnderDir。
// 边界：数据库读取失败时保守返回 true，表示目录不可删除；不删除文件、不写日志。

import (
	"strings"

	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// hasExternalManagedFilesUnderDir 判断目录下是否存在其它包管理的文件。
// 职责：防止清理目录时误删仍被其它安装包引用的文件目录。
// 使用：计划目录和组件目录删除前调用。
// 边界：数据库读取失败时保守返回 true，表示目录不可删除。
func hasExternalManagedFilesUnderDir(db *gorm.DB, dir string, packageKey string, componentKeys []string) bool {
	normalizedDir := strings.TrimRight(installpath.NormalizeForCompare(dir), "/") + "/"
	componentSet := map[string]struct{}{}
	for _, componentKey := range componentKeys {
		componentKey = strings.TrimSpace(componentKey)
		if componentKey != "" {
			componentSet[componentKey] = struct{}{}
		}
	}
	records := make([]installedModel.ModInstalledFile, 0)
	if err := db.Select("package_key", "component_key", "target_path").Find(&records).Error; err != nil {
		return true
	}
	for _, record := range records {
		if !strings.HasPrefix(installpath.NormalizeForCompare(record.TargetPath), normalizedDir) {
			continue
		}
		if record.PackageKey != packageKey {
			return true
		}
		if _, ok := componentSet[record.ComponentKey]; ok && !isComponentSharedByOtherPackage(db, packageKey, record.ComponentKey) {
			continue
		}
		return true
	}
	return false
}

// isComponentSharedByOtherPackage 判断组件是否仍被其它主包引用。
// 职责：辅助目录清理判断当前组件文件是否可以视为本包独占。
// 使用：hasExternalManagedFilesUnderDir 遍历目录文件记录时调用。
// 边界：组件 key 为空返回 false；数据库读取失败时保守返回 true。
func isComponentSharedByOtherPackage(db *gorm.DB, packageKey string, componentKey string) bool {
	if strings.TrimSpace(componentKey) == "" {
		return false
	}
	var count int64
	if err := db.Model(&installedModel.ModInstalledPackageComponent{}).
		Where("package_key <> ? AND component_key = ?", packageKey, componentKey).
		Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}
