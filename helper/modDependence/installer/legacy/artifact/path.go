package artifact

// 文件职责：解析旧官方依赖 artifact 的缓存目录和下载 URL。
// 使用：旧版安装入口调用 BufferDir，下载流程调用 resolveDependencyDownloadURL。
// 边界：只返回路径或 URL，不创建完整安装目录、不下载文件、不读取 dependency zip 内容。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/fileops"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// BufferDir 返回旧官方整合包缓存根目录。
// 职责：基于当前可执行文件目录构造旧安装器下载和解压缓冲路径。
// 使用：旧整合包安装入口在处理依赖前调用。
// 边界：只返回路径不创建目录；无法解析可执行文件路径时返回错误。
func BufferDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), fileops.BufferRootDirName, "modDependencies"), nil
}

// resolveDependencyDownloadURL 解析旧官方依赖下载地址。
// 职责：兼容绝对 URL 与项目 HTTPClient 可解析的相对 URL。
// 使用：downloadDependencyZip 发起 HTTP 请求前调用。
// 边界：空地址、解析失败或全局 HTTPClient 未初始化时返回错误。
func resolveDependencyDownloadURL(rawURL string) (string, error) {
	trimmedURL := strings.TrimSpace(rawURL)
	if trimmedURL == "" {
		return "", fmt.Errorf("download url is empty")
	}

	parsedURL, err := url.Parse(trimmedURL)
	if err != nil {
		return "", fmt.Errorf("parse download url: %w", err)
	}
	if parsedURL.Scheme != "" && parsedURL.Host != "" {
		return parsedURL.String(), nil
	}
	if core.GLOBAL.HTTPClient == nil {
		return "", fmt.Errorf("http client is not initialized")
	}

	return core.GLOBAL.HTTPClient.ResolveURL(trimmedURL)
}
