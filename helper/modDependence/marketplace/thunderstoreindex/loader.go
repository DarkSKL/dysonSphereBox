package thunderstoreindex

// 文件职责：封装 Thunderstore 全量包索引的远端加载与缓存兜底。
// 使用：marketplace provider 在需要 Thunderstore 原始 Package 列表时调用 LoadPackages。
// 边界：只返回 Thunderstore 原始 API DTO，不执行市场展示过滤、统一 DTO 转换、安装决策或文件下载。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	marketplaceCache "DysonSphereBox/helper/modDependence/marketplace/cache"
	"context"
	"fmt"
	"strings"
)

// 索引 provider 集合负责按优先级加载 Thunderstore 原始包列表。
// 职责：描述远端索引来源的统一接口，让新版分片链路和旧版完整链路共享同一个对外入口。
// 使用：LoadPackages 内部按顺序调用 provider，服务层和安装器不直接感知 provider 差异。
// 边界：provider 只返回原始 Package 列表，不写缓存、不做市场过滤、不下载 zip。
type indexProvider interface {
	// Load 加载当前 provider 对应的 Thunderstore 包索引。
	// 职责：从单一来源读取并解码为 Thunderstore Package 列表。
	// 使用：LoadPackages 在缓存不可用或强制刷新时按优先级调用。
	// 边界：失败时返回带上下文的错误，由 LoadPackages 决定是否继续兜底。
	Load(ctx context.Context) ([]thunderstoreAPI.Package, error)
	// Name 返回 provider 诊断名称。
	// 职责：为聚合错误和日志提供稳定来源标识。
	// 使用：LoadPackages 汇总失败原因时读取。
	// 边界：不参与业务判断，不能暴露为服务层契约。
	Name() string
}

// LoadPackages 加载 Thunderstore 全量包索引。
// 职责：按缓存策略读取本地缓存，或经新版/旧版索引 provider 加载远端包索引。
// 使用：marketplace.ListRemotePackages 和 marketplace.ResolveRemotePackage 获取 Thunderstore 包索引时调用。
// 边界：允许缓存时优先读有效缓存；远端全部失败时读取任意可用缓存兜底；成功请求后刷新缓存。
// 逻辑流：1. 根据 useCache 尝试读取缓存 => 2. 读取用户配置决定新旧 provider 优先级 => 3. 成功则写入缓存并返回 => 4. 单个 provider 失败记录日志并继续兜底 => 5. 远端全部失败时读取缓存或返回聚合错误。
func LoadPackages(ctx context.Context, useCache bool) ([]thunderstoreAPI.Package, error) {
	if useCache {
		if cachedPackages, ok := marketplaceCache.ReadThunderstorePackages(); ok {
			return cachedPackages, nil
		}
	}

	preferredMode := preferredIndexMode()
	providers := orderedProviders()
	failures := make([]string, 0, len(providers)+1)
	for _, provider := range providers {
		packages, err := provider.Load(ctx)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s returned %v", provider.Name(), err))
			recordProviderFailure(provider.Name(), preferredMode, err)
			continue
		}
		marketplaceCache.WriteThunderstorePackages(packages)
		return packages, nil
	}

	if cachedPackages, ok := marketplaceCache.ReadThunderstorePackages(); ok {
		recordCacheFallback(preferredMode, failures)
		return cachedPackages, nil
	}
	failures = append(failures, "cache unavailable")

	return nil, fmt.Errorf("Thunderstore index load failed: %s", strings.Join(failures, "; "))
}
