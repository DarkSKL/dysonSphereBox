package installstore

// 文件职责：保存 V2 安装器主包的完成状态。
// 使用：所有 artifact 组件保存成功后由安装编排流程调用，用于标记主包 installed/done。
// 边界：会清理当前包安装会话备份并重置 manifest；不负责复制文件、记录审计日志或发进度事件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/modelconvert"
	cleanupBuffer "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/buffer"
	"gorm.io/datatypes"
	"gorm.io/gorm/clause"
)

// SaveRemotePackage 保存主包安装完成状态。
// 职责：把远端包 DTO 转为 installed package 模型，并标记 installed/done。
// 使用：所有 artifact 组件保存成功后调用。
// 边界：会清理当前包安装会话备份并清空 manifest；失败时上层执行清理流程。
func SaveRemotePackage(pack modDependenceResponse.RemoteModPackageDTO) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	record, err := modelconvert.FromRemotePackage(pack)
	if err != nil {
		return err
	}
	record.InstallState = string(installConstant.StateInstalled)
	record.InstallStage = "done"
	record.InstallStatus = "安装完成"
	record.InstallProgress = 100
	record.ErrorMessage = ""
	record.CurrentSpeed = 0
	if err := cleanupBuffer.CleanupSessionBackups(pack.Key); err != nil {
		return err
	}
	record.InstallManifest = datatypes.JSON([]byte("{}"))
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "package_key"}}, UpdateAll: true}).Create(&record).Error
}
