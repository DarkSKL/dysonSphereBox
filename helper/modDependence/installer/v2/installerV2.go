package v2

// 文件职责：提供 V2 远端 Mod 包安装主入口。
// 使用：由 service/modDependence 的 InstallerService 发起普通安装或冲突继续后的安装流程时调用。
// 边界：只负责校验入口参数、注册取消句柄并串联安装阶段；准备、artifact 队列和完成记录由同包独立文件维护。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"fmt"
	"strings"
)

// InstallRemoteModPackage 安装统一来源 Mod 包。
// 职责：以默认重置会话模式启动 V2 远端包安装。
// 使用：支持 Thunderstore 与手动导入来源；Thunderstore 会先递归安装依赖。
// 边界：所有写入文件都会记录清单，便于卸载时精确清理 patchers、plugins 与 config。
func InstallRemoteModPackage(pack modDependenceResponse.RemoteModPackageDTO) (*contract.PackageInstallResult, error) {
	return InstallRemoteModPackageWithOptions(pack, contract.InstallRemotePackageOptions{ResetSession: true})
}

// InstallRemoteModPackageWithOptions 带用户冲突决策安装统一来源 Mod 包。
// 职责：编排 BepInEx 准备、依赖解析、artifact 下载/解压/复制、记录持久化和失败清理。
// 使用：首次安装不传决策；用户处理冲突后传入具体文件的覆盖、跳过或失败选择继续安装。
// 边界：遇到冲突时保存 manifest 并返回可恢复错误；取消或失败时按会话清理已写文件。
// 逻辑流：1. 注册取消句柄并初始化会话 => 2. 准备 BepInEx 和缓存目录 => 3. 解析主包与依赖 artifact => 4. 逐个处理文件并记录组件 => 5. 保存主包完成状态并返回结果。
func InstallRemoteModPackageWithOptions(pack modDependenceResponse.RemoteModPackageDTO, options contract.InstallRemotePackageOptions) (*contract.PackageInstallResult, error) {
	if strings.TrimSpace(pack.Key) == "" {
		return nil, fmt.Errorf("package key is required")
	}
	installCtx, unregisterInstallCancellation := cancel.Register(pack.Key)
	defer unregisterInstallCancellation()

	preparation, err := prepareRemotePackageInstall(installCtx, pack, options)
	if err != nil {
		return nil, err
	}

	results, err := installRemoteArtifacts(installCtx, pack, options, preparation)
	if err != nil {
		return nil, err
	}

	return completeRemotePackageInstall(installCtx, pack, preparation, results)
}
