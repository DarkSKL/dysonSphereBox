package core

// 文件职责：提供项目内直接访问外部 HTTPS 资源时复用的标准 HTTP 客户端构造。
// 使用：helper 或第三方适配器需要直接请求 Thunderstore、Gitee 或下载资源时调用。
// 边界：只配置标准库 http.Client 的通用传输行为，不解析业务响应、不记录业务日志。

import (
	"DysonSphereBox/core/diagnostics"
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// NewHTTPStandardClient 创建允许跳过证书校验的标准 HTTP 客户端。
// 职责：统一规避用户网络代理、加速器或拦截软件导致的 x509 unknown authority 错误。
// 使用：请求 Thunderstore、Gitee 或直接下载远程文件的代码通过该函数创建客户端客户端。
// 边界：跳过证书校验会降低 HTTPS 身份验证强度；调用方仍需负责 HTTP 状态码、文件完整性和业务响应校验。
func NewHTTPStandardClient(timeout time.Duration) *http.Client {
	return newHTTPStandardClient(timeout, 0)
}

// NewHTTPStandardClientWithStageTimeout 创建带连接阶段超时的标准 HTTP 客户端。
// 职责：限制连接、TLS 握手和响应头等待，同时保留标准客户端诊断与 TLS 行为。
// 使用：长总预算的文件下载需要单独限制网络阶段时调用。
// 边界：只影响返回的独立客户端，不修改现有共享标准客户端。
func NewHTTPStandardClientWithStageTimeout(timeout, stageTimeout time.Duration) *http.Client {
	return newHTTPStandardClient(timeout, stageTimeout)
}

func newHTTPStandardClient(timeout, stageTimeout time.Duration) *http.Client {
	transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	if stageTimeout > 0 {
		transport.DialContext = (&net.Dialer{Timeout: stageTimeout, KeepAlive: 30 * time.Second}).DialContext
		transport.TLSHandshakeTimeout = stageTimeout
		transport.ResponseHeaderTimeout = stageTimeout
	}
	return &http.Client{Timeout: timeout, Transport: diagnostics.Transport{Base: transport}}
}
