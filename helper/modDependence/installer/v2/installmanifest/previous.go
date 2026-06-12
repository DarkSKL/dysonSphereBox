package installmanifest

// 文件职责：捕获覆盖安装前的历史主包和组件快照。
// 使用：安装入口和 artifact 追加 manifest 时保存失败清理可恢复证据。
// 边界：只读取 installed 模型并转换快照，不写 manifest、不恢复记录、不处理文件系统。

import (
	"strings"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionmodel"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
	"gorm.io/gorm"
)

// CapturePreviousPackage 捕获覆盖安装前的主包快照。
// 职责：在重新安装已完成包前保存可恢复的 installed package 字段。
// 使用：安装入口 ResetSession=true 时调用，供失败或取消清理恢复历史记录。
// 边界：只捕获 installed 或空状态记录；不存在或非稳定状态时返回 nil。
func CapturePreviousPackage(packageKey string) (*sessionstate.PackageSnapshot, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	record := installedModel.ModInstalledPackage{}
	if err := db.Where("package_key = ?", strings.TrimSpace(packageKey)).Take(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if record.InstallState != string(installConstant.StateInstalled) && record.InstallState != "" {
		return nil, nil
	}
	return sessionmodel.PackageToSnapshot(record), nil
}

// appendComponentSnapshot 追加组件安装前的可恢复快照。
// 职责：从 installed component 表读取组件旧记录并去重保存。
// 使用：组件被本次会话写入前调用，供失败清理恢复组件表。
// 边界：组件不存在或读取失败时静默跳过，不阻断安装流程。
func appendComponentSnapshot(db *gorm.DB, snapshots []sessionstate.ComponentSnapshot, componentKey string) []sessionstate.ComponentSnapshot {
	componentKey = strings.TrimSpace(componentKey)
	if componentKey == "" {
		return snapshots
	}
	for _, snapshot := range snapshots {
		if snapshot.ComponentKey == componentKey {
			return snapshots
		}
	}
	component := installedModel.ModInstalledComponent{}
	if err := db.Where("component_key = ?", componentKey).Take(&component).Error; err != nil {
		return snapshots
	}
	return append(snapshots, sessionmodel.ComponentToSnapshot(component))
}
