package files

// 文件职责：清理 V2 安装会话写入的物理文件，并串联数据库记录恢复。
// 使用：清理入口在补全 manifest 后调用，用于处理失败、中断或取消安装留下的文件和记录。
// 边界：只处理当前安装会话证据内的文件；系统日志由 logging 子包记录，数据库记录清理由 record 子包执行。
// 逻辑流：1. 收集会话文件记录 => 2. 反向恢复或删除物理文件 => 3. 删除本次文件/组件关联记录 => 4. 恢复旧文件和组件记录 => 5. 删除孤立组件并刷新清理日志。

import (
	cleanupFile "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/file"
	"DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/logging"
	cleanupRecord "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/record"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	"gorm.io/gorm"
	"os"
)

// CleanupFiles 清理安装会话写入的文件和数据库记录。
// 职责：按 manifest 和数据库记录恢复覆盖文件、删除未共享文件、恢复旧归属记录和组件快照。
// 使用：sessioncleanup/entry.CleanupInterrupted 在核心清理阶段调用。
// 边界：系统日志在数据库事务后写入，避免 SQLite 写锁嵌套；共享文件只清理记录不删除物理文件。
func CleanupFiles(db *gorm.DB, packageKey string, manifest sessionstate.Manifest) error {
	records, err := sessionsnapshot.InstalledFileRecords(db, packageKey, manifest)
	if err != nil {
		return err
	}
	sessionFiles := sessionsnapshot.UniqueFiles(manifest.InstalledFiles)
	sessionFiles = sessionsnapshot.AppendRecordFallbackFiles(sessionFiles, records)
	for index := len(sessionFiles) - 1; index >= 0; index-- {
		file := sessionFiles[index]
		if len(file.PreviousRecords) > 0 {
			err := cleanupFile.Restore(file)
			logging.RecordItem(packageKey, "file", file.TargetPath, "restore", "目标文件覆盖过旧文件，恢复安装前版本", err)
			if err != nil {
				return err
			}
			continue
		}
		shared, err := cleanupFile.IsShared(db, packageKey, file)
		if err != nil {
			return err
		}
		if !shared {
			err := v2Uninstall.RemoveManagedFile(file.TargetPath)
			logging.RecordItem(packageKey, "file", file.TargetPath, "remove", "移除本次未完成安装写入的文件", err)
			if err != nil {
				return err
			}
		} else {
			logging.RecordItem(packageKey, "file", file.TargetPath, "skip", "目标文件仍被其它安装包引用，跳过删除", nil)
		}
		v2Uninstall.CleanupEmptyInstallParentDirs(file.TargetPath)
		if file.BackupPath != "" {
			err := os.Remove(file.BackupPath)
			if os.IsNotExist(err) {
				err = nil
			}
			logging.RecordItem(packageKey, "backup", file.BackupPath, "remove", "移除安装回滚备份文件", err)
		}
	}
	pendingLogs, transactionErr := cleanupRecord.CleanupRecords(db, packageKey, manifest, sessionFiles, records)
	logging.FlushItems(pendingLogs)
	return transactionErr
}
