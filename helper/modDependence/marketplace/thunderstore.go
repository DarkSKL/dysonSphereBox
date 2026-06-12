package marketplace

// 文件职责：加载 Thunderstore 包索引并转换为统一远端包分页数据。
// 使用：由市场 Provider 在 source=thunderstore 时调用，供前端按关键字、分页和废弃包策略浏览。
// 边界：索引缓存和远端兜底由 thunderstoreindex 负责；本文件不下载 Mod 文件、不写安装记录、不处理冲突。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/marketplace/contract"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreindex"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoremeta"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoreview"
	"context"
	"sort"
)

// listThunderstoreRemotePackages 读取 Thunderstore 包索引并返回统一分页。
// 职责：加载索引、过滤非 DSP Mod、转换统一 DTO 并执行本地分页。
// 使用：ListRemotePackages 在 Source 为 thunderstore 时调用。
// 边界：不下载 Mod 文件，不写安装记录；缓存读取和远端失败兜底由 thunderstoreindex.LoadPackages 处理。
// 逻辑流：1. 加载 Thunderstore 包索引 => 2. 过滤工具/框架/废弃包 => 3. 选择最新 active 版本 => 4. 校验 BepInEx 依赖链 => 5. 转换、搜索过滤并分页。
func listThunderstoreRemotePackages(ctx context.Context, query contract.RemotePackageQuery) (modDependenceResponse.RemotePackageListData, error) {
	packages, err := thunderstoreindex.LoadPackages(ctx, query.UseCache)
	if err != nil {
		return modDependenceResponse.RemotePackageListData{}, err
	}

	list := make([]modDependenceResponse.RemoteModPackageDTO, 0, len(packages))
	for _, pkg := range packages {
		if thunderstoreview.IsExternalOrFrameworkPackage(pkg) {
			continue
		}
		if pkg.IsDeprecated && !query.IncludeDeprecated {
			continue
		}
		version, ok := thunderstoremeta.LatestActiveVersion(pkg.Versions)
		if !ok {
			continue
		}
		if !thunderstoreview.VersionDependsOnBepInEx(pkg, version, packages, map[string]struct{}{}) {
			continue
		}
		remotePack := thunderstoreview.PackageToRemote(pkg, version, packages)
		if matchesPackageKeyword(remotePack, query.Keyword) {
			list = append(list, remotePack)
		}
	}

	sort.SliceStable(list, func(left int, right int) bool {
		return list[left].UpdateTime > list[right].UpdateTime
	})

	total := len(list)
	start := (query.Page - 1) * query.PageSize
	if start > total {
		start = total
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return modDependenceResponse.RemotePackageListData{
		List:     list[start:end],
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
		Source:   query.Source,
	}, nil
}
