package modDependence

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence"
	"DysonSphereBox/utils"

	"golang.org/x/sys/windows"
	"gorm.io/gorm"
)

const (
	thunderstoreDSPPackageIndexURL = "https://thunderstore.io/c/dyson-sphere-program/api/v1/package/"
	bepInExOwner                   = "xiaoye97"
	bepInExName                    = "BepInEx"
	bufferRootDirName              = "buffer"
	bepInExBufferDirName           = "bepInEx"
)

type BepInExStatus struct {
	GameDir          string   `json:"gameDir"`
	Installed        bool     `json:"installed"`
	BepInExDir       string   `json:"bepInExDir"`
	MissingFiles     []string `json:"missingFiles"`
	CoreExists       bool     `json:"coreExists"`
	PreloaderExists  bool     `json:"preloaderExists"`
	WinHTTPExists    bool     `json:"winHTTPExists"`
	DoorstopExists   bool     `json:"doorstopExists"`
	DoorstopValid    bool     `json:"doorstopValid"`
	InstalledVersion string   `json:"installedVersion"`
}

type BepInExPackage struct {
	Owner        string   `json:"owner"`
	Name         string   `json:"name"`
	FullName     string   `json:"fullName"`
	Version      string   `json:"version"`
	DownloadURL  string   `json:"downloadUrl"`
	Dependencies []string `json:"dependencies"`
}

type BepInExInstallResult struct {
	GameDir     string          `json:"gameDir"`
	BufferDir   string          `json:"bufferDir"`
	Installed   bool            `json:"installed"`
	Skipped     bool            `json:"skipped"`
	Message     string          `json:"message"`
	Package     *BepInExPackage `json:"package,omitempty"`
	ZipPath     string          `json:"zipPath,omitempty"`
	ExtractPath string          `json:"extractPath,omitempty"`
	Status      BepInExStatus   `json:"status"`
}

// BepInExUninstallResult 表示 BepInEx 卸载后的本地清理结果。
// 用途：设置页展示卸载反馈；边界：只删除 BepInEx 注入相关文件和本地 Mod 安装记录，不触碰游戏本体目录。
type BepInExUninstallResult struct {
	GameDir        string        `json:"gameDir"`
	Removed        bool          `json:"removed"`
	Skipped        bool          `json:"skipped"`
	Message        string        `json:"message"`
	RemovedPaths   []string      `json:"removedPaths"`
	ClearedRecords bool          `json:"clearedRecords"`
	Status         BepInExStatus `json:"status"`
}

func GetConfiguredGameDir() (string, error) {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.GameStartFilePath == nil {
		return "", fmt.Errorf("configure is not initialized")
	}

	configuredPath, err := core.GLOBAL.Configure.GameStartFilePath.Get()
	if err != nil {
		return "", err
	}

	gameDir := utils.NormalizeDysonSphereDir(configuredPath)
	if gameDir == "" {
		return "", fmt.Errorf("GameStartFilePath is not configured or invalid")
	}

	if err := assertDysonSphereGameDir(gameDir); err != nil {
		return "", err
	}

	return gameDir, nil
}

func CheckBepInExStatus(gameDir string) BepInExStatus {
	gameDir = utils.CleanPath(gameDir)
	bepInExDir := filepath.Join(gameDir, "BepInEx")

	status := BepInExStatus{
		GameDir:      gameDir,
		BepInExDir:   bepInExDir,
		MissingFiles: make([]string, 0),
	}

	requiredFiles := []struct {
		relativePath string
		present      *bool
	}{
		{filepath.Join("BepInEx", "core", "BepInEx.dll"), &status.CoreExists},
		{filepath.Join("BepInEx", "core", "BepInEx.Preloader.dll"), &status.PreloaderExists},
		{"winhttp.dll", &status.WinHTTPExists},
		{"doorstop_config.ini", &status.DoorstopExists},
	}

	for _, requiredFile := range requiredFiles {
		exists := utils.FileExists(filepath.Join(gameDir, requiredFile.relativePath))
		*requiredFile.present = exists
		if !exists {
			status.MissingFiles = append(status.MissingFiles, requiredFile.relativePath)
		}
	}

	status.DoorstopValid = isDoorstopConfigValid(gameDir)
	if status.DoorstopExists && !status.DoorstopValid {
		status.MissingFiles = append(status.MissingFiles, "doorstop_config.ini 配置未启用或目标程序集错误")
	}
	status.Installed = len(status.MissingFiles) == 0
	status.InstalledVersion = readBepInExVersion(gameDir)

	return status
}

func SearchAndLoadBepInEx(selectedPath string) (*BepInExStatus, error) {
	gameDir := utils.NormalizeDysonSphereDir(selectedPath)
	if gameDir == "" {
		return nil, fmt.Errorf("selected directory does not contain %s", utils.DysonSphereExeName)
	}

	if err := assertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}

	status := CheckBepInExStatus(gameDir)
	return &status, nil
}

