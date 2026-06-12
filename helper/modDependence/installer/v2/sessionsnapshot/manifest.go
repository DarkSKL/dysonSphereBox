package sessionsnapshot

// 文件职责：读取并恢复 V2 安装会话 manifest 与历史主包快照。
// 使用：冲突继续、失败清理、缓存清理和覆盖安装恢复旧主包时调用。
// 边界：不执行文件删除、目录清理、复制回滚或安装状态保存；manifest 解析失败时返回空状态。

import (
	"encoding/json"
	"strings"

	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionmodel"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LoadManifest 读取并解析安装会话 manifest。
// 职责：从 installed package 的 InstallManifest 中恢复会话状态。
// 使用：冲突继续、失败清理、缓存清理和会话追加流程调用。
// 边界：数据库不可用、记录不存在或 JSON 解析失败时返回空 manifest。
func LoadManifest(packageKey string) sessionstate.Manifest {
	db, err := shared.RequireDatabase()
	if err != nil {
		return sessionstate.Manifest{}
	}
	record := installedModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		return sessionstate.Manifest{}
	}
	manifest := sessionstate.Manifest{}
	if len(record.InstallManifest) > 0 {
		_ = json.Unmarshal(record.InstallManifest, &manifest)
	}
	return manifest
}

// RestorePreviousPackage 恢复覆盖安装前的主包记录。
// 职责：根据 manifest.PreviousPackage upsert 原 installed package。
// 使用：失败清理完成后恢复用户原本已安装版本。
// 边界：没有 PreviousPackage 时直接返回 nil；不恢复文件系统内容。
func RestorePreviousPackage(db *gorm.DB, packageKey string, manifest sessionstate.Manifest) error {
	if manifest.PreviousPackage == nil {
		return nil
	}
	record := sessionmodel.SnapshotToPackage(packageKey, *manifest.PreviousPackage)
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "package_key"}}, UpdateAll: true}).Create(&record).Error
}
