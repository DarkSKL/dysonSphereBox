package updater

// 文件职责：实现应用自更新的下载、校验与替换编排流程。
// 使用：系统更新服务在用户确认更新后调用 UpdateAppSelf。
// 边界：只编排自更新 helper，不处理前端提示或系统日志；写入成功后仍需要用户重启应用。

import (
	"context"
	"errors"
	"fmt"

	updaterApply "DysonSphereBox/helper/updater/apply"
	updaterClient "DysonSphereBox/helper/updater/client"
	updaterRelease "DysonSphereBox/helper/updater/release"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// UpdateAppSelf 下载最新 Release 附件并替换当前可执行文件。
// 职责：在存在新版本时下载、校验并应用应用自更新文件。
// 使用：由系统更新服务在用户确认更新后调用。
// 边界：写入成功后需要用户重启应用才会运行新版本；无更新时不会下载资产。
// 逻辑流：1. 复用检查更新结果 => 2. 无更新则直接返回 => 3. 重新检测 Release 防止目标丢失 => 4. 定位当前可执行文件 => 5. 下载校验并替换可执行文件。
func UpdateAppSelf(ctx context.Context) (*UpdateResult, error) {
	check, err := CheckAppUpdate(ctx)
	if err != nil {
		return nil, err
	}
	if !check.HasUpdate {
		return &UpdateResult{Updated: false, TargetVersion: check.LatestVersion, Message: "当前已经是最新版本。", Check: *check}, nil
	}
	updater, source, err := updaterClient.New(appUpdateAPIBaseURL, appUpdateChecksum)
	if err != nil {
		return nil, err
	}
	release, found, err := updater.DetectLatest(ctx, updaterRelease.Repository(appUpdateOwner, appUpdateRepo))
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
	if err := updaterApply.ApplyRelease(ctx, source, release, executablePath, updaterApply.Config{
		CommandName:  appUpdateCommand,
		ChecksumName: appUpdateChecksum,
	}); err != nil {
		return nil, fmt.Errorf("update app executable: %w", err)
	}
	return &UpdateResult{Updated: true, TargetVersion: release.Version(), Message: "更新文件已写入，重启应用后生效。", Check: *check}, nil
}
