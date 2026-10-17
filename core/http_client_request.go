package core

// 文件职责：提供 HTTPClient 的请求创建、发送和 URL 解析能力。
// 使用：service/helper 发起远端请求前调用 NewRequest、ResolveURL 或 Do。
// 边界：只负责请求对象和网络发送，不关闭响应体、不解析业务错误。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// NewRequest 基于 BaseURL 创建 HTTP 请求。
// 职责：把业务传入的路径解析成标准 http.Request。
// 使用：service/helper 发起远端请求前调用。
// 边界：path 可以是相对路径或绝对 URL；绝对 URL 会按 Go 标准库规则覆盖 BaseURL。
func (model *HTTPClient) NewRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	requestURL, err := model.ResolveURL(path)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, method, requestURL, body)
}

// Do 发送 HTTP 请求并返回原始响应。
// 职责：转发标准库 HTTP 请求。
// 使用：调用方需要自行解析响应体时调用。
// 边界：只负责发送请求，不关闭响应体，不解析业务错误。
func (model *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	if model == nil || model.client == nil {
		return nil, errors.New("http client is not initialized")
	}

	return model.client.Do(request)
}

// DoNoRedirect发送不自动跟随重定向的HTTP请求。
// 职责：让敏感鉴权头不会被公告接口重定向带往其他目标。
// 使用：携带token且协议要求固定目标的读取确认请求调用。
// 边界：仅复制标准客户端策略，不改变共享连接池和全局重定向行为。
func (model *HTTPClient) DoNoRedirect(request *http.Request) (*http.Response, error) {
	if model == nil || model.client == nil {
		return nil, errors.New("http client is not initialized")
	}
	clientCopy := *model.client
	clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return clientCopy.Do(request)
}

// ResolveURL 将相对路径解析为完整请求地址。
// 职责：把 BaseURL 与业务路径组合为可请求 URL。
// 使用：NewRequest 或需要预览请求地址时调用。
// 边界：BaseURL 未配置或 path 无法解析时返回错误。
func (model *HTTPClient) ResolveURL(path string) (string, error) {
	if model == nil {
		return "", errors.New("http client is not initialized")
	}
	if model.baseURL == nil {
		return "", errors.New("base url is not configured")
	}

	requestURL, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse request url: %w", err)
	}

	return model.baseURL.ResolveReference(requestURL).String(), nil
}
