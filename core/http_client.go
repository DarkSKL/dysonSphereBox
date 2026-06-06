package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// HTTPClient 封装带 BaseURL 的全局 HTTP 客户端。
// 用途：服务层需要访问统一后端接口时，通过 GLOBAL.HTTPClient 复用连接与基础地址。
// 边界：BaseURL 允许为空以保证应用可启动，真正发起请求时会返回明确错误。
type HTTPClient struct {
	client  *http.Client
	baseURL *url.URL
}

// InitHTTPClient 从 Configure.BaseURL 读取地址并创建全局 HTTP 客户端。
// 使用：数据库和配置初始化完成后调用；历史配置无效时会降级为空 BaseURL，避免阻塞设置页修复。
func InitHTTPClient() (*HTTPClient, error) {
	if GLOBAL.Configure == nil || GLOBAL.Configure.BaseURL == nil {
		return nil, errors.New("configure base url is not initialized")
	}

	baseURL, err := GLOBAL.Configure.BaseURL.Get()
	if err != nil {
		return nil, err
	}

	httpClient, err := NewHTTPClient(baseURL)
	if err != nil {
		return NewHTTPClient("")
	}

	return httpClient, nil
}

// NewHTTPClient 根据传入的 BaseURL 创建 HTTP 客户端。
// 边界：空 BaseURL 会创建可保存的客户端；发起请求前仍需要配置有效地址。
func NewHTTPClient(baseURL string) (*HTTPClient, error) {
	parsedBaseURL, err := parseBaseURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &HTTPClient{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		baseURL: parsedBaseURL,
	}, nil
}

// BaseURL 返回当前客户端保存的基础地址。
func (model *HTTPClient) BaseURL() string {
	if model == nil || model.baseURL == nil {
		return ""
	}

	return model.baseURL.String()
}

// NewRequest 基于 BaseURL 创建 HTTP 请求。
// path 可以是相对路径或绝对 URL；绝对 URL 会按 Go 标准库规则覆盖 BaseURL。
func (model *HTTPClient) NewRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	requestURL, err := model.ResolveURL(path)
	if err != nil {
		return nil, err
	}

	return http.NewRequestWithContext(ctx, method, requestURL, body)
}

// Do 发送 HTTP 请求并返回原始响应。
func (model *HTTPClient) Do(request *http.Request) (*http.Response, error) {
	if model == nil || model.client == nil {
		return nil, errors.New("http client is not initialized")
	}

	return model.client.Do(request)
}

// ResolveURL 将相对路径解析为完整请求地址。
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

func parseBaseURL(baseURL string) (*url.URL, error) {
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		return nil, nil
	}

	parsedBaseURL, err := url.Parse(trimmedBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("base url must include scheme and host")
	}
	if parsedBaseURL.Path != "" && !strings.HasSuffix(parsedBaseURL.Path, "/") {
		parsedBaseURL.Path += "/"
	}

	return parsedBaseURL, nil
}