// UninstallBepInEx 移除游戏目录下的 BepInEx 与全部基于 BepInEx 的本地 Mod 记录。
// 边界：删除范围限定为游戏目录内的 BepInEx 目录、Doorstop 注入文件和本地 SQLite 安装记录。
func UninstallBepInEx() (*BepInExUninstallResult, error) {
	gameDir, err := GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}

	if err := assertDysonSphereGameDir(gameDir); err != nil {
		return nil, err
	}

	status := CheckBepInExStatus(gameDir)
	removeTargets := []string{
		filepath.Join(gameDir, "BepInEx"),
		filepath.Join(gameDir, "winhttp.dll"),
		filepath.Join(gameDir, "doorstop_config.ini"),
		filepath.Join(gameDir, ".doorstop_version"),
		filepath.Join(gameDir, "changelog.txt"),
	}

	removedPaths := make([]string, 0, len(removeTargets))
	for _, targetPath := range removeTargets {
		if !isSubPath(gameDir, targetPath) {
			return nil, fmt.Errorf("remove path escapes game directory: %s", targetPath)
		}

		if _, err := os.Stat(targetPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		if err := os.RemoveAll(targetPath); err != nil {
			return nil, err
		}
		removedPaths = append(removedPaths, targetPath)
	}

	clearedRecords, err := clearInstalledModDependencyRecords()
	if err != nil {
		return nil, err
	}

	nextStatus := CheckBepInExStatus(gameDir)
	skipped := !status.Installed && len(removedPaths) == 0 && !clearedRecords
	message := "BepInEx 与相关 Mod 依赖已卸载"
	if skipped {
		message = "未检测到可卸载的 BepInEx 文件或本地 Mod 记录"
	}

	return &BepInExUninstallResult{
		GameDir:        gameDir,
		Removed:        !skipped,
		Skipped:        skipped,
		Message:        message,
		RemovedPaths:   removedPaths,
		ClearedRecords: clearedRecords,
		Status:         nextStatus,
	}, nil
}

func InstallBepInEx() (*BepInExInstallResult, error) {
	gameDir, err := GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}

	status := CheckBepInExStatus(gameDir)
	if status.CoreExists && status.PreloaderExists && status.WinHTTPExists && status.DoorstopExists && !status.DoorstopValid {
		if err := ensureDoorstopConfig(gameDir); err != nil {
			return nil, err
		}
		status = CheckBepInExStatus(gameDir)
		return &BepInExInstallResult{
			GameDir:   gameDir,
			Installed: false,
			Skipped:   true,
			Message:   "BepInEx 已安装，已修复 Doorstop 配置",
			Status:    status,
		}, nil
	}
	if status.Installed {
		if err := ensureDoorstopConfig(gameDir); err != nil {
			return nil, err
		}
		status = CheckBepInExStatus(gameDir)
		return &BepInExInstallResult{
			GameDir:   gameDir,
			Installed: false,
			Skipped:   true,
			Message:   "BepInEx 已安装，已确认 Doorstop 配置",
			Status:    status,
		}, nil
	}

	pkg, err := ResolveLatestBepInExPackage()
	if err != nil {
		return nil, err
	}

	bufferDir, err := bepInExBufferDir()
	if err != nil {
		return nil, err
	}

	zipPath, err := downloadPackage(bufferDir, pkg)
	if err != nil {
		return nil, err
	}

	extractPath, err := extractPackage(bufferDir, zipPath, pkg)
	if err != nil {
		return nil, err
	}

	packRoot := filepath.Join(extractPath, "BepInExPack")
	if info, err := os.Stat(packRoot); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("BepInEx package structure is invalid: missing BepInExPack")
	}

	if err := copyDirectoryContents(packRoot, gameDir); err != nil {
		return nil, err
	}
	if err := ensureDoorstopConfig(gameDir); err != nil {
		return nil, err
	}

	nextStatus := CheckBepInExStatus(gameDir)
	if !nextStatus.Installed {
		return nil, fmt.Errorf("BepInEx install incomplete, missing: %s", strings.Join(nextStatus.MissingFiles, ", "))
	}

	return &BepInExInstallResult{
		GameDir:     gameDir,
		BufferDir:   bufferDir,
		Installed:   true,
		Skipped:     false,
		Message:     "BepInEx 安装完成",
		Package:     pkg,
		ZipPath:     zipPath,
		ExtractPath: extractPath,
		Status:      nextStatus,
	}, nil
}

