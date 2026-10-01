package installstore

// 文件职责：保存 V2 安装器主包的阶段性状态。
// 使用：安装开始、进度变化、失败、冲突持久化和恢复流程调用，用于前端展示可恢复状态。
// 边界：不会覆盖 InstallManifest，避免丢失恢复和冲突决策数据；不负责发进度事件或写审计日志。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/modelconvert"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/datatypes"
	"gorm.io/gorm"
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

// BeginRemotePackage 原子初始化 running 状态及恢复快照。
// 职责：防止旧主包状态已经覆盖而恢复证据尚未保存；使用：安装准备阶段调用；边界：继续会话保留 manifest，事务失败保留原记录。
func BeginRemotePackage(pack modDependenceResponse.RemoteModPackageDTO, previous *sessionstate.PackageSnapshot, reset bool) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	record, err := modelconvert.FromRemotePackage(pack)
	if err != nil {
		return err
	}
	record.InstallState, record.InstallStage, record.InstallStatus = string(installConstant.StateRunning), "prepare", "准备安装"
	record.InstallProgress = 1
	record.UpdatedAt = time.Now()
	return db.Transaction(func(tx *gorm.DB) error {
		if reset {
			manifest := sessionstate.Manifest{PreviousPackage: previous, StartedAt: time.Now().Format(time.RFC3339Nano)}
			if previous != nil {
				if err := tx.Model(&installedModel.ModInstalledPackageComponent{}).Where("package_key = ?", pack.Key).
					Pluck("component_key", &manifest.PreviousPackageComponents).Error; err != nil {
					return err
				}
			}
			content, err := json.Marshal(manifest)
			if err != nil {
				return err
			}
			record.InstallManifest = datatypes.JSON(content)
			return tx.Where("package_key = ?", pack.Key).Assign(record).FirstOrCreate(&record).Error
		}
		return tx.Omit("InstallManifest").Where("package_key = ?", pack.Key).Assign(record).FirstOrCreate(&record).Error
	})
}
