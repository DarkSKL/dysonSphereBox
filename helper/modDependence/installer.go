package modDependence

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"
	modDependenceResponse "DysonSphereBox/model/modDependence/response"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const modDependencyBufferDirName = "modDependencies"

// DependencyInstallResult 表示单个 Mod 依赖的安装或卸载结果。
// 用途：返回给前端展示整合包安装明细；边界：路径仅用于本机状态展示，不参与前端拼接文件操作。
type DependencyInstallResult struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	PluginDir string `json:"pluginDir"`
	ZipPath   string `json:"zipPath"`
	Installed bool   `json:"installed"`
	Removed   bool   `json:"removed"`
	Skipped   bool   `json:"skipped"`
	Message   string `json:"message"`
}

// PackageInstallResult 表示整合包安装器的整体执行结果。
// 用途：前端根据 Installed、Uninstalled 和 Dependencies 同步市场卡片状态。
type PackageInstallResult struct {
	PackageID     uint                      `json:"packageId"`
	PackageName   string                    `json:"packageName"`
	GameDir       string                    `json:"gameDir"`
	PluginRoot    string                    `json:"pluginRoot"`
	Installed     bool                      `json:"installed"`
	Uninstalled   bool                      `json:"uninstalled"`
	Skipped       bool                      `json:"skipped"`
	Message       string                    `json:"message"`
	Dependencies  []DependencyInstallResult `json:"dependencies"`
	BepInExStatus BepInExStatus             `json:"bepInExStatus"`
}

// InstallModDependencyPackage 安装远端整合包 DTO 中声明的全部 Mod 依赖。
// 使用：InstallerService 接收前端选中的市场卡片后调用；会先确保 BepInEx 可用。
// 边界：依赖必须包含可下载的 zip 地址，安装成功后才写入本地 SQLite 安装记录。
func InstallModDependencyPackage(pack modDependenceResponse.ModDependencyPackageDTO) (*PackageInstallResult, error) {
	if pack.ID == 0 {
		return nil, fmt.Errorf("package id is required")
	}
	if len(pack.Dependencies) == 0 {
		return nil, fmt.Errorf("package has no dependencies to install")
	}

	bepInExResult, err := InstallBepInEx()
	if err != nil {
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		return nil, fmt.Errorf("BepInEx is not ready: %s", strings.Join(bepInExResult.Status.MissingFiles, ", "))
	}

	gameDir := bepInExResult.Status.GameDir
	pluginRoot := filepath.Join(gameDir, "BepInEx", "plugins")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		return nil, err
	}

	staleDependencies, err := stalePackageDependencies(pack.ID, pack.Dependencies)
	if err != nil {
		return nil, err
	}

	bufferDir, err := modDependencyBufferDir()
	if err != nil {
		return nil, err
	}

	dependencyResults := make([]DependencyInstallResult, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		result, err := installDependency(bufferDir, pluginRoot, dependency)
		if err != nil {
			return nil, err
		}
		dependencyResults = append(dependencyResults, result)
	}

	if err := saveInstalledPackage(pack); err != nil {
		return nil, err
	}
	if err := removeStaleDependencyPluginDirs(pluginRoot, staleDependencies); err != nil {
		return nil, err
	}

	return &PackageInstallResult{
		PackageID:     pack.ID,
		PackageName:   stringValue(pack.PackageName),
		GameDir:       gameDir,
		PluginRoot:    pluginRoot,
		Installed:     true,
		Uninstalled:   false,
		Skipped:       false,
		Message:       "整合包安装完成",
		Dependencies:  dependencyResults,
		BepInExStatus: bepInExResult.Status,
	}, nil
}