func ResolveLatestBepInExPackage() (*BepInExPackage, error) {
	packages, err := fetchThunderstorePackages()
	if err != nil {
		return nil, err
	}

	for _, pkg := range packages {
		if pkg.Owner != bepInExOwner || pkg.Name != bepInExName {
			continue
		}

		version, ok := latestActiveVersion(pkg.Versions)
		if !ok {
			return nil, fmt.Errorf("BepInEx has no active Thunderstore version")
		}

		return &BepInExPackage{
			Owner:        pkg.Owner,
			Name:         pkg.Name,
			FullName:     version.FullName,
			Version:      version.VersionNumber,
			DownloadURL:  version.DownloadURL,
			Dependencies: append([]string(nil), version.Dependencies...),
		}, nil
	}

	return nil, fmt.Errorf("Thunderstore package not found: %s/%s", bepInExOwner, bepInExName)
}

type thunderstorePackage struct {
	Owner    string                `json:"owner"`
	Name     string                `json:"name"`
	Versions []thunderstoreVersion `json:"versions"`
}

type thunderstoreVersion struct {
	IsActive      bool     `json:"is_active"`
	DateCreated   string   `json:"date_created"`
	FullName      string   `json:"full_name"`
	VersionNumber string   `json:"version_number"`
	DownloadURL   string   `json:"download_url"`
	Dependencies  []string `json:"dependencies"`
}

func fetchThunderstorePackages() ([]thunderstorePackage, error) {
	client := http.Client{Timeout: 60 * time.Second}
	response, err := client.Get(thunderstoreDSPPackageIndexURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Thunderstore returned HTTP %d", response.StatusCode)
	}

	var packages []thunderstorePackage
	if err := json.NewDecoder(response.Body).Decode(&packages); err != nil {
		return nil, err
	}

	return packages, nil
}

func latestActiveVersion(versions []thunderstoreVersion) (thunderstoreVersion, bool) {
	var latest thunderstoreVersion
	hasLatest := false
	for _, version := range versions {
		if !version.IsActive {
			continue
		}
		if !hasLatest || compareThunderstoreDate(version.DateCreated, latest.DateCreated) > 0 {
			latest = version
			hasLatest = true
		}
	}

	return latest, hasLatest
}

func compareThunderstoreDate(left string, right string) int {
	leftTime, leftErr := parseThunderstoreTime(left)
	rightTime, rightErr := parseThunderstoreTime(right)
	if leftErr == nil && rightErr == nil {
		if leftTime.After(rightTime) {
			return 1
		}
		if leftTime.Before(rightTime) {
			return -1
		}
		return 0
	}

	return strings.Compare(left, right)
}

func parseThunderstoreTime(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}

	var parseErr error
	for _, format := range formats {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed, nil
		}
		parseErr = err
	}

	if parseErr == nil {
		parseErr = errors.New("empty time")
	}

	return time.Time{}, parseErr
}

func downloadPackage(bufferDir string, pkg *BepInExPackage) (string, error) {
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf("%s-%s-%s.zip", pkg.Owner, pkg.Name, pkg.Version))
	if isValidZipFile(zipPath) {
		return zipPath, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	client := http.Client{Timeout: 180 * time.Second}
	response, err := client.Get(pkg.DownloadURL)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download %s/%s returned HTTP %d", pkg.Owner, pkg.Name, response.StatusCode)
	}

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(partialFile, response.Body)
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
		return "", fmt.Errorf("downloaded file is not a valid zip: %s", zipPath)
	}

	return zipPath, nil
}

func extractPackage(bufferDir string, zipPath string, pkg *BepInExPackage) (string, error) {
	extractPath := filepath.Join(bufferDir, fmt.Sprintf("extract-%s-%s-%s", pkg.Owner, pkg.Name, pkg.Version))
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

func extractZipFile(file *zip.File, destinationDir string) error {
	targetPath := filepath.Join(destinationDir, file.Name)
	if !isSubPath(destinationDir, targetPath) {
		return fmt.Errorf("zip entry escapes destination: %s", file.Name)
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, file.Mode())
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func copyDirectoryContents(sourceDir string, destinationDir string) error {
	return filepath.WalkDir(sourceDir, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}

		destinationPath := filepath.Join(destinationDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, 0o755)
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		return copyFile(sourcePath, destinationPath, info.Mode())
	})
}

func copyFile(sourcePath string, destinationPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o755); err != nil {
		return err
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer target.Close()

	_, err = io.Copy(target, source)
	return err
}

func isValidZipFile(path string) bool {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer reader.Close()

	return true
}

func bepInExBufferDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), bufferRootDirName, bepInExBufferDirName), nil
}

func assertDysonSphereGameDir(gameDir string) error {
	requiredPaths := []string{
		utils.DysonSphereExeName,
		"DSPGAME_Data",
		"MonoBleedingEdge",
	}

	missingPaths := make([]string, 0)
	for _, requiredPath := range requiredPaths {
		if _, err := os.Stat(filepath.Join(gameDir, requiredPath)); err != nil {
			if os.IsNotExist(err) {
				missingPaths = append(missingPaths, requiredPath)
				continue
			}
			return err
		}
	}

	if len(missingPaths) > 0 {
		return fmt.Errorf("game directory is incomplete, missing: %s", strings.Join(missingPaths, ", "))
	}

	return nil
}

