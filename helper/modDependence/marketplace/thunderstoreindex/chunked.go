package thunderstoreindex

// 文件职责：实现 Thunderstore 新版 package-listing-index 分片索引 provider。
// 使用：LoadPackages 在缓存不可用或强制刷新时优先调用，用于恢复 DSP 市场、依赖和 BepInEx 元数据链路。
// 边界：只负责新版索引入口、chunk 下载和 gzip JSON 解码，不写缓存、不做包筛选、不下载 Mod zip。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// chunkedIndexProvider 从 Thunderstore 新版分片索引读取完整包列表。
// 职责：访问 package-listing-index，解析 chunk URL，并聚合所有 chunk 中的 Package。
// 使用：LoadPackages 将其作为 Thunderstore 主链路 provider。
// 边界：任一 chunk 失败即返回错误，由 LoadPackages 决定是否切换旧版 provider 或缓存。
type chunkedIndexProvider struct{}

// Name 返回新版分片 provider 的诊断名称。
// 职责：为 Thunderstore 索引聚合错误提供来源标识。
// 使用：LoadPackages 汇总 provider 失败原因时调用。
// 边界：不作为用户可配置项，也不暴露给服务层。
func (chunkedIndexProvider) Name() string {
	return "chunked"
}

// Load 加载 Thunderstore 新版分片包索引。
// 职责：请求索引入口、解压 chunk 列表、逐个请求 chunk 并聚合包列表。
// 使用：LoadPackages 在本地缓存不可用或需要远端刷新时优先调用。
// 边界：返回的是 Thunderstore 原始 Package 列表，不过滤废弃包、不转换统一市场 DTO。
// 逻辑流：1. 下载新版 index blob => 2. 解压为 chunk URL 列表 => 3. 逐个下载 chunk blob => 4. 解压并聚合 Package => 5. 空结果视为失败交给上层兜底。
func (provider chunkedIndexProvider) Load(ctx context.Context) ([]thunderstoreAPI.Package, error) {
	client := http.Client{Timeout: 60 * time.Second}
	indexBytes, err := getThunderstoreBytes(ctx, &client, thunderstoreAPI.ChunkedIndexURL)
	if err != nil {
		return nil, fmt.Errorf("index request: %w", err)
	}

	var chunkURLs []string
	if err := decodeGzipJSON(indexBytes, &chunkURLs); err != nil {
		return nil, fmt.Errorf("index decode: %w", err)
	}
	if len(chunkURLs) == 0 {
		return nil, fmt.Errorf("index contains no chunks")
	}

	packages := make([]thunderstoreAPI.Package, 0)
	for index, chunkURL := range chunkURLs {
		if chunkURL == "" {
			return nil, fmt.Errorf("chunk %d url is empty", index)
		}
		chunkBytes, err := getThunderstoreBytes(ctx, &client, chunkURL)
		if err != nil {
			return nil, fmt.Errorf("chunk %d request: %w", index, err)
		}

		var chunkPackages []thunderstoreAPI.Package
		if err := decodeGzipJSON(chunkBytes, &chunkPackages); err != nil {
			return nil, fmt.Errorf("chunk %d decode: %w", index, err)
		}
		packages = append(packages, chunkPackages...)
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("chunks contain no packages")
	}

	return packages, nil
}

// getThunderstoreBytes 读取 Thunderstore 索引相关 URL 的完整响应体。
// 职责：统一设置请求头、执行 HTTP GET，并把非 2xx 状态转为错误。
// 使用：chunkedIndexProvider 下载 index blob 和 chunk blob 时调用。
// 边界：只返回原始字节，不解码 gzip、不解析 JSON、不写入缓存。
func getThunderstoreBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json, application/octet-stream")
	request.Header.Set("User-Agent", "DysonSphereBox/1.0")

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("HTTP %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
