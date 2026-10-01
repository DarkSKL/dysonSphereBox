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
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"encoding/json"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveRemotePackage 保存主包安装完成状态。
// 职责：在同一事务中切换旧版本引用并标记主包 installed/done。
// 使用：所有 artifact 组件保存成功后调用。
// 边界：数据库提交失败保留 manifest 和备份供上层回滚；提交后备份清理不影响成功状态。
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
	manifest := sessionstate.Manifest{}
	record.InstallManifest = datatypes.JSON([]byte("{}"))
	if err := db.Transaction(func(tx *gorm.DB) error {
		var current installedModel.ModInstalledPackage
		if err := tx.Where("package_key = ?", pack.Key).Take(&current).Error; err != nil {
			return err
		}
		if len(current.InstallManifest) > 0 {
			if err := json.Unmarshal(current.InstallManifest, &manifest); err != nil {
				return err
			}
		}
		if err := commitVersionReplacements(tx, manifest); err != nil {
			return err
		}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "package_key"}}, UpdateAll: true}).Create(&record).Error
	}); err != nil {
		return err
	}
	cleanupBuffer.CleanupManifestBackups(manifest)
	return nil
}
