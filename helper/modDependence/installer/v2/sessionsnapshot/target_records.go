package sessionsnapshot

// 文件职责：读取复制目标路径覆盖前的 installed file 归属快照。
// 使用：复制文件前捕获可恢复的历史归属，失败恢复时还原旧记录。
// 边界：只读取数据库记录，不判断物理文件是否存在、不删除或恢复记录。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionmodel"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// LoadTargetRecords 加载目标路径现有的文件归属记录。
// 职责：读取覆盖前目标文件所有匹配路径写法的 installed file 记录。
// 使用：复制文件前捕获可恢复的历史归属时调用。
// 边界：只读取记录，不判断文件是否存在；路径候选由 installpath 生成。
func LoadTargetRecords(db *gorm.DB, targetPath string) ([]sessionstate.FileRecordSnapshot, error) {
	records := make([]installedModel.ModInstalledFile, 0)
	if err := db.Where("lower(target_path) IN ?", installpath.ValuesForQuery(targetPath)).
		Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]sessionstate.FileRecordSnapshot, 0, len(records))
	for _, record := range records {
		result = append(result, sessionmodel.FileRecordToSnapshot(record))
	}
	return result, nil
}

// FilterPreviousRecords 过滤目标路径旧归属记录。
// 职责：避免首次安装时把当前包自身记录当作可恢复旧记录。
// 使用：复制文件捕获 PreviousRecords 后调用。
// 边界：覆盖安装已有主包时保留当前包记录，便于失败恢复历史状态。
func FilterPreviousRecords(records []sessionstate.FileRecordSnapshot, packageKey string, hasPreviousInstalledPackage bool) []sessionstate.FileRecordSnapshot {
	result := make([]sessionstate.FileRecordSnapshot, 0, len(records))
	for _, record := range records {
		if record.PackageKey == packageKey && !hasPreviousInstalledPackage {
			continue
		}
		result = append(result, record)
	}
	return result
}
