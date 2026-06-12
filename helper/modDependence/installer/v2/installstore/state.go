package installstore

// 文件职责：保存 V2 安装器主包的阶段性状态。
// 使用：安装开始、进度变化、失败、冲突持久化和恢复流程调用，用于前端展示可恢复状态。
// 边界：不会覆盖 InstallManifest，避免丢失恢复和冲突决策数据；不负责发进度事件或写审计日志。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/modelconvert"
	"time"
)

// SaveRemotePackageState 保存或初始化主包安装状态。
// 职责：在安装开始、失败、冲突等阶段写入可恢复状态。
// 使用：安装入口和冲突持久化流程调用。
// 边界：不会覆盖 InstallManifest，避免丢失恢复和冲突决策数据。
func SaveRemotePackageState(pack modDependenceResponse.RemoteModPackageDTO, state string, stage string, status string, percent int, errorMessage string, bytesPerSecond int64) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	record, err := modelconvert.FromRemotePackage(pack)
	if err != nil {
		return err
	}
	record.InstallState = state
	record.InstallStage = stage
	record.InstallStatus = status
	record.InstallProgress = progresscalc.Clamp(percent)
	record.ErrorMessage = errorMessage
	record.CurrentSpeed = bytesPerSecond
	record.UpdatedAt = time.Now()

	return db.
		Omit("InstallManifest").
		Where("package_key = ?", record.PackageKey).
		Assign(record).
		FirstOrCreate(&record).
		Error
}
