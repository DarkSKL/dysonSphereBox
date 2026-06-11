package modDependence

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"
	modDependenceResponse "DysonSphereBox/model/modDependence/response"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const modDependencyV2BufferDirName = "modDependenciesV2"

// installRemotePackageTestHooks 只在集成测试中使用，用来把取消精确卡在安装阶段边界。
// 边界：生产环境保持 nil，不改变真实安装流程。
var installRemotePackageTestHooks struct {
	AfterPlannedFiles  func(packageKey string, componentKey string)
	AfterArtifactFiles func(packageKey string, componentKey string)
	AfterSaveArtifact  func(packageKey string, componentKey string)
}

type installArtifact struct {
	PackageKey string
	// ParentPackageKey 表示用户实际点击安装的主包 key。
	// 用途：Thunderstore 依赖安装时，进度和文件清单仍归属主包，卸载主包才能清理依赖文件。
	ParentPackageKey string
	ComponentKey     string
	Source           modDependenceResponse.ModSource
	Name             string
	DisplayName      string
	Version          string
	DownloadURL      string
	LocalPath        string
	ArtifactKind     modDependenceResponse.ArtifactKind
}

type plannedInstallFile struct {
	PackageKey      string `json:"packageKey"`
	ComponentKey    string `json:"componentKey"`
	SourcePath      string `json:"sourcePath"`
	TargetPath      string `json:"targetPath"`
	RelativePath    string `json:"relativePath"`
	Sha256          string `json:"sha256"`
	FileSize        int64  `json:"fileSize"`
	TargetExisted   bool
	BackupPath      string
	PreviousRecords []installSessionFileRecordDTO
}

// InstallConflictAction 表示用户对安装冲突的处理方式。
// 用途：前端弹窗选择覆盖、跳过或终止安装后传回安装器继续执行。
type InstallConflictAction string

const (
	// InstallConflictActionOverwrite 覆盖当前目标文件，并把该文件的管理权转给当前安装包。
	InstallConflictActionOverwrite InstallConflictAction = "overwrite"
	// InstallConflictActionSkip 跳过当前目标文件，不写入当前安装包文件清单。
	InstallConflictActionSkip InstallConflictAction = "skip"
	// InstallConflictActionFail 终止当前安装，并保留失败状态给用户重新处理。
	InstallConflictActionFail InstallConflictAction = "fail"
)

// InstallConflictDTO 描述一次需要用户处理的文件冲突。
// 用途：通过 Wails 事件、安装快照和继续安装接口在前后端之间传递冲突上下文。
type InstallConflictDTO struct {
	PackageKey               string `json:"packageKey"`
	ComponentKey             string `json:"componentKey"`
	ComponentName            string `json:"componentName"`
	ComponentVersion         string `json:"componentVersion"`
	TargetPath               string `json:"targetPath"`
	RelativePath             string `json:"relativePath"`
	IncomingSha256           string `json:"incomingSha256"`
	ExistingSha256           string `json:"existingSha256"`
	ExistingPackageKey       string `json:"existingPackageKey"`
	ExistingPackageName      string `json:"existingPackageName"`
	ExistingComponentKey     string `json:"existingComponentKey"`
	ExistingComponentName    string `json:"existingComponentName"`
	ExistingComponentVersion string `json:"existingComponentVersion"`
	Message                  string `json:"message"`
}

// InstallRemotePackageOptions 表示安装器可恢复执行时使用的用户决策。
// 边界：冲突决策优先按具体文件匹配；同一组件的后续版本冲突会复用用户选择，避免 DLL/PDB/XML 等文件逐个弹窗。
type InstallRemotePackageOptions struct {
	Decisions    []InstallConflictDecisionDTO `json:"decisions"`
	ResetSession bool                         `json:"-"`
}

// InstallConflictDecisionDTO 记录用户对一次组件文件冲突的选择。
type InstallConflictDecisionDTO struct {
	TargetPath   string                `json:"targetPath"`
	ComponentKey string                `json:"componentKey"`
	Action       InstallConflictAction `json:"action"`
}

type installFileConflictError struct {
	Conflict InstallConflictDTO
}

func (err *installFileConflictError) Error() string {
	return err.Conflict.Message
}

type plannedUninstallFile struct {
	record modDependenceModel.ModInstalledFile
	shared bool
}

type plannedCopyRollback struct {
	targetPath string
	backupPath string
	existed    bool
}

// InstallRemoteModPackage 安装 Thunderstore Mod 包。
// 使用：支持国内源、Thunderstore 与手动导入模型；Thunderstore 会先递归安装依赖。
// 边界：所有写入文件都会记录清单，便于卸载时精确清理 patchers、plugins 与 config。
func InstallRemoteModPackage(pack modDependenceResponse.RemoteModPackageDTO) (*PackageInstallResult, error) {
	return InstallRemoteModPackageWithOptions(pack, InstallRemotePackageOptions{ResetSession: true})
}

// InstallRemoteModPackageWithOptions 带用户冲突决策安装 Thunderstore Mod 包。
// 使用：首次安装不传决策；用户处理冲突后传入具体文件的覆盖、跳过或失败选择继续安装。
func InstallRemoteModPackageWithOptions(pack modDependenceResponse.RemoteModPackageDTO, options InstallRemotePackageOptions) (*PackageInstallResult, error) {
	if strings.TrimSpace(pack.Key) == "" {
		return nil, fmt.Errorf("package key is required")
	}
	installCtx, unregisterInstallCancellation := registerInstallCancellation(pack.Key)
	defer unregisterInstallCancellation()

	var previousPackage *installSessionPackageDTO
	if options.ResetSession {
		var captureErr error
		previousPackage, captureErr = capturePreviousInstallSessionPackageSnapshot(pack.Key)
		if captureErr != nil {
			return nil, captureErr
		}
	}
	if err := saveRemotePackageInstallState(pack, installProgressStateRunning, "prepare", "准备安装", 1, "", 0); err != nil {
		return nil, err
	}
	if options.ResetSession {
		if err := resetInstallSessionManifest(pack.Key, previousPackage); err != nil {
			return nil, err
		}
	}
	emitPackageProgress(pack.Key, "prepare", "准备安装", 1)
	if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
		return nil, err
	}

	bepInExResult, err := InstallBepInEx()
	if err != nil {
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		return nil, fmt.Errorf("BepInEx is not ready: %s", strings.Join(bepInExResult.Status.MissingFiles, ", "))
	}
	emitPackageProgress(pack.Key, "prepare", "BepInEx 已就绪", 8)
	if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
		return nil, err
	}

	gameDir := bepInExResult.Status.GameDir
	bufferDir, err := modDependencyV2BufferDir()
	if err != nil {
		return nil, err
	}

	artifacts, err := resolveInstallArtifacts(pack)
	if err != nil {
		return nil, err
	}
	if err := appendInstallSessionPlannedArtifacts(pack.Key, artifacts); err != nil {
		return nil, err
	}
	emitPackageProgress(pack.Key, "prepare", "依赖解析完成", 12)
	if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
		return nil, err
	}

	results := make([]DependencyInstallResult, 0, len(artifacts))
	for index, artifact := range artifacts {
		if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
			return nil, err
		}
		artifactStart := 12 + index*78/len(artifacts)
		artifactEnd := 12 + (index+1)*78/len(artifacts)
		recordPercent := maxInt(artifactStart, artifactEnd-3)
		emitPackageProgress(pack.Key, "install", fmt.Sprintf("正在处理 %s", artifact.DisplayName), artifactStart)
		fileRecords, zipPath, err := installArtifactFiles(installCtx, bufferDir, gameDir, artifact, artifactStart, recordPercent, options)
		if err != nil {
			if IsInstallConflictError(err) {
				if conflictErr := persistInstallConflict(pack, err, options); conflictErr != nil {
					return nil, conflictErr
				}
				return nil, err
			}
			return nil, cleanupFailedRemotePackageInstall(pack.Key, wrapArtifactInstallError("install", artifact, err))
		}
		if installRemotePackageTestHooks.AfterArtifactFiles != nil {
			installRemotePackageTestHooks.AfterArtifactFiles(pack.Key, artifact.ComponentKey)
		}
		if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
			return nil, err
		}
		emitPackageProgress(pack.Key, "record", fmt.Sprintf("正在记录 %s", artifact.DisplayName), recordPercent)
		if err := saveInstalledArtifact(pack, artifact, fileRecords); err != nil {
			return nil, cleanupFailedRemotePackageInstall(pack.Key, wrapArtifactInstallError("record", artifact, err))
		}
		if installRemotePackageTestHooks.AfterSaveArtifact != nil {
			installRemotePackageTestHooks.AfterSaveArtifact(pack.Key, artifact.ComponentKey)
		}
		if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
			return nil, err
		}
		emitPackageProgress(pack.Key, "install", fmt.Sprintf("%s 安装完成", artifact.DisplayName), artifactEnd)
		results = append(results, DependencyInstallResult{
			Key:       artifact.ComponentKey,
			Name:      artifact.DisplayName,
			Version:   artifact.Version,
			ZipPath:   zipPath,
			Installed: true,
			Message:   "组件安装完成",
		})
	}

	if err := checkInstallCancelledWithCleanup(installCtx, pack.Key); err != nil {
		return nil, err
	}

	emitPackageProgress(pack.Key, "record", "正在写入安装记录", 94)
	if err := saveInstalledRemotePackage(pack); err != nil {
		return nil, cleanupFailedRemotePackageInstall(pack.Key, err)
	}
	emitPackageProgress(pack.Key, "done", "安装完成", 100)

	return &PackageInstallResult{
		PackageKey:    pack.Key,
		PackageID:     pack.LegacyID,
		PackageName:   pack.DisplayName,
		GameDir:       gameDir,
		PluginRoot:    filepath.Join(gameDir, "BepInEx", "plugins"),
		Installed:     true,
		Message:       "Mod 包安装完成",
		Dependencies:  results,
		BepInExStatus: bepInExResult.Status,
	}, nil
}

