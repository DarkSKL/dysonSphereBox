package copyfiles

// 文件职责：准备 V2 文件复制的目标路径状态和覆盖备份。
// 使用：copyPlannedFile 在冲突决策允许复制后调用，生成回滚项并补齐文件的旧归属信息。
// 边界：不复制文件、不追加 manifest；目录创建失败或备份失败时返回错误交给外层回滚。

import (
	"DysonSphereBox/helper/modDependence/installer/v2/copyrollback"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"os"
	"path/filepath"
)

// prepareTargetFile 读取目标状态并准备覆盖备份。
// 职责：记录目标是否已存在、加载旧归属记录、必要时备份旧文件并创建目标目录。
// 使用：copyPlannedFile 在真正复制文件前调用。
// 边界：不复制文件；目录创建失败时返回错误并由外层回滚既有步骤。
func (session *copySession) prepareTargetFile(file installplan.File) (installplan.CopyRollback, installplan.File, error) {
	_, statErr := os.Stat(file.TargetPath)
	targetExisted := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return installplan.CopyRollback{}, file, statErr
	}

	rollbackItem := installplan.CopyRollback{TargetPath: file.TargetPath, Existed: targetExisted}
	file.TargetExisted = targetExisted
	previousRecords, err := sessionsnapshot.LoadTargetRecords(session.db, file.TargetPath)
	if err != nil {
		return rollbackItem, file, err
	}
	file.PreviousRecords = sessionsnapshot.FilterPreviousRecords(previousRecords, session.packageKey, session.hasPreviousInstalledPackage)
	if targetExisted {
		backupPath, err := copyrollback.BackupTargetFile(file.TargetPath)
		if err != nil {
			return rollbackItem, file, err
		}
		rollbackItem.BackupPath = backupPath
		file.BackupPath = backupPath
	}
	if err := os.MkdirAll(filepath.Dir(file.TargetPath), 0o755); err != nil {
		return rollbackItem, file, err
	}

	return rollbackItem, file, nil
}
