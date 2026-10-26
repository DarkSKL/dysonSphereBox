package diagnostics

// 文件职责：采集共享 HTTP 客户端的失败证据。使用：标准客户端 Transport 包装；边界：不保存认证头或正文，不重试请求。

import (
	"DysonSphereBox/constant/logging"
	"context"
	"errors"
	"net/http"
	"time"
)

// Transport 职责：包装现有网络传输；使用：HTTP 客户端初始化；边界：保持原响应与错误。
type Transport struct{ Base http.RoundTripper }

// CloseIdleConnections 释放底层标准 transport 的空闲连接。
// 职责：透传连接池清理能力，避免一次性客户端保留空闲 socket。
// 使用：调用方完成独立 HTTP 客户端响应后调用。
// 边界：不关闭正在使用的请求连接，也不改变 RoundTrip 行为。
func (t Transport) CloseIdleConnections() {
	if transport, ok := t.Base.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

// RoundTrip 职责：记录网络错误、HTTP 失败和慢请求；使用：http.Client；边界：200 不代表业务成功，响应解析失败由业务边界补充。
func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	begin := time.Now()
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	response, err := base.RoundTrip(r)
	elapsed := time.Since(begin)
	if errors.Is(err, context.Canceled) && errors.Is(r.Context().Err(), context.Canceled) {
		return response, err
	}
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	if err == nil && status >= 200 && status < 400 {
		Default.ResolveNetwork(r.Method, r.URL.String())
	}
	if err == nil && status < 400 && elapsed < 10*time.Second {
		return response, err
	}
	level, reason := logging.Warn, "HTTP 请求耗时超过十秒"
	data := map[string]interface{}{"method": r.Method, "url": r.URL.String(), "statusCode": status, "durationMs": elapsed.Milliseconds()}
	if err != nil {
		level, reason = logging.Error, "HTTP 传输返回错误"
		data["error"] = ErrorData(err)
		data["result"] = "failed"
	} else if status >= 400 {
		level, reason = logging.Error, "HTTP 返回非成功状态码"
		data["result"] = "failed"
	}
	Default.Record(logging.Entry{Action: "远端请求", Level: level, Source: "backend.http", Reason: reason, Namespace: "system.network.request", Handler: logging.Unknown, Data: data})
	return response, err
}
