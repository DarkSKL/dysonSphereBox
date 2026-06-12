package directory

// 文件职责：清理 V2 安装会话计划涉及的空目录。
// 使用：失败、中断或取消安装在完成文件/数据库核心清理后调用 CleanupPlannedDirs。
// 边界：只编排目录清理入口，不删除文件记录、不恢复文件、不保存 manifest。

import (
	"path/filepath"
	"strings"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/logging"
	cleanupShared "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	v2Uninstall "DysonSphereBox/helper/modDependence/installer/v2/uninstall"
	"gorm.io/gorm"
	"os"
)

// CleanupPlannedDirs 清理会话计划涉及的空组件目录。
// 职责：按 manifest 中的目标目录和组件目录候选删除安全范围内的空闲目录。
// 使用：核心文件清理完成后调用，用于移除复制过程中创建的空目录。
// 边界：目录不在 BepInEx 管理根内或仍包含其它安装包管理文件时跳过。
// 逻辑流：1. 汇总组件 key => 2. 按深度优先清理计划目标目录 => 3. 清理插件/patcher 组件目录候选 => 4. 向上清理空父目录。
func CleanupPlannedDirs(db *gorm.DB, gameDir string, packageKey string, manifest sessionstate.Manifest) {
	componentKeys := cleanupShared.ComponentKeys(manifest)
	for _, targetDir := range sortedTargetDirs(manifest.PlannedTargetDirs) {
		cleanupTargetDir(db, gameDir, packageKey, targetDir, componentKeys)
	}
	for _, component := range manifest.PlannedComponents {
		cleanupComponentInstallDir(db, gameDir, packageKey, component.InstallDir, componentKeys)
	}
}

// cleanupComponentInstallDir 清理单个组件目录候选。
// 职责：在 plugins 与 patchers 根下移除本次安装组件可能创建的空目录。
// 使用：CleanupPlannedDirs 遍历计划组件时调用。
// 边界：目录越界或仍包含其它包管理文件时跳过。
func cleanupComponentInstallDir(db *gorm.DB, gameDir string, packageKey string, installDir string, componentKeys []string) {
	installDir = strings.TrimSpace(installDir)
	if installDir == "" {
		return
	}
	for _, rootName := range []string{"plugins", "patchers"} {
		rootDir := filepath.Join(gameDir, "BepInEx", rootName)
		targetDir := filepath.Join(rootDir, installDir)
		if !fileops.IsSubPath(rootDir, targetDir) {
			continue
		}
		if hasExternalManagedFilesUnderDir(db, targetDir, packageKey, componentKeys) {
			logging.RecordItem(packageKey, "dir", targetDir, "skip", "组件目录仍包含其它安装包管理的文件，跳过删除", nil)
			continue
		}
		err := os.RemoveAll(targetDir)
		logging.RecordItem(packageKey, "dir", targetDir, "remove", "移除本次安装组件目录兜底候选", err)
		v2Uninstall.CleanupEmptyInstallParentDirs(filepath.Join(targetDir, ".cleanup"))
	}
}
