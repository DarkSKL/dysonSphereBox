package copyrollback

// 文件职责：封装 V2 安装文件复制阶段的回滚与备份文件创建。
// 使用：由 copyfiles.CopyPlannedFiles 在复制失败、取消或覆盖既有目标文件前后调用。
// 边界：只处理当前复制会话的物理文件恢复，不修改安装会话 manifest、不写数据库记录、不记录服务审计日志。
// 逻辑流：1. 覆盖前创建备份 => 2. 复制失败或取消时反向遍历计划 => 3. 恢复备份或删除新文件 => 4. 清理空父目录。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Buffer "DysonSphereBox/helper/modDependence/installer/v2/buffer"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RollbackCopiedFiles 按复制计划反向恢复已经写入的文件。
// 职责：删除新写入文件，或把备份文件还原到原目标路径。
// 使用：复制阶段遇到取消、冲突失败、文件写入错误或会话追加失败时调用。
// 边界：回滚尽力执行且忽略单项错误；上层负责返回原始失败原因。
func RollbackCopiedFiles(plan []installplan.CopyRollback) {
	for index := len(plan) - 1; index >= 0; index-- {
		item := plan[index]
		if item.Existed {
			_ = os.Remove(item.TargetPath)
			if item.BackupPath != "" {
				_ = os.Rename(item.BackupPath, item.TargetPath)
			}
			continue
		}
		_ = os.Remove(item.TargetPath)
		if item.BackupPath != "" {
			_ = os.Remove(item.BackupPath)
		}
		v2Uninstall.CleanupEmptyInstallParentDirs(item.TargetPath)
	}
}

// BackupTargetFile 为即将覆盖的目标文件创建回滚备份。
// 职责：把目标文件复制到安装缓存 rollback 目录，供失败回滚恢复。
// 使用：copyfiles.CopyPlannedFiles 在发现目标文件已存在且允许覆盖时调用。
// 边界：不删除原文件，不判断冲突决策；缓存目录不可用或复制失败时返回错误。
func BackupTargetFile(targetPath string) (string, error) {
	backupDir, err := v2Buffer.Dir()
	if err != nil {
		return "", err
	}
	backupPath := filepath.Join(backupDir, "rollback", fmt.Sprintf("%d-%s", time.Now().UnixNano(), shared.SanitizePathSegment(filepath.Base(targetPath), "file")))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", err
	}
	if err := fileops.CopyFile(targetPath, backupPath, 0o644); err != nil {
		return "", err
	}
	return backupPath, nil
}
