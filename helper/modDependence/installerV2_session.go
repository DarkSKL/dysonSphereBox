package modDependence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"DysonSphereBox/model/modDependence"
	modDependenceResponse "DysonSphereBox/model/modDependence/response"
	system "DysonSphereBox/model/system"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type installConflictManifest struct {
	Conflict            *InstallConflictDTO              `json:"conflict,omitempty"`
	Decisions           []InstallConflictDecisionDTO     `json:"decisions,omitempty"`
	StartedAt           string                           `json:"startedAt,omitempty"`
	PlannedComponents   []installSessionComponentPlanDTO `json:"plannedComponents,omitempty"`
	PlannedTargetDirs   []string                         `json:"plannedTargetDirs,omitempty"`
	InstalledComponents []string                         `json:"installedComponents,omitempty"`
	InstalledFiles      []installSessionFileDTO          `json:"installedFiles,omitempty"`
	PreviousPackage     *installSessionPackageDTO        `json:"previousPackage,omitempty"`
	PreviousComponents  []installSessionComponentDTO     `json:"previousComponents,omitempty"`
	CleanedAt           string                           `json:"cleanedAt,omitempty"`
}

type installSessionFileDTO struct {
	PackageKey      string                        `json:"packageKey"`
	ComponentKey    string                        `json:"componentKey"`
	SourcePath      string                        `json:"sourcePath,omitempty"`
	TargetPath      string                        `json:"targetPath"`
	RelativePath    string                        `json:"relativePath"`
	Sha256          string                        `json:"sha256"`
	FileSize        int64                         `json:"fileSize"`
	TargetExisted   bool                          `json:"targetExisted,omitempty"`
	BackupPath      string                        `json:"backupPath,omitempty"`
	PreviousRecords []installSessionFileRecordDTO `json:"previousRecords,omitempty"`
}

type installSessionFileRecordDTO struct {
	PackageKey   string    `json:"packageKey"`
	ComponentKey string    `json:"componentKey"`
	SourcePath   string    `json:"sourcePath"`
	TargetPath   string    `json:"targetPath"`
	RelativePath string    `json:"relativePath"`
	Sha256       string    `json:"sha256"`
	FileSize     int64     `json:"fileSize"`
	CreatedAt    time.Time `json:"createdAt"`
}

