package core

// 文件职责：定义应用全局 HTTP 客户端基础类型和构造逻辑。
// 使用：启动流程初始化 GLOBAL.HTTPClient，service/helper 运行期复用该客户端访问统一后端接口。
// 边界：只封装 BaseURL 与标准库客户端，不解析业务响应、不处理 Mod 安装或市场查询。

import (
	"net/http"
	"net/url"
	"time"
)

// defaultHTTPTimeout 是全局 HTTP 客户端的默认超时时间。
// 职责：限制远端请求最长等待时间，避免桌面端调用长期挂起。
// 使用：NewHTTPClient 创建 http.Client 时读取。
// 边界：只影响通过 core.HTTPClient 发出的请求，不覆盖第三方库内部超时。
const defaultHTTPTimeout = 30 * time.Second

// HTTPClient 封装带 BaseURL 的全局 HTTP 客户端。
// 职责：组合标准 http.Client 与可配置 BaseURL。
// 使用：服务层需要访问统一后端接口时，通过 GLOBAL.HTTPClient 复用连接与基础地址。
// 边界：BaseURL 允许为空以保证应用可启动，真正发起请求时会返回明确错误。
type HTTPClient struct {
	// client 是实际发起请求的标准库 HTTP 客户端。
	// 职责：承载超时和连接复用能力。
	// 使用：Do 方法转发请求时调用。
	// 边界：不包含业务鉴权或响应解析逻辑。
	client *http.Client
	// baseURL 是配置解析后的基础请求地址。
	// 职责：为相对路径请求提供根地址。
	// 使用：ResolveURL 解析请求路径时读取。
	// 边界：允许为空，空值会在请求解析阶段返回明确错误。
	baseURL *url.URL
}

// NewHTTPClient 根据传入的 BaseURL 创建 HTTP 客户端。
// 职责：解析基础地址并配置标准库 HTTP 客户端。
// 使用：InitHTTPClient 或测试需要构造独立客户端时调用。
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
// 职责：暴露当前解析后的 BaseURL 字符串。
// 使用：设置页或诊断日志需要展示 HTTP 配置时调用。
// 边界：客户端或 BaseURL 为空时返回空字符串。
func (model *HTTPClient) BaseURL() string {
	if model == nil || model.baseURL == nil {
		return ""
	}

	return model.baseURL.String()
}
