package directory

// 文件职责：清理单个 V2 会话计划目标目录。
// 使用：CleanupPlannedDirs 遍历计划目录时调用 cleanupTargetDir。
// 边界：不允许删除 plugins、patchers、config 根目录本身；不满足条件时只记录跳过日志。

import (
	"os"
	"path/filepath"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/logging"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	"gorm.io/gorm"
)

// cleanupTargetDir 清理单个计划目标目录。
// 职责：校验目录位于 BepInEx 管理根内，且没有外部安装包管理文件后删除。
// 使用：CleanupPlannedDirs 遍历计划目录时调用。
// 边界：不允许删除 plugins、patchers、config 根目录本身；不满足条件时只记录跳过日志。
func cleanupTargetDir(db *gorm.DB, gameDir string, packageKey string, targetDir string, componentKeys []string) {
	targetDir = filepath.Clean(targetDir)
	managedRoots := []string{
		filepath.Join(gameDir, "BepInEx", "plugins"),
		filepath.Join(gameDir, "BepInEx", "patchers"),
		filepath.Join(gameDir, "BepInEx", "config"),
	}
	allowed := false
	for _, rootDir := range managedRoots {
		if fileops.IsSubPath(rootDir, targetDir) && !installpath.Equal(rootDir, targetDir) {
			allowed = true
			break
		}
	}
	if !allowed || hasExternalManagedFilesUnderDir(db, targetDir, packageKey, componentKeys) {
		logging.RecordItem(packageKey, "dir", targetDir, "skip", "目录不在可清理范围内或仍包含已管理文件", nil)
		return
	}
	err := os.RemoveAll(targetDir)
	logging.RecordItem(packageKey, "dir", targetDir, "remove", "移除本次安装计划涉及的空闲目录", err)
	v2Uninstall.CleanupEmptyInstallParentDirs(filepath.Join(targetDir, ".cleanup"))
}
