package updater

// 文件职责：检查新版本并准备经过校验的更新文件。
// 使用：系统更新服务调用 PrepareAppUpdate，保存返回的安装准备结果。
// 边界：准备期间不替换当前程序；实际替换由服务在应用退出时交接。

import (
	"context"
	"errors"
	"fmt"

	updaterApply "DysonSphereBox/helper/updater/apply"
	updaterClient "DysonSphereBox/helper/updater/client"
	"DysonSphereBox/helper/updater/progress"
	updaterRelease "DysonSphereBox/helper/updater/release"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// ErrUpdateInProgress 职责：标识下载准备仍在执行；使用：服务和关闭钩子识别忙碌状态；边界：不用于助手交接失败。
var ErrUpdateInProgress = errors.New("更新包正在下载或校验，请完成后再退出")

// PrepareAppUpdate 职责：下载并校验最新版本，返回退出安装所需信息；使用：系统更新服务调用；边界：无更新不创建文件，准备成功不等于安装成功。
// 逻辑流：1. 检查版本 => 2. 重新定位 Release => 3. 解析当前程序路径 => 4. 下载校验并暂存新版。
func PrepareAppUpdate(ctx context.Context, report progress.Reporter) (*UpdateResult, *updaterApply.PreparedUpdate, error) {
	check, err := CheckAppUpdate(ctx)
	if err != nil {
		return nil, nil, err
	}
	if !check.HasUpdate {
		return &UpdateResult{TargetVersion: check.LatestVersion, Message: "当前已经是最新版本。", Check: *check}, nil, nil
	}
	updater, source, err := updaterClient.New(appUpdateAPIBaseURL, appUpdateChecksum)
	if err != nil {
		return nil, nil, err
	}
	release, found, err := updater.DetectLatest(ctx, updaterRelease.Repository(appUpdateOwner, appUpdateRepo))
	if err != nil {
		return nil, nil, fmt.Errorf("detect latest app release before update: %w", err)
	}
	if !found {
		return nil, nil, errors.New("latest app release disappeared before update")
	}
	executablePath, err := selfupdate.ExecutablePath()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve executable path: %w", err)
	}
	prepared, err := updaterApply.PrepareRelease(ctx, source, release, executablePath, updaterApply.Config{
		CommandName: appUpdateCommand, ChecksumName: appUpdateChecksum, DownloadProgress: report,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("prepare app update: %w", err)
	}
	return &UpdateResult{
		Prepared: true, TargetVersion: release.Version(),
		Message: "更新包已下载并校验，退出后将安装并自动重新打开应用。", Check: *check,
	}, prepared, nil
}
