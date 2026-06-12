package core

// 文件职责：解析并规范化 HTTPClient 的 BaseURL。
// 使用：NewHTTPClient 创建客户端前调用 parseBaseURL。
// 边界：只校验基础地址格式，不访问网络、不读取配置、不决定业务请求路径。

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// parseBaseURL 解析并规范化基础请求地址。
// 职责：校验 scheme/host 并补齐路径尾部斜杠。
// 使用：NewHTTPClient 创建客户端前调用。
// 边界：空字符串返回 nil 且不报错，格式不合法时返回错误。
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
