package updater

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"DysonSphereBox/core"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	selfupdateApply "github.com/creativeprojects/go-selfupdate/update"
)

const (
	appUpdateOwner      = "Dark-Skl"
	appUpdateRepo       = "dyson-sphere-box"
	appUpdateAPIBaseURL = "https://gitee.com/api/v5"
	appUpdateChecksum   = "checksums.txt"
	appUpdateCommand    = "DysonSphereBox"
)

// CheckResult 是前端展示应用更新状态的数据结构。
type CheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	AssetName      string `json:"assetName"`
	AssetURL       string `json:"assetUrl"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	ReleaseURL     string `json:"releaseUrl"`
	PublishedAt    string `json:"publishedAt"`
	OS             string `json:"os"`
	Arch           string `json:"arch"`
}

// UpdateResult 是执行自更新后的返回结果。
type UpdateResult struct {
	Updated       bool        `json:"updated"`
	TargetVersion string      `json:"targetVersion"`
	Message       string      `json:"message"`
	Check         CheckResult `json:"check"`
}

// CheckAppUpdate 检查 Gitee Release 中是否存在新版本。
func CheckAppUpdate(ctx context.Context) (*CheckResult, error) {
	updater, _, err := newSelfUpdater()
	if err != nil {
		return nil, err
	}

	release, found, err := updater.DetectLatest(ctx, repository())
	if err != nil {
		return nil, fmt.Errorf("detect latest app release: %w", err)
	}

	result := &CheckResult{
		CurrentVersion: core.Version,
		OS:             runtime.GOOS,
		Arch:           runtime.GOARCH,
	}
	if !found {
		return result, nil
	}

	result.LatestVersion = release.Version()
	result.AssetName = release.AssetName
	result.AssetURL = release.AssetURL
	result.ReleaseName = release.Name
	result.ReleaseNotes = release.ReleaseNotes
	result.ReleaseURL = release.URL
	result.PublishedAt = formatTime(release.PublishedAt)
	result.HasUpdate = isReleaseNewer(release, core.Version)

	return result, nil
}

// UpdateAppSelf 下载最新 Release 附件并替换当前可执行文件。
func UpdateAppSelf(ctx context.Context) (*UpdateResult, error) {
	check, err := CheckAppUpdate(ctx)
	if err != nil {
		return nil, err
	}
	if !check.HasUpdate {
		return &UpdateResult{
			Updated:       false,
			TargetVersion: check.LatestVersion,
			Message:       "当前已经是最新版本。",
			Check:         *check,
		}, nil
	}

	updater, source, err := newSelfUpdater()
	if err != nil {
		return nil, err
	}

	release, found, err := updater.DetectLatest(ctx, repository())
	if err != nil {
		return nil, fmt.Errorf("detect latest app release before update: %w", err)
	}
	if !found {
		return nil, errors.New("latest app release disappeared before update")
	}

	executablePath, err := selfupdate.ExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("resolve executable path: %w", err)
	}

	if err := applyRelease(ctx, source, release, executablePath); err != nil {
		return nil, fmt.Errorf("update app executable: %w", err)
	}

	return &UpdateResult{
		Updated:       true,
		TargetVersion: release.Version(),
		Message:       "更新文件已写入，重启应用后生效。",
		Check:         *check,
	}, nil
}

func newSelfUpdater() (*selfupdate.Updater, *GiteeSource, error) {
	source, err := NewGiteeSource(GiteeConfig{
		APIBaseURL: appUpdateAPIBaseURL,
	})
	if err != nil {
		return nil, nil, err
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: source,
		Validator: &selfupdate.ChecksumValidator{
			UniqueFilename: appUpdateChecksum,
		},
	})
	if err != nil {
		return nil, nil, err
	}

	return updater, source, nil
}

func applyRelease(ctx context.Context, source *GiteeSource, release *selfupdate.Release, executablePath string) error {
	reader, err := source.DownloadReleaseAsset(ctx, release, release.AssetID)
	if err != nil {
		return fmt.Errorf("download release asset %q: %w", release.AssetName, err)
	}
	defer func(reader io.ReadCloser) {
		err := reader.Close()
		if err != nil {
			fmt.Printf("failed to close reader: %s\n", err)
		}
	}(reader)

	assetData, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("read release asset %q: %w", release.AssetName, err)
	}
	if err := validateReleaseAsset(ctx, source, release, assetData); err != nil {
		return err
	}

	executableReader, err := selfupdate.DecompressCommand(
		bytes.NewReader(assetData),
		release.AssetName,
		archiveExecutableName(),
		release.OS,
		release.Arch,
	)
	if err != nil {
		return err
	}

	if err := selfupdateApply.Apply(executableReader, selfupdateApply.Options{
		TargetPath: executablePath,
	}); err != nil {
		return err
	}

	return ensureWindowsExecutableExtension(executablePath)
}

func validateReleaseAsset(ctx context.Context, source *GiteeSource, release *selfupdate.Release, assetData []byte) error {
	if len(release.ValidationChain) == 0 {
		return fmt.Errorf("validation file not found: %q", appUpdateChecksum)
	}

	validator := &selfupdate.ChecksumValidator{UniqueFilename: appUpdateChecksum}
	validationName := release.AssetName
	validationData := assetData
	for _, validationAsset := range release.ValidationChain {
		reader, err := source.DownloadReleaseAsset(ctx, release, validationAsset.ValidationAssetID)
		if err != nil {
			return fmt.Errorf("download validation asset %q: %w", validationAsset.ValidationAssetName, err)
		}

		nextValidationData, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return fmt.Errorf("read validation asset %q: %w", validationAsset.ValidationAssetName, readErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close validation asset %q: %w", validationAsset.ValidationAssetName, closeErr)
		}

		if err := validator.Validate(validationName, validationData, nextValidationData); err != nil {
			return fmt.Errorf("validate release asset %q: %w", validationName, err)
		}

		validationName = validationAsset.ValidationAssetName
		validationData = nextValidationData
	}

	return nil
}

func archiveExecutableName() string {
	if runtime.GOOS == "windows" {
		return appUpdateCommand + ".exe"
	}

	return appUpdateCommand
}

func ensureWindowsExecutableExtension(executablePath string) error {
	if runtime.GOOS != "windows" || strings.EqualFold(filepath.Ext(executablePath), ".exe") {
		return nil
	}

	executablePathWithExt := executablePath + ".exe"
	_ = os.Remove(executablePathWithExt)
	if err := os.Rename(executablePath, executablePathWithExt); err != nil {
		return fmt.Errorf("rename updated executable to %q: %w", executablePathWithExt, err)
	}

	return nil
}

func repository() selfupdate.Repository {
	return selfupdate.NewRepositorySlug(appUpdateOwner, appUpdateRepo)
}

func isReleaseNewer(release *selfupdate.Release, current string) bool {
	if release == nil {
		return false
	}

	normalizedCurrent := strings.TrimSpace(current)
	if normalizedCurrent == "" || strings.Contains(normalizedCurrent, "dev") {
		return true
	}
	if !strings.HasPrefix(strings.ToLower(normalizedCurrent), "v") {
		normalizedCurrent = "v" + normalizedCurrent
	}

	return release.GreaterThan(normalizedCurrent)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Local().Format("2006-01-02 15:04:05")
}
