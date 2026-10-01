package v2

// 文件职责：准备 V2 远端包安装所需的 BepInEx 前置环境。
// 使用：prepareRemotePackageInstall 在安装主包和依赖前调用，确保游戏目录具备 BepInEx 注入基础。
// 边界：只调用 BepInEx 安装 helper 并校验结果，不解析 Mod artifact、不复制 Mod 文件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"
	bepinexInstall "DysonSphereBox/helper/modDependence/bepinex/install"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"context"
	"fmt"
	"strings"
)

// prepareBepInExForRemoteInstall 安装或确认 BepInEx 环境。
// 职责：调用 BepInEx 安装器并验证安装后状态可用于 Mod 安装。
// 使用：prepareRemotePackageInstall 在缓存目录和 artifact 队列准备前调用。
// 边界：BepInEx 缺失时返回错误阻断安装；不负责展示审计日志。
func prepareBepInExForRemoteInstall(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO) (*bepinexContract.BepInExInstallResult, error) {
	bepInExResult, err := bepinexInstall.InstallBepInExWithContext(ctx)
	if err != nil {
		return nil, err
	}
	if !bepInExResult.Status.Installed {
		return nil, fmt.Errorf("BepInEx is not ready: %s", strings.Join(bepInExResult.Status.MissingFiles, ", "))
	}
	progressEvent.Package(pack.Key, "prepare", "BepInEx 已就绪", 8)
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return nil, err
	}
	return bepInExResult, nil
}
