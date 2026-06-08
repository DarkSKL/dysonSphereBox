package system

import (
	"context"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	appUpdater "DysonSphereBox/helper/updater"
)

// UpdateService 提供前端可调用的应用自更新能力。
// 用途：检查 Gitee Release 最新版本，并在用户确认后下载、校验和替换当前程序。
// 边界：更新写入后需要用户重启应用；服务不负责自动关闭当前 Wails 进程。
type UpdateService struct{}

// GetAppVersion 返回当前应用构建版本。
func (service *UpdateService) GetAppVersion() string {
	return core.Version
}

// CheckAppUpdate 检查 Gitee Release 是否存在可用的新版本。
func (service *UpdateService) CheckAppUpdate() (*appUpdater.CheckResult, error) {
	result, err := appUpdater.CheckAppUpdate(context.Background())
	if err != nil {
		systemHelper.RecordUpdateCheckFailure(core.Version, err)
		return nil, err
	}

	systemHelper.RecordUpdateCheckSuccess(core.Version, result)
	return result, nil
}

// UpdateAppSelf 下载并应用最新版本。
func (service *UpdateService) UpdateAppSelf() (*appUpdater.UpdateResult, error) {
	result, err := appUpdater.UpdateAppSelf(context.Background())
	if err != nil {
		systemHelper.RecordUpdateSelfFailure(core.Version, err)
		return nil, err
	}

	systemHelper.RecordUpdateSelfSuccess(core.Version, result)
	return result, nil
}
