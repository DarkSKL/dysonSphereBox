package source

// 文件职责：统一构造 Gitee API 与附件下载请求。
// 使用：ListReleases 和 DownloadReleaseAsset 在发起 HTTP 请求前调用 newRequest。
// 边界：只创建请求和填充头/query，不执行网络请求、不解析响应、不记录 token。

import (
	"context"
	"fmt"
	"net/http"
)

// newRequest 构造 Gitee API 或附件下载请求。
// 职责：统一设置上下文、基础请求头和可选 access_token。
// 使用：ListReleases 和 DownloadReleaseAsset 在发起 HTTP 请求前调用。
// 边界：不会执行请求；token 以 query 形式附加，只用于 Gitee 相关 URL。
func (source *Source) newRequest(ctx context.Context, method string, requestURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create gitee request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "DysonSphereBox-Updater")
	if source.token != "" {
		query := request.URL.Query()
		query.Set("access_token", source.token)
		request.URL.RawQuery = query.Encode()
	}
	return request, nil
}
