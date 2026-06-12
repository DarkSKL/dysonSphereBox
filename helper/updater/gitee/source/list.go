package source

// 文件职责：从 Gitee Release API 读取发布列表并刷新附件缓存。
// 使用：go-selfupdate.DetectLatest 通过 Source.ListReleases 检测最新版本时调用。
// 边界：只读取 Release 列表并做 DTO 适配，不下载附件、不应用更新、不决定前端提示。

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// ListReleases 从 Gitee 读取仓库 Release 列表。
// 职责：调用 Gitee Release API，解码发布列表并转换为 go-selfupdate.SourceRelease。
// 使用：go-selfupdate.DetectLatest 检测最新版本时调用。
// 边界：会刷新附件缓存，供后续 DownloadReleaseAsset 根据 assetID 下载；HTTP 非 2xx 或 JSON 异常会返回错误。
// 逻辑流：1. 从 Repository 获取 owner/repo => 2. 请求 Gitee Release 列表 => 3. 解码并规范化 Release 与附件 => 4. 刷新附件缓存 => 5. 返回 SourceRelease 列表。
func (source *Source) ListReleases(ctx context.Context, repository selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	owner, repo, err := repository.GetSlug()
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases", source.apiBaseURL, url.PathEscape(owner), url.PathEscape(repo))
	request, err := source.newRequest(ctx, http.MethodGet, endpoint)
	if err != nil {
		return nil, err
	}
	response, err := source.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request gitee releases: %w", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Printf("failed to close response body: %s\n", err)
		}
	}(response.Body)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		return nil, fmt.Errorf("gitee releases api returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var releases []giteeRelease
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decode gitee releases: %w", err)
	}
	source.assets = make(map[int64]giteeAsset)
	sourceReleases := make([]selfupdate.SourceRelease, 0, len(releases))
	for releaseIndex := range releases {
		release := &releases[releaseIndex]
		release.normalize()
		for assetIndex := range release.Assets {
			asset := &release.Assets[assetIndex]
			asset.normalize(release.ID)
			source.assets[asset.ID] = *asset
		}
		sourceReleases = append(sourceReleases, release)
	}
	return sourceReleases, nil
}