// ContinueRemoteModPackageInstall 按用户选择继续待处理的冲突安装。
// 边界：只恢复 install_state=conflict 的安装包；选择失败会把当前包标记为 failed。
func ContinueRemoteModPackageInstall(packageKey string, action InstallConflictAction) (*PackageInstallResult, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}
	record := modDependenceModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		return nil, err
	}
	if record.InstallState != installProgressStateConflict {
		return nil, fmt.Errorf("当前安装包没有待处理的安装冲突")
	}
	manifest := installConflictManifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	if manifest.Conflict == nil {
		return nil, fmt.Errorf("没有可继续处理的安装冲突")
	}
	if action == InstallConflictActionFail {
		err := fmt.Errorf("用户选择终止安装：%s", manifest.Conflict.Message)
		err = cleanupFailedRemotePackageInstall(record.PackageKey, err)
		markPackageInstallConflictFailed(record.PackageKey, err)
		return nil, err
	}
	if action != InstallConflictActionOverwrite && action != InstallConflictActionSkip {
		return nil, fmt.Errorf("unsupported conflict action: %s", action)
	}

	manifest.Decisions = appendOrReplaceConflictDecision(manifest.Decisions, InstallConflictDecisionDTO{
		TargetPath:   manifest.Conflict.TargetPath,
		ComponentKey: manifest.Conflict.ComponentKey,
		Action:       action,
	})
	manifest.Conflict = nil
	pack := modDependenceResponse.RemoteModPackageDTO{}
	if err := json.Unmarshal(record.Raw, &pack); err != nil {
		return nil, err
	}
	if err := saveInstallConflictManifest(record.PackageKey, installProgressStateRunning, "prepare", "继续安装", 1, "", &manifest); err != nil {
		return nil, err
	}
	return InstallRemoteModPackageWithOptions(pack, InstallRemotePackageOptions{Decisions: manifest.Decisions})
}

// UninstallRemoteModPackage 按统一 packageKey 卸载 V2 安装包。
// 边界：只删除由该 packageKey 记录且未被其它包共享的文件，避免误删用户手动文件。
func UninstallRemoteModPackage(packageKey string) (*PackageInstallResult, error) {
	packageKey = strings.TrimSpace(packageKey)
	if packageKey == "" {
		return nil, fmt.Errorf("package key is required")
	}
	emitUninstallProgress(packageKey, "uninstall", "准备卸载", 5)

	gameDir, err := GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	var installedPackage modDependenceModel.ModInstalledPackage
	result := db.Where("package_key = ?", packageKey).Take(&installedPackage)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			emitUninstallProgress(packageKey, "done", "未找到安装记录，已跳过卸载", 100)
			return &PackageInstallResult{
				PackageKey:    packageKey,
				GameDir:       gameDir,
				PluginRoot:    filepath.Join(gameDir, "BepInEx", "plugins"),
				Uninstalled:   true,
				Skipped:       true,
				Message:       "Mod 包未安装，已跳过卸载",
				BepInExStatus: CheckBepInExStatus(gameDir),
			}, nil
		}
		return nil, result.Error
	}

	fileRecords, err := findPackageUninstallFiles(db, packageKey)
	if err != nil {
		return nil, err
	}
	uninstallPlan, err := buildDatabaseUninstallPlan(db, packageKey, fileRecords)
	if err != nil {
		return nil, err
	}

	removed := make([]DependencyInstallResult, 0, len(fileRecords))
	totalFiles := len(uninstallPlan)
	for index, plan := range uninstallPlan {
		emitUninstallProgress(packageKey, "uninstall", fmt.Sprintf("正在移除文件 %d/%d", index+1, totalFiles), 10+progressPercent(int64(index+1), int64(maxInt(totalFiles, 1)))*70/100)
		if !plan.shared {
			if err := removeManagedFile(plan.record.TargetPath); err != nil {
				return nil, err
			}
		}
		removed = append(removed, DependencyInstallResult{
			Key:       plan.record.ComponentKey,
			Name:      filepath.Base(plan.record.TargetPath),
			PluginDir: filepath.Dir(plan.record.TargetPath),
			Removed:   !plan.shared,
			Skipped:   plan.shared,
			Message:   uninstallFileMessage(plan.shared),
		})
	}

	emitUninstallProgress(packageKey, "record", "正在清理安装记录", 88)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("package_key = ?", packageKey).Delete(&modDependenceModel.ModInstalledFile{}).Error; err != nil {
			return err
		}
		if err := deleteLegacyComponentFileRecords(tx, packageKey); err != nil {
			return err
		}
		if err := tx.Where("package_key = ?", packageKey).Delete(&modDependenceModel.ModInstalledPackageComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&installedPackage).Error; err != nil {
			return err
		}
		return deleteUnreferencedV2Components(tx)
	}); err != nil {
		return nil, err
	}

	cleanupEmptyManagedDirs(filepath.Join(gameDir, "BepInEx"))
	emitUninstallProgress(packageKey, "done", "卸载完成", 100)
	return &PackageInstallResult{
		PackageKey:    packageKey,
		PackageName:   installedPackage.DisplayName,
		GameDir:       gameDir,
		PluginRoot:    filepath.Join(gameDir, "BepInEx", "plugins"),
		Uninstalled:   true,
		Message:       "Mod 包卸载完成",
		Dependencies:  removed,
		BepInExStatus: CheckBepInExStatus(gameDir),
	}, nil
}

