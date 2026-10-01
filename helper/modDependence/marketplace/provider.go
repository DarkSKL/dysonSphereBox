package marketplace

// 文件职责：提供统一 Mod 市场 Provider 分派入口与远端包 key 解析能力。
// 使用：由 MarketplaceService 查询远端包列表，或安装器按依赖 key 回查 Thunderstore 包详情时调用。
// 边界：只分派 Thunderstore 和手动来源，不访问本项目服务器、不安装文件或写安装记录。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	sourceConstant "DysonSphereBox/constant/modDependence/source"
	"DysonSphereBox/helper/modDependence/marketplace/contract"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreindex"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoremeta"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreview"
	"context"
	"fmt"
	"strings"
)

// ListRemotePackages 按来源返回统一 Mod 包分页数据。
// 职责：按统一来源分派市场 Provider 并返回统一分页 DTO。
// 使用：MarketplaceService 的加载和刷新方法调用。
// 边界：Thunderstore 使用全量缓存后在本地分页；manual 当前返回空列表。
// 逻辑流：1. 归一化查询参数 => 2. 按 Source 分派 Provider => 3. Thunderstore 本地分页 => 4. 返回统一 RemotePackageListData。
func ListRemotePackages(ctx context.Context, query contract.RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	query = normalizeRemotePackageQuery(query)
	switch query.Source {
	case sourceConstant.ModSourceThunderstore:
		return listThunderstoreRemotePackages(ctx, query)
	case sourceConstant.ModSourceManual:
		return modDependenceResponse.RemotePackageListData{
			List:     []modDependenceResponse.RemoteModPackageDTO{},
			Total:    0,
			Page:     query.Page,
			PageSize: query.PageSize,
			Source:   query.Source,
		}, nil
	default:
		return modDependenceResponse.RemotePackageListData{}, fmt.Errorf("unsupported mod source: %s", query.Source)
	}
}

// ResolveLatestRemotePackage 按 Thunderstore namespace/name 解析最新 active 版本。
// 职责：为 R2 同款普通 Mod 依赖策略提供最新可用版本的统一 DTO。
// 使用：安装依赖解析器在普通 Mod 安装时调用。
// 边界：只支持 Thunderstore；无 active 版本或索引加载失败时返回错误。
func ResolveLatestRemotePackage(ctx context.Context, namespace string, name string) (*modDependenceResponse.RemoteModPackageDTO, error) {
	packages, err := thunderstoreindex.LoadPackages(ctx, true)
	if err != nil {
		return nil, err
	}
	for _, pkg := range packages {
		if !strings.EqualFold(pkg.Owner, namespace) || !strings.EqualFold(pkg.Name, name) {
			continue
		}
		version, ok := thunderstoremeta.LatestActiveVersion(pkg.Versions)
		if !ok {
			return nil, fmt.Errorf("Thunderstore package has no active version: %s-%s", namespace, name)
		}
		result := thunderstoreview.PackageToRemote(pkg, version, packages)
		return &result, nil
	}
	return nil, fmt.Errorf("Thunderstore package not found: %s-%s", namespace, name)
}

// ResolveRemotePackage 根据统一 key 解析单个远端包。
// 职责：根据统一包 key 从来源索引中解析完整远端包 DTO。
// 使用：V2 安装器递归解析 Thunderstore 依赖时调用。
// 边界：目前只有 Thunderstore 支持按 key 回查；其它来源返回未实现错误。
// 逻辑流：1. 拆分统一 key => 2. 判断来源 => 3. Thunderstore 加载包索引 => 4. 遍历版本匹配 key => 5. 转换为统一 DTO 返回。
func ResolveRemotePackage(ctx context.Context, key string) (*modDependenceResponse.RemoteModPackageDTO, error) {
	parts := strings.Split(key, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid package key: %s", key)
	}
	source := sourceConstant.ModSource(parts[0])
	switch source {
	case sourceConstant.ModSourceThunderstore:
		packages, err := thunderstoreindex.LoadPackages(ctx, true)
		if err != nil {
			return nil, err
		}
		for _, pkg := range packages {
			for _, version := range pkg.Versions {
				candidate := thunderstoreview.PackageKey(pkg, version)
				if candidate == key {
					result := thunderstoreview.PackageToRemote(pkg, version, packages)
					return &result, nil
				}
			}
		}
		return nil, fmt.Errorf("Thunderstore package not found: %s", key)
	default:
		return nil, fmt.Errorf("resolve package is not implemented for source: %s", source)
	}
}

// normalizeRemotePackageQuery 归一化市场查询参数。
// 职责：为来源、页码和分页大小提供稳定默认值与上限。
// 使用：ListRemotePackages 分派 Provider 前调用。
// 边界：不修改关键字和缓存策略；只返回新的查询值。
func normalizeRemotePackageQuery(query contract.RemotePackageQuery) contract.RemotePackageQuery {
	if query.Source == "" {
		query.Source = sourceConstant.ModSourceThunderstore
	}
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize < 1 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	return query
}
