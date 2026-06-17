package thunderstoreindex

// 文件职责：实现 Thunderstore 旧版 api/v1/package 完整索引 provider。
// 使用：LoadPackages 在新版分片索引失败后调用，用于兼容 Thunderstore 旧接口短暂恢复或社区差异。
// 边界：只作为兜底链路，不作为主索引来源；不写缓存、不做包筛选、不处理 BepInEx 特例。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// legacyIndexProvider 从 Thunderstore 旧版完整包接口读取包列表。
// 职责：保留 api/v1/package 旧响应契约的兼容能力。
// 使用：LoadPackages 在 chunkedIndexProvider 失败后调用。
// 边界：该接口当前可能返回 404，失败时只返回错误并交由上层继续缓存兜底。
type legacyIndexProvider struct{}

// Name 返回旧版 provider 的诊断名称。
// 职责：为 Thunderstore 索引聚合错误提供来源标识。
// 使用：LoadPackages 汇总 provider 失败原因时调用。
// 边界：不作为用户可配置项，也不暴露给服务层。
func (legacyIndexProvider) Name() string {
	return "legacy"
}

// Load 加载 Thunderstore 旧版完整包索引。
// 职责：请求 LegacyIndexURL 并直接解码为 Thunderstore Package 列表。
// 使用：LoadPackages 在新版 provider 失败后调用，保持旧方案作为兜底存在。
// 边界：HTTP 非 2xx 或 JSON 解析失败时返回错误，不读取或写入本地缓存。
// 逻辑流：1. 构造旧版索引请求 => 2. 校验 HTTP 状态 => 3. 解码 Package 列表 => 4. 空结果视为失败。
func (legacyIndexProvider) Load(ctx context.Context) ([]thunderstoreAPI.Package, error) {
	client := http.Client{Timeout: 60 * time.Second}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, thunderstoreAPI.LegacyIndexURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "DysonSphereBox/1.0")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}

	var packages []thunderstoreAPI.Package
	if err := json.NewDecoder(response.Body).Decode(&packages); err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("legacy index contains no packages")
	}

	return packages, nil
}