// ListInstalledPackageKeys 返回 V2 已安装包 key 列表。
func ListInstalledPackageKeys() ([]string, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0)
	if err := db.Model(&modDependenceModel.ModInstalledPackage{}).
		Where("install_state = ? OR install_state = '' OR install_state IS NULL", installProgressStateInstalled).
		Order("package_key ASC").
		Pluck("package_key", &keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// ListPackageInstallProgressSnapshots 返回本地已持久化的安装中、失败和待处理冲突状态。
// 用途：前端页面刷新或重新进入 Mod 页后，从关键安装包表恢复卡片状态。
func ListPackageInstallProgressSnapshots() ([]InstallProgressSnapshot, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	records := make([]modDependenceModel.ModInstalledPackage, 0)
	if err := db.
		Where("install_state IN ?", []string{installProgressStateRunning, installProgressStateFailed, installProgressStateConflict}).
		Order("updated_at DESC").
		Find(&records).Error; err != nil {
		return nil, err
	}

	snapshots := make([]InstallProgressSnapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, InstallProgressSnapshot{
			PackageKey:     record.PackageKey,
			Stage:          record.InstallStage,
			Status:         record.InstallStatus,
			Percent:        record.InstallProgress,
			BytesPerSecond: record.CurrentSpeed,
			Message:        record.InstallStatus,
			State:          record.InstallState,
			ErrorMessage:   record.ErrorMessage,
			Conflict:       packageInstallConflictFromManifest(record.InstallManifest),
			UpdatedAt:      record.UpdatedAt,
		})
	}
	return snapshots, nil
}

func resolveInstallArtifacts(pack modDependenceResponse.RemoteModPackageDTO) ([]installArtifact, error) {
	switch pack.Source {
	case modDependenceResponse.ModSourceOfficial:
		artifacts := make([]installArtifact, 0, len(pack.Dependencies))
		for _, dependency := range pack.Dependencies {
			artifacts = append(artifacts, dependencyToArtifact(pack.Key, dependency))
		}
		return artifacts, nil
	case modDependenceResponse.ModSourceThunderstore:
		visited := map[string]struct{}{}
		artifacts := make([]installArtifact, 0)
		for _, dependency := range pack.Dependencies {
			if err := appendThunderstoreDependencyArtifacts(&artifacts, visited, pack.Key, dependency.Key); err != nil {
				return nil, err
			}
		}
		artifacts = append(artifacts, packageToArtifact(pack.Key, pack))
		return artifacts, nil
	case modDependenceResponse.ModSourceManual:
		return []installArtifact{packageToArtifact(pack.Key, pack)}, nil
	default:
		return nil, fmt.Errorf("unsupported source: %s", pack.Source)
	}
}

func appendThunderstoreDependencyArtifacts(artifacts *[]installArtifact, visited map[string]struct{}, parentPackageKey string, key string) error {
	if key == "" {
		return nil
	}
	if isBepInExDependencyKey(key) {
		return nil
	}
	if _, ok := visited[key]; ok {
		return nil
	}
	visited[key] = struct{}{}

	pack, err := ResolveRemotePackage(context.Background(), key)
	if err != nil {
		return err
	}
	for _, dependency := range pack.Dependencies {
		if err := appendThunderstoreDependencyArtifacts(artifacts, visited, parentPackageKey, dependency.Key); err != nil {
			return err
		}
	}
	*artifacts = append(*artifacts, packageToArtifact(parentPackageKey, *pack))
	return nil
}

func dependencyToArtifact(packageKey string, dependency modDependenceResponse.RemoteModDependencyDTO) installArtifact {
	displayName := dependency.DisplayName
	if displayName == "" {
		displayName = dependency.Name
	}
	return installArtifact{
		PackageKey:       packageKey,
		ParentPackageKey: packageKey,
		ComponentKey:     dependency.Key,
		Source:           dependency.Source,
		Name:             dependency.Name,
		DisplayName:      displayName,
		Version:          dependency.Version,
		DownloadURL:      dependency.DownloadURL,
		LocalPath:        dependency.LocalPath,
		ArtifactKind:     dependency.ArtifactKind,
	}
}

func packageToArtifact(packageKey string, pack modDependenceResponse.RemoteModPackageDTO) installArtifact {
	return installArtifact{
		PackageKey:       packageKey,
		ParentPackageKey: packageKey,
		ComponentKey:     pack.Key,
		Source:           pack.Source,
		Name:             pack.Name,
		DisplayName:      pack.DisplayName,
		Version:          pack.Version,
		DownloadURL:      pack.DownloadURL,
		LocalPath:        pack.LocalPath,
		ArtifactKind:     pack.ArtifactKind,
	}
}

