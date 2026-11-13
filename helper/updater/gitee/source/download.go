package source

// 文件职责：下载 go-selfupdate 已选择的 Gitee Release 附件内容。
// 使用：helper/updater/apply 在应用更新或下载 checksum 校验文件时调用 DownloadReleaseAsset。
// 边界：assetID 必须来自最近一次 ListReleases 缓存；不负责选择资产、不校验 checksum、不落盘文件。

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// assetDownloadReader 职责：保留下载响应体及实际内容长度；使用：下载进度读取器通过 ContentLength 获取总量；边界：关闭仍委托原始响应体，不缓存或预读内容。
type assetDownloadReader struct {
	// ReadCloser 职责：承载实际下载流；使用：调用方读取并关闭；边界：不改变原始读取错误。
	io.ReadCloser
	// contentLength 职责：保存最终响应的内容长度；使用：进度统计优先采用；边界：负数表示服务器未提供长度。
	contentLength int64
}

// ContentLength 职责：提供最终下载响应的字节总量；使用：进度读取器计算真实比例；边界：未知长度返回负数，不根据已读取量猜测。
func (reader *assetDownloadReader) ContentLength() int64 {
	return reader.contentLength
}

// DownloadReleaseAsset 下载指定 Gitee Release 附件。
// 职责：根据 go-selfupdate 选中的 assetID 下载对应附件内容。
// 使用：applyRelease 和 validateReleaseAsset 通过 Source 下载更新包或校验文件。
// 边界：assetID 必须来自最近一次 ListReleases 建立的缓存；HTTP 非 2xx 会关闭响应体并返回错误。
// 逻辑流：1. 从附件缓存查找 assetID => 2. 校验下载地址 => 3. 构造二进制下载请求 => 4. 执行 HTTP 请求 => 5. 返回可读取响应体。
func (source *Source) DownloadReleaseAsset(ctx context.Context, _ *selfupdate.Release, assetID int64) (io.ReadCloser, error) {
	asset, ok := source.assets[assetID]
	if !ok {
		return nil, fmt.Errorf("gitee release asset not found: %d", assetID)
	}
	if asset.BrowserDownloadURL == "" {
		return nil, fmt.Errorf("gitee release asset %q has no download url", asset.Name)
	}
	request, err := source.newRequest(ctx, http.MethodGet, asset.BrowserDownloadURL)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/octet-stream, */*")
	response, err := source.downloadClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download gitee release asset %q: %w", asset.Name, err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Printf("failed to close response body: %s\n", err)
			}
		}(response.Body)
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("download gitee release asset %q returned %s: %s", asset.Name, response.Status, strings.TrimSpace(string(body)))
	}
	return &assetDownloadReader{ReadCloser: response.Body, contentLength: response.ContentLength}, nil
}
