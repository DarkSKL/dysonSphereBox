package file

// 文件职责：封装 V2 安装会话文件的物理恢复、共享判断和 manifest 文件移除。
// 使用：失败清理恢复或删除已写文件，复制阶段回滚后同步移除已回滚的会话文件记录。
// 边界：不负责扫描会话文件集合、不写清理审计日志、不删除数据库文件记录；manifest 保存委托 conflictstate。

import (
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/gorm"
	"os"
	"path/filepath"
)

// Restore 恢复单个会话文件的物理状态。
// 职责：优先用备份文件恢复覆盖前目标，或删除首次安装写入的新文件。
// 使用：sessioncleanup/files.CleanupFiles 反向清理文件时调用。
// 边界：目标原本存在但备份缺失时不主动删除；备份读取异常会返回错误。
func Restore(file sessionstate.File) error {
	if file.BackupPath != "" {
		if _, err := os.Stat(file.BackupPath); err == nil {
			_ = os.Remove(file.TargetPath)
			if err := os.MkdirAll(filepath.Dir(file.TargetPath), 0o755); err != nil {
				return err
			}
			return os.Rename(file.BackupPath, file.TargetPath)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !file.TargetExisted {
		return v2Uninstall.RemoveManagedFile(file.TargetPath)
	}
	return nil
}

// IsShared 判断会话文件是否仍被其它包共享。
// 职责：把会话文件 DTO 转为 installed file 记录后复用卸载共享判断。
// 使用：清理未完成安装文件前调用，决定物理文件是否可删除。
// 边界：只根据数据库归属判断共享，不读取文件内容。
func IsShared(db *gorm.DB, packageKey string, file sessionstate.File) (bool, error) {
	record := installedModel.ModInstalledFile{
		PackageKey:   file.PackageKey,
		ComponentKey: file.ComponentKey,
		TargetPath:   file.TargetPath,
	}
	return v2Uninstall.IsFileShared(db, packageKey, record)
}

// RemoveFromManifest 从 manifest 中移除未成功保留的计划文件。
// 职责：复制回滚后同步 InstallManifest，避免后续清理重复处理已回滚文件。
// 使用：copyfiles.CopyPlannedFiles 回滚复制会话时调用。
// 边界：文件列表为空时直接返回；数据库缺少安装记录时返回错误。
func RemoveFromManifest(packageKey string, files []installplan.File) error {
	if len(files) == 0 {
		return nil
	}
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	record := installedModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := sessionstate.Manifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	manifest.InstalledFiles = removePlannedFiles(manifest.InstalledFiles, files)
	return conflictstate.SaveManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

// removePlannedFiles 从会话文件列表中剔除指定计划文件。
// 职责：按 packageKey、componentKey 和 targetPath 精确移除已经回滚的文件。
// 使用：RemoveFromManifest 更新 manifest 前调用。
// 边界：只处理内存切片，不触碰数据库或文件系统。
func removePlannedFiles(sessionFiles []sessionstate.File, plannedFiles []installplan.File) []sessionstate.File {
	result := make([]sessionstate.File, 0, len(sessionFiles))
	for _, sessionFile := range sessionFiles {
		remove := false
		for _, plannedFile := range plannedFiles {
			if sessionFile.PackageKey == plannedFile.PackageKey && sessionFile.ComponentKey == plannedFile.ComponentKey && installpath.Equal(sessionFile.TargetPath, plannedFile.TargetPath) {
				remove = true
				break
			}
		}
		if !remove {
			result = append(result, sessionFile)
		}
	}
	return result
}