func installArtifactFiles(ctx context.Context, bufferDir string, gameDir string, artifact installArtifact, progressStart int, progressEnd int, options InstallRemotePackageOptions) ([]plannedInstallFile, string, error) {
	if err := checkInstallCancelled(ctx); err != nil {
		return nil, "", err
	}
	emitPackageProgress(artifact.PackageKey, "download", fmt.Sprintf("正在获取 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 5))
	artifactPath, err := resolveArtifactPath(ctx, bufferDir, artifact, mapProgress(progressStart, progressEnd, 5), mapProgress(progressStart, progressEnd, 40))
	if err != nil {
		return nil, "", wrapArtifactInstallError("download", artifact, err)
	}

	switch artifact.ArtifactKind {
	case modDependenceResponse.ArtifactKindRemoteZip, modDependenceResponse.ArtifactKindLocalZip, "":
		if err := checkInstallCancelled(ctx); err != nil {
			return nil, "", err
		}
		emitPackageProgress(artifact.PackageKey, "extract", fmt.Sprintf("正在解压 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 45))
		extractPath, err := extractArtifactZip(ctx, bufferDir, artifactPath, artifact, mapProgress(progressStart, progressEnd, 45), mapProgress(progressStart, progressEnd, 55))
		if err != nil {
			return nil, "", wrapArtifactInstallError("extract", artifact, err)
		}
		if err := checkInstallCancelled(ctx); err != nil {
			return nil, "", err
		}
		emitPackageProgress(artifact.PackageKey, "plan", fmt.Sprintf("正在分析 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 58))
		files, err := planExtractedArtifactFiles(ctx, extractPath, gameDir, artifact)
		if err != nil {
			return nil, "", wrapArtifactInstallError("plan", artifact, err)
		}
		if err := appendInstallSessionPlannedFiles(artifact.installRecordPackageKey(), files); err != nil {
			return nil, "", wrapArtifactInstallError("plan", artifact, err)
		}
		if installRemotePackageTestHooks.AfterPlannedFiles != nil {
			installRemotePackageTestHooks.AfterPlannedFiles(artifact.installRecordPackageKey(), artifact.ComponentKey)
		}
		if err := checkInstallCancelled(ctx); err != nil {
			return nil, "", err
		}
		emitPackageProgress(artifact.PackageKey, "copy", fmt.Sprintf("正在复制 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 70))
		files, err = copyPlannedFiles(ctx, files, artifact, options, mapProgress(progressStart, progressEnd, 70), progressEnd)
		if err != nil {
			return nil, "", wrapArtifactInstallError("copy", artifact, err)
		}
		return files, artifactPath, nil
	case modDependenceResponse.ArtifactKindLocalDir:
		if err := checkInstallCancelled(ctx); err != nil {
			return nil, "", err
		}
		emitPackageProgress(artifact.PackageKey, "plan", fmt.Sprintf("正在分析 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 20))
		files, err := planExtractedArtifactFiles(ctx, artifactPath, gameDir, artifact)
		if err != nil {
			return nil, "", wrapArtifactInstallError("plan", artifact, err)
		}
		if err := appendInstallSessionPlannedFiles(artifact.installRecordPackageKey(), files); err != nil {
			return nil, "", wrapArtifactInstallError("plan", artifact, err)
		}
		if installRemotePackageTestHooks.AfterPlannedFiles != nil {
			installRemotePackageTestHooks.AfterPlannedFiles(artifact.installRecordPackageKey(), artifact.ComponentKey)
		}
		if err := checkInstallCancelled(ctx); err != nil {
			return nil, "", err
		}
		emitPackageProgress(artifact.PackageKey, "copy", fmt.Sprintf("正在复制 %s", artifact.DisplayName), mapProgress(progressStart, progressEnd, 55))
		files, err = copyPlannedFiles(ctx, files, artifact, options, mapProgress(progressStart, progressEnd, 55), progressEnd)
		if err != nil {
			return nil, "", wrapArtifactInstallError("copy", artifact, err)
		}
		return files, artifactPath, nil
	default:
		return nil, "", fmt.Errorf("unsupported artifact kind: %s", artifact.ArtifactKind)
	}
}

func resolveArtifactPath(ctx context.Context, bufferDir string, artifact installArtifact, progressStart int, progressEnd int) (string, error) {
	switch artifact.ArtifactKind {
	case modDependenceResponse.ArtifactKindLocalZip, modDependenceResponse.ArtifactKindLocalDir:
		if err := checkInstallCancelled(ctx); err != nil {
			return "", err
		}
		if artifact.LocalPath == "" {
			return "", fmt.Errorf("local path is required for %s", artifact.ComponentKey)
		}
		emitPackageProgress(artifact.PackageKey, "download", fmt.Sprintf("%s 已使用本地文件", artifact.DisplayName), progressEnd)
		return artifact.LocalPath, nil
	default:
		if artifact.DownloadURL == "" {
			return "", fmt.Errorf("download url is required for %s", artifact.ComponentKey)
		}
		return downloadRemoteArtifact(ctx, bufferDir, artifact, progressStart, progressEnd)
	}
}

func downloadRemoteArtifact(ctx context.Context, bufferDir string, artifact installArtifact, progressStart int, progressEnd int) (string, error) {
	if err := checkInstallCancelled(ctx); err != nil {
		return "", err
	}
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf("%s.zip", sanitizePathSegment(artifact.ComponentKey, "artifact")))
	if isValidZipFile(zipPath) {
		emitPackageProgress(artifact.PackageKey, "download", fmt.Sprintf("%s 已使用下载缓存", artifact.DisplayName), progressEnd)
		return zipPath, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	downloadURL, err := resolveArtifactDownloadURL(artifact.DownloadURL)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: 180 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download %s returned HTTP %d", artifact.DisplayName, response.StatusCode)
	}
	emitPackageProgress(artifact.PackageKey, "download", fmt.Sprintf("开始下载 %s", artifact.DisplayName), progressStart)

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", err
	}
	progressBody := newProgressReader(ctx, artifact.PackageKey, "download", fmt.Sprintf("正在下载 %s", artifact.DisplayName), response.Body, response.ContentLength, progressStart, progressEnd)
	_, copyErr := io.Copy(partialFile, progressBody)
	closeErr := partialFile.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return "", closeErr
	}

	if err := os.Rename(partialPath, zipPath); err != nil {
		return "", err
	}
	if !isValidZipFile(zipPath) {
		_ = os.Remove(zipPath)
		return "", fmt.Errorf("downloaded artifact is not a valid zip: %s", zipPath)
	}
	emitPackageProgress(artifact.PackageKey, "download", fmt.Sprintf("%s 下载完成", artifact.DisplayName), progressEnd)
	return zipPath, nil
}

func extractArtifactZip(ctx context.Context, bufferDir string, zipPath string, artifact installArtifact, progressStart int, progressEnd int) (string, error) {
	extractPath := filepath.Join(bufferDir, fmt.Sprintf("extract-%s", sanitizePathSegment(artifact.ComponentKey, "artifact")))
	if !isSubPath(bufferDir, extractPath) {
		return "", fmt.Errorf("extract path escapes buffer directory: %s", extractPath)
	}
	if err := checkInstallCancelled(ctx); err != nil {
		return "", err
	}
	if err := os.RemoveAll(extractPath); err != nil {
		return "", err
	}
	if err := os.MkdirAll(extractPath, 0o755); err != nil {
		return "", err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	totalFiles := len(reader.File)
	for index, file := range reader.File {
		if err := checkInstallCancelled(ctx); err != nil {
			return "", err
		}
		emitPackageProgress(artifact.PackageKey, "extract", fmt.Sprintf("正在解压文件 %d/%d", index+1, totalFiles), mapProgress(progressStart, progressEnd, progressPercent(int64(index+1), int64(maxInt(totalFiles, 1)))))
		if err := extractZipFile(file, extractPath); err != nil {
			return "", err
		}
	}
	return extractPath, nil
}

func planExtractedArtifactFiles(ctx context.Context, extractPath string, gameDir string, artifact installArtifact) ([]plannedInstallFile, error) {
	if err := validateArtifactInstallLayout(extractPath, artifact); err != nil {
		return nil, err
	}

	files := make([]plannedInstallFile, 0)
	err := filepath.WalkDir(extractPath, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if err := checkInstallCancelled(ctx); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(extractPath, sourcePath)
		if err != nil {
			return err
		}
		normalizedRelativePath := filepath.ToSlash(relativePath)

		targetRelativePath, err := resolveArtifactTargetRelativePath(normalizedRelativePath, artifact)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(gameDir, targetRelativePath)
		if !isSubPath(gameDir, targetPath) {
			return fmt.Errorf("artifact target escapes game directory: %s", targetPath)
		}

		hash, size, err := fileSHA256(sourcePath)
		if err != nil {
			return err
		}
		files = append(files, plannedInstallFile{
			PackageKey:   artifact.installRecordPackageKey(),
			ComponentKey: artifact.ComponentKey,
			SourcePath:   sourcePath,
			TargetPath:   targetPath,
			RelativePath: targetRelativePath,
			Sha256:       hash,
			FileSize:     size,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("artifact %s has no installable files", artifact.DisplayName)
	}
	return files, nil
}

func validateArtifactInstallLayout(extractPath string, artifact installArtifact) error {
	hasBepInExPack := false
	hasSpecialBepInExRoot := false
	hasAnyDLL := false
	hasAnyExecutable := false

	err := filepath.WalkDir(extractPath, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(extractPath, sourcePath)
		if err != nil {
			return err
		}
		normalizedRelativePath := normalizeArtifactEntryPath(relativePath)
		lowerRelativePath := strings.ToLower(normalizedRelativePath)
		if strings.HasPrefix(lowerRelativePath, "bepinexpack/") {
			hasBepInExPack = true
		}
		_, _, hasRoot, _ := splitArtifactInstallRoot(normalizedRelativePath)
		if hasRoot {
			hasSpecialBepInExRoot = true
		}

		extension := strings.ToLower(filepath.Ext(lowerRelativePath))
		if extension == ".dll" {
			hasAnyDLL = true
		}
		if extension == ".exe" || extension == ".msi" {
			hasAnyExecutable = true
		}
		return nil
	})
	if err != nil {
		return err
	}

	if hasBepInExPack {
		return fmt.Errorf("%s 是 BepInEx 框架包，应通过 BepInEx 安装器处理，不能作为普通 Mod 安装", artifact.DisplayName)
	}
	if hasSpecialBepInExRoot || hasAnyDLL {
		return nil
	}
	if hasAnyExecutable {
		return fmt.Errorf("%s 看起来是桌面工具或独立安装器，暂不支持自动安装到游戏 Mod 目录", artifact.DisplayName)
	}

	return fmt.Errorf("%s 未识别到 BepInEx 插件、patcher 或配置文件，已阻止自动安装", artifact.DisplayName)
}

func resolveArtifactTargetRelativePath(relativePath string, artifact installArtifact) (string, error) {
	normalizedRelativePath := normalizeArtifactEntryPath(relativePath)
	root, rest, hasRoot, fromBepInEx := splitArtifactInstallRoot(normalizedRelativePath)
	if hasRoot {
		if fromBepInEx {
			return filepath.FromSlash(normalizedRelativePath), nil
		}
		switch root {
		case "plugins", "patchers":
			return resolveScopedBepInExRelativePath(root, rest, artifact), nil
		case "config":
			return filepath.Join("BepInEx", "config", filepath.FromSlash(rest)), nil
		}
	}

	return filepath.Join("BepInEx", "plugins", artifactInstallDirName(artifact), filepath.FromSlash(normalizedRelativePath)), nil
}

func normalizeArtifactEntryPath(relativePath string) string {
	normalizedPath := strings.ReplaceAll(relativePath, "\\", "/")
	normalizedPath = path.Clean(normalizedPath)
	if normalizedPath == "." {
		return ""
	}
	return strings.Trim(normalizedPath, "/")
}

// splitArtifactInstallRoot 识别包内是否声明了 BepInEx 目标根目录。
// 边界：带 BepInEx 前缀时视为包作者显式声明的完整目标路径，不再额外改写目录。
func splitArtifactInstallRoot(relativePath string) (root string, rest string, hasRoot bool, fromBepInEx bool) {
	normalizedPath := normalizeArtifactEntryPath(relativePath)
	lowerPath := strings.ToLower(normalizedPath)
	if strings.HasPrefix(lowerPath, "bepinex/") {
		root, rest = splitFirstArtifactPathSegment(normalizedPath[len("bepinex/"):])
		return strings.ToLower(root), rest, true, true
	}

	root, rest = splitFirstArtifactPathSegment(normalizedPath)
	switch strings.ToLower(root) {
	case "plugins", "patchers", "config":
		return strings.ToLower(root), rest, true, false
	default:
		return "", normalizedPath, false, false
	}
}

func splitFirstArtifactPathSegment(relativePath string) (string, string) {
	relativePath = normalizeArtifactEntryPath(relativePath)
	if relativePath == "" {
		return "", ""
	}
	segments := strings.SplitN(relativePath, "/", 2)
	if len(segments) == 1 {
		return segments[0], ""
	}
	return segments[0], segments[1]
}

func resolveScopedBepInExRelativePath(root string, rest string, artifact installArtifact) string {
	installDir := artifactInstallDirName(artifact)
	if rest == "" {
		return filepath.Join("BepInEx", root, installDir)
	}
	if hasArtifactDirectoryPrefix(rest, artifact) {
		return filepath.Join("BepInEx", root, filepath.FromSlash(rest))
	}
	return filepath.Join("BepInEx", root, installDir, filepath.FromSlash(rest))
}

func hasArtifactDirectoryPrefix(relativePath string, artifact installArtifact) bool {
	firstSegment, rest := splitFirstArtifactPathSegment(relativePath)
	if firstSegment == "" || rest == "" {
		return false
	}
	for _, candidate := range artifactDirectoryCandidateNames(artifact) {
		if sameArtifactPathSegment(firstSegment, candidate) {
			return true
		}
	}
	return false
}

func artifactDirectoryCandidateNames(artifact installArtifact) []string {
	return []string{
		artifactInstallDirName(artifact),
		artifact.DisplayName,
		artifact.Name,
		artifact.sourceDependencyID(),
		artifact.ComponentKey,
	}
}

func sameArtifactPathSegment(left string, right string) bool {
	left = strings.ToLower(sanitizePathSegment(left, ""))
	right = strings.ToLower(sanitizePathSegment(right, ""))
	return left != "" && left == right
}

func artifactInstallDirName(artifact installArtifact) string {
	return artifact.installDirName()
}

func (artifact installArtifact) installDirName() string {
	for _, candidate := range []string{artifact.DisplayName, artifact.Name, artifact.sourceDependencyID(), artifact.ComponentKey} {
		if sanitized := sanitizePathSegment(candidate, "artifact"); sanitized != "artifact" {
			return sanitized
		}
	}
	return "artifact"
}

func isBepInExDependencyKey(key string) bool {
	return strings.HasPrefix(key, fmt.Sprintf("thunderstore:%s:%s:", bepInExOwner, bepInExName))
}

// installRecordPackageKey 返回文件清单应写入的安装包 key。
// 边界：依赖组件的 componentKey 独立保留，但 packageKey 必须指向主包，避免主包卸载找不到依赖文件。
func (artifact installArtifact) installRecordPackageKey() string {
	if strings.TrimSpace(artifact.ParentPackageKey) != "" {
		return artifact.ParentPackageKey
	}
	return artifact.PackageKey
}

// sourceDependencyID 从统一 key 中提取来源侧依赖标识。
// 用途：显示名缺失时作为目录名兜底，避免直接生成 official-4-0.5.1 这类完整 key 目录。
func (artifact installArtifact) sourceDependencyID() string {
	parts := strings.Split(artifact.ComponentKey, ":")
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return ""
}

// findPackageUninstallFiles 查找主包卸载时需要处理的文件清单。
// 边界：兼容早期依赖文件误写到依赖 package_key 的记录，通过组件关联补找这些历史残留。
func findPackageUninstallFiles(db *gorm.DB, packageKey string) ([]modDependenceModel.ModInstalledFile, error) {
	fileRecords := make([]modDependenceModel.ModInstalledFile, 0)
	if err := db.
		Where("package_key = ?", packageKey).
		Or("component_key IN (?)", db.
			Model(&modDependenceModel.ModInstalledPackageComponent{}).
			Select("component_key").
			Where("package_key = ?", packageKey),
		).
		Order("target_path ASC").
		Find(&fileRecords).Error; err != nil {
		return nil, err
	}
	return fileRecords, nil
}

// buildDatabaseUninstallPlan 基于当前安装记录生成卸载文件计划。
// 用途：卸载执行阶段只按数据库快照删除文件，不在事务内反复查询共享关系，避免 SQLite 写锁卡住进度。
func buildDatabaseUninstallPlan(db *gorm.DB, packageKey string, records []modDependenceModel.ModInstalledFile) ([]plannedUninstallFile, error) {
	plan := make([]plannedUninstallFile, 0, len(records))
	for _, record := range records {
		shared, err := isUninstallFileShared(db, packageKey, record)
		if err != nil {
			return nil, err
		}
		plan = append(plan, plannedUninstallFile{
			record: record,
			shared: shared,
		})
	}
	return plan, nil
}

// deleteLegacyComponentFileRecords 清理早期安装器写错归属的依赖文件记录。
// 边界：若组件仍被其它主包引用，只保留记录与文件，避免破坏共享依赖。
func deleteLegacyComponentFileRecords(tx *gorm.DB, packageKey string) error {
	componentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key = ?", packageKey)
	sharedComponentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key <> ?", packageKey)
	return tx.
		Where("component_key IN (?)", componentKeys).
		Where("component_key NOT IN (?)", sharedComponentKeys).
		Delete(&modDependenceModel.ModInstalledFile{}).
		Error
}

// isUninstallFileShared 判断目标文件是否仍被其它安装包引用。
// 用途：卸载主包时保护共享依赖文件；历史记录归属异常时优先按 componentKey 判断共享关系。
func isUninstallFileShared(tx *gorm.DB, packageKey string, record modDependenceModel.ModInstalledFile) (bool, error) {
	var relatedPackageCount int64
	if strings.TrimSpace(record.ComponentKey) != "" {
		if err := tx.Model(&modDependenceModel.ModInstalledPackageComponent{}).
			Where("package_key <> ? AND component_key = ?", packageKey, record.ComponentKey).
			Count(&relatedPackageCount).Error; err != nil {
			return false, err
		}
		if relatedPackageCount > 0 {
			return true, nil
		}
	}

	var sharedFileCount int64
	componentKeys := tx.
		Model(&modDependenceModel.ModInstalledPackageComponent{}).
		Select("component_key").
		Where("package_key = ?", packageKey)
	if err := tx.Model(&modDependenceModel.ModInstalledFile{}).
		Where("lower(target_path) IN ?", normalizeInstallTargetPathValuesForQuery(record.TargetPath)).
		Where("NOT (package_key = ? OR component_key IN (?))", packageKey, componentKeys).
		Count(&sharedFileCount).Error; err != nil {
		return false, err
	}
	return sharedFileCount > 0, nil
}

func copyPlannedFiles(ctx context.Context, files []plannedInstallFile, artifact installArtifact, options InstallRemotePackageOptions, progressStart int, progressEnd int) ([]plannedInstallFile, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}
	packageKey := artifact.installRecordPackageKey()
	manifest := loadInstallConflictManifest(packageKey)
	hasPreviousInstalledPackage := manifest.PreviousPackage != nil

	rollbackPlan := make([]plannedCopyRollback, 0, len(files))
	overwrittenFiles := make([]plannedInstallFile, 0, len(files))
	installableFiles := make([]plannedInstallFile, 0, len(files))
	rollbackCopySession := func() {
		rollbackCopiedFiles(rollbackPlan)
		_ = removeInstallSessionFiles(packageKey, installableFiles)
	}
	totalFiles := len(files)
	for index, file := range files {
		if err := checkInstallCancelled(ctx); err != nil {
			rollbackCopySession()
			return nil, err
		}
		emitPackageProgress(file.PackageKey, "copy", fmt.Sprintf("正在复制文件 %d/%d", index+1, totalFiles), mapProgress(progressStart, progressEnd, progressPercent(int64(index+1), int64(maxInt(totalFiles, 1)))))
		conflict, err := inspectFileInstallConflict(db, file, artifact)
		if err != nil {
			rollbackCopySession()
			return nil, err
		}
		if conflict != nil {
			decision := resolveConflictDecision(options, file)
			switch decision {
			case InstallConflictActionOverwrite:
				if info, err := os.Stat(file.TargetPath); err == nil && info.IsDir() {
					rollbackCopySession()
					return nil, fmt.Errorf("目标路径是目录，无法用文件覆盖：%s", file.TargetPath)
				} else if err != nil && !os.IsNotExist(err) {
					rollbackCopySession()
					return nil, err
				}
				overwrittenFiles = append(overwrittenFiles, file)
			case InstallConflictActionSkip:
				continue
			case InstallConflictActionFail:
				rollbackCopySession()
				return nil, fmt.Errorf("用户选择终止安装：%s", conflict.Message)
			default:
				rollbackCopySession()
				return nil, &installFileConflictError{Conflict: *conflict}
			}
		}
		_, statErr := os.Stat(file.TargetPath)
		targetExisted := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			rollbackCopySession()
			return nil, statErr
		}
		rollbackItem := plannedCopyRollback{
			targetPath: file.TargetPath,
			existed:    targetExisted,
		}
		file.TargetExisted = targetExisted
		previousRecords, err := loadInstallTargetRecords(db, file.TargetPath)
		if err != nil {
			rollbackCopySession()
			return nil, err
		}
		file.PreviousRecords = filterInstallTargetPreviousRecords(previousRecords, packageKey, hasPreviousInstalledPackage)
		if targetExisted {
			backupPath, err := backupInstallTargetFile(file.TargetPath)
			if err != nil {
				rollbackCopySession()
				return nil, err
			}
			rollbackItem.backupPath = backupPath
			file.BackupPath = backupPath
		}
		if err := os.MkdirAll(filepath.Dir(file.TargetPath), 0o755); err != nil {
			rollbackCopySession()
			return nil, err
		}
		if err := copyFileWithContext(ctx, file.SourcePath, file.TargetPath, 0o644); err != nil {
			rollbackPlan = append(rollbackPlan, rollbackItem)
			rollbackCopySession()
			return nil, err
		}
		rollbackPlan = append(rollbackPlan, rollbackItem)
		if err := checkInstallCancelled(ctx); err != nil {
			rollbackCopySession()
			return nil, err
		}
		installableFiles = append(installableFiles, file)
		if err := appendInstallSessionArtifact(packageKey, artifact.ComponentKey, []plannedInstallFile{file}); err != nil {
			rollbackCopySession()
			return nil, err
		}
	}
	for _, file := range overwrittenFiles {
		if err := releaseConflictingFileRecords(db, file); err != nil {
			rollbackCopySession()
			return nil, err
		}
	}
	return installableFiles, nil
}

func maxInt(left int, right int) int {
	if left > right {
		return left
	}
	return right
}

func rollbackCopiedFiles(plan []plannedCopyRollback) {
	for index := len(plan) - 1; index >= 0; index-- {
		item := plan[index]
		if item.existed {
			_ = os.Remove(item.targetPath)
			if item.backupPath != "" {
				_ = os.Rename(item.backupPath, item.targetPath)
			}
			continue
		}
		_ = os.Remove(item.targetPath)
		if item.backupPath != "" {
			_ = os.Remove(item.backupPath)
		}
		cleanupEmptyInstallParentDirs(item.targetPath)
	}
}

func backupInstallTargetFile(targetPath string) (string, error) {
	backupDir, err := modDependencyV2BufferDir()
	if err != nil {
		return "", err
	}
	backupPath := filepath.Join(backupDir, "rollback", fmt.Sprintf("%d-%s", time.Now().UnixNano(), sanitizePathSegment(filepath.Base(targetPath), "file")))
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return "", err
	}
	if err := copyFile(targetPath, backupPath, 0o644); err != nil {
		return "", err
	}
	return backupPath, nil
}

func cleanupCopyBackups(plan []plannedCopyRollback) {
	for _, item := range plan {
		if item.backupPath != "" {
			_ = os.Remove(item.backupPath)
		}
	}
}

func inspectFileInstallConflict(db *gorm.DB, file plannedInstallFile, artifact installArtifact) (*InstallConflictDTO, error) {
	info, err := os.Stat(file.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return buildDirectoryInstallConflictDTO(file, artifact), nil
	}

	existingHash, _, err := fileSHA256(file.TargetPath)
	if err != nil {
		return nil, err
	}
	if existingHash == file.Sha256 {
		return nil, nil
	}

	managedRecord := modDependenceModel.ModInstalledFile{}
	result := db.
		Where("lower(target_path) IN ? AND package_key <> ?", normalizeInstallTargetPathValuesForQuery(file.TargetPath), file.PackageKey).
		Take(&managedRecord)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return nil, result.Error
	}
	if result.Error == nil {
		return buildInstallConflictDTO(db, file, artifact, existingHash, managedRecord), nil
	}

	var samePackageCount int64
	if err := db.Model(&modDependenceModel.ModInstalledFile{}).
		Where("lower(target_path) IN ? AND package_key = ?", normalizeInstallTargetPathValuesForQuery(file.TargetPath), file.PackageKey).
		Count(&samePackageCount).Error; err != nil {
		return nil, err
	}
	if samePackageCount > 0 {
		return nil, nil
	}

	return &InstallConflictDTO{
		PackageKey:       file.PackageKey,
		ComponentKey:     file.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		TargetPath:       file.TargetPath,
		RelativePath:     file.RelativePath,
		IncomingSha256:   file.Sha256,
		ExistingSha256:   existingHash,
		Message:          fmt.Sprintf("%s 已存在且不是当前整合包记录的文件，需要选择覆盖、跳过或失败", filepath.Base(file.TargetPath)),
	}, nil
}

func buildDirectoryInstallConflictDTO(file plannedInstallFile, artifact installArtifact) *InstallConflictDTO {
	return &InstallConflictDTO{
		PackageKey:       file.PackageKey,
		ComponentKey:     file.ComponentKey,
		ComponentName:    artifact.DisplayName,
		ComponentVersion: artifact.Version,
		TargetPath:       file.TargetPath,
		RelativePath:     file.RelativePath,
		IncomingSha256:   file.Sha256,
		Message:          fmt.Sprintf("%s 已存在且是目录，不能直接作为文件覆盖；通常是旧版本路径大小写冲突或包内目录结构异常", filepath.Base(file.TargetPath)),
	}
}

func buildInstallConflictDTO(db *gorm.DB, file plannedInstallFile, artifact installArtifact, existingHash string, managedRecord modDependenceModel.ModInstalledFile) *InstallConflictDTO {
	conflict := &InstallConflictDTO{
		PackageKey:           file.PackageKey,
		ComponentKey:         file.ComponentKey,
		ComponentName:        artifact.DisplayName,
		ComponentVersion:     artifact.Version,
		TargetPath:           file.TargetPath,
		RelativePath:         file.RelativePath,
		IncomingSha256:       file.Sha256,
		ExistingSha256:       existingHash,
		ExistingPackageKey:   managedRecord.PackageKey,
		ExistingComponentKey: managedRecord.ComponentKey,
		Message:              fmt.Sprintf("%s 已由其它整合包管理，当前版本不同，需要选择覆盖、跳过或失败", filepath.Base(file.TargetPath)),
	}

	installedPackage := modDependenceModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", managedRecord.PackageKey).Take(&installedPackage).Error; err == nil {
		conflict.ExistingPackageName = installedPackage.DisplayName
	}
	installedComponent := modDependenceModel.ModInstalledComponent{}
	if err := db.Where("component_key = ?", managedRecord.ComponentKey).Take(&installedComponent).Error; err == nil {
		conflict.ExistingComponentName = installedComponent.DisplayName
		conflict.ExistingComponentVersion = installedComponent.Version
	}
	return conflict
}

func resolveConflictDecision(options InstallRemotePackageOptions, file plannedInstallFile) InstallConflictAction {
	for _, decision := range options.Decisions {
		if sameInstallTargetPath(decision.TargetPath, file.TargetPath) {
			return decision.Action
		}
	}
	for _, decision := range options.Decisions {
		if strings.TrimSpace(file.ComponentKey) != "" && decision.ComponentKey == file.ComponentKey {
			return decision.Action
		}
	}
	return ""
}

func releaseConflictingFileRecords(db *gorm.DB, file plannedInstallFile) error {
	return db.
		Where("lower(target_path) IN ? AND package_key <> ?", normalizeInstallTargetPathValuesForQuery(file.TargetPath), file.PackageKey).
		Delete(&modDependenceModel.ModInstalledFile{}).
		Error
}

func persistInstallConflict(pack modDependenceResponse.RemoteModPackageDTO, err error, options InstallRemotePackageOptions) error {
	var conflictErr *installFileConflictError
	if !errors.As(err, &conflictErr) {
		return nil
	}
	manifest := loadInstallConflictManifest(pack.Key)
	manifest.Conflict = &conflictErr.Conflict
	if len(options.Decisions) > 0 {
		manifest.Decisions = options.Decisions
	}
	errorMessage := conflictErr.Error()
	if saveErr := saveInstallConflictManifest(pack.Key, installProgressStateConflict, "conflict", "等待处理文件冲突", 100, errorMessage, &manifest); saveErr != nil {
		return saveErr
	}
	emitInstallProgress(installProgressPayload{
		PackageKey:   pack.Key,
		Stage:        "conflict",
		Status:       "等待处理文件冲突",
		Percent:      100,
		Message:      errorMessage,
		State:        installProgressStateConflict,
		ErrorMessage: errorMessage,
		Conflict:     &conflictErr.Conflict,
	})
	return nil
}

func saveInstallConflictManifest(packageKey string, state string, stage string, status string, percent int, errorMessage string, manifest *installConflictManifest) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	content, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return db.Model(&modDependenceModel.ModInstalledPackage{}).
		Where("package_key = ?", packageKey).
		Updates(map[string]interface{}{
			"install_state":    state,
			"install_stage":    stage,
			"install_status":   status,
			"install_progress": clampProgress(percent),
			"error_message":    errorMessage,
			"install_manifest": datatypes.JSON(content),
			"updated_at":       time.Now(),
		}).Error
}

func markPackageInstallConflictFailed(packageKey string, err error) {
	errorMessage := ""
	if err != nil {
		errorMessage = err.Error()
	}
	_ = saveInstallConflictManifest(packageKey, installProgressStateFailed, "failed", "安装失败", 100, errorMessage, &installConflictManifest{})
	MarkPackageProgressFailure(packageKey, "安装失败", err)
}

func appendOrReplaceConflictDecision(decisions []InstallConflictDecisionDTO, nextDecision InstallConflictDecisionDTO) []InstallConflictDecisionDTO {
	next := make([]InstallConflictDecisionDTO, 0, len(decisions)+1)
	replaced := false
	for _, decision := range decisions {
		if sameConflictDecisionScope(decision, nextDecision) {
			next = append(next, nextDecision)
			replaced = true
			continue
		}
		next = append(next, decision)
	}
	if !replaced {
		next = append(next, nextDecision)
	}
	return next
}

func sameConflictDecisionScope(left InstallConflictDecisionDTO, right InstallConflictDecisionDTO) bool {
	if sameInstallTargetPath(left.TargetPath, right.TargetPath) {
		return true
	}
	if strings.TrimSpace(right.ComponentKey) != "" && left.ComponentKey == right.ComponentKey {
		return true
	}
	return false
}

func sameInstallTargetPath(left string, right string) bool {
	return normalizeInstallTargetPathForCompare(left) == normalizeInstallTargetPathForCompare(right)
}

// normalizeInstallTargetPathForCompare 将安装目标路径转成 SQLite 可比较格式。
// 边界：Windows 文件系统默认不区分大小写，且历史记录可能混用斜杠，因此内存比较统一使用 slash 语义。
func normalizeInstallTargetPathForCompare(path string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
}

// normalizeInstallTargetPathValuesForQuery 返回兼容历史路径写法的数据库匹配值。
// 边界：SQLite 不理解 Windows 路径语义，只能用多候选值覆盖旧记录里的正反斜杠差异。
func normalizeInstallTargetPathValuesForQuery(path string) []string {
	cleaned := strings.ToLower(filepath.Clean(path))
	slash := strings.ToLower(filepath.ToSlash(filepath.Clean(path)))
	backslash := strings.ReplaceAll(slash, "/", "\\")
	values := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for _, value := range []string{cleaned, slash, backslash} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func packageInstallConflictFromManifest(raw datatypes.JSON) *InstallConflictDTO {
	if len(raw) == 0 {
		return nil
	}
	manifest := installConflictManifest{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil
	}
	return manifest.Conflict
}

func saveInstalledArtifact(pack modDependenceResponse.RemoteModPackageDTO, artifact installArtifact, files []plannedInstallFile) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}

	componentRaw, _ := json.Marshal(artifact)
	component := modDependenceModel.ModInstalledComponent{
		ComponentKey:       artifact.ComponentKey,
		Source:             string(artifact.Source),
		SourceDependencyID: artifact.ComponentKey,
		Name:               artifact.Name,
		DisplayName:        artifact.DisplayName,
		Version:            artifact.Version,
		DownloadURL:        artifact.DownloadURL,
		ArtifactKind:       string(artifact.ArtifactKind),
		LocalPath:          artifact.LocalPath,
		Raw:                datatypes.JSON(componentRaw),
		UpdatedAt:          time.Now(),
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "component_key"}}, UpdateAll: true}).Create(&component).Error; err != nil {
			return err
		}

		relation := modDependenceModel.ModInstalledPackageComponent{
			PackageKey:   pack.Key,
			ComponentKey: artifact.ComponentKey,
			CreatedAt:    time.Now(),
		}
		if err := tx.Where("package_key = ? AND component_key = ?", relation.PackageKey, relation.ComponentKey).
			FirstOrCreate(&relation).Error; err != nil {
			return err
		}

		for _, file := range files {
			record := modDependenceModel.ModInstalledFile{
				PackageKey:   file.PackageKey,
				ComponentKey: file.ComponentKey,
				SourcePath:   file.SourcePath,
				TargetPath:   file.TargetPath,
				RelativePath: file.RelativePath,
				Sha256:       file.Sha256,
				FileSize:     file.FileSize,
				CreatedAt:    time.Now(),
			}
			if err := tx.Where("package_key = ? AND lower(target_path) IN ?", file.PackageKey, normalizeInstallTargetPathValuesForQuery(file.TargetPath)).
				Assign(record).
				FirstOrCreate(&record).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func saveInstalledRemotePackage(pack modDependenceResponse.RemoteModPackageDTO) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record, err := remotePackageToInstalledModel(pack)
	if err != nil {
		return err
	}
	record.InstallState = installProgressStateInstalled
	record.InstallStage = "done"
	record.InstallStatus = "安装完成"
	record.InstallProgress = 100
	record.ErrorMessage = ""
	record.CurrentSpeed = 0
	if err := cleanupInstallSessionBackups(pack.Key); err != nil {
		return err
	}
	record.InstallManifest = datatypes.JSON([]byte("{}"))
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "package_key"}}, UpdateAll: true}).Create(&record).Error
}

