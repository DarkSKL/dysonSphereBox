package copyfiles

// 文件职责：执行 V2 安装计划中的单个文件复制步骤。
// 使用：CopyPlannedFiles 在遍历 installplan.File 时调用，将冲突处理、备份、复制、manifest 追加集中在单文件边界。
// 边界：不主动回滚整个会话；冲突、进度和目标准备由同包独立文件维护，返回错误后由外层统一回滚。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"os"
)

// copyPlannedFile 复制单个规划文件并追加安装会话。
// 职责：检查取消状态、备份旧文件、先保存恢复证据再复制目标文件。
// 使用：CopyPlannedFiles 的文件循环逐项调用。
// 边界：冲突选择为跳过时返回 nil 且不加入 installableFiles；错误不在此处回滚整个会话。
// 逻辑流：1. 检查取消 => 2. 发送复制进度 => 3. 处理冲突 => 4. 备份并持久化 manifest => 5. 复制文件并返回受管清单。
func (session *copySession) copyPlannedFile(index int, totalFiles int, file installplan.File) error {
	if err := cancel.Check(session.ctx); err != nil {
		return err
	}

	session.emitProgress(index, totalFiles, file)
	shouldCopy, err := session.prepareConflict(file)
	if err != nil || !shouldCopy {
		return err
	}

	rollbackItem, preparedFile, err := session.prepareTargetFile(file)
	if err != nil {
		return err
	}
	if err := installmanifest.AppendArtifact(session.packageKey, session.artifact.ComponentKey, []installplan.File{preparedFile}); err != nil {
		// 证据保存失败时还未写目标文件，可以直接释放新建备份。
		if rollbackItem.BackupPath != "" {
			_ = os.Remove(rollbackItem.BackupPath)
		}
		return err
	}
	session.rollbackPlan = append(session.rollbackPlan, rollbackItem)
	if err := fileops.CopyFileWithContext(session.ctx, preparedFile.SourcePath, preparedFile.TargetPath, 0o644); err != nil {
		return err
	}

	if err := cancel.Check(session.ctx); err != nil {
		return err
	}
	if !preparedFile.Tracked {
		return nil
	}
	session.installableFiles = append(session.installableFiles, preparedFile)
	return nil
}
