package thunderstoreview

// 文件职责：封装 Thunderstore 包展示过滤、依赖链判断与统一 key 生成规则。
// 使用：marketplace 根包在构造 Thunderstore 市场列表和解析单包 key 时调用。
// 边界：只基于已加载的 Thunderstore 原始索引做纯规则判断，不请求远端、不读写缓存、不修改安装状态。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	bepinexConstant "DysonSphereBox/constant/modDependence/bepinex"
	"fmt"
	"strings"
)

// IsExternalOrFrameworkPackage 判断包是否应从普通 Mod 市场列表隐藏。
// 职责：过滤 BepInEx 本体和 Thunderstore Tools 分类包。
// 使用：marketplace Thunderstore 列表遍历包索引时调用。
// 边界：只影响市场展示，不影响依赖解析和安装器按 key 解析依赖。
func IsExternalOrFrameworkPackage(pkg thunderstoreAPI.Package) bool {
	if pkg.Owner == bepinexConstant.Owner && pkg.Name == bepinexConstant.Name {
		return true
	}
	for _, category := range pkg.Categories {
		if strings.EqualFold(category, "Tools") {
			return true
		}
	}
	return false
}

// VersionDependsOnBepInEx 判断版本依赖链是否最终依赖 BepInEx。
// 职责：递归解析 Thunderstore 依赖，确认包适合 DSP/BepInEx 安装链路。
// 使用：marketplace Thunderstore 列表过滤市场包时调用。
// 边界：循环依赖通过 visited 截断；无法解析的依赖不会使结果为 true。
// 逻辑流：1. 根据包和版本生成访问 key => 2. 检查循环访问 => 3. 遍历依赖声明 => 4. 命中 BepInEx 直接返回 true => 5. 查找依赖版本并递归判断。
func VersionDependsOnBepInEx(pkg thunderstoreAPI.Package, version thunderstoreAPI.Version, packages []thunderstoreAPI.Package, visited map[string]struct{}) bool {
	packageID := fullName(pkg) + "-" + version.VersionNumber
	if _, exists := visited[packageID]; exists {
		return false
	}
	visited[packageID] = struct{}{}

	for _, dependency := range version.Dependencies {
		dependencyInfo := parseDependency(dependency, packages)
		if dependencyInfo.Namespace == bepinexConstant.Owner && dependencyInfo.Name == bepinexConstant.Name {
			return true
		}
		dependencyPackage, dependencyVersion, ok := findPackageVersion(packages, dependencyInfo.Namespace, dependencyInfo.Name, dependencyInfo.Version)
		if !ok {
			continue
		}
		if VersionDependsOnBepInEx(dependencyPackage, dependencyVersion, packages, visited) {
			return true
		}
	}

	return false
}

// findPackageVersion 在包索引中查找指定依赖版本。
// 职责：按 namespace、name 和 version 定位 active Thunderstore 版本。
// 使用：VersionDependsOnBepInEx 递归判断依赖链时调用。
// 边界：只返回 active 版本；找不到时返回 false 和零值。
func findPackageVersion(packages []thunderstoreAPI.Package, namespace string, name string, version string) (thunderstoreAPI.Package, thunderstoreAPI.Version, bool) {
	for _, pkg := range packages {
		if pkg.Owner != namespace || pkg.Name != name {
			continue
		}
		for _, pkgVersion := range pkg.Versions {
			if pkgVersion.VersionNumber == version && pkgVersion.IsActive {
				return pkg, pkgVersion, true
			}
		}
	}
	return thunderstoreAPI.Package{}, thunderstoreAPI.Version{}, false
}

// PackageKey 生成统一 Thunderstore 包 key。
// 职责：把来源、命名空间、名称和版本组合成跨层稳定身份。
// 使用：marketplace DTO 转换、ResolveRemotePackage 匹配和安装记录使用。
// 边界：不做字符串清洗，调用方传入的 Thunderstore 字段应来自来源索引。
func PackageKey(pkg thunderstoreAPI.Package, version thunderstoreAPI.Version) string {
	return fmt.Sprintf("thunderstore:%s:%s:%s", pkg.Owner, pkg.Name, version.VersionNumber)
}

// fullName 返回 Thunderstore 包完整名。
// 职责：兼容来源 FullName 缺失时的 owner-name 组合。
// 使用：依赖循环检测和依赖解析时调用。
// 边界：只处理包名文本，不包含版本号。
func fullName(pkg thunderstoreAPI.Package) string {
	if pkg.FullName != "" {
		return pkg.FullName
	}
	return pkg.Owner + "-" + pkg.Name
}

// splitFullName 拆分 Thunderstore 完整包名。
// 职责：把 `namespace-name` 形式拆成命名空间和包名。
// 使用：parseDependency 处理依赖字符串时调用。
// 边界：格式不符合时命名空间为空、名称保留原始值。
func splitFullName(fullName string) (string, string) {
	parts := strings.SplitN(fullName, "-", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	return parts[0], parts[1]
}
