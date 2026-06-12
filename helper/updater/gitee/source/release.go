package source

// 文件职责：定义 Gitee Release DTO，并适配 go-selfupdate.SourceRelease 接口。
// 使用：由 gitee.Source.ListReleases 解码远端发布列表后交给 go-selfupdate 做版本检测。
// 边界：只描述发布元数据、附件列表和接口适配方法，不下载附件、不比较业务版本、不写入本地文件。

// giteeRelease 描述 Gitee Release API 的单个发布项。
// 职责：承载发布版本、状态、说明、发布时间和附件列表，并适配 go-selfupdate.SourceRelease。
// 使用：Source.ListReleases 解码远端响应后返回给 go-selfupdate 检测最新版本。
// 边界：只描述 Release 元数据，不负责资产下载、版本比较或校验。
type giteeRelease struct {
	// ID 是 Gitee Release ID 或根据标签生成的稳定 ID。
	// 职责：作为 go-selfupdate 识别 Release 的唯一标识。
	// 使用：normalize 在缺失时补齐，GetID 返回给更新器。
	// 边界：不写入本地持久化，只用于本次更新检测。
	ID int64 `json:"id"`
	// TagName 是 Release 标签名。
	// 职责：提供版本比较所需的远端版本文本。
	// 使用：GetTagName 返回给 go-selfupdate 解析版本。
	// 边界：标签格式异常时由 go-selfupdate 版本比较流程处理。
	TagName string `json:"tag_name"`
	// Prerelease 标记该 Release 是否为预发布版本。
	// 职责：向 go-selfupdate 暴露发布阶段信息。
	// 使用：GetPrerelease 返回给更新器筛选版本。
	// 边界：本 helper 不在字段层面强制排除预发布。
	Prerelease bool `json:"prerelease"`
	// Draft 标记该 Release 是否为草稿。
	// 职责：向 go-selfupdate 暴露草稿状态。
	// 使用：GetDraft 返回给更新器筛选版本。
	// 边界：本 helper 不直接展示草稿版本。
	Draft bool `json:"draft"`
	// Name 是 Release 展示名称。
	// 职责：提供前端更新提示使用的发布标题。
	// 使用：normalize 在缺失时回退到标签名，GetName 返回。
	// 边界：不参与版本比较。
	Name string `json:"name"`
	// Body 是 Release 发布说明正文。
	// 职责：承载更新内容说明。
	// 使用：GetReleaseNotes 返回后写入 CheckResult。
	// 边界：不解析 Markdown，不做内容清洗。
	Body string `json:"body"`
	// HTMLURL 是 Gitee Release 页面地址。
	// 职责：提供用户可打开的发布详情链接。
	// 使用：GetURL 返回后写入 CheckResult。
	// 边界：缺失时 normalize 尝试使用 URL 兜底。
	HTMLURL string `json:"html_url"`
	// URL 是 Gitee API 返回的发布资源地址。
	// 职责：作为 HTMLURL 缺失时的兜底链接。
	// 使用：normalize 补齐 HTMLURL 时读取。
	// 边界：可能不是浏览器友好的页面地址。
	URL string `json:"url"`
	// CreatedAt 是 Release 创建时间。
	// 职责：在发布时间缺失时提供兜底时间。
	// 使用：GetPublishedAt 在 PublishedAt 为零值时返回。
	// 边界：只影响展示时间，不参与版本比较。
	CreatedAt giteeTime `json:"created_at"`
	// PublishedAt 是 Release 发布时间。
	// 职责：提供前端展示的发布时间来源。
	// 使用：GetPublishedAt 优先返回该字段。
	// 边界：空值会回退到 CreatedAt。
	PublishedAt giteeTime `json:"published_at"`
	// Assets 是 Release 附件列表。
	// 职责：提供 go-selfupdate 选择平台资产和校验文件的候选集合。
	// 使用：GetAssets 返回 SourceAsset 列表，ListReleases 同时将其缓存到 Source。
	// 边界：附件是否可下载由 DownloadReleaseAsset 再次校验 URL。
	Assets []giteeAsset `json:"assets"`
}