// UninstallModDependencyPackage 卸载本地 SQLite 中记录的整合包。
// 使用：前端点击已安装卡片的卸载按钮时调用；会移除不再被其它整合包引用的插件目录。
// 边界：找不到记录时按已卸载处理，避免重复点击产生破坏性副作用。
func UninstallModDependencyPackage(packageID uint) (*PackageInstallResult, error) {
	if packageID == 0 {
		return nil, fmt.Errorf("package id is required")
	}

	gameDir, err := GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}
	pluginRoot := filepath.Join(gameDir, "BepInEx", "plugins")

	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	var pack modDependenceModel.ModDependencyPackage
	queryResult := db.Preload("Dependencies").Where("id = ?", packageID).Find(&pack)
	if queryResult.Error != nil {
		return nil, queryResult.Error
	}
	if queryResult.RowsAffected == 0 {
		return &PackageInstallResult{
			PackageID:   packageID,
			GameDir:     gameDir,
			PluginRoot:  pluginRoot,
			Uninstalled: true,
			Skipped:     true,
			Message:     "整合包未安装，已跳过卸载",
		}, nil
	}

	dependencyResults := make([]DependencyInstallResult, 0, len(pack.Dependencies))
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, dependency := range pack.Dependencies {
			shared, err := dependencySharedByOtherPackages(tx, dependency.ID, packageID)
			if err != nil {
				return err
			}

			result := DependencyInstallResult{
				ID:      dependency.ID,
				Name:    stringValue(dependency.ModName),
				Version: stringValue(dependency.ModVersion),
				Skipped: shared,
			}
			result.PluginDir = dependencyPluginDir(pluginRoot, result.Name)
			if shared {
				result.Message = "仍被其它整合包引用，保留插件目录"
			} else {
				if err := removeDependencyPluginDir(pluginRoot, result.PluginDir); err != nil {
					return err
				}
				result.Removed = true
				result.Message = "插件目录已移除"
			}
			dependencyResults = append(dependencyResults, result)
		}

		if err := tx.Model(&pack).Association("Dependencies").Clear(); err != nil {
			return err
		}
		if err := tx.Delete(&pack).Error; err != nil {
			return err
		}

		for _, dependency := range pack.Dependencies {
			shared, err := dependencySharedByAnyPackage(tx, dependency.ID)
			if err != nil {
				return err
			}
			if !shared {
				if err := tx.Delete(&modDependenceModel.ModDependency{}, dependency.ID).Error; err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &PackageInstallResult{
		PackageID:     packageID,
		PackageName:   stringValue(pack.PackageName),
		GameDir:       gameDir,
		PluginRoot:    pluginRoot,
		Uninstalled:   true,
		Dependencies:  dependencyResults,
		Message:       "整合包卸载完成",
		BepInExStatus: CheckBepInExStatus(gameDir),
	}, nil
}

// ListInstalledPackageIds 返回本地已安装整合包的远端 ID 列表。
// 用途：Mod 市场加载后同步卡片安装状态；边界：仅读取本地 SQLite，不访问远端接口。
func ListInstalledPackageIds() ([]uint, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	ids := make([]uint, 0)
	if err := db.Model(&modDependenceModel.ModDependencyPackage{}).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}

	return ids, nil
}

func installDependency(bufferDir string, pluginRoot string, dependency modDependenceResponse.ModDependencyDTO) (DependencyInstallResult, error) {
	name := stringValue(dependency.ModName)
	if name == "" {
		return DependencyInstallResult{}, fmt.Errorf("dependency %d modName is required", dependency.ID)
	}
	if stringValue(dependency.Url) == "" {
		return DependencyInstallResult{}, fmt.Errorf("dependency %s download url is required", name)
	}

	zipPath, usedCache, err := downloadDependencyZip(bufferDir, dependency)
	if err != nil {
		return DependencyInstallResult{}, err
	}

	extractPath, err := extractDependencyZip(bufferDir, zipPath, dependency)
	if err != nil {
		return DependencyInstallResult{}, err
	}

	pluginDir := dependencyPluginDir(pluginRoot, name)
	if !isSubPath(pluginRoot, pluginDir) {
		return DependencyInstallResult{}, fmt.Errorf("plugin path escapes plugin root: %s", pluginDir)
	}
	if err := os.RemoveAll(pluginDir); err != nil {
		return DependencyInstallResult{}, err
	}
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		return DependencyInstallResult{}, err
	}
	if err := copyDirectoryContents(extractPath, pluginDir); err != nil {
		return DependencyInstallResult{}, err
	}

	return DependencyInstallResult{
		ID:        dependency.ID,
		Name:      name,
		Version:   stringValue(dependency.ModVersion),
		PluginDir: pluginDir,
		ZipPath:   zipPath,
		Installed: true,
		Message:   dependencyInstallMessage(usedCache),
	}, nil
}

func downloadDependencyZip(bufferDir string, dependency modDependenceResponse.ModDependencyDTO) (string, bool, error) {
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", false, err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf(
		"%d-%s-%s.zip",
		dependency.ID,
		sanitizePathSegment(stringValue(dependency.ModName), "mod"),
		sanitizePathSegment(stringValue(dependency.ModVersion), "version"),
	))
	if isValidZipFile(zipPath) {
		return zipPath, true, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", false, err
	}

	downloadURL, err := resolveDependencyDownloadURL(stringValue(dependency.Url))
	if err != nil {
		return "", false, err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", false, err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", false, err
	}

	client := http.Client{Timeout: 180 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", false, fmt.Errorf("download %s returned HTTP %d", stringValue(dependency.ModName), response.StatusCode)
	}

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", false, err
	}
	_, copyErr := io.Copy(partialFile, response.Body)
	closeErr := partialFile.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return "", false, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return "", false, closeErr
	}

	if err := os.Rename(partialPath, zipPath); err != nil {
		return "", false, err
	}
	if !isValidZipFile(zipPath) {
		_ = os.Remove(zipPath)
		return "", false, fmt.Errorf("downloaded dependency is not a valid zip: %s", zipPath)
	}

	return zipPath, false, nil
}