func isSubPath(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)

	relativePath, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return relativePath == "." || (!strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) && relativePath != "..")
}

func readBepInExVersion(gameDir string) string {
	if version := readWindowsFileVersion(filepath.Join(gameDir, "BepInEx", "core", "BepInEx.dll")); version != "" {
		return version
	}

	content, err := os.ReadFile(filepath.Join(gameDir, ".doorstop_version"))
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

func isDoorstopConfigValid(gameDir string) bool {
	config, err := readDoorstopConfig(gameDir)
	if err != nil {
		return false
	}

	return strings.EqualFold(config["enabled"], "true") &&
		strings.EqualFold(filepath.Clean(config["targetassembly"]), filepath.Clean(filepath.Join("BepInEx", "core", "BepInEx.Preloader.dll")))
}

func readDoorstopConfig(gameDir string) (map[string]string, error) {
	content, err := os.ReadFile(filepath.Join(gameDir, "doorstop_config.ini"))
	if err != nil {
		return nil, err
	}

	config := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		config[strings.ToLower(strings.TrimSpace(parts[0]))] = strings.Trim(strings.TrimSpace(parts[1]), `"`)
	}

	return config, nil
}

func ensureDoorstopConfig(gameDir string) error {
	cfgPath := filepath.Join(gameDir, "doorstop_config.ini")
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	nextContent := upsertDoorstopSetting(string(content), "enabled", "true")
	nextContent = upsertDoorstopSetting(nextContent, "targetAssembly", `BepInEx\core\BepInEx.Preloader.dll`)
	if nextContent == string(content) {
		return nil
	}

	return os.WriteFile(cfgPath, []byte(nextContent), 0o644)
}

func clearInstalledModDependencyRecords() (bool, error) {
	if core.GLOBAL.DB == nil {
		return false, fmt.Errorf("database is not initialized")
	}

	var packageCount int64
	if err := core.GLOBAL.DB.Table("mod_dependency_packages").Count(&packageCount).Error; err != nil {
		return false, err
	}
	var dependencyCount int64
	if err := core.GLOBAL.DB.Table("mod_dependencies").Count(&dependencyCount).Error; err != nil {
		return false, err
	}

	err := core.GLOBAL.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("DELETE FROM mod_dependency_package_relations").Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&modDependenceModel.ModDependencyPackage{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&modDependenceModel.ModDependency{}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return packageCount > 0 || dependencyCount > 0, nil
}

func upsertDoorstopSetting(content string, key string, value string) string {
	pattern := regexp.MustCompile(`(?mi)^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*).*$`)
	replacement := "${1}" + value
	if pattern.MatchString(content) {
		return pattern.ReplaceAllString(content, replacement)
	}

	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\r\n"
	}

	return content + key + "=" + value + "\r\n"
}

func readWindowsFileVersion(path string) string {
	if !utils.FileExists(path) {
		return ""
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}

	versionDLL := windows.NewLazySystemDLL("version.dll")
	getFileVersionInfoSizeW := versionDLL.NewProc("GetFileVersionInfoSizeW")
	getFileVersionInfoW := versionDLL.NewProc("GetFileVersionInfoW")
	verQueryValueW := versionDLL.NewProc("VerQueryValueW")

	size, _, _ := getFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(pathPtr)), 0)
	if size == 0 {
		return ""
	}

	buffer := make([]byte, size)
	ok, _, _ := getFileVersionInfoW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		size,
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if ok == 0 {
		return ""
	}

	rootPtr, err := windows.UTF16PtrFromString(`\`)
	if err != nil {
		return ""
	}

	var fixedInfoPtr uintptr
	var fixedInfoLength uint32
	ok, _, _ = verQueryValueW.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&fixedInfoPtr)),
		uintptr(unsafe.Pointer(&fixedInfoLength)),
	)
	if ok == 0 || fixedInfoPtr == 0 || fixedInfoLength < uint32(unsafe.Sizeof(vsFixedFileInfo{})) {
		return ""
	}

	fixedInfo := (*vsFixedFileInfo)(unsafe.Pointer(fixedInfoPtr))
	if fixedInfo.Signature != 0xFEEF04BD {
		return ""
	}

	return fmt.Sprintf(
		"%d.%d.%d.%d",
		fixedInfo.FileVersionMS>>16,
		fixedInfo.FileVersionMS&0xffff,
		fixedInfo.FileVersionLS>>16,
		fixedInfo.FileVersionLS&0xffff,
	)
}

type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}
