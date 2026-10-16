package system

// 文件职责：提供应用版本读取、更新检查和自更新执行的 Wails 服务。
// 使用：设置页或关于页通过生成绑定展示版本、检查 Gitee Release 并触发更新。
// 边界：只编排更新 helper 和审计日志；下载、校验、替换文件由 helper/updater 负责。

import (
	"context"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	appUpdater "DysonSphereBox/helper/updater"
)

// UpdateService 提供前端可调用的应用自更新能力。
// 职责：检查 Gitee Release 最新版本，并在用户确认后下载、校验和替换当前程序。
// 使用：设置页或关于页的更新检查与立即更新按钮通过 Wails 绑定调用。
// 边界：更新写入后需要用户重启应用；服务不负责自动关闭当前 Wails 进程。
type UpdateService struct{}

// GetAppVersion 返回当前应用构建版本。
// 职责：暴露当前构建注入的应用版本号。
// 使用：关于页、更新页或诊断信息展示当前版本时调用。
// 边界：只返回 core.Version，不访问远端版本源。
func (service *UpdateService) GetAppVersion() string {
	return core.Version
}

// CheckAppUpdate 检查 Gitee Release 是否存在可用的新版本。
// 职责：调用更新 helper 检查最新版本，只记录检查失败。
// 使用：前端点击检查更新或页面初始化更新状态时调用。
// 边界：只检查不写入程序文件；网络或版本源错误会记录日志并返回错误。
func (service *UpdateService) CheckAppUpdate() (*appUpdater.CheckResult, error) {
	result, err := appUpdater.CheckAppUpdate(context.Background())
	if err != nil {
		systemHelper.RecordUpdateCheckFailure(core.Version, err)
		return nil, err
	}

	return result, nil
}

// UpdateAppSelf 下载并应用最新版本。
// 职责：调用更新 helper 下载、校验并替换当前可执行文件。
// 使用：用户确认更新后由前端调用。
// 边界：更新完成后需要用户重启；失败时保留当前程序并记录错误日志。
// 逻辑流：1. 调用自更新 helper => 2. 失败则记录失败日志并返回错误 => 3. 成功则记录更新结果 => 4. 返回重启提示所需信息。
func (service *UpdateService) UpdateAppSelf() (*appUpdater.UpdateResult, error) {
	result, err := appUpdater.UpdateAppSelf(context.Background())
	if err != nil {
		systemHelper.RecordUpdateSelfFailure(core.Version, err)
		return nil, err
	}

	systemHelper.RecordUpdateSelfSuccess(core.Version, result)
	return result, nil
}
