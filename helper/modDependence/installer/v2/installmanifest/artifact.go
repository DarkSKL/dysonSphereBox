package installmanifest

// 文件职责：把已成功写入的 artifact 组件和文件追加到 V2 安装会话 manifest。
// 使用：单个 artifact 文件复制成功后、数据库组件记录保存前后调用 AppendArtifact。
// 边界：只更新 manifest，不创建最终 installed file 记录；数据库缺失安装包时返回错误。

import (
	"encoding/json"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// AppendArtifact 追加已成功写入的组件与文件到会话 manifest。
// 职责：记录已安装组件、组件旧快照和已复制文件，支撑失败清理与回滚。
// 使用：单个 artifact 文件复制成功后、数据库组件记录保存前后调用。
// 边界：只更新 manifest，不创建 installed file 最终记录；数据库缺失安装包时返回错误。
// 逻辑流：1. 读取当前 manifest => 2. 追加组件 key 与旧组件快照 => 3. 合并文件快照 => 4. 保存 running 状态 manifest。
func AppendArtifact(packageKey string, componentKey string, files []installplan.File) error {
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
	manifest.InstalledComponents = sessionutil.AppendUniqueString(manifest.InstalledComponents, componentKey)
	manifest.PreviousComponents = appendComponentSnapshot(db, manifest.PreviousComponents, componentKey)
	for _, file := range files {
		manifest.InstalledFiles = sessionutil.AppendOrReplaceSessionFile(manifest.InstalledFiles, sessionstate.File{PackageKey: file.PackageKey, ComponentKey: file.ComponentKey, SourcePath: file.SourcePath, TargetPath: file.TargetPath, RelativePath: file.RelativePath, Sha256: file.Sha256, FileSize: file.FileSize, TargetExisted: file.TargetExisted, BackupPath: file.BackupPath, PreviousRecords: file.PreviousRecords})
	}
	return conflictstate.SaveManifest(packageKey, string(installConstant.StateRunning), record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}
