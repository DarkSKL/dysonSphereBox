package marketplace

// 文件职责：解析官方市场源返回的相对或绝对 URL。
// 使用：official_convert.go 转换包和依赖的下载地址、页面地址时调用。
// 边界：只复用 core HTTPClient 的基础地址规则；未初始化或解析失败时返回原始输入。

import (
	"DysonSphereBox/core"
	"strings"
)

// resolveOfficialRemoteURL 解析官方源相对或绝对下载地址。
// 职责：使用 core HTTPClient 的基础地址规则补齐官方源 URL。
// 使用：官方包和依赖 DTO 转换下载地址、页面地址时调用。
// 边界：HTTPClient 未初始化或解析失败时返回原始 URL。
func resolveOfficialRemoteURL(rawURL string) string {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" || core.GLOBAL.HTTPClient == nil {
		return trimmedURL
	}
	resolvedURL, err := core.GLOBAL.HTTPClient.ResolveURL(trimmedURL)
	if err != nil {
		return trimmedURL
	}
	return resolvedURL
}
