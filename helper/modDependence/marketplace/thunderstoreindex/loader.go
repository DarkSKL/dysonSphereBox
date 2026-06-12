package thunderstoreindex

// 文件职责：封装 Thunderstore 全量包索引的远端加载与缓存兜底。
// 使用：marketplace provider 在需要 Thunderstore 原始 Package 列表时调用 LoadPackages。
// 边界：只返回 Thunderstore 原始 API DTO，不执行市场展示过滤、统一 DTO 转换、安装决策或文件下载。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	marketplaceCache "DysonSphereBox/helper/modDependence/marketplace/cache"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// LoadPackages 加载 Thunderstore 全量包索引。
// 职责：按缓存策略读取本地缓存或请求 Thunderstore 远端索引。
// 使用：marketplace.ListRemotePackages 和 marketplace.ResolveRemotePackage 获取 Thunderstore 包索引时调用。
// 边界：允许缓存时优先读缓存；远端请求失败时尽量回退旧缓存；成功请求后刷新缓存。
// 逻辑流：1. 根据 useCache 尝试读取缓存 => 2. 构造 Thunderstore 请求 => 3. 远端失败则回退缓存 => 4. 解码包索引 => 5. 写入缓存并返回。
func LoadPackages(ctx context.Context, useCache bool) ([]thunderstoreAPI.Package, error) {
	if useCache {
		if cachedPackages, ok := marketplaceCache.ReadThunderstorePackages(); ok {
			return cachedPackages, nil
		}
	}

	client := http.Client{Timeout: 60 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, thunderstoreAPI.IndexURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		if cachedPackages, ok := marketplaceCache.ReadThunderstorePackages(); ok {
			return cachedPackages, nil
		}
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if cachedPackages, ok := marketplaceCache.ReadThunderstorePackages(); ok {
			return cachedPackages, nil
		}
		return nil, fmt.Errorf("Thunderstore returned HTTP %d", response.StatusCode)
	}

	var packages []thunderstoreAPI.Package
	if err := json.NewDecoder(response.Body).Decode(&packages); err != nil {
		return nil, err
	}
	marketplaceCache.WriteThunderstorePackages(packages)
	return packages, nil
}
