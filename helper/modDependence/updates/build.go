package updates

// 文件职责：将安装摘要和已加载的 Thunderstore 索引转换为更新列表。
// 使用：InstallerService 单次加载索引后调用 BuildUpdates，也可用于纯数据推导。
// 边界：不访问数据库、缓存、网络或文件系统；不编排安装或记录审计日志。

import (
	"strings"

	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/installer/contract"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoremeta"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreview"
)

// BuildUpdates 基于已加载数据构造严格递增的更新列表。
// 职责：按 Owner/Name 匹配安装身份，选择最新 active 版本并复用市场 DTO 转换。
// 使用：服务层传入安装摘要和一次加载的完整索引；返回结果保持安装摘要的顺序。
// 边界：过滤非 Thunderstore、身份缺失、废弃和框架包；版本无效、相等或更低时跳过，不修改输入数据。
// 逻辑流：1. 建立可用包身份索引 => 2. 匹配已安装包 => 3. 选取最新 active 版本 => 4. 比较语义版本并生成 DTO。
func BuildUpdates(installed []contract.InstalledPackageSummary, packages []thunderstoreAPI.Package) []PackageUpdate {
	packagesByIdentity := make(map[[2]string]thunderstoreAPI.Package, len(packages))
	for _, pkg := range packages {
		if pkg.IsDeprecated || thunderstoreview.IsExternalOrFrameworkPackage(pkg) {
			continue
		}
		packagesByIdentity[packageIdentity(pkg.Owner, pkg.Name)] = pkg
	}
	result := make([]PackageUpdate, 0)
	for _, local := range installed {
		identity := packageIdentity(local.Namespace, local.Name)
		if local.Source != string(sourceConstant.ModSourceThunderstore) || identity[0] == "" || identity[1] == "" {
			continue
		}
		pkg, ok := packagesByIdentity[identity]
		if !ok {
			continue
		}
		latest, ok := thunderstoremeta.LatestActiveVersion(pkg.Versions)
		if !ok || !thunderstoremeta.IsNewerVersion(latest.VersionNumber, local.Version) {
			continue
		}
		result = append(result, PackageUpdate{
			PackageKey:     local.PackageKey,
			CurrentVersion: local.Version,
			LatestVersion:  latest.VersionNumber,
			LatestPackage:  thunderstoreview.PackageToRemote(pkg, latest, packages),
		})
	}
	return result
}

// packageIdentity 职责：生成不含版本的双字段身份；使用：检查和执行更新时统一比较 Owner/Name；边界：只忽略大小写和首尾空白，不解析包 key 或合并不同字段。
func packageIdentity(namespace string, name string) [2]string {
	return [2]string{strings.ToLower(strings.TrimSpace(namespace)), strings.ToLower(strings.TrimSpace(name))}
}
