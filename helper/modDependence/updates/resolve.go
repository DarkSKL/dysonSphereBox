package updates

// 文件职责：提供更新目标的只读查询与身份、版本校验。
// 使用：InstallerService.UpdateRemotePackage 在调用安装入口前，通过 ResolveUpdate 获取可信远端 DTO。
// 边界：复用安装摘要查询和 Thunderstore 索引缓存；不安装、不迁移入口、不修改用户模组或决定审计。

import (
	"context"
	"fmt"
	"strings"

	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/installer/contract"
	v2Query "DysonSphereBox/helper/modDependence/installer/v2/query"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreindex"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoremeta"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreview"
)

// ResolveUpdate 重新查询已安装入口并解析同身份的更高 active 版本。
// 职责：校验旧包仍已安装且来源为 Thunderstore，验证目标 key 身份、可用状态和语义版本递增。
// 使用：更新服务传入旧包 key 与目标 key；允许复用检查更新时已加载的索引缓存。
// 边界：拒绝废弃、框架、inactive、跨身份、等版本和降级目标；无效版本返回错误，不回退字符串比较。
// 逻辑流：1. 读取当前安装摘要 => 2. 校验旧入口与目标身份 => 3. 单次加载可复用索引 => 4. 匹配并校验 active 版本 => 5. 转换可信 DTO。
func ResolveUpdate(installedKey string, targetKey string) (*modDependenceResponse.RemoteModPackageDTO, error) {
	installedKey = strings.TrimSpace(installedKey)
	targetKey = strings.TrimSpace(targetKey)
	if installedKey == "" || targetKey == "" {
		return nil, fmt.Errorf("已安装包 key 和更新目标 key 不能为空")
	}
	installed, err := v2Query.ListInstalledPackageSummaries()
	if err != nil {
		return nil, err
	}
	var local *contract.InstalledPackageSummary
	for i := range installed {
		if installed[i].PackageKey == installedKey {
			local = &installed[i]
			break
		}
	}
	if local == nil {
		return nil, fmt.Errorf("原包已不处于已安装状态：%s", installedKey)
	}
	identity := packageIdentity(local.Namespace, local.Name)
	if local.Source != string(sourceConstant.ModSourceThunderstore) || identity[0] == "" || identity[1] == "" {
		return nil, fmt.Errorf("仅支持更新身份完整的已安装 Thunderstore 包：%s", installedKey)
	}
	parts := strings.Split(targetKey, ":")
	if len(parts) != 4 || parts[0] != string(sourceConstant.ModSourceThunderstore) || strings.TrimSpace(parts[3]) == "" {
		return nil, fmt.Errorf("无效的 Thunderstore 更新目标 key：%s", targetKey)
	}
	if packageIdentity(parts[1], parts[2]) != identity {
		return nil, fmt.Errorf("更新目标与已安装包的命名空间或名称不一致：%s", targetKey)
	}

	packages, err := thunderstoreindex.LoadPackages(context.Background(), true)
	if err != nil {
		return nil, err
	}
	for _, pkg := range packages {
		if packageIdentity(pkg.Owner, pkg.Name) != identity {
			continue
		}
		if pkg.IsDeprecated || thunderstoreview.IsExternalOrFrameworkPackage(pkg) {
			return nil, fmt.Errorf("更新目标已废弃或属于框架工具包：%s", targetKey)
		}
		for _, version := range pkg.Versions {
			if thunderstoreview.PackageKey(pkg, version) != targetKey {
				continue
			}
			if !version.IsActive {
				return nil, fmt.Errorf("更新目标版本已不可用：%s", targetKey)
			}
			if !thunderstoremeta.IsNewerVersion(version.VersionNumber, local.Version) {
				return nil, fmt.Errorf("更新目标语义版本必须严格高于当前版本：%s -> %s", local.Version, version.VersionNumber)
			}
			remote := thunderstoreview.PackageToRemote(pkg, version, packages)
			return &remote, nil
		}
	}
	return nil, fmt.Errorf("Thunderstore 更新目标不存在：%s", targetKey)
}
