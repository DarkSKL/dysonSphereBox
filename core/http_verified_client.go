package core

// 文件职责：为需要验证 TLS 身份的应用服务器请求构造独立 HTTP 客户端。
// 使用：代理下载等携带敏感凭证且必须验证目标服务器的 helper 创建并使用该客户端。
// 边界：不改变共用跳过证书校验的 transport；连接、TLS 握手和响应头等待均受 60 秒限制，禁止自动跟随重定向。

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// verifiedHTTPTimeout 限制代理请求每次连接、TLS 握手与响应头阶段的等待时间。
// 职责：集中维护代理专用 transport 的阶段超时上限。
// 使用：NewHTTPVerifiedClient 创建专用客户端时读取。
// 边界：请求总预算由调用方 context 控制，不配置 http.Client 总 Timeout。
const verifiedHTTPTimeout = 60 * time.Second

// NewHTTPVerifiedClient 按当前有效 server 地址创建正常验证 TLS 的客户端。
// 职责：为敏感凭证请求提供独立连接池、证书校验和禁止重定向策略。
// 使用：helper 在已捕获当前服务器地址后创建客户端并通过 HTTPClient.Do 发送请求。
// 边界：不会修改 GLOBAL.ServerClient 或共用不安全 transport；总请求时限由调用方 context 管理。
func NewHTTPVerifiedClient(baseURL string) (*HTTPClient, error) {
	parsedBaseURL, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	if parsedBaseURL == nil {
		return nil, fmt.Errorf("verified HTTP client requires a server address")
	}
	if parsedBaseURL.Scheme != "https" && !(parsedBaseURL.Scheme == "http" && isLoopbackHost(parsedBaseURL.Hostname())) {
		return nil, fmt.Errorf("verified HTTP client requires HTTPS server address")
	}
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: verifiedHTTPTimeout, KeepAlive: 30 * time.Second}).DialContext,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		TLSHandshakeTimeout:   verifiedHTTPTimeout,
		ResponseHeaderTimeout: verifiedHTTPTimeout,
	}
	return &HTTPClient{
		baseURL:              parsedBaseURL,
		closeIdleConnections: transport.CloseIdleConnections,
		client: &http.Client{
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

// isLoopbackHost 仅允许本机开发使用明文 HTTP。
// 职责：限制 HTTP 代理目标为 IPv4、IPv6 或 localhost loopback。
// 使用：NewHTTPVerifiedClient 校验 scheme 与主机。
// 边界：其他 HTTP 主机一律拒绝；HTTPS 不受此判断影响。
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
