package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const (
	defaultGiteeAPIBaseURL = "https://gitee.com/api/v5"
	defaultHTTPTimeout     = 30 * time.Second
)

// GiteeConfig 描述 Gitee Release 更新源配置。
// 用途：允许后续替换 API 域名或接入私有仓库 token；公开仓库 Token 可为空。
type GiteeConfig struct {
	APIBaseURL string
	Token      string
	HTTPClient *http.Client
}

// GiteeSource 将 Gitee Release API 适配为 go-selfupdate 的 Source。
// 边界：只依赖 Release 列表和附件下载地址，不修改 Gitee 仓库内容。
type GiteeSource struct {
	apiBaseURL string
	token      string
	httpClient *http.Client
	assets     map[int64]giteeAsset
}

// NewGiteeSource 创建 GiteeSource。
func NewGiteeSource(config GiteeConfig) (*GiteeSource, error) {
	apiBaseURL := strings.TrimSpace(config.APIBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = defaultGiteeAPIBaseURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")
	if _, err := url.ParseRequestURI(apiBaseURL); err != nil {
		return nil, fmt.Errorf("parse gitee api base url: %w", err)
	}

	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &GiteeSource{
		apiBaseURL: apiBaseURL,
		token:      strings.TrimSpace(config.Token),
		httpClient: httpClient,
		assets:     make(map[int64]giteeAsset),
	}, nil
}

// ListReleases 从 Gitee 读取仓库 Release 列表。
func (source *GiteeSource) ListReleases(ctx context.Context, repository selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	owner, repo, err := repository.GetSlug()
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(
		"%s/repos/%s/%s/releases",
		source.apiBaseURL,
		url.PathEscape(owner),
		url.PathEscape(repo),
	)

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

// DownloadReleaseAsset 下载指定 Gitee Release 附件。
func (source *GiteeSource) DownloadReleaseAsset(ctx context.Context, _ *selfupdate.Release, assetID int64) (io.ReadCloser, error) {
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

	response, err := source.httpClient.Do(request)
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

	return response.Body, nil
}

func (source *GiteeSource) newRequest(ctx context.Context, method string, requestURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create gitee request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "DysonSphereBox-Updater")
	if source.token != "" {
		query := request.URL.Query()
		query.Set("access_token", source.token)
		request.URL.RawQuery = query.Encode()
	}

	return request, nil
}

type giteeRelease struct {
	ID          int64        `json:"id"`
	TagName     string       `json:"tag_name"`
	Prerelease  bool         `json:"prerelease"`
	Draft       bool         `json:"draft"`
	Name        string       `json:"name"`
	Body        string       `json:"body"`
	HTMLURL     string       `json:"html_url"`
	URL         string       `json:"url"`
	CreatedAt   giteeTime    `json:"created_at"`
	PublishedAt giteeTime    `json:"published_at"`
	Assets      []giteeAsset `json:"assets"`
}

func (release *giteeRelease) normalize() {
	if release.ID == 0 {
		release.ID = stableID(release.TagName)
	}
	if release.Name == "" {
		release.Name = release.TagName
	}
	if release.HTMLURL == "" && release.URL != "" {
		release.HTMLURL = release.URL
	}
}

func (release *giteeRelease) GetID() int64 {
	return release.ID
}

func (release *giteeRelease) GetTagName() string {
	return release.TagName
}

func (release *giteeRelease) GetDraft() bool {
	return release.Draft
}

func (release *giteeRelease) GetPrerelease() bool {
	return release.Prerelease
}

func (release *giteeRelease) GetPublishedAt() time.Time {
	if !time.Time(release.PublishedAt).IsZero() {
		return time.Time(release.PublishedAt)
	}

	return time.Time(release.CreatedAt)
}

func (release *giteeRelease) GetReleaseNotes() string {
	return release.Body
}

func (release *giteeRelease) GetName() string {
	return release.Name
}

func (release *giteeRelease) GetURL() string {
	return release.HTMLURL
}

func (release *giteeRelease) GetAssets() []selfupdate.SourceAsset {
	assets := make([]selfupdate.SourceAsset, 0, len(release.Assets))
	for index := range release.Assets {
		assets = append(assets, &release.Assets[index])
	}

	return assets
}

type giteeAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int    `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	DownloadURL        string `json:"download_url"`
	URL                string `json:"url"`
}

func (asset *giteeAsset) normalize(releaseID int64) {
	if asset.BrowserDownloadURL == "" {
		if asset.DownloadURL != "" {
			asset.BrowserDownloadURL = asset.DownloadURL
		} else {
			asset.BrowserDownloadURL = asset.URL
		}
	}
	if asset.ID == 0 {
		asset.ID = stableID(strconv.FormatInt(releaseID, 10) + ":" + asset.Name + ":" + asset.BrowserDownloadURL)
	}
}

func (asset *giteeAsset) GetID() int64 {
	return asset.ID
}

func (asset *giteeAsset) GetName() string {
	return asset.Name
}

func (asset *giteeAsset) GetSize() int {
	return asset.Size
}

func (asset *giteeAsset) GetBrowserDownloadURL() string {
	return asset.BrowserDownloadURL
}

type giteeTime time.Time

func (model *giteeTime) UnmarshalJSON(data []byte) error {
	value := strings.Trim(string(data), `"`)
	if value == "" || value == "null" {
		*model = giteeTime(time.Time{})
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("parse gitee time %q: %w", value, err)
	}

	*model = giteeTime(parsed)
	return nil
}

func stableID(value string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(value))

	return int64(hash.Sum64() & 0x7fffffffffffffff)
}

var _ selfupdate.Source = (*GiteeSource)(nil)
var _ selfupdate.SourceRelease = (*giteeRelease)(nil)
var _ selfupdate.SourceAsset = (*giteeAsset)(nil)
