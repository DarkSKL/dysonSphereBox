package source

// 文件职责：定义 Gitee Release 附件 DTO，并适配 go-selfupdate.SourceAsset 接口。
// 使用：由 gitee.Source 在读取 Release 列表后缓存附件，并由更新器选择当前平台资产时调用。
// 边界：只描述附件元数据和下载地址，不执行 HTTP 下载、不校验 checksum、不决定前端提示。

import "strconv"

// giteeAsset 描述 Gitee Release 附件的原始响应和适配字段。
// 职责：承载附件 ID、名称、大小和下载地址，并实现 go-selfupdate.SourceAsset。
// 使用：giteeRelease.GetAssets 返回给 go-selfupdate 选择当前平台资产，Source 缓存后用于下载。
// 边界：只描述远端附件元数据，不负责实际 HTTP 下载或 checksum 校验。
type giteeAsset struct {
	// ID 是 Gitee 附件 ID 或根据名称和地址生成的稳定 ID。
	// 职责：作为 go-selfupdate 下载资产时回传的唯一标识。
	// 使用：normalize 在缺失原始 ID 时补齐，Source.assets 以它作为键。
	// 边界：只在单次 Release 列表缓存内使用，不写入数据库。
	ID int64 `json:"id"`
	// Name 是附件文件名。
	// 职责：让 go-selfupdate 匹配平台资产并用于错误信息。
	// 使用：GetName、normalize 和校验流程读取。
	// 边界：不保证一定包含平台信息，实际选择由 go-selfupdate 规则负责。
	Name string `json:"name"`
	// Size 是附件字节大小。
	// 职责：向 go-selfupdate 暴露远端资产大小。
	// 使用：GetSize 返回该值。
	// 边界：仅用于元数据展示或选择辅助，不用于下载完整性校验。
	Size int `json:"size"`
	// BrowserDownloadURL 是优先使用的浏览器下载地址。
	// 职责：保存可直接下载附件内容的 URL。
	// 使用：DownloadReleaseAsset 构造附件下载请求时读取。
	// 边界：normalize 会尝试用 DownloadURL 或 URL 兜底，兜底后仍为空则下载失败。
	BrowserDownloadURL string `json:"browser_download_url"`
	// DownloadURL 是 Gitee API 返回的备用下载地址。
	// 职责：在 browser_download_url 缺失时提供兜底来源。
	// 使用：giteeAsset.normalize 填充 BrowserDownloadURL 时读取。
	// 边界：不会被直接下载，必须先规范化到 BrowserDownloadURL。
	DownloadURL string `json:"download_url"`
	// URL 是 Gitee API 返回的附件资源地址。
	// 职责：作为最后一级下载地址兜底。
	// 使用：giteeAsset.normalize 在其它下载地址缺失时读取。
	// 边界：可能是 API 地址而非二进制地址，下载失败时由 DownloadReleaseAsset 返回错误。
	URL string `json:"url"`
}

// normalize 规范化 Gitee Release 附件字段。
// 职责：补齐下载地址和稳定附件 ID。
// 使用：Source.ListReleases 遍历 Release.Assets 时调用。
// 边界：只处理元数据兜底，不访问网络，不验证 URL 是否真实可下载。
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

// GetID 返回附件稳定 ID，用于适配 go-selfupdate.SourceAsset。
// 职责：实现 SourceAsset ID 读取接口。
// 使用：go-selfupdate 选择资产后将该 ID 回传给 DownloadReleaseAsset。
// 边界：返回 normalize 后的值，未规范化时可能为 0。
func (asset *giteeAsset) GetID() int64 {
	return asset.ID
}

// GetName 返回 Gitee 附件文件名。
// 职责：实现 SourceAsset 文件名读取接口。
// 使用：go-selfupdate 用于平台匹配、校验文件识别和错误提示。
// 边界：不保证文件名一定包含操作系统或架构信息。
func (asset *giteeAsset) GetName() string {
	return asset.Name
}

// GetSize 返回 Gitee 附件大小，用于适配 go-selfupdate.SourceAsset。
// 职责：实现 SourceAsset 尺寸读取接口。
// 使用：由 go-selfupdate 在筛选或展示资产元数据时调用。
// 边界：直接返回远端声明值，不校验实际下载字节数。
func (asset *giteeAsset) GetSize() int {
	return asset.Size
}

// GetBrowserDownloadURL 返回 Gitee 附件的浏览器下载地址。
// 职责：实现 SourceAsset 下载地址读取接口。
// 使用：go-selfupdate 或本地下载逻辑需要展示资产 URL 时调用。
// 边界：地址在 normalize 中已做 download_url/url 兜底；为空时由下载阶段报错。
func (asset *giteeAsset) GetBrowserDownloadURL() string {
	return asset.BrowserDownloadURL
}
