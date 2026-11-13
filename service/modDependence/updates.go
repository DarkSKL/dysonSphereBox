package modDependence

// 文件职责：提供已安装市场包的更新检查与更新安装入口。
// 使用：模组市场通过 InstallerService 检查更新，再以旧包和目标包 key 发起更新。
// 边界：只编排查询、校验和现有安装服务；不直接下载、修改文件或改变冲突策略。

import (
	"context"
	"fmt"
	"strings"

	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Query "DysonSphereBox/helper/modDependence/installer/v2/query"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreindex"
	"DysonSphereBox/helper/modDependence/updates"
)

// CheckInstalledPackageUpdates 检查本地已安装 Thunderstore 包的可用更新。
// 职责：先读取并筛选安装摘要，再加载一次全量索引，由纯查询 helper 构造更新列表。
// 使用：模组市场加载或刷新更新状态时调用，forceRefresh 决定是否绕过现有索引缓存。
// 边界：身份缺失、废弃、框架或版本无法确认递增的包不返回更新；无候选时不请求远端，不执行安装。
// 逻辑流：1. 查询已安装摘要 => 2. 筛选 Thunderstore 完整身份 => 3. 单次加载索引 => 4. 构造严格递增的更新列表。
func (service *InstallerService) CheckInstalledPackageUpdates(forceRefresh bool) ([]updates.PackageUpdate, error) {
	installed, err := v2Query.ListInstalledPackageSummaries()
	if err != nil {
		return nil, err
	}
	candidates := make([]contract.InstalledPackageSummary, 0, len(installed))
	for _, pkg := range installed {
		if pkg.Source == string(sourceConstant.ModSourceThunderstore) && strings.TrimSpace(pkg.Namespace) != "" && strings.TrimSpace(pkg.Name) != "" {
			candidates = append(candidates, pkg)
		}
	}
	if len(candidates) == 0 {
		return []updates.PackageUpdate{}, nil
	}

	packages, err := thunderstoreindex.LoadPackages(context.Background(), !forceRefresh)
	if err != nil {
		return nil, err
	}
	return updates.BuildUpdates(candidates, packages), nil
}

// UpdateRemotePackage 将仍已安装的 Thunderstore 包更新到指定的更高 active 版本。
// 职责：重新校验本地入口和远端目标，复用受控安装选项与安装审计，并确认旧入口已被迁移。
// 使用：模组市场以 PackageUpdate.PackageKey 和 LatestPackage.Key 发起单包更新。
// 边界：拒绝跨来源、跨身份、等版本和降级；取消、回滚、版本替换沿用安装器，进度仍使用目标 key。
// 逻辑流：1. 校验 key => 2. 解析并校验更新目标 => 3. 强制主包安装以完成版本替换 => 4. 确认旧入口消失且目标已安装，否则返回错误。
func (service *InstallerService) UpdateRemotePackage(packageKey string, targetPackageKey string) (*contract.PackageInstallResult, error) {
	packageKey = strings.TrimSpace(packageKey)
	targetPackageKey = strings.TrimSpace(targetPackageKey)
	if packageKey == "" || targetPackageKey == "" {
		return nil, fmt.Errorf("已安装包 key 和更新目标 key 不能为空")
	}
	remote, err := updates.ResolveUpdate(packageKey, targetPackageKey)
	if err != nil {
		return nil, err
	}
	result, err := service.installRemotePackageWithOptions(*remote, contract.InstallRemotePackageOptions{
		ResetSession:       true,
		ForceRootReinstall: true,
	})
	if err != nil {
		return nil, err
	}

	installedKeys, err := v2Query.ListInstalledPackageKeys()
	if err != nil {
		return nil, fmt.Errorf("更新安装返回后无法确认入口迁移：%w", err)
	}
	targetInstalled := false
	for _, key := range installedKeys {
		if key == packageKey {
			return nil, fmt.Errorf("更新尚未完成：旧安装入口仍然存在：%s", packageKey)
		}
		if key == remote.Key {
			targetInstalled = true
		}
	}
	if !targetInstalled {
		return nil, fmt.Errorf("更新尚未完成：目标包未处于已安装状态：%s", remote.Key)
	}
	return result, nil
}
