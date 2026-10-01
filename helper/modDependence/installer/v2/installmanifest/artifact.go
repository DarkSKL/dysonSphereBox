package installmanifest

// 文件职责：把 artifact 组件和写入前文件恢复证据追加到 V2 安装会话 manifest。
// 使用：单个文件复制前、数据库组件记录保存前调用 AppendArtifact。
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

// AppendArtifact 追加组件与文件恢复证据到会话 manifest。
// 职责：记录待写组件、旧快照和每次复制前的备份，支撑失败清理与回滚。
// 使用：单文件真正写入前调用，无受管文件的组件在保存记录前调用。
// 边界：只更新 manifest，不创建 installed file 最终记录；数据库缺失安装包时返回错误。
// 逻辑流：1. 读取当前 manifest => 2. 追加组件 key 与旧组件快照 => 3. 合并文件快照 => 4. 保存 running 状态 manifest。
func AppendArtifact(packageKey string, componentKey string, files []installplan.File) error {
	return appendArtifact(packageKey, componentKey, files, false)
}

// AppendReusedArtifact 记录仅复用的组件，不把原有文件视为本次写入。
// 职责：在建立共享关联前保存清理证据，使失败清理仅解除新增引用。
// 使用：安装器 ReuseExisting 分支调用。
// 边界：不修改组件或文件记录；数据库或 manifest 保存失败时返回错误。
func AppendReusedArtifact(packageKey string, componentKey string) error {
	return appendArtifact(packageKey, componentKey, nil, true)
}

// appendArtifact 合并组件与文件证据并保存 manifest。
// 职责：统一处理普通安装和依赖复用的组件快照。
// 使用：AppendArtifact 与 AppendReusedArtifact 委托调用。
// 边界：复用标记只影响清理范围，不判断组件是否可复用。
func appendArtifact(packageKey string, componentKey string, files []installplan.File, reused bool) error {
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
		if err := json.Unmarshal(record.InstallManifest, &manifest); err != nil {
			return err
		}
	}
	manifest.InstalledComponents = sessionutil.AppendUniqueString(manifest.InstalledComponents, componentKey)
	if reused {
		manifest.ReusedComponents = sessionutil.AppendUniqueString(manifest.ReusedComponents, componentKey)
	}
	manifest.PreviousComponents = appendComponentSnapshot(db, manifest.PreviousComponents, componentKey)
	for _, file := range files {
		evidence := sessionstate.File{PackageKey: file.PackageKey, ComponentKey: file.ComponentKey, SourcePath: file.SourcePath, TargetPath: file.TargetPath, RelativePath: file.RelativePath, Sha256: file.Sha256, FileSize: file.FileSize, TargetExisted: file.TargetExisted, BackupPath: file.BackupPath, PreviousRecords: file.PreviousRecords}
		// 每次写入独立保留备份，重复复制和多个组件覆盖同一路径时可以逆序恢复。
		if file.Tracked {
			manifest.InstalledFiles = append(manifest.InstalledFiles, evidence)
		} else {
			manifest.UntrackedFiles = append(manifest.UntrackedFiles, evidence)
		}
	}
	return conflictstate.SaveManifest(packageKey, string(installConstant.StateRunning), record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}
