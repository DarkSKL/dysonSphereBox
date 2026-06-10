package modDependence

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"
)

type legacyScopedFileRepairPlan struct {
	record           modDependenceModel.ModInstalledFile
	currentPath      string
	nextPath         string
	nextRelativePath string
}

// RepairLegacyScopedBepInExFiles 修复早期安装器把 plugins/patchers 文件裸露到根目录的问题。
// 用途：启动后迁移数据库中仍指向 BepInEx/plugins/*.dll 或 BepInEx/patchers/*.dll 的受管理文件。
// 边界：只迁移哈希完全匹配的已管理文件；目标存在且不是本次迁移源文件时跳过，交给安装冲突流程处理。
func RepairLegacyScopedBepInExFiles() error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	records, err := loadLegacyScopedBepInExFileRecords()
	if err != nil {
		return err
	}
	plans := buildLegacyScopedFileRepairPlans(records)
	sortLegacyScopedFileRepairPlans(plans)
	for _, plan := range plans {
		if err := repairLegacyScopedFile(plan); err != nil {
			return err
		}
	}
	return nil
}

func loadLegacyScopedBepInExFileRecords() ([]modDependenceModel.ModInstalledFile, error) {
	records := make([]modDependenceModel.ModInstalledFile, 0)
	err := core.GLOBAL.DB.
		Where("relative_path LIKE ?", "BepInEx\\plugins\\%").
		Or("relative_path LIKE ?", "BepInEx/plugins/%").
		Or("relative_path LIKE ?", "BepInEx\\patchers\\%").
		Or("relative_path LIKE ?", "BepInEx/patchers/%").
		Order("target_path ASC").
		Find(&records).Error
	return records, err
}

func buildLegacyScopedFileRepairPlans(records []modDependenceModel.ModInstalledFile) []legacyScopedFileRepairPlan {
	plans := make([]legacyScopedFileRepairPlan, 0)
	for _, record := range records {
		nextRelativePath, ok := repairLegacyScopedRelativePath(record)
		if !ok || sameFilesystemPath(nextRelativePath, record.RelativePath) {
			continue
		}

		bepInExRoot := resolveBepInExRootFromTarget(record.TargetPath)
		if bepInExRoot == "" {
			continue
		}
		nextPath := filepath.Join(filepath.Dir(bepInExRoot), nextRelativePath)
		if !isSubPath(filepath.Dir(bepInExRoot), nextPath) {
			continue
		}
		plans = append(plans, legacyScopedFileRepairPlan{
			record:           record,
			currentPath:      filepath.Clean(record.TargetPath),
			nextPath:         filepath.Clean(nextPath),
			nextRelativePath: nextRelativePath,
		})
	}
	return plans
}

func sortLegacyScopedFileRepairPlans(plans []legacyScopedFileRepairPlan) {
	sort.SliceStable(plans, func(leftIndex int, rightIndex int) bool {
		return legacyPlanBlocksTargetDirectory(plans[leftIndex], plans) && !legacyPlanBlocksTargetDirectory(plans[rightIndex], plans)
	})
}

func legacyPlanBlocksTargetDirectory(plan legacyScopedFileRepairPlan, plans []legacyScopedFileRepairPlan) bool {
	for _, candidate := range plans {
		if sameFilesystemPath(plan.currentPath, filepath.Dir(candidate.nextPath)) {
			return true
		}
	}
	return false
}

func repairLegacyScopedRelativePath(record modDependenceModel.ModInstalledFile) (string, bool) {
	normalizedRelativePath := normalizeArtifactEntryPath(record.RelativePath)
	root, rest, hasRoot, fromBepInEx := splitArtifactInstallRoot(normalizedRelativePath)
	if !hasRoot || !fromBepInEx || (root != "plugins" && root != "patchers") {
		return "", false
	}
	if rest == "" || strings.Contains(rest, "/") {
		return "", false
	}

	artifact := installedFileRecordArtifact(record)
	return resolveScopedBepInExRelativePath(root, rest, artifact), true
}

func installedFileRecordArtifact(record modDependenceModel.ModInstalledFile) installArtifact {
	component := modDependenceModel.ModInstalledComponent{}
	if core.GLOBAL.DB != nil && strings.TrimSpace(record.ComponentKey) != "" {
		_ = core.GLOBAL.DB.Where("component_key = ?", record.ComponentKey).Take(&component).Error
	}
	return installArtifact{
		PackageKey:   record.PackageKey,
		ComponentKey: record.ComponentKey,
		Name:         component.Name,
		DisplayName:  component.DisplayName,
		Version:      component.Version,
	}
}

func resolveBepInExRootFromTarget(targetPath string) string {
	cleanPath := filepath.Clean(targetPath)
	lowerPath := strings.ToLower(cleanPath)
	marker := string(filepath.Separator) + "bepinex"
	index := strings.LastIndex(lowerPath, marker)
	if index < 0 {
		if strings.EqualFold(cleanPath, "BepInEx") || strings.HasSuffix(lowerPath, marker) {
			return cleanPath
		}
		return ""
	}
	return cleanPath[:index+len(marker)]
}

func repairLegacyScopedFile(plan legacyScopedFileRepairPlan) error {
	currentHash, _, err := fileSHA256(plan.currentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !strings.EqualFold(currentHash, plan.record.Sha256) {
		return nil
	}

	if sameFilesystemPath(plan.currentPath, plan.nextPath) {
		return nil
	}
	if sameFilesystemPath(plan.currentPath, filepath.Dir(plan.nextPath)) {
		return repairLegacyScopedFileWithBlockingDirectory(plan)
	}
	if _, err := os.Stat(plan.nextPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plan.nextPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(plan.currentPath, plan.nextPath); err != nil {
		return err
	}

	return core.GLOBAL.DB.Model(&modDependenceModel.ModInstalledFile{}).
		Where("id = ?", plan.record.ID).
		Updates(map[string]interface{}{
			"target_path":   plan.nextPath,
			"relative_path": plan.nextRelativePath,
		}).Error
}

func repairLegacyScopedFileWithBlockingDirectory(plan legacyScopedFileRepairPlan) error {
	tempPath := legacyScopedRepairTempPath(plan.currentPath)
	if _, err := os.Stat(tempPath); err == nil {
		return fmt.Errorf("legacy migration temp file already exists: %s", tempPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(plan.currentPath, tempPath); err != nil {
		return err
	}

	completed := false
	defer func() {
		if !completed {
			_ = os.Rename(tempPath, plan.currentPath)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(plan.nextPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tempPath, plan.nextPath); err != nil {
		return err
	}
	completed = true

	return core.GLOBAL.DB.Model(&modDependenceModel.ModInstalledFile{}).
		Where("id = ?", plan.record.ID).
		Updates(map[string]interface{}{
			"target_path":   plan.nextPath,
			"relative_path": plan.nextRelativePath,
		}).Error
}

func legacyScopedRepairTempPath(path string) string {
	return path + ".dsb-migrate"
}

func sameFilesystemPath(left string, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
