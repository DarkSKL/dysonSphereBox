package installmanifest

// 文件职责：把计划处理的 artifact 和文件目录追加到 V2 安装会话 manifest。
// 使用：依赖解析和 artifact 文件规划完成后调用，便于失败或中断清理空目录。
// 边界：只记录计划，不代表组件已安装；不删除目录、不复制文件、不保存最终 installed 记录。

import (
	"encoding/json"
	"path/filepath"

	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// AppendPlannedArtifacts 追加本次安装计划处理的组件目录。
// 职责：把 artifact 队列转换为清理可用的组件目录计划。
// 使用：依赖解析完成后调用，便于失败清理删除空组件目录。
// 边界：只记录计划，不代表组件已安装；重复组件会被覆盖为最新目录。
func AppendPlannedArtifacts(packageKey string, artifacts []v2Artifact.Artifact) error {
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
	for _, artifact := range artifacts {
		manifest.PlannedComponents = sessionutil.AppendOrReplacePlannedComponent(manifest.PlannedComponents, sessionstate.ComponentPlan{ComponentKey: artifact.ComponentKey, InstallDir: artifact.InstallDirName()})
	}
	return conflictstate.SaveManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

// AppendPlannedFiles 追加计划写入文件的目标目录。
// 职责：记录本次安装可能创建或使用的目标目录，给中断清理兜底。
// 使用：artifact 文件规划完成后、真正复制文件前调用。
// 边界：只记录目录，不删除文件、不判断目录是否可清理。
func AppendPlannedFiles(packageKey string, files []installplan.File) error {
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
	for _, file := range files {
		manifest.PlannedTargetDirs = sessionutil.AppendUniqueTargetDir(manifest.PlannedTargetDirs, filepath.Dir(file.TargetPath))
	}
	return conflictstate.SaveManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}
