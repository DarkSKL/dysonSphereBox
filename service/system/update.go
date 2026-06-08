package system

import (
	"context"

	"DysonSphereBox/core"
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
	return appUpdater.CheckAppUpdate(context.Background())
}

// UpdateAppSelf 下载并应用最新版本。
func (service *UpdateService) UpdateAppSelf() (*appUpdater.UpdateResult, error) {
	return appUpdater.UpdateAppSelf(context.Background())
}