func saveRemotePackageInstallState(pack modDependenceResponse.RemoteModPackageDTO, state string, stage string, status string, percent int, errorMessage string, bytesPerSecond int64) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}
	record, err := remotePackageToInstalledModel(pack)
	if err != nil {
		return err
	}
	record.InstallState = state
	record.InstallStage = stage
	record.InstallStatus = status
	record.InstallProgress = clampProgress(percent)
	record.ErrorMessage = errorMessage
	record.CurrentSpeed = bytesPerSecond
	record.UpdatedAt = time.Now()

	return db.
		Omit("InstallManifest").
		Where("package_key = ?", record.PackageKey).
		Assign(record).
		FirstOrCreate(&record).
		Error
}

func updatePackageInstallProgress(payload installProgressPayload) {
	db, err := requireDatabase()
	if err != nil {
		return
	}
	state := payload.State
	if state == installProgressStateDone {
		state = installProgressStateInstalled
	}
	updates := map[string]interface{}{
		"install_state":    state,
		"install_stage":    payload.Stage,
		"install_status":   payload.Status,
		"install_progress": payload.Percent,
		"current_speed":    payload.BytesPerSecond,
		"updated_at":       time.Now(),
	}
	if payload.ErrorMessage != "" {
		updates["error_message"] = payload.ErrorMessage
	}
	_ = db.Model(&modDependenceModel.ModInstalledPackage{}).
		Where("package_key = ?", payload.PackageKey).
		Updates(updates).
		Error
}

