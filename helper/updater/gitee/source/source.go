package source

// 文件职责：定义 Gitee 更新源对象并完成基础配置归一化。
// 使用：helper/updater/client 通过 NewSource 创建 go-selfupdate 可使用的更新源。
// 边界：只负责对象构造和内存缓存初始化，不访问网络、不下载附件、不应用更新文件。

import (
	"DysonSphereBox/core"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Source 将 Gitee Release API 适配为 go-selfupdate 的 Source。
// 职责：保存 Gitee API 地址、访问令牌、HTTP 客户端和最近 Release 附件缓存。
// 使用：client.New 注入 go-selfupdate.Updater 作为 Source。
// 边界：只依赖 Release 列表和附件下载地址，不修改 Gitee 仓库内容；附件缓存来自最近一次 ListReleases。
type Source struct {
	// apiBaseURL 是规范化后的 Gitee API 根地址。
	// 职责：用于拼接 Release 列表接口。
	// 使用：ListReleases 构造请求端点时读取。
	// 边界：不包含仓库路径或查询参数。
	apiBaseURL string
	// token 是可选的 Gitee access_token。
	// 职责：为 Gitee API 请求提供访问凭据。
	// 使用：newRequest 在非空时写入 URL query。
	// 边界：不写入日志，不用于非 Gitee 请求。
	token string
	// httpClient 是更新源专用 HTTP 客户端。
	// 职责：执行 Release 列表和附件下载请求。
	// 使用：ListReleases 与 DownloadReleaseAsset 调用 Do。
	// 边界：只实现 go-selfupdate.Source 所需的数据访问，不提供 Mod 市场或账号 API。
	httpClient *http.Client
	// assets 缓存最近一次 Release 列表中的附件。
	// 职责：让 DownloadReleaseAsset 可通过 go-selfupdate 回传的 assetID 找到下载地址。
	// 使用：ListReleases 刷新，DownloadReleaseAsset 读取。
	// 边界：缓存不是持久化状态；调用下载前必须先完成一次 ListReleases。
	assets map[int64]giteeAsset
}

// NewSource 创建 Source。
// 职责：校验并规范化 Gitee 更新源配置。
// 使用：自更新 helper 初始化时注入 API 地址、令牌和可选 HTTP 客户端。
// 边界：只创建 Source 对象，不访问网络；API 地址非法时返回错误。
func NewSource(config Config) (*Source, error) {
	apiBaseURL := strings.TrimSpace(config.APIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultGiteeAPIBaseURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if _, err := url.ParseRequestURI(apiBaseURL); err != nil {
		return nil, fmt.Errorf("parse gitee api base url: %w", err)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = core.NewHTTPStandardClient(defaultHTTPTimeout)
	}
	return &Source{apiBaseURL: apiBaseURL, token: strings.TrimSpace(config.Token), httpClient: httpClient, assets: make(map[int64]giteeAsset)}, nil
}
