package installmanifest

// 文件职责：在移除旧版本文件前持久化恢复证据。
// 使用：versionreplace 记录组件映射和文件备份。
// 边界：只写 manifest，不删除文件、不切换组件引用。

import (
	"encoding/json"
	"slices"

	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// AppendVersionReplacement 保存待提交的组件映射和已备份的旧文件。
// 职责：在文件删除前建立可跨进程恢复的 manifest 证据。
// 使用：版本准备阶段调用，files 为空时只保存组件映射。
// 边界：同目标保留最早备份；读取或保存失败阻止删除原文件。
func AppendVersionReplacement(packageKey string, replacements []sessionstate.VersionReplacement, files []sessionstate.File) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	var record installedModel.ModInstalledPackage
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := sessionstate.Manifest{}
	if len(record.InstallManifest) > 0 {
		if err := json.Unmarshal(record.InstallManifest, &manifest); err != nil {
			return err
		}
	}
	for _, replacement := range replacements {
		if !slices.Contains(manifest.VersionReplacements, replacement) {
			manifest.VersionReplacements = append(manifest.VersionReplacements, replacement)
		}
	}
	for _, file := range files {
		exists := false
		for _, previous := range manifest.RemovedVersionFiles {
			if installpath.Equal(previous.TargetPath, file.TargetPath) {
				exists = true
				break
			}
		}
		if !exists {
			manifest.RemovedVersionFiles = append(manifest.RemovedVersionFiles, file)
		}
	}
	return conflictstate.SaveManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}
