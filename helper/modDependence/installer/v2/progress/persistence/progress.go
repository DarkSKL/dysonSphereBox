package persistence

// 文件职责：把 V2 运行时安装进度同步到本地 installed package 记录。
// 使用：event.Emit 在发送事件时调用，用于页面刷新后仍可恢复安装、失败或冲突状态。
// 边界：不创建安装包、不写 manifest 内容、不参与事务回滚；数据库不可用时静默跳过以避免影响主流程。

import (
	"time"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/progress/payload"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
)

// UpdatePackageInstallProgress 根据运行时进度事件更新安装表状态。
// 职责：把进度事件同步到 installed package，支撑页面刷新后的状态恢复。
// 使用：event.Emit 发送事件前后调用。
// 边界：数据库不可用时静默跳过；只更新进度字段，不创建新的远端包业务数据。
func UpdatePackageInstallProgress(data payload.Payload) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return
	}
	state := data.State
	if state == string(installConstant.StateDone) {
		state = string(installConstant.StateInstalled)
	}
	updates := map[string]interface{}{
		"install_state":    state,
		"install_stage":    data.Stage,
		"install_status":   data.Status,
		"install_progress": data.Percent,
		"current_speed":    data.BytesPerSecond,
		"updated_at":       time.Now(),
	}
	if data.ErrorMessage != "" {
		updates["error_message"] = data.ErrorMessage
	}
	_ = db.Model(&modDependenceModel.ModInstalledPackage{}).
		Where("package_key = ?", data.PackageKey).
		Updates(updates).
		Error
}
