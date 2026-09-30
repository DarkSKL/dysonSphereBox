package artifactfetch

// 文件职责：解析 V2 artifact 远端下载地址。
// 使用：downloadRemoteArtifact 发起 HTTP 请求前调用 resolveDownloadURL。
// 边界：只返回可请求 URL，不创建请求、不下载文件、不写缓存。

import (
	"fmt"
	"net/url"
	"strings"
)

// resolveDownloadURL 解析 artifact 下载地址。
// 职责：校验 artifact 提供的绝对 HTTP(S) 下载 URL。
// 使用：downloadRemoteArtifact 发起请求前调用。
// 边界：空地址、相对地址、非 HTTP(S) 地址或缺少主机时返回错误。
func resolveDownloadURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", fmt.Errorf("download url is empty")
	}
	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", fmt.Errorf("parse download url: %w", err)
	}
	if (parsedURL.Scheme == "http" || parsedURL.Scheme == "https") && parsedURL.Host != "" {
		return parsedURL.String(), nil
	}
	return "", fmt.Errorf("download url must be an absolute HTTP(S) URL")
}
