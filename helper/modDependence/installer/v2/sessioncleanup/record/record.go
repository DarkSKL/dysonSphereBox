package record

// 文件职责：清理或恢复 V2 安装会话涉及的文件记录、组件关系和旧组件快照。
// 使用：失败、中断或取消安装在物理文件清理后进入数据库事务时调用。
// 边界：只处理数据库记录和待刷新日志项，不删除物理文件、不扫描 manifest、不写系统日志。
// 逻辑流：1. 删除本次未完成安装文件记录 => 2. 删除本次组件关联并恢复旧关联 => 3. 恢复覆盖前文件归属 => 4. 恢复旧组件记录 => 5. 删除孤立组件。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionmodel"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// CleanupRecords 清理未完成安装记录并恢复覆盖前快照。
// 职责：在单个事务中删除本次文件/组件关系，恢复旧文件归属和组件记录。
// 使用：sessioncleanup/files.CleanupFiles 完成物理文件处理后调用。
// 边界：返回待写系统日志项，调用方需在事务结束后刷新，避免 SQLite 写锁嵌套。
func CleanupRecords(db *gorm.DB, packageKey string, manifest sessionstate.Manifest, sessionFiles []sessionstate.File, records []installedModel.ModInstalledFile) ([]sessionstate.CleanupLogItem, error) {
	pendingLogs := make([]sessionstate.CleanupLogItem, 0)
	appendPendingLog := func(kind string, target string, action string, reason string, err error) {
		pendingLogs = append(pendingLogs, sessionstate.CleanupLogItem{
			PackageKey: packageKey,
			Kind:       kind,
			Target:     target,
			Action:     action,
			Reason:     reason,
			Err:        err,
		})
	}
	transactionErr := db.Transaction(func(tx *gorm.DB) error {
		for _, record := range records {
			err := tx.Where("package_key = ? AND lower(target_path) IN ?", record.PackageKey, installpath.ValuesForQuery(record.TargetPath)).
				Delete(&installedModel.ModInstalledFile{}).Error
			if err != nil {
				appendPendingLog("record", record.TargetPath, "remove", "清理未完成安装的文件记录", err)
				return err
			}
			appendPendingLog("record", record.TargetPath, "remove", "清理未完成安装的文件记录", nil)
		}
		if len(manifest.InstalledComponents) > 0 {
			if err := tx.Where("package_key = ? AND component_key IN ?", packageKey, manifest.InstalledComponents).
				Delete(&installedModel.ModInstalledPackageComponent{}).Error; err != nil {
				appendPendingLog("record", "", "remove", "清理未完成安装的组件关联记录", err)
				return err
			}
			appendPendingLog("record", "", "remove", "清理未完成安装的组件关联记录", nil)
		}
		for _, componentKey := range manifest.PreviousPackageComponents {
			relation := installedModel.ModInstalledPackageComponent{PackageKey: packageKey, ComponentKey: componentKey}
			if err := tx.Where("package_key = ? AND component_key = ?", packageKey, componentKey).
				FirstOrCreate(&relation).Error; err != nil {
				return err
			}
		}
		for index := len(sessionFiles) - 1; index >= 0; index-- {
			file := sessionFiles[index]
			for _, previousRecord := range file.PreviousRecords {
				startedAt, _ := time.Parse(time.RFC3339Nano, manifest.StartedAt)
				// 同一会话前一轮复制的记录不是旧归属，恢复它会留下已删除新文件的孤立清单。
				if previousRecord.PackageKey == packageKey && (manifest.PreviousPackage == nil || (!startedAt.IsZero() && !previousRecord.CreatedAt.Before(startedAt))) {
					continue
				}
				record := sessionmodel.SnapshotToFileRecord(previousRecord)
				if err := tx.Where("package_key = ? AND lower(target_path) IN ?", record.PackageKey, installpath.ValuesForQuery(record.TargetPath)).
					Assign(record).
					FirstOrCreate(&record).Error; err != nil {
					appendPendingLog("record", record.TargetPath, "restore", "恢复安装前的文件归属记录", err)
					return err
				}
				appendPendingLog("record", record.TargetPath, "restore", "恢复安装前的文件归属记录", nil)
			}
		}
		for _, component := range manifest.PreviousComponents {
			model := sessionmodel.SnapshotToComponent(component)
			if strings.TrimSpace(model.ComponentKey) == "" {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "component_key"}}, UpdateAll: true}).Create(&model).Error; err != nil {
				appendPendingLog("record", model.ComponentKey, "restore", "恢复安装前的组件记录", err)
				return err
			}
			appendPendingLog("record", model.ComponentKey, "restore", "恢复安装前的组件记录", nil)
		}
		return v2Uninstall.DeleteUnreferencedComponents(tx)
	})
	return pendingLogs, transactionErr
}
