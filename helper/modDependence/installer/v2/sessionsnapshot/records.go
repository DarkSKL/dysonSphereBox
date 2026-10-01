package sessionsnapshot

// 文件职责：按 V2 会话 manifest 与数据库记录收集安装文件证据。
// 使用：失败清理和中断恢复在删除或恢复文件记录前调用 InstalledFileRecords。
// 边界：只读取数据库，不删除记录；manifest 缺失时尽量用会话开始时间窗口补救。

import (
	"slices"
	"strings"
	"time"

	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// InstalledFileRecords 查询当前会话相关的已安装文件记录。
// 职责：按已安装组件、manifest 文件和 startedAt 兜底时间窗口收集数据库文件记录。
// 使用：失败清理和中断恢复在删除或恢复文件记录前调用。
// 边界：只读取数据库，不删除记录；manifest 缺失时尽量用时间窗口补救。
// 逻辑流：1. 按已安装组件查询 => 2. 按 manifest 目标路径补查 => 3. 按 startedAt 时间窗口兜底 => 4. 逐条按目标路径去重。
func InstalledFileRecords(db *gorm.DB, packageKey string, manifest sessionstate.Manifest) ([]installedModel.ModInstalledFile, error) {
	records := make([]installedModel.ModInstalledFile, 0)
	if len(manifest.InstalledComponents) > 0 {
		if err := db.Where("package_key = ? AND component_key IN ?", packageKey, manifest.InstalledComponents).
			Find(&records).Error; err != nil {
			return nil, err
		}
	}
	for _, file := range manifest.InstalledFiles {
		record := installedModel.ModInstalledFile{}
		result := db.Where("package_key = ? AND lower(target_path) IN ?", file.PackageKey, installpath.ValuesForQuery(file.TargetPath)).
			Take(&record)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				continue
			}
			return nil, result.Error
		}
		records = appendUniqueInstalledFileRecord(records, record)
	}
	fallbackRecords, err := fallbackInstalledFileRecords(db, packageKey, manifest)
	if err != nil {
		return nil, err
	}
	for _, record := range fallbackRecords {
		records = appendUniqueInstalledFileRecord(records, record)
	}
	sessionRecords := make([]installedModel.ModInstalledFile, 0, len(records))
	for _, record := range records {
		if !slices.Contains(manifest.ReusedComponents, record.ComponentKey) && !manifest.IsReplacedComponent(record.ComponentKey) && BelongsToSession(record, manifest) {
			sessionRecords = append(sessionRecords, record)
		}
	}
	return sessionRecords, nil
}

// fallbackInstalledFileRecords 按会话开始时间兜底查找文件记录。
// 职责：在 manifest 未完整记录文件时，找出本次会话开始后写入的文件记录。
// 使用：InstalledFileRecords 作为清理证据补充调用。
// 边界：StartedAt 缺失或解析失败时返回空；不跨 packageKey 查询。
func fallbackInstalledFileRecords(db *gorm.DB, packageKey string, manifest sessionstate.Manifest) ([]installedModel.ModInstalledFile, error) {
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(manifest.StartedAt))
	if err != nil || startedAt.IsZero() {
		return nil, nil
	}
	records := make([]installedModel.ModInstalledFile, 0)
	err = db.
		Where("package_key = ? AND created_at >= ?", packageKey, startedAt.Add(-time.Second)).
		Find(&records).Error
	return records, err
}
