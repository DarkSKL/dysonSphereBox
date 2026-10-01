package versionreplace

// 文件职责：备份并移除新版本不再包含的旧组件文件。
// 使用：新文件复制成功后、组件记录保存前调用。
// 边界：只处理 Thunderstore 同 Owner-Name 的受管旧文件，不删除未跟踪配置；共享引用在最终提交时切换。
// 逻辑流：1. 查询旧组件 => 2. 保存版本映射 => 3. 对比新旧目标 => 4. 先备份并持久化证据再移除残留。

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"DysonSphereBox/helper/modDependence/installer/v2/copyrollback"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	"DysonSphereBox/helper/modDependence/installer/v2/installpath"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionsnapshot"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// Prepare 移除新版本不再包含的旧文件并保存恢复证据。
// 职责：避免旧 DLL 继续加载，并保留安装失败所需备份。
// 使用：artifact_step 在文件复制成功后调用。
// 边界：复用依赖和非 Thunderstore 跳过；原数据库记录保留到提交，目标必须是 BepInEx 内普通文件。
func Prepare(ctx context.Context, bufferDir string, gameDir string, artifact v2Artifact.Artifact, incoming []installplan.File) error {
	if artifact.ReuseExisting || artifact.Source != sourceConstant.ModSourceThunderstore {
		return nil
	}
	identity := artifactlayout.ModName(artifact.Descriptor())
	if identity == "" {
		return nil
	}
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	var components []installedModel.ModInstalledComponent
	if err := db.Model(&installedModel.ModInstalledComponent{}).Distinct("mod_installed_components.*").
		Joins("JOIN mod_installed_package_components ON mod_installed_package_components.component_key = mod_installed_components.component_key").
		Where("lower(mod_installed_components.source) = ? AND mod_installed_components.component_key <> ?", string(sourceConstant.ModSourceThunderstore), artifact.ComponentKey).
		Find(&components).Error; err != nil {
		return err
	}
	oldKeys := make([]string, 0)
	replacements := make([]sessionstate.VersionReplacement, 0)
	for _, component := range components {
		oldIdentity := artifactlayout.ModName(v2Artifact.FromInstalledComponent(component).Descriptor())
		if strings.EqualFold(identity, oldIdentity) {
			oldKeys = append(oldKeys, component.ComponentKey)
			replacements = append(replacements, sessionstate.VersionReplacement{OldComponentKey: component.ComponentKey, NewComponentKey: artifact.ComponentKey})
		}
	}
	if len(oldKeys) == 0 {
		return nil
	}
	if err := installmanifest.AppendVersionReplacement(artifact.InstallRecordPackageKey(), replacements, nil); err != nil {
		return err
	}
	var records []installedModel.ModInstalledFile
	if err := db.Where("component_key IN ?", oldKeys).Order("target_path ASC").Find(&records).Error; err != nil {
		return err
	}
	newPaths := make(map[string]struct{}, len(incoming))
	for _, file := range incoming {
		newPaths[installpath.NormalizeForCompare(file.TargetPath)] = struct{}{}
	}
	manifest := sessionsnapshot.LoadManifest(artifact.InstallRecordPackageKey())
	for _, file := range manifest.RemovedVersionFiles {
		newPaths[installpath.NormalizeForCompare(file.TargetPath)] = struct{}{}
	}
	for _, record := range records {
		if err := cancel.Check(ctx); err != nil {
			return err
		}
		// 旧版曾把 config 纳入受管清单，新规则升级时只释放旧归属，不能删除用户配置。
		if fileops.IsSubPath(filepath.Join(gameDir, "BepInEx", "config"), record.TargetPath) {
			continue
		}
		pathKey := installpath.NormalizeForCompare(record.TargetPath)
		if _, keep := newPaths[pathKey]; keep {
			continue
		}
		var otherOwners int64
		if err := db.Model(&installedModel.ModInstalledFile{}).
			Where("lower(target_path) IN ? AND component_key NOT IN ?", installpath.ValuesForQuery(record.TargetPath), oldKeys).
			Count(&otherOwners).Error; err != nil {
			return err
		}
		if otherOwners > 0 {
			continue
		}
		if err := removeOldFile(bufferDir, gameDir, artifact.InstallRecordPackageKey(), record); err != nil {
			return err
		}
		newPaths[pathKey] = struct{}{}
	}
	return nil
}

// removeOldFile 先保存可恢复证据，再删除一个受管旧文件。
// 职责：把物理删除置于 manifest 持久化之后，保障取消和中断恢复。
// 使用：Prepare 确认目标未被新文件或其它组件接管后调用。
// 边界：拒绝越界、目录和符号链接；文件缺失跳过，保存失败移除新备份并保留原文件。
func removeOldFile(bufferDir string, gameDir string, packageKey string, record installedModel.ModInstalledFile) error {
	root := filepath.Join(gameDir, "BepInEx")
	if !fileops.IsSubPath(root, record.TargetPath) {
		return fmt.Errorf("旧组件目标超出 BepInEx 目录：%s", record.TargetPath)
	}
	info, err := os.Lstat(record.TargetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("旧组件目标不是普通文件：%s", record.TargetPath)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	realTarget, err := filepath.EvalSymlinks(record.TargetPath)
	if err != nil {
		return err
	}
	if !fileops.IsSubPath(realRoot, realTarget) {
		return fmt.Errorf("旧组件目标解析后超出 BepInEx 目录：%s", record.TargetPath)
	}
	backup, err := copyrollback.BackupTargetFileInDir(record.TargetPath, bufferDir)
	if err != nil {
		return err
	}
	file := sessionstate.File{PackageKey: record.PackageKey, ComponentKey: record.ComponentKey, TargetPath: record.TargetPath, TargetExisted: true, BackupPath: backup}
	if err := installmanifest.AppendVersionReplacement(packageKey, nil, []sessionstate.File{file}); err != nil {
		_ = os.Remove(backup)
		return err
	}
	return os.Remove(record.TargetPath)
}
