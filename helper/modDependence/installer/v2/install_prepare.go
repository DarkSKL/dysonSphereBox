package v2

// 文件职责：串联 V2 远端包安装前置准备阶段。
// 使用：InstallRemoteModPackageWithOptions 注册取消上下文后调用，获取 BepInEx 状态、缓存目录和待安装 artifact 队列。
// 边界：不复制 artifact 文件、不保存组件记录、不保存主包完成状态；BepInEx 和 artifact 队列准备由同包独立文件维护。
// 逻辑流：1. 捕获旧安装快照 => 2. 保存 running 状态并重置 manifest => 3. 准备 BepInEx 和缓存目录 => 4. 解析并记录计划 artifact。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	installConstant "DysonSphereBox/constant/modDependence/install"
	bepinexContract "DysonSphereBox/helper/modDependence/bepinex/contract"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/installmanifest"
	"DysonSphereBox/helper/modDependence/installer/v2/installstore"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	cleanupEntry "DysonSphereBox/helper/modDependence/installer/v2/sessioncleanup/entry"
	"DysonSphereBox/helper/modDependence/installer/v2/sessionstate"
	"context"
)

// installPreparation 保存安装准备阶段产生的共享上下文。
// 职责：承载后续 artifact 安装和完成结果所需的 BepInEx 状态、目录和 artifact 队列。
// 使用：installRemoteArtifacts 和 completeRemotePackageInstall 读取该结构，避免主入口重复传递多个零散参数。
// 边界：只在单次安装流程内使用，不持久化、不暴露给服务层或前端。
type installPreparation struct {
	// bepInExResult 职责：保存 BepInEx 安装或检测结果；使用：完成结果返回给前端；边界：失败时不会创建 preparation。
	bepInExResult *bepinexContract.BepInExInstallResult
	// gameDir 职责：保存游戏根目录；使用：artifact 文件复制和最终结果构造；边界：来自 BepInEx 状态，不在此结构中二次校验。
	gameDir string
	// bufferDir 职责：保存 V2 安装缓存目录；使用：下载、解压和本地 artifact 处理；边界：创建和清理由 buffer/sessioncleanup 子包负责。
	bufferDir string
	// artifacts 职责：保存主包和依赖解析后的安装队列；使用：installRemoteArtifacts 按顺序安装；边界：队列内容由 artifactresolve 负责去重和过滤。
	artifacts []v2Artifact.Artifact
}

// prepareRemotePackageInstall 准备远端包安装环境。
// 职责：初始化安装状态、准备 BepInEx、解析 artifact 队列并记录计划。
// 使用：InstallRemoteModPackageWithOptions 注册取消上下文后调用。
// 边界：不会复制文件或保存组件记录；准备阶段失败会触发会话清理。
func prepareRemotePackageInstall(ctx context.Context, pack modDependenceResponse.RemoteModPackageDTO, options contract.InstallRemotePackageOptions) (*installPreparation, error) {
	previousPackage, err := capturePreviousPackageIfNeeded(pack, options)
	if err != nil {
		return nil, err
	}
	if err := installstore.SaveRemotePackageState(pack, string(installConstant.StateRunning), "prepare", "准备安装", 1, "", 0); err != nil {
		return nil, err
	}
	if options.ResetSession {
		if err := installmanifest.Reset(pack.Key, previousPackage); err != nil {
			return nil, err
		}
	}

	progressEvent.Package(pack.Key, "prepare", "准备安装", 1)
	if err := cleanupEntry.CheckCancelled(ctx, pack.Key); err != nil {
		return nil, err
	}

	bepInExResult, err := prepareBepInExForRemoteInstall(ctx, pack)
	if err != nil {
		return nil, err
	}

	bufferDir, artifacts, err := prepareArtifactsForRemoteInstall(ctx, pack)
	if err != nil {
		return nil, err
	}

	return &installPreparation{
		bepInExResult: bepInExResult,
		gameDir:       bepInExResult.Status.GameDir,
		bufferDir:     bufferDir,
		artifacts:     artifacts,
	}, nil
}

// capturePreviousPackageIfNeeded 按安装选项捕获旧主包快照。
// 职责：在重置会话前保存旧安装记录，供失败恢复和覆盖记录过滤使用。
// 使用：prepareRemotePackageInstall 初始化 manifest 前调用。
// 边界：ResetSession 为 false 时返回 nil，不读取数据库、不修改 manifest。
func capturePreviousPackageIfNeeded(pack modDependenceResponse.RemoteModPackageDTO, options contract.InstallRemotePackageOptions) (*sessionstate.PackageSnapshot, error) {
	if !options.ResetSession {
		return nil, nil
	}
	return installmanifest.CapturePreviousPackage(pack.Key)
}
