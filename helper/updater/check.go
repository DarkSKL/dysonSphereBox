package updater

// 文件职责：实现应用自更新的远端版本检查流程。
// 使用：系统更新服务在用户检查更新或页面初始化时调用 CheckAppUpdate。
// 边界：只读取远端 Release 元数据，不下载资产、不替换可执行文件，也不记录服务层日志。

import (
	"DysonSphereBox/core"
	updaterClient "DysonSphereBox/helper/updater/client"
	updaterRelease "DysonSphereBox/helper/updater/release"
	"context"
	"fmt"
	"runtime"
)

// CheckAppUpdate 检查 Gitee Release 中是否存在新版本。
// 职责：读取 Gitee Release 元数据并判断是否存在可更新版本。
// 使用：由系统更新服务或设置页检查更新动作调用。
// 边界：只读取远端元数据，不下载或替换本地可执行文件。
// 逻辑流：1. 初始化 Gitee 更新源 => 2. 检测最新 Release => 3. 组装当前平台检查结果 => 4. 计算是否需要更新并返回。
func CheckAppUpdate(ctx context.Context) (*CheckResult, error) {
	updater, _, err := updaterClient.New(appUpdateAPIBaseURL, appUpdateChecksum)
	if err != nil {
		return nil, err
	}
	release, found, err := updater.DetectLatest(ctx, updaterRelease.Repository(appUpdateOwner, appUpdateRepo))
	if err != nil {
		return nil, fmt.Errorf("detect latest app release: %w", err)
	}
	result := &CheckResult{CurrentVersion: core.Version, OS: runtime.GOOS, Arch: runtime.GOARCH}
	if !found {
		return result, nil
	}
	result.LatestVersion = release.Version()
	result.AssetName = release.AssetName
	result.AssetURL = release.AssetURL
	result.ReleaseName = release.Name
	result.ReleaseNotes = release.ReleaseNotes
	result.ReleaseURL = release.URL
	result.PublishedAt = updaterRelease.FormatTime(release.PublishedAt)
	result.HasUpdate = updaterRelease.IsNewer(release, core.Version)
	return result, nil
}