func dependencyInstallMessage(usedCache bool) string {
	if usedCache {
		return "依赖安装完成，已优先使用本地缓存"
	}

	return "依赖安装完成"
}

func extractDependencyZip(bufferDir string, zipPath string, dependency modDependenceResponse.ModDependencyDTO) (string, error) {
	extractPath := filepath.Join(bufferDir, fmt.Sprintf(
		"extract-%d-%s-%s",
		dependency.ID,
		sanitizePathSegment(stringValue(dependency.ModName), "mod"),
		sanitizePathSegment(stringValue(dependency.ModVersion), "version"),
	))
	if !isSubPath(bufferDir, extractPath) {
		return "", fmt.Errorf("extract path escapes buffer directory: %s", extractPath)
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

	for _, file := range reader.File {
		if err := extractZipFile(file, extractPath); err != nil {
			return "", err
		}
	}

	return extractPath, nil
}

func saveInstalledPackage(pack modDependenceResponse.ModDependencyPackageDTO) error {
	db, err := requireDatabase()
	if err != nil {
		return err
	}

	packageRecord, dependencyRecords, err := packageDTOToModel(pack)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for index := range dependencyRecords {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&dependencyRecords[index]).Error; err != nil {
				return err
			}
		}

		packageWithoutDependencies := packageRecord
		packageWithoutDependencies.Dependencies = nil
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&packageWithoutDependencies).Error; err != nil {
			return err
		}

		packageRecord.Dependencies = dependencyRecords
		if err := tx.Model(&packageRecord).Association("Dependencies").Replace(dependencyRecords); err != nil {
			return err
		}

		return deleteUnreferencedDependencies(tx)
	})
}

func stalePackageDependencies(packageID uint, nextDependencies []modDependenceResponse.ModDependencyDTO) ([]modDependenceModel.ModDependency, error) {
	db, err := requireDatabase()
	if err != nil {
		return nil, err
	}

	var installedPackage modDependenceModel.ModDependencyPackage
	queryResult := db.Preload("Dependencies").Where("id = ?", packageID).Find(&installedPackage)
	if queryResult.Error != nil {
		return nil, queryResult.Error
	}
	if queryResult.RowsAffected == 0 {
		return nil, nil
	}

	nextDependencyIDs := make(map[uint]struct{}, len(nextDependencies))
	for _, dependency := range nextDependencies {
		nextDependencyIDs[dependency.ID] = struct{}{}
	}

	staleDependencies := make([]modDependenceModel.ModDependency, 0)
	for _, dependency := range installedPackage.Dependencies {
		if _, exists := nextDependencyIDs[dependency.ID]; exists {
			continue
		}

		shared, err := dependencySharedByOtherPackages(db, dependency.ID, packageID)
		if err != nil {
			return nil, err
		}
		if !shared {
			staleDependencies = append(staleDependencies, dependency)
		}
	}

	return staleDependencies, nil
}

func removeStaleDependencyPluginDirs(pluginRoot string, dependencies []modDependenceModel.ModDependency) error {
	for _, dependency := range dependencies {
		pluginDir := dependencyPluginDir(pluginRoot, stringValue(dependency.ModName))
		if err := removeDependencyPluginDir(pluginRoot, pluginDir); err != nil {
			return err
		}
	}

	return nil
}

func deleteUnreferencedDependencies(tx *gorm.DB) error {
	return tx.
		Where("id NOT IN (SELECT DISTINCT mod_dependency_id FROM mod_dependency_package_relations)").
		Delete(&modDependenceModel.ModDependency{}).
		Error
}