func remotePackageToInstalledModel(pack modDependenceResponse.RemoteModPackageDTO) (modDependenceModel.ModInstalledPackage, error) {
	tags, err := json.Marshal(pack.Tags)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	gameVersions, err := json.Marshal(pack.GameVersions)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	raw, err := json.Marshal(pack)
	if err != nil {
		return modDependenceModel.ModInstalledPackage{}, err
	}
	now := time.Now()
	return modDependenceModel.ModInstalledPackage{
		PackageKey:      pack.Key,
		Source:          string(pack.Source),
		SourcePackageID: pack.SourcePackageID,
		Namespace:       pack.Namespace,
		Name:            pack.Name,
		DisplayName:     pack.DisplayName,
		Version:         pack.Version,
		Description:     pack.Description,
		Author:          pack.Author,
		DownloadURL:     pack.DownloadURL,
		PackageURL:      pack.PackageURL,
		CoverURL:        pack.CoverURL,
		ArtifactKind:    string(pack.ArtifactKind),
		LocalPath:       pack.LocalPath,
		DownloadCount:   pack.DownloadCount,
		DependencyCount: pack.DependencyCount,
		PackageSize:     pack.PackageSize,
		Tags:            datatypes.JSON(tags),
		GameVersions:    datatypes.JSON(gameVersions),
		Trustworthiness: pack.Trustworthiness,
		Raw:             datatypes.JSON(raw),
		InstalledAt:     now,
		UpdatedAt:       now,
	}, nil
}

func resolveArtifactDownloadURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", fmt.Errorf("download url is empty")
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", fmt.Errorf("parse download url: %w", err)
	}
	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		return parsedURL.String(), nil
	}
	if core.GLOBAL.HTTPClient == nil {
		return "", fmt.Errorf("http client is not initialized")
	}
	return core.GLOBAL.HTTPClient.ResolveURL(trimmedURL)
}

func deleteUnreferencedV2Components(tx *gorm.DB) error {
	return tx.
		Where("component_key NOT IN (SELECT DISTINCT component_key FROM mod_installed_package_components)").
		Delete(&modDependenceModel.ModInstalledComponent{}).
		Error
}

func removeManagedFile(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.Remove(path)
}

func cleanupEmptyManagedDirs(root string) {
	for _, child := range []string{"plugins", "patchers", "config"} {
		cleanupEmptyDirs(filepath.Join(root, child), filepath.Join(root, child))
	}
}

func cleanupEmptyInstallParentDirs(filePath string) {
	if strings.TrimSpace(filePath) == "" {
		return
	}
	dir := filepath.Dir(filePath)
	for {
		base := strings.ToLower(filepath.Base(dir))
		if base == "plugins" || base == "patchers" || base == "config" || base == "bepinex" || dir == filepath.Dir(dir) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}

func cleanupEmptyDirs(root string, current string) bool {
	entries, err := os.ReadDir(current)
	if err != nil {
		return false
	}
	empty := true
	for _, entry := range entries {
		if !entry.IsDir() {
			empty = false
			continue
		}
		childPath := filepath.Join(current, entry.Name())
		if cleanupEmptyDirs(root, childPath) {
			_ = os.Remove(childPath)
		} else {
			empty = false
		}
	}
	return empty && current != root
}

func uninstallFileMessage(shared bool) string {
	if shared {
		return "文件仍被其它包引用，已保留"
	}
	return "文件已移除"
}

func fileSHA256(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func modDependencyV2BufferDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exePath), bufferRootDirName, modDependencyV2BufferDirName), nil
}