type installSessionPackageDTO struct {
	Source          string         `json:"source"`
	SourcePackageID string         `json:"sourcePackageId"`
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	DisplayName     string         `json:"displayName"`
	Version         string         `json:"version"`
	Description     string         `json:"description"`
	Author          string         `json:"author"`
	DownloadURL     string         `json:"downloadUrl"`
	PackageURL      string         `json:"packageUrl"`
	CoverURL        string         `json:"coverUrl"`
	ArtifactKind    string         `json:"artifactKind"`
	LocalPath       string         `json:"localPath"`
	DownloadCount   int64          `json:"downloadCount"`
	DependencyCount int64          `json:"dependencyCount"`
	PackageSize     int64          `json:"packageSize"`
	Tags            datatypes.JSON `json:"tags"`
	GameVersions    datatypes.JSON `json:"gameVersions"`
	Trustworthiness int64          `json:"trustworthiness"`
	InstallState    string         `json:"installState"`
	InstallStage    string         `json:"installStage"`
	InstallStatus   string         `json:"installStatus"`
	InstallProgress int            `json:"installProgress"`
	ErrorMessage    string         `json:"errorMessage"`
	CurrentSpeed    int64          `json:"currentSpeed"`
	Raw             datatypes.JSON `json:"raw"`
	InstalledAt     time.Time      `json:"installedAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

type installSessionComponentDTO struct {
	ComponentKey       string         `json:"componentKey"`
	Source             string         `json:"source"`
	SourceDependencyID string         `json:"sourceDependencyId"`
	Namespace          string         `json:"namespace"`
	Name               string         `json:"name"`
	DisplayName        string         `json:"displayName"`
	Version            string         `json:"version"`
	Description        string         `json:"description"`
	DownloadURL        string         `json:"downloadUrl"`
	PackageURL         string         `json:"packageUrl"`
	ArtifactKind       string         `json:"artifactKind"`
	LocalPath          string         `json:"localPath"`
	Raw                datatypes.JSON `json:"raw"`
	UpdatedAt          time.Time      `json:"updatedAt"`
}

type installSessionComponentPlanDTO struct {
	ComponentKey string `json:"componentKey"`
	InstallDir   string `json:"installDir"`
}

type installCleanupLogItem struct {
	packageKey string
	kind       string
	target     string
	action     string
	reason     string
	err        error
}

func resetInstallSessionManifest(packageKey string, previousPackage *installSessionPackageDTO) error {
	return saveInstallConflictManifest(packageKey, installProgressStateRunning, "prepare", "准备安装", 1, "", &installConflictManifest{
		PreviousPackage: previousPackage,
		StartedAt:       time.Now().Format(time.RFC3339Nano),
	})
}

func capturePreviousInstallSessionPackageSnapshot(packageKey string) (*installSessionPackageDTO, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if record.InstallState != installProgressStateInstalled && record.InstallState != "" {
		return nil, nil
	}
	return packageModelToSessionPackage(record), nil
}

func appendInstallSessionArtifact(packageKey string, componentKey string, files []plannedInstallFile) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	manifest.InstalledComponents = appendUniqueString(manifest.InstalledComponents, componentKey)
	manifest.PreviousComponents = appendSessionComponentSnapshot(db, manifest.PreviousComponents, componentKey)
	for _, file := range files {
		manifest.InstalledFiles = appendOrReplaceSessionFile(manifest.InstalledFiles, installSessionFileDTO{
			PackageKey:      file.PackageKey,
			ComponentKey:    file.ComponentKey,
			SourcePath:      file.SourcePath,
			TargetPath:      file.TargetPath,
			RelativePath:    file.RelativePath,
			Sha256:          file.Sha256,
			FileSize:        file.FileSize,
			TargetExisted:   file.TargetExisted,
			BackupPath:      file.BackupPath,
			PreviousRecords: file.PreviousRecords,
		})
	}
	return saveInstallConflictManifest(packageKey, installProgressStateRunning, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

func appendInstallSessionPlannedArtifacts(packageKey string, artifacts []installArtifact) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	for _, artifact := range artifacts {
		manifest.PlannedComponents = appendOrReplacePlannedComponent(manifest.PlannedComponents, installSessionComponentPlanDTO{
			ComponentKey: artifact.ComponentKey,
			InstallDir:   artifactInstallDirName(artifact),
		})
	}
	return saveInstallConflictManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

func appendInstallSessionPlannedFiles(packageKey string, files []plannedInstallFile) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	for _, file := range files {
		manifest.PlannedTargetDirs = appendUniqueInstallTargetDir(manifest.PlannedTargetDirs, filepath.Dir(file.TargetPath))
	}
	return saveInstallConflictManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

func appendOrReplacePlannedComponent(components []installSessionComponentPlanDTO, next installSessionComponentPlanDTO) []installSessionComponentPlanDTO {
	if strings.TrimSpace(next.ComponentKey) == "" || strings.TrimSpace(next.InstallDir) == "" {
		return components
	}
	result := make([]installSessionComponentPlanDTO, 0, len(components)+1)
	replaced := false
	for _, component := range components {
		if component.ComponentKey == next.ComponentKey {
			result = append(result, next)
			replaced = true
			continue
		}
		result = append(result, component)
	}
	if !replaced {
		result = append(result, next)
	}
	return result
}

func appendSessionComponentSnapshot(db *gorm.DB, snapshots []installSessionComponentDTO, componentKey string) []installSessionComponentDTO {
	componentKey = strings.TrimSpace(componentKey)
	if componentKey == "" {
		return snapshots
	}
	for _, snapshot := range snapshots {
		if snapshot.ComponentKey == componentKey {
			return snapshots
		}
	}
	component := modDependence.ModInstalledComponent{}
	if err := db.Where("component_key = ?", componentKey).Take(&component).Error; err != nil {
		return snapshots
	}
	return append(snapshots, componentModelToSessionComponent(component))
}

func appendUniqueString(values []string, next string) []string {
	next = strings.TrimSpace(next)
	if next == "" {
		return values
	}
	for _, value := range values {
		if value == next {
			return values
		}
	}
	return append(values, next)
}

func appendUniqueInstallTargetDir(values []string, next string) []string {
	next = strings.TrimSpace(filepath.Clean(next))
	if next == "" || next == "." {
		return values
	}
	for _, value := range values {
		if sameInstallTargetPath(value, next) {
			return values
		}
	}
	return append(values, next)
}

func appendOrReplaceSessionFile(files []installSessionFileDTO, next installSessionFileDTO) []installSessionFileDTO {
	result := make([]installSessionFileDTO, 0, len(files)+1)
	replaced := false
	for _, file := range files {
		if sameInstallTargetPath(file.TargetPath, next.TargetPath) && file.PackageKey == next.PackageKey && file.ComponentKey == next.ComponentKey {
			result = append(result, next)
			replaced = true
			continue
		}
		result = append(result, file)
	}
	if !replaced {
		result = append(result, next)
	}
	return result
}

func cleanupInterruptedRemotePackageInstall(packageKey string, status string) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		return err
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	manifest = enrichInstallSessionCleanupManifest(db, record, manifest)
	if strings.TrimSpace(manifest.CleanedAt) != "" {
		recordInstallCleanupItem(packageKey, "session", "", "skip", "清理会话已完成，跳过重复清理", nil)
		return nil
	}
	recordInstallCleanupStart(record, status, manifest)
	if err := cleanupInstallSessionFiles(db, record.PackageKey, manifest); err != nil {
		recordInstallCleanupDone(record.PackageKey, status, manifest, err)
		return err
	}
	manifest.CleanedAt = time.Now().Format(time.RFC3339Nano)
	if err := saveInstallConflictManifest(record.PackageKey, installProgressStateFailed, "failed", status, clampProgress(record.InstallProgress), status, &manifest); err != nil {
		recordInstallCleanupDone(record.PackageKey, status, manifest, err)
		return err
	}
	if err := restorePreviousInstallSessionPackage(db, record.PackageKey, manifest); err != nil {
		recordInstallCleanupDone(record.PackageKey, status, manifest, err)
		return err
	}
	gameDir, err := GetConfiguredGameDir()
	if err == nil {
		cleanupInstallSessionPlannedDirs(db, gameDir, record.PackageKey, manifest)
		cleanupEmptyManagedDirs(filepath.Join(gameDir, "BepInEx"))
	} else {
		recordInstallCleanupItem(record.PackageKey, "gameDir", "", "skip", "未获取到游戏目录，跳过目录兜底清理", err)
	}
	if bufferDir, err := modDependencyV2BufferDir(); err == nil {
		cleanupInstallSessionBuffers(bufferDir, record.PackageKey, manifest)
	} else {
		recordInstallCleanupItem(record.PackageKey, "buffer", "", "skip", "未获取到安装缓存目录，跳过缓存兜底清理", err)
	}
	recordInstallCleanupDone(record.PackageKey, status, manifest, nil)
	return nil
}

func hasInstallSessionCleanupEvidence(manifest installConflictManifest) bool {
	return len(manifest.InstalledComponents) > 0 ||
		len(manifest.InstalledFiles) > 0 ||
		strings.TrimSpace(manifest.StartedAt) != ""
}

func cleanupFailedRemotePackageInstall(packageKey string, err error) error {
	status := "安装失败"
	if IsInstallCancelledError(err) {
		status = "安装已取消"
	}
	if cleanupErr := cleanupInterruptedRemotePackageInstall(packageKey, status); cleanupErr != nil {
		return fmt.Errorf("%w；清理失败：%v", err, cleanupErr)
	}
	return err
}

func checkInstallCancelledWithCleanup(ctx context.Context, packageKey string) error {
	if err := checkInstallCancelled(ctx); err != nil {
		return cleanupFailedRemotePackageInstall(packageKey, err)
	}
	return nil
}

func enrichInstallSessionCleanupManifest(db *gorm.DB, record modDependence.ModInstalledPackage, manifest installConflictManifest) installConflictManifest {
	for _, fileRecord := range loadCleanupFileRecords(db, record.PackageKey) {
		if strings.TrimSpace(fileRecord.ComponentKey) != "" {
			manifest.InstalledComponents = appendUniqueString(manifest.InstalledComponents, fileRecord.ComponentKey)
		}
		manifest.InstalledFiles = appendOrReplaceSessionFile(manifest.InstalledFiles, installSessionFileDTO{
			PackageKey:    fileRecord.PackageKey,
			ComponentKey:  fileRecord.ComponentKey,
			SourcePath:    fileRecord.SourcePath,
			TargetPath:    fileRecord.TargetPath,
			RelativePath:  fileRecord.RelativePath,
			Sha256:        fileRecord.Sha256,
			FileSize:      fileRecord.FileSize,
			TargetExisted: false,
		})
		manifest.PlannedTargetDirs = appendUniqueInstallTargetDir(manifest.PlannedTargetDirs, filepath.Dir(fileRecord.TargetPath))
	}
	for _, component := range loadCleanupComponents(db, record.PackageKey, manifest.InstalledComponents) {
		manifest.InstalledComponents = appendUniqueString(manifest.InstalledComponents, component.ComponentKey)
		manifest.PlannedComponents = appendOrReplacePlannedComponent(manifest.PlannedComponents, installSessionComponentPlanDTO{
			ComponentKey: component.ComponentKey,
			InstallDir: installArtifact{
				ComponentKey: component.ComponentKey,
				DisplayName:  component.DisplayName,
				Name:         component.Name,
			}.installDirName(),
		})
	}
	manifest = appendRawPackageCleanupPlans(record, manifest)
	return manifest
}

func loadCleanupFileRecords(db *gorm.DB, packageKey string) []modDependence.ModInstalledFile {
	records, err := findPackageUninstallFiles(db, packageKey)
	if err != nil {
		return nil
	}
	return records
}

func loadCleanupComponents(db *gorm.DB, packageKey string, componentKeys []string) []modDependence.ModInstalledComponent {
	keys := append([]string(nil), componentKeys...)
	relationKeys := make([]string, 0)
	if err := db.Model(&modDependence.ModInstalledPackageComponent{}).
		Where("package_key = ?", packageKey).
		Pluck("component_key", &relationKeys).Error; err == nil {
		for _, key := range relationKeys {
			keys = appendUniqueString(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	components := make([]modDependence.ModInstalledComponent, 0)
	_ = db.Where("component_key IN ?", keys).Find(&components).Error
	return components
}

func appendRawPackageCleanupPlans(record modDependence.ModInstalledPackage, manifest installConflictManifest) installConflictManifest {
	pack := modDependenceResponse.RemoteModPackageDTO{}
	if len(record.Raw) == 0 || json.Unmarshal(record.Raw, &pack) != nil {
		return manifest
	}
	manifest.PlannedComponents = appendOrReplacePlannedComponent(manifest.PlannedComponents, installSessionComponentPlanDTO{
		ComponentKey: pack.Key,
		InstallDir: installArtifact{
			ComponentKey: pack.Key,
			DisplayName:  pack.DisplayName,
			Name:         pack.Name,
		}.installDirName(),
	})
	for _, dependency := range pack.Dependencies {
		if isBepInExDependencyKey(dependency.Key) {
			continue
		}
		manifest.PlannedComponents = appendOrReplacePlannedComponent(manifest.PlannedComponents, installSessionComponentPlanDTO{
			ComponentKey: dependency.Key,
			InstallDir: installArtifact{
				ComponentKey: dependency.Key,
				DisplayName:  dependency.DisplayName,
				Name:         dependency.Name,
			}.installDirName(),
		})
	}
	return manifest
}

func cleanupInstallSessionFiles(db *gorm.DB, packageKey string, manifest installConflictManifest) error {
	records, err := sessionInstalledFileRecords(db, packageKey, manifest)
	if err != nil {
		return err
	}
	sessionFiles := uniqueSessionFiles(manifest.InstalledFiles)
	sessionFiles = appendRecordFallbackSessionFiles(sessionFiles, records)
	for index := len(sessionFiles) - 1; index >= 0; index-- {
		file := sessionFiles[index]
		if len(file.PreviousRecords) > 0 {
			err := restoreInstallSessionFile(file)
			recordInstallCleanupItem(packageKey, "file", file.TargetPath, "restore", "目标文件覆盖过旧文件，恢复安装前版本", err)
			if err != nil {
				return err
			}
			continue
		}
		shared, err := isSessionFileShared(db, packageKey, file)
		if err != nil {
			return err
		}
		if !shared {
			err := removeManagedFile(file.TargetPath)
			recordInstallCleanupItem(packageKey, "file", file.TargetPath, "remove", "移除本次未完成安装写入的文件", err)
			if err != nil {
				return err
			}
		} else {
			recordInstallCleanupItem(packageKey, "file", file.TargetPath, "skip", "目标文件仍被其它安装包引用，跳过删除", nil)
		}
		cleanupEmptyInstallParentDirs(file.TargetPath)
		if file.BackupPath != "" {
			err := os.Remove(file.BackupPath)
			if os.IsNotExist(err) {
				err = nil
			}
			recordInstallCleanupItem(packageKey, "backup", file.BackupPath, "remove", "移除安装回滚备份文件", err)
		}
	}
	pendingLogs := make([]installCleanupLogItem, 0)
	appendPendingLog := func(kind string, target string, action string, reason string, err error) {
		pendingLogs = append(pendingLogs, installCleanupLogItem{
			packageKey: packageKey,
			kind:       kind,
			target:     target,
			action:     action,
			reason:     reason,
			err:        err,
		})
	}
	transactionErr := db.Transaction(func(tx *gorm.DB) error {
		for _, record := range records {
			err := tx.Where("package_key = ? AND lower(target_path) IN ?", record.PackageKey, normalizeInstallTargetPathValuesForQuery(record.TargetPath)).
				Delete(&modDependence.ModInstalledFile{}).Error
			if err != nil {
				appendPendingLog("record", record.TargetPath, "remove", "清理未完成安装的文件记录", err)
				return err
			}
			appendPendingLog("record", record.TargetPath, "remove", "清理未完成安装的文件记录", nil)
		}
		if len(manifest.InstalledComponents) > 0 {
			if err := tx.Where("package_key = ? AND component_key IN ?", packageKey, manifest.InstalledComponents).
				Delete(&modDependence.ModInstalledPackageComponent{}).Error; err != nil {
				appendPendingLog("record", "", "remove", "清理未完成安装的组件关联记录", err)
				return err
			}
			appendPendingLog("record", "", "remove", "清理未完成安装的组件关联记录", nil)
		}
		for _, file := range sessionFiles {
			for _, previousRecord := range file.PreviousRecords {
				record := sessionFileRecordToModel(previousRecord)
				if err := tx.Where("package_key = ? AND lower(target_path) IN ?", record.PackageKey, normalizeInstallTargetPathValuesForQuery(record.TargetPath)).
					Assign(record).
					FirstOrCreate(&record).Error; err != nil {
					appendPendingLog("record", record.TargetPath, "restore", "恢复安装前的文件归属记录", err)
					return err
				}
				appendPendingLog("record", record.TargetPath, "restore", "恢复安装前的文件归属记录", nil)
			}
		}
		for _, component := range manifest.PreviousComponents {
			model := sessionComponentToModel(component)
			if strings.TrimSpace(model.ComponentKey) == "" {
				continue
			}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "component_key"}}, UpdateAll: true}).Create(&model).Error; err != nil {
				appendPendingLog("record", model.ComponentKey, "restore", "恢复安装前的组件记录", err)
				return err
			}
			appendPendingLog("record", model.ComponentKey, "restore", "恢复安装前的组件记录", nil)
		}
		return deleteUnreferencedV2Components(tx)
	})
	flushInstallCleanupLogItems(pendingLogs)
	return transactionErr
}

func cleanupInstallSessionPlannedDirs(db *gorm.DB, gameDir string, packageKey string, manifest installConflictManifest) {
	componentKeys := installSessionCleanupComponentKeys(manifest)
	for _, targetDir := range sortedInstallSessionTargetDirs(manifest.PlannedTargetDirs) {
		cleanupInstallSessionTargetDir(db, gameDir, packageKey, targetDir, componentKeys)
	}
	for _, component := range manifest.PlannedComponents {
		installDir := strings.TrimSpace(component.InstallDir)
		if installDir == "" {
			continue
		}
		for _, rootName := range []string{"plugins", "patchers"} {
			rootDir := filepath.Join(gameDir, "BepInEx", rootName)
			targetDir := filepath.Join(rootDir, installDir)
			if !isSubPath(rootDir, targetDir) {
				continue
			}
			if hasExternalManagedFilesUnderDir(db, targetDir, packageKey, componentKeys) {
				recordInstallCleanupItem(packageKey, "dir", targetDir, "skip", "组件目录仍包含其它安装包管理的文件，跳过删除", nil)
				continue
			}
			err := os.RemoveAll(targetDir)
			recordInstallCleanupItem(packageKey, "dir", targetDir, "remove", "移除本次安装组件目录兜底候选", err)
			cleanupEmptyInstallParentDirs(filepath.Join(targetDir, ".cleanup"))
		}
	}
}

func sortedInstallSessionTargetDirs(dirs []string) []string {
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(filepath.Clean(dir))
		if dir == "" || dir == "." {
			continue
		}
		result = appendUniqueInstallTargetDir(result, dir)
	}
	sort.SliceStable(result, func(leftIndex int, rightIndex int) bool {
		return len(result[leftIndex]) > len(result[rightIndex])
	})
	return result
}

func cleanupInstallSessionTargetDir(db *gorm.DB, gameDir string, packageKey string, targetDir string, componentKeys []string) {
	targetDir = filepath.Clean(targetDir)
	managedRoots := []string{
		filepath.Join(gameDir, "BepInEx", "plugins"),
		filepath.Join(gameDir, "BepInEx", "patchers"),
		filepath.Join(gameDir, "BepInEx", "config"),
	}
	allowed := false
	for _, rootDir := range managedRoots {
		if isSubPath(rootDir, targetDir) && !sameInstallTargetPath(rootDir, targetDir) {
			allowed = true
			break
		}
	}
	if !allowed || hasExternalManagedFilesUnderDir(db, targetDir, packageKey, componentKeys) {
		recordInstallCleanupItem(packageKey, "dir", targetDir, "skip", "目录不在可清理范围内或仍包含已管理文件", nil)
		return
	}
	err := os.RemoveAll(targetDir)
	recordInstallCleanupItem(packageKey, "dir", targetDir, "remove", "移除本次安装计划涉及的空闲目录", err)
	cleanupEmptyInstallParentDirs(filepath.Join(targetDir, ".cleanup"))
}

func recordInstallCleanupStart(record modDependence.ModInstalledPackage, status string, manifest installConflictManifest) {
	recordRemotePackageCleanupLog("cleanupRemotePackageInstallStart", system.LogLevelInfo, "开始清理未完成的 Thunderstore Mod 包安装", nil, map[string]interface{}{
		"packageKey":              record.PackageKey,
		"packageName":             record.DisplayName,
		"source":                  record.Source,
		"status":                  status,
		"installStage":            record.InstallStage,
		"installStatus":           record.InstallStatus,
		"installProgress":         record.InstallProgress,
		"startedAt":               manifest.StartedAt,
		"plannedComponentCount":   len(manifest.PlannedComponents),
		"plannedTargetDirCount":   len(manifest.PlannedTargetDirs),
		"installedComponentCount": len(manifest.InstalledComponents),
		"installedFileCount":      len(manifest.InstalledFiles),
	})
}

func recordInstallCleanupItem(packageKey string, kind string, target string, action string, reason string, err error) {
	level := system.LogLevelInfo
	success := true
	if err != nil {
		level = system.LogLevelError
		success = false
	}
	content := fmt.Sprintf("Thunderstore Mod 包清理：%s %s", action, kind)
	recordRemotePackageCleanupLog("cleanupRemotePackageInstallItem", level, content, err, map[string]interface{}{
		"packageKey": packageKey,
		"kind":       kind,
		"target":     target,
		"action":     action,
		"reason":     reason,
		"success":    success,
	})
}

// flushInstallCleanupLogItems 在数据库写事务完成后再写系统日志。
// 边界：SQLite 同库事务内不能嵌套写 system_logs，否则取消安装时容易等待写锁。
func flushInstallCleanupLogItems(items []installCleanupLogItem) {
	for _, item := range items {
		recordInstallCleanupItem(item.packageKey, item.kind, item.target, item.action, item.reason, item.err)
	}
}

func recordInstallCleanupDone(packageKey string, status string, manifest installConflictManifest, err error) {
	level := system.LogLevelInfo
	event := "cleanupRemotePackageInstallDone"
	content := "Thunderstore Mod 包未完成安装清理完成"
	if err != nil {
		level = system.LogLevelError
		event = "cleanupRemotePackageInstallFailed"
		content = "Thunderstore Mod 包未完成安装清理失败"
	}
	recordRemotePackageCleanupLog(event, level, content, err, map[string]interface{}{
		"packageKey":              packageKey,
		"status":                  status,
		"plannedComponentCount":   len(manifest.PlannedComponents),
		"plannedTargetDirCount":   len(manifest.PlannedTargetDirs),
		"installedComponentCount": len(manifest.InstalledComponents),
		"installedFileCount":      len(manifest.InstalledFiles),
		"cleanedAt":               manifest.CleanedAt,
		"success":                 err == nil,
	})
}

func hasExternalManagedFilesUnderDir(db *gorm.DB, dir string, packageKey string, componentKeys []string) bool {
	normalizedDir := strings.TrimRight(normalizeInstallTargetPathForCompare(dir), "/") + "/"
	componentSet := map[string]struct{}{}
	for _, componentKey := range componentKeys {
		componentKey = strings.TrimSpace(componentKey)
		if componentKey != "" {
			componentSet[componentKey] = struct{}{}
		}
	}
	records := make([]modDependence.ModInstalledFile, 0)
	if err := db.Select("package_key", "component_key", "target_path").Find(&records).Error; err != nil {
		return true
	}
	for _, record := range records {
		if !strings.HasPrefix(normalizeInstallTargetPathForCompare(record.TargetPath), normalizedDir) {
			continue
		}
		if record.PackageKey != packageKey {
			return true
		}
		if _, ok := componentSet[record.ComponentKey]; ok && !isComponentSharedByOtherPackage(db, packageKey, record.ComponentKey) {
			continue
		}
		return true
	}
	return false
}

func isComponentSharedByOtherPackage(db *gorm.DB, packageKey string, componentKey string) bool {
	if strings.TrimSpace(componentKey) == "" {
		return false
	}
	var count int64
	if err := db.Model(&modDependence.ModInstalledPackageComponent{}).
		Where("package_key <> ? AND component_key = ?", packageKey, componentKey).
		Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

func installSessionCleanupComponentKeys(manifest installConflictManifest) []string {
	keys := make([]string, 0, len(manifest.InstalledComponents)+len(manifest.PlannedComponents))
	for _, key := range manifest.InstalledComponents {
		keys = appendUniqueString(keys, key)
	}
	for _, component := range manifest.PlannedComponents {
		keys = appendUniqueString(keys, component.ComponentKey)
	}
	return keys
}

func cleanupInstallSessionBuffers(bufferDir string, packageKey string, manifest installConflictManifest) {
	for _, componentKey := range appendUniqueString(installSessionCleanupComponentKeys(manifest), packageKey) {
		if isBepInExDependencyKey(componentKey) {
			continue
		}
		segment := sanitizePathSegment(componentKey, "artifact")
		targets := []string{
			filepath.Join(bufferDir, "downloads", segment+".zip.partial"),
			filepath.Join(bufferDir, fmt.Sprintf("extract-%s", segment)),
		}
		for _, target := range targets {
			if !isSubPath(bufferDir, target) {
				recordInstallCleanupItem(packageKey, "buffer", target, "skip", "缓存路径不在安装缓存目录内，跳过清理", nil)
				continue
			}
			err := os.RemoveAll(target)
			if os.IsNotExist(err) {
				err = nil
			}
			recordInstallCleanupItem(packageKey, "buffer", target, "remove", "移除本次未完成安装的下载或解压缓存", err)
		}
	}
}

func appendRecordFallbackSessionFiles(files []installSessionFileDTO, records []modDependence.ModInstalledFile) []installSessionFileDTO {
	result := make([]installSessionFileDTO, 0, len(files)+len(records))
	result = append(result, files...)
	for _, record := range records {
		exists := false
		for _, file := range result {
			if file.PackageKey == record.PackageKey && file.ComponentKey == record.ComponentKey && sameInstallTargetPath(file.TargetPath, record.TargetPath) {
				exists = true
				break
			}
		}
		if exists {
			continue
		}
		result = append(result, installSessionFileDTO{
			PackageKey:    record.PackageKey,
			ComponentKey:  record.ComponentKey,
			SourcePath:    record.SourcePath,
			TargetPath:    record.TargetPath,
			RelativePath:  record.RelativePath,
			Sha256:        record.Sha256,
			FileSize:      record.FileSize,
			TargetExisted: false,
		})
	}
	return result
}

func sessionInstalledFileRecords(db *gorm.DB, packageKey string, manifest installConflictManifest) ([]modDependence.ModInstalledFile, error) {
	records := make([]modDependence.ModInstalledFile, 0)
	if len(manifest.InstalledComponents) > 0 {
		if err := db.Where("package_key = ? AND component_key IN ?", packageKey, manifest.InstalledComponents).
			Find(&records).Error; err != nil {
			return nil, err
		}
	}
	for _, file := range manifest.InstalledFiles {
		record := modDependence.ModInstalledFile{}
		result := db.Where("package_key = ? AND lower(target_path) IN ?", file.PackageKey, normalizeInstallTargetPathValuesForQuery(file.TargetPath)).
			Take(&record)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				continue
			}
			return nil, result.Error
		}
		records = appendUniqueInstalledFileRecord(records, record)
	}
	fallbackRecords, err := sessionFallbackInstalledFileRecords(db, packageKey, manifest)
	if err != nil {
		return nil, err
	}
	for _, record := range fallbackRecords {
		records = appendUniqueInstalledFileRecord(records, record)
	}
	return records, nil
}

func sessionFallbackInstalledFileRecords(db *gorm.DB, packageKey string, manifest installConflictManifest) ([]modDependence.ModInstalledFile, error) {
	startedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(manifest.StartedAt))
	if err != nil || startedAt.IsZero() {
		return nil, nil
	}
	records := make([]modDependence.ModInstalledFile, 0)
	err = db.
		Where("package_key = ? AND created_at >= ?", packageKey, startedAt.Add(-time.Second)).
		Find(&records).Error
	return records, err
}

func appendUniqueInstalledFileRecord(records []modDependence.ModInstalledFile, next modDependence.ModInstalledFile) []modDependence.ModInstalledFile {
	for _, record := range records {
		if record.PackageKey == next.PackageKey && sameInstallTargetPath(record.TargetPath, next.TargetPath) {
			return records
		}
	}
	return append(records, next)
}

func uniqueSessionFiles(files []installSessionFileDTO) []installSessionFileDTO {
	result := make([]installSessionFileDTO, 0, len(files))
	for _, file := range files {
		exists := false
		for _, current := range result {
			if current.PackageKey == file.PackageKey && current.ComponentKey == file.ComponentKey && sameInstallTargetPath(current.TargetPath, file.TargetPath) {
				exists = true
				break
			}
		}
		if !exists {
			result = append(result, file)
		}
	}
	return result
}

func restoreInstallSessionFile(file installSessionFileDTO) error {
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
		return removeManagedFile(file.TargetPath)
	}
	return nil
}

func isSessionFileShared(db *gorm.DB, packageKey string, file installSessionFileDTO) (bool, error) {
	record := modDependence.ModInstalledFile{
		PackageKey:   file.PackageKey,
		ComponentKey: file.ComponentKey,
		TargetPath:   file.TargetPath,
	}
	return isUninstallFileShared(db, packageKey, record)
}

func removeInstallSessionFiles(packageKey string, files []plannedInstallFile) error {
	if len(files) == 0 {
		return nil
	}
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", packageKey).Take(&record).Error; err != nil {
		return err
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	manifest.InstalledFiles = removePlannedFilesFromSessionManifest(manifest.InstalledFiles, files)
	return saveInstallConflictManifest(packageKey, record.InstallState, record.InstallStage, record.InstallStatus, record.InstallProgress, record.ErrorMessage, &manifest)
}

func removePlannedFilesFromSessionManifest(sessionFiles []installSessionFileDTO, plannedFiles []plannedInstallFile) []installSessionFileDTO {
	result := make([]installSessionFileDTO, 0, len(sessionFiles))
	for _, sessionFile := range sessionFiles {
		remove := false
		for _, plannedFile := range plannedFiles {
			if sessionFile.PackageKey == plannedFile.PackageKey && sessionFile.ComponentKey == plannedFile.ComponentKey && sameInstallTargetPath(sessionFile.TargetPath, plannedFile.TargetPath) {
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

func loadInstallTargetRecords(db *gorm.DB, targetPath string) ([]installSessionFileRecordDTO, error) {
	records := make([]modDependence.ModInstalledFile, 0)
	if err := db.Where("lower(target_path) IN ?", normalizeInstallTargetPathValuesForQuery(targetPath)).
		Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]installSessionFileRecordDTO, 0, len(records))
	for _, record := range records {
		result = append(result, fileRecordModelToSession(record))
	}
	return result, nil
}

func filterInstallTargetPreviousRecords(records []installSessionFileRecordDTO, packageKey string, hasPreviousInstalledPackage bool) []installSessionFileRecordDTO {
	result := make([]installSessionFileRecordDTO, 0, len(records))
	for _, record := range records {
		if record.PackageKey == packageKey && !hasPreviousInstalledPackage {
			continue
		}
		result = append(result, record)
	}
	return result
}

func loadInstallConflictManifest(packageKey string) installConflictManifest {
	db, err := requireDatabase()
	if err != nil {
		return installConflictManifest{}
	}
	record := modDependence.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		return installConflictManifest{}
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	return manifest
}

func cleanupInstallSessionBackups(packageKey string) error {
	manifest := loadInstallConflictManifest(packageKey)
	for _, file := range manifest.InstalledFiles {
		if file.BackupPath != "" {
			_ = os.Remove(file.BackupPath)
		}
	}
	return nil
}

func restorePreviousInstallSessionPackage(db *gorm.DB, packageKey string, manifest installConflictManifest) error {
	if manifest.PreviousPackage == nil {
		return nil
	}
	record := sessionPackageToModel(packageKey, *manifest.PreviousPackage)
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "package_key"}}, UpdateAll: true}).Create(&record).Error
}

func packageModelToSessionPackage(record modDependence.ModInstalledPackage) *installSessionPackageDTO {
	return &installSessionPackageDTO{
		Source:          record.Source,
		SourcePackageID: record.SourcePackageID,
		Namespace:       record.Namespace,
		Name:            record.Name,
		DisplayName:     record.DisplayName,
		Version:         record.Version,
		Description:     record.Description,
		Author:          record.Author,
		DownloadURL:     record.DownloadURL,
		PackageURL:      record.PackageURL,
		CoverURL:        record.CoverURL,
		ArtifactKind:    record.ArtifactKind,
		LocalPath:       record.LocalPath,
		DownloadCount:   record.DownloadCount,
		DependencyCount: record.DependencyCount,
		PackageSize:     record.PackageSize,
		Tags:            record.Tags,
		GameVersions:    record.GameVersions,
		Trustworthiness: record.Trustworthiness,
		InstallState:    record.InstallState,
		InstallStage:    record.InstallStage,
		InstallStatus:   record.InstallStatus,
		InstallProgress: record.InstallProgress,
		ErrorMessage:    record.ErrorMessage,
		CurrentSpeed:    record.CurrentSpeed,
		Raw:             record.Raw,
		InstalledAt:     record.InstalledAt,
		UpdatedAt:       record.UpdatedAt,
	}
}

func sessionPackageToModel(packageKey string, snapshot installSessionPackageDTO) modDependence.ModInstalledPackage {
	return modDependence.ModInstalledPackage{
		PackageKey:      packageKey,
		Source:          snapshot.Source,
		SourcePackageID: snapshot.SourcePackageID,
		Namespace:       snapshot.Namespace,
		Name:            snapshot.Name,
		DisplayName:     snapshot.DisplayName,
		Version:         snapshot.Version,
		Description:     snapshot.Description,
		Author:          snapshot.Author,
		DownloadURL:     snapshot.DownloadURL,
		PackageURL:      snapshot.PackageURL,
		CoverURL:        snapshot.CoverURL,
		ArtifactKind:    snapshot.ArtifactKind,
		LocalPath:       snapshot.LocalPath,
		DownloadCount:   snapshot.DownloadCount,
		DependencyCount: snapshot.DependencyCount,
		PackageSize:     snapshot.PackageSize,
		Tags:            snapshot.Tags,
		GameVersions:    snapshot.GameVersions,
		Trustworthiness: snapshot.Trustworthiness,
		InstallManifest: datatypes.JSON([]byte("{}")),
		InstallState:    snapshot.InstallState,
		InstallStage:    snapshot.InstallStage,
		InstallStatus:   snapshot.InstallStatus,
		InstallProgress: snapshot.InstallProgress,
		ErrorMessage:    snapshot.ErrorMessage,
		CurrentSpeed:    snapshot.CurrentSpeed,
		Raw:             snapshot.Raw,
		InstalledAt:     snapshot.InstalledAt,
		UpdatedAt:       snapshot.UpdatedAt,
	}
}

func componentModelToSessionComponent(record modDependence.ModInstalledComponent) installSessionComponentDTO {
	return installSessionComponentDTO{
		ComponentKey:       record.ComponentKey,
		Source:             record.Source,
		SourceDependencyID: record.SourceDependencyID,
		Namespace:          record.Namespace,
		Name:               record.Name,
		DisplayName:        record.DisplayName,
		Version:            record.Version,
		Description:        record.Description,
		DownloadURL:        record.DownloadURL,
		PackageURL:         record.PackageURL,
		ArtifactKind:       record.ArtifactKind,
		LocalPath:          record.LocalPath,
		Raw:                record.Raw,
		UpdatedAt:          record.UpdatedAt,
	}
}

func sessionComponentToModel(snapshot installSessionComponentDTO) modDependence.ModInstalledComponent {
	return modDependence.ModInstalledComponent{
		ComponentKey:       snapshot.ComponentKey,
		Source:             snapshot.Source,
		SourceDependencyID: snapshot.SourceDependencyID,
		Namespace:          snapshot.Namespace,
		Name:               snapshot.Name,
		DisplayName:        snapshot.DisplayName,
		Version:            snapshot.Version,
		Description:        snapshot.Description,
		DownloadURL:        snapshot.DownloadURL,
		PackageURL:         snapshot.PackageURL,
		ArtifactKind:       snapshot.ArtifactKind,
		LocalPath:          snapshot.LocalPath,
		Raw:                snapshot.Raw,
		UpdatedAt:          snapshot.UpdatedAt,
	}
}

func fileRecordModelToSession(record modDependence.ModInstalledFile) installSessionFileRecordDTO {
	return installSessionFileRecordDTO{
		PackageKey:   record.PackageKey,
		ComponentKey: record.ComponentKey,
		SourcePath:   record.SourcePath,
		TargetPath:   record.TargetPath,
		RelativePath: record.RelativePath,
		Sha256:       record.Sha256,
		FileSize:     record.FileSize,
		CreatedAt:    record.CreatedAt,
	}
}

func sessionFileRecordToModel(snapshot installSessionFileRecordDTO) modDependence.ModInstalledFile {
	return modDependence.ModInstalledFile{
		PackageKey:   snapshot.PackageKey,
		ComponentKey: snapshot.ComponentKey,
		SourcePath:   snapshot.SourcePath,
		TargetPath:   snapshot.TargetPath,
		RelativePath: snapshot.RelativePath,
		Sha256:       snapshot.Sha256,
		FileSize:     snapshot.FileSize,
		CreatedAt:    snapshot.CreatedAt,
	}
}