func packageDTOToModel(pack modDependenceResponse.ModDependencyPackageDTO) (modDependenceModel.ModDependencyPackage, []modDependenceModel.ModDependency, error) {
	updateTime, err := parseOptionalRemoteTime(pack.UpdateTime)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}
	gameVersions, err := jsonValue(pack.GameVersions)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}
	tags, err := jsonValue(pack.Tags)
	if err != nil {
		return modDependenceModel.ModDependencyPackage{}, nil, err
	}

	dependencies := make([]modDependenceModel.ModDependency, 0, len(pack.Dependencies))
	for _, dependency := range pack.Dependencies {
		dependencyRecord, err := dependencyDTOToModel(dependency)
		if err != nil {
			return modDependenceModel.ModDependencyPackage{}, nil, err
		}
		dependencies = append(dependencies, dependencyRecord)
	}

	return modDependenceModel.ModDependencyPackage{
		ID:                 pack.ID,
		PackageName:        cloneStringPointer(pack.PackageName),
		PackageVersion:     cloneStringPointer(pack.PackageVersion),
		PackageDescription: cloneStringPointer(pack.PackageDescription),
		PackageSize:        cloneInt64Pointer(pack.PackageSize),
		DownloadCount:      cloneInt64Pointer(pack.DownloadCount),
		DependencyCount:    cloneInt64Pointer(pack.DependencyCount),
		GameVersions:       gameVersions,
		Tags:               tags,
		Source:             cloneStringPointer(pack.Source),
		Url:                cloneStringPointer(pack.Url),
		CoverUrl:           cloneStringPointer(pack.CoverUrl),
		Author:             cloneStringPointer(pack.Author),
		Status:             cloneIntPointer(pack.Status),
		Trustworthiness:    cloneInt64Pointer(pack.Trustworthiness),
		UpdateTime:         updateTime,
		Config:             cloneStringPointer(pack.Config),
		Ex:                 cloneStringPointer(pack.Ex),
		Dependencies:       dependencies,
	}, dependencies, nil
}

func dependencyDTOToModel(dependency modDependenceResponse.ModDependencyDTO) (modDependenceModel.ModDependency, error) {
	updateTime, err := parseOptionalRemoteTime(dependency.UpdateTime)
	if err != nil {
		return modDependenceModel.ModDependency{}, err
	}
	gameVersions, err := jsonValue(dependency.GameVersions)
	if err != nil {
		return modDependenceModel.ModDependency{}, err
	}

	return modDependenceModel.ModDependency{
		ID:              dependency.ID,
		ModName:         cloneStringPointer(dependency.ModName),
		ModVersion:      cloneStringPointer(dependency.ModVersion),
		ModDescription:  cloneStringPointer(dependency.ModDescription),
		GameVersions:    gameVersions,
		UpdateTime:      updateTime,
		Source:          cloneStringPointer(dependency.Source),
		Url:             cloneStringPointer(dependency.Url),
		Trustworthiness: cloneInt64Pointer(dependency.Trustworthiness),
		Config:          cloneStringPointer(dependency.Config),
		Ex:              cloneStringPointer(dependency.Ex),
	}, nil
}

func dependencySharedByOtherPackages(db *gorm.DB, dependencyID uint, packageID uint) (bool, error) {
	var count int64
	err := db.Table("mod_dependency_package_relations").
		Where("mod_dependency_id = ? AND mod_dependency_package_id <> ?", dependencyID, packageID).
		Count(&count).Error
	return count > 0, err
}

func dependencySharedByAnyPackage(db *gorm.DB, dependencyID uint) (bool, error) {
	var count int64
	err := db.Table("mod_dependency_package_relations").
		Where("mod_dependency_id = ?", dependencyID).
		Count(&count).Error
	return count > 0, err
}

func removeDependencyPluginDir(pluginRoot string, pluginDir string) error {
	if pluginDir == "" || !isSubPath(pluginRoot, pluginDir) {
		return fmt.Errorf("plugin path escapes plugin root: %s", pluginDir)
	}

	return os.RemoveAll(pluginDir)
}

func dependencyPluginDir(pluginRoot string, name string) string {
	return filepath.Join(pluginRoot, sanitizePathSegment(name, "mod"))
}

func modDependencyBufferDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), bufferRootDirName, modDependencyBufferDirName), nil
}

func resolveDependencyDownloadURL(rawURL string) (string, error) {
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

func requireDatabase() (*gorm.DB, error) {
	if core.GLOBAL.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	return core.GLOBAL.DB, nil
}

func jsonValue(value interface{}) (datatypes.JSON, error) {
	if value == nil {
		return datatypes.JSON([]byte("null")), nil
	}

	content, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	return datatypes.JSON(content), nil
}

func parseOptionalRemoteTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	trimmedValue := strings.TrimSpace(*value)
	for _, format := range formats {
		parsed, err := time.Parse(format, trimmedValue)
		if err == nil {
			return &parsed, nil
		}
	}

	return nil, nil
}

func sanitizePathSegment(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}

	invalidPattern := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)
	value = invalidPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, ". ")
	if value == "" {
		return fallback
	}
	if len(value) > 80 {
		value = value[:80]
	}

	return value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return strings.TrimSpace(*value)
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}

	nextValue := *value
	return &nextValue
}
