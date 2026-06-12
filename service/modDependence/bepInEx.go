package modDependence

// 文件职责：提供 BepInEx 前置框架安装、卸载、更新和状态检测的 Wails 服务。
// 使用：设置页、Mod 页或开服准备流程需要维护 BepInEx 环境时调用。
// 边界：服务层只编排 helper 和审计日志；具体下载、解压、文件校验由 helper 子包负责。

import (
	bepinexAudit "DysonSphereBox/helper/modDependence/audit/bepinex"
	bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"
	bepinexInstall "DysonSphereBox/helper/modDependence/bepinex/install"
	"DysonSphereBox/helper/modDependence/bepinex/remote"
	bepinexStatus "DysonSphereBox/helper/modDependence/bepinex/status"
	bepinexUninstall "DysonSphereBox/helper/modDependence/bepinex/uninstall"
	"DysonSphereBox/helper/modDependence/gamepath"
)

// BepInExService 提供前端可调用的 BepInEx 安装、卸载、检测和更新能力。
// 职责：编排 BepInEx 前置框架相关 Wails 服务方法并记录操作日志。
// 使用：设置页、Mod 页或开服准备流程需要检测/安装/卸载 BepInEx 时调用。
// 边界：服务层负责记录操作日志，具体文件下载、解压和状态检测由 helper 层完成。
type BepInExService struct {
}

// Install 安装 BepInEx 前置框架。
// 职责：触发 BepInEx 安装并记录安装结果。
// 使用：设置页或 Mod 页需要补齐前置环境时调用；失败会记录系统日志并返回错误。
// 边界：下载、解压、Doorstop 配置修复由 helper 处理，服务层不直接操作游戏目录。
func (service *BepInExService) Install() (*bepinexContract.BepInExInstallResult, error) {
	result, err := bepinexInstall.InstallBepInEx()
	if err != nil {
		bepinexAudit.RecordBepInExFailure("install", "BepInEx 安装失败", err)
		return nil, err
	}

	bepinexAudit.RecordBepInExInstallResult("install", result)
	return result, nil
}

// Uninstall 卸载当前游戏目录中的 BepInEx 前置框架。
// 职责：触发 BepInEx 卸载并记录卸载结果。
// 使用：设置页用户明确移除前置框架时调用。
// 边界：卸载流程由 helper 清理文件和安装记录，服务层只负责日志和结果透传。
func (service *BepInExService) Uninstall() (*bepinexContract.BepInExUninstallResult, error) {
	result, err := bepinexUninstall.UninstallBepInEx()
	if err != nil {
		bepinexAudit.RecordBepInExFailure("uninstall", "BepInEx 卸载失败", err)
		return nil, err
	}

	bepinexAudit.RecordBepInExUninstallSuccess(result)
	return result, nil
}

// Update 更新或补装 BepInEx 前置框架。
// 职责：在未安装时执行安装，在已安装时返回跳过结果。
// 使用：前端点击更新/修复 BepInEx 时调用。
// 边界：当前版本已安装时跳过重复安装，并返回可展示的跳过结果。
// 逻辑流：1. 读取当前状态 => 2. 已安装则返回跳过结果 => 3. 未安装则调用安装 helper => 4. 记录更新结果。
func (service *BepInExService) Update() (*bepinexContract.BepInExInstallResult, error) {
	status, err := service.GetStatus()
	if err != nil {
		return nil, err
	}
	if status.Installed {
		result := &bepinexContract.BepInExInstallResult{
			GameDir:   status.GameDir,
			Installed: false,
			Skipped:   true,
			Message:   "BepInEx 已安装，跳过重复安装",
			Status:    *status,
		}
		bepinexAudit.RecordBepInExInstallResult("update", result)
		return result, nil
	}

	result, err := bepinexInstall.InstallBepInEx()
	if err != nil {
		bepinexAudit.RecordBepInExFailure("update", "BepInEx 更新失败", err)
		return nil, err
	}

	bepinexAudit.RecordBepInExInstallResult("update", result)
	return result, nil
}

// GetLatestVersion 返回可用的最新 BepInEx 包信息。
// 职责：查询远端可用的最新 BepInEx 包元数据。
// 使用：前端展示可安装版本或更新提示时调用。
// 边界：读取前会先确认游戏目录配置有效，避免在未配置路径时做远端请求。
func (service *BepInExService) GetLatestVersion() (*bepinexContract.BepInExPackage, error) {
	if _, err := gamepath.GetConfiguredGameDir(); err != nil {
		return nil, err
	}

	return remote.ResolveLatestBepInExPackage()
}

// GetStatus 返回当前配置游戏目录下的 BepInEx 状态。
// 职责：读取当前游戏目录中的 BepInEx 文件和配置状态。
// 使用：设置页、Mod 页和开服页刷新前置框架状态时调用。
// 边界：未配置游戏目录时返回错误，不猜测默认路径。
func (service *BepInExService) GetStatus() (*bepinexContract.BepInExStatus, error) {
	gameDir, err := gamepath.GetConfiguredGameDir()
	if err != nil {
		return nil, err
	}

	status := bepinexStatus.CheckBepInExStatus(gameDir)
	return &status, nil
}

// SearchAndLoadBepInEx 检测用户指定路径或已配置路径中的 BepInEx 状态。
// 职责：根据用户选择路径或当前配置路径检测 BepInEx 状态。
// 使用：前端选择游戏目录后调用，用于立即刷新前置框架状态。
// 边界：指定路径不会在此处写入配置；空路径时回退到 GetStatus。
func (service *BepInExService) SearchAndLoadBepInEx(selectedPath string) (*bepinexContract.BepInExStatus, error) {
	if selectedPath != "" {
		return bepinexStatus.SearchAndLoadBepInEx(selectedPath)
	}

	return service.GetStatus()
}
