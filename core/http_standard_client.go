package core

// 文件职责：提供项目内直接访问外部 HTTPS 资源时复用的标准 HTTP 客户端构造。
// 使用：helper 或第三方适配器需要直接请求 Thunderstore、Gitee 或下载资源时调用。
// 边界：只配置标准库 http.Client 的通用传输行为，不处理 BaseURL、不解析业务响应、不记录业务日志。

import (
	"crypto/tls"
	"net/http"
	"time"
)

// NewHTTPStandardClient 创建允许跳过证书校验的标准 HTTP 客户端。
// 职责：统一规避用户网络代理、加速器或拦截软件导致的 x509 unknown authority 错误。
// 使用：所有不经过 HTTPClient BaseURL 封装的外部 HTTPS 请求都应通过该函数创建客户端。
// 边界：跳过证书校验会降低 HTTPS 身份验证强度；调用方仍需负责 HTTP 状态码、文件完整性和业务响应校验。
func NewHTTPStandardClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}
