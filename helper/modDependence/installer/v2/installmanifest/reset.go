package installmanifest

// 文件职责：初始化 V2 安装会话 manifest。
// 使用：远端包首次安装或覆盖安装开始前调用 Reset。
// 边界：只写 manifest 和安装状态，不清理文件、不解析依赖、不发送进度事件。

import (
	"time"

	installConstant "DysonSphereBox/constant/modDependence/install"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"DysonSphereBox/helper/modDependence/installer/v2/conflictstate"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	installedModel "DysonSphereBox/model/modDependence/installed"
)

// Reset 重置当前包的安装会话 manifest。
// 职责：初始化 running 状态下的会话起点、历史包与组件关联快照和开始时间。
// 使用：远端包首次安装或覆盖安装开始前调用。
// 边界：只写 manifest 和安装状态，不清理文件、不解析依赖、不发送进度事件。
func Reset(packageKey string, previousPackage *sessionstate.PackageSnapshot) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}
	var previousComponents []string
	if previousPackage != nil {
		if err := db.Model(&installedModel.ModInstalledPackageComponent{}).
			Where("package_key = ?", packageKey).Pluck("component_key", &previousComponents).Error; err != nil {
			return err
		}
	}
	return conflictstate.SaveManifest(packageKey, string(installConstant.StateRunning), "prepare", "准备安装", 1, "", &sessionstate.Manifest{PreviousPackage: previousPackage, PreviousPackageComponents: previousComponents, StartedAt: time.Now().Format(time.RFC3339Nano)})
}
