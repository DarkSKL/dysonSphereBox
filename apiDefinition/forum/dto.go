package forum

// 文件职责：定义社区论坛与服务端交互及 Wails 绑定的请求、响应与本地文件 DTO。
// 使用：由 service/forum、helper/forum 及前端 Wails 生成绑定共同引用。
// 边界：仅承载接口契约与桌面编排数据结构，不声明数据库持久化模型或业务方法。

// Category 描述服务端返回的论坛分类。
// 职责：承载分类 ID、编码、名称、排序与启用状态。
// 使用：列表筛选栏和发帖分类选择器渲染。
// 边界：分类列表来自服务端，前端不写死枚举。
type Category struct {
	// ID 职责：分类唯一正整数标识；使用：发帖与筛选传参；边界：正整数。
	ID uint `json:"id"`
	// Code 职责：分类稳定编码；使用：辅助识别；边界：服务端维护。
	Code string `json:"code"`
	// Name 职责：分类展示名称；使用：界面展示；边界：非空文本。
	Name string `json:"name"`
	// Sort 职责：分类排序权重；使用：展示顺序；边界：服务端已排序。
	Sort int `json:"sort"`
	// Enabled 职责：分类是否启用；使用：旧帖可能引用已停用分类；边界：发帖仅可选启用分类。
	Enabled bool `json:"enabled"`
}

// PostSummary 描述论坛列表中的单条帖子摘要。
// 职责：提供列表分页和详情页头所需的公共字段。
// 使用：PostPage.List 与 PostDetail 内嵌复用。
// 边界：不包含正文和资源详情。
type PostSummary struct {
	// ID 职责：帖子正整数 ID；使用：进入详情与举报；边界：正整数。
	ID uint `json:"id"`
	// Category 职责：帖子所属分类；使用：分类徽章展示；边界：可能为已停用分类。
	Category Category `json:"category"`
	// Title 职责：帖子标题；使用：列表与详情标题展示；边界：纯文本展示。
	Title string `json:"title"`
	// AuthorName 职责：发帖作者展示名；使用：署名展示；边界：服务端返回。
	AuthorName string `json:"authorName"`
	// CreatedAt 职责：发帖时间 RFC3339 字符串；使用：时间展示；边界：服务端生成。
	CreatedAt string `json:"createdAt"`
	// ReplyCount 职责：可见回复总数；使用：列表与详情计数；边界：非负整数。
	ReplyCount int64 `json:"replyCount"`
}

// BodyBlock 描述服务端解析出的 Markdown 正文或资源块。
// 职责：向前端确认正文资源插入点及资源展示状态。
// 使用：帖子详情与回复渲染时按序定位图片、视频、资源卡或下架占位。
// 边界：仅在 bodyFormat 为 markdown 时由服务端返回。
type BodyBlock struct {
	// Type 职责：块类型，取值为 markdown 或 resource；使用：区分正文段和资源插入点；边界：严格按契约值判断。
	Type string `json:"type"`
	// Text 职责：markdown 块的文本内容；使用：辅助核对正文；边界：仅 markdown 块存在。
	Text string `json:"text,omitempty"`
	// ResourceID 职责：资源块关联的资源 ID；使用：在 resources 中查找可见资源；边界：仅 resource 块存在。
	ResourceID uint `json:"resourceId,omitempty"`
	// State 职责：资源块状态，取值为 visible、takenDown 或 unavailable；使用：原位展示资源或下架提示；边界：仅 resource 块存在。
	State string `json:"state,omitempty"`
}

// Resource 描述可见帖或可见回复携带的单个资源元数据。
// 职责：承载蓝图、模组、存档、图片或视频的展示与获取信息。
// 使用：渲染资源卡、媒体播放器及触发复制、下载、导入或打开市场。
// 边界：下架资源不会出现在列表中，只通过 BodyBlock.State 体现。
type Resource struct {
	// ID 职责：资源正整数 ID；使用：媒体读取、下载与举报；边界：正整数。
	ID uint `json:"id"`
	// Type 职责：资源类型（blueprint/save/mod/image/video）；使用：分发渲染组件与导入逻辑；边界：契约枚举。
	Type string `json:"type"`
	// Source 职责：资源来源（upload/inline/thunderstore）；使用：决定获取方式；边界：契约枚举。
	Source string `json:"source"`
	// Name 职责：资源展示名或原始文件名；使用：卡片展示及导入落盘名；边界：导入前需做安全文件名清理。
	Name string `json:"name"`
	// BlueprintCode 职责：内联蓝图编码；使用：inline 蓝图直接复制或导入；边界：托管文件为空。
	BlueprintCode string `json:"blueprintCode,omitempty"`
	// Size 职责：托管文件字节数；使用：展示体积及下载校验；边界：inline/thunderstore 为 0。
	Size int64 `json:"size,omitempty"`
	// SHA256 职责：托管文件小写十六进制 SHA-256；使用：下载后完整性校验；边界：托管文件非空。
	SHA256 string `json:"sha256,omitempty"`
	// Namespace 职责：Thunderstore 包命名空间；使用：市场定位；边界：仅 thunderstore 来源。
	Namespace string `json:"namespace,omitempty"`
	// PackageName 职责：Thunderstore 包名；使用：市场定位；边界：仅 thunderstore 来源。
	PackageName string `json:"packageName,omitempty"`
	// Version 职责：Thunderstore 指定版本号；使用：存在时按版本定位；边界：普通包页可为空。
	Version string `json:"version,omitempty"`
	// URL 职责：规范化 Thunderstore 链接；使用：展示与外链参考；边界：仅 thunderstore 来源。
	URL string `json:"url,omitempty"`
	// ContentType 职责：图片或视频真实 MIME 类型；使用：媒体展示辅助；边界：仅媒体资源。
	ContentType string `json:"contentType,omitempty"`
}

// PostDetail 描述单篇帖子的完整阅读数据。
// 职责：组合帖子摘要、正文格式/块与可见资源列表。
// 使用：帖子详情页渲染。
// 边界：隐藏帖由接口返回 notFound。
type PostDetail struct {
	// ID 职责：帖子正整数 ID；使用：加载回复与举报；边界：正整数。
	ID uint `json:"id"`
	// Category 职责：帖子所属分类；使用：展示分类标签；边界：允许展示停用分类。
	Category Category `json:"category"`
	// Title 职责：帖子标题；使用：详情页主标题；边界：纯文本。
	Title string `json:"title"`
	// AuthorName 职责：作者展示名；使用：署名展示；边界：服务端返回。
	AuthorName string `json:"authorName"`
	// CreatedAt 职责：发帖时间 RFC3339 字符串；使用：时间展示；边界：服务端生成。
	CreatedAt string `json:"createdAt"`
	// ReplyCount 职责：可见回复数；使用：回复区计数；边界：非负整数。
	ReplyCount int64 `json:"replyCount"`
	// Body 职责：帖子原文（纯文本或已绑定真实 ID 的 Markdown）；使用：全文解析或纯文本渲染；边界：最长 10,000 字符。
	Body string `json:"body"`
	// BodyFormat 职责：正文格式（plain 或 markdown）；使用：决定渲染管线；边界：缺省按 plain 处理。
	BodyFormat string `json:"bodyFormat"`
	// BodyBlocks 职责：Markdown 正文确认后的块序列；使用：按位插入资源与下架提示；边界：plain 帖为空。
	BodyBlocks []BodyBlock `json:"bodyBlocks,omitempty"`
	// Resources 职责：帖子可见资源列表；使用：按 ID 绑定媒体与资源卡，或旧帖附件区展示；边界：不包含已下架资源。
	Resources []Resource `json:"resources"`
}

// Reply 描述帖子下的单条可见回复。
// 职责：承载回复正文、Markdown 块与本回复自有图片资源。
// 使用：帖子详情回复分页列表及提交回复后的返回展示。
// 边界：回复图片仅出现在本回复的 Resources 中。
type Reply struct {
	// ID 职责：回复正整数 ID；使用：列表键与举报；边界：正整数。
	ID uint `json:"id"`
	// PostID 职责：所属帖子 ID；使用：归属核对；边界：正整数。
	PostID uint `json:"postId"`
	// AuthorName 职责：回复作者展示名；使用：署名展示；边界：服务端返回。
	AuthorName string `json:"authorName"`
	// CreatedAt 职责：回复时间 RFC3339 字符串；使用：时间展示；边界：服务端生成。
	CreatedAt string `json:"createdAt"`
	// Body 职责：回复原文；使用：纯文本或 Markdown 全文解析；边界：最长 2,000 字符。
	Body string `json:"body"`
	// BodyFormat 职责：正文格式（plain 或 markdown）；使用：决定渲染方式；边界：缺省按 plain 处理。
	BodyFormat string `json:"bodyFormat"`
	// BodyBlocks 职责：Markdown 回复的块序列；使用：按位插入回复图片或下架占位；边界：plain 回复为空。
	BodyBlocks []BodyBlock `json:"bodyBlocks,omitempty"`
	// Resources 职责：本回复可见图片资源列表；使用：按 ID 绑定回复图片；边界：不包含已下架图片。
	Resources []Resource `json:"resources,omitempty"`
}

// PostPage 描述帖子分页结果。
// 职责：映射服务端统一分页响应。
// 使用：帖子列表页渲染和分页器同步。
// 边界：List 保证初始化为非 nil 切片便于前端遍历。
type PostPage struct {
	// Page 职责：当前页码；使用：分页器同步；边界：从 1 开始。
	Page int `json:"page"`
	// PageSize 职责：每页条数；使用：分页器同步；边界：1 到 100。
	PageSize int `json:"pageSize"`
	// Total 职责：符合条件的可见帖子总数；使用：计算总页数；边界：非负整数。
	Total int64 `json:"total"`
	// List 职责：当前页帖子摘要；使用：列表渲染；边界：无数据时为空数组。
	List []PostSummary `json:"list"`
}

// ReplyPage 描述回复分页结果。
// 职责：映射服务端回复分页响应。
// 使用：帖子详情回复区渲染。
// 边界：List 保证初始化为非 nil 切片。
type ReplyPage struct {
	// Page 职责：当前页码；使用：分页器同步；边界：从 1 开始。
	Page int `json:"page"`
	// PageSize 职责：每页条数；使用：分页器同步；边界：1 到 100。
	PageSize int `json:"pageSize"`
	// Total 职责：可见回复总数；使用：计算总页数；边界：非负整数。
	Total int64 `json:"total"`
	// List 职责：当前页回复列表；使用：回复渲染；边界：无数据时为空数组。
	List []Reply `json:"list"`
}

// IDResult 描述发帖或举报成功返回的标识结果。
// 职责：统一承载服务端 `{id}` 响应。
// 使用：发帖成功后跳转详情，举报成功后反馈确认。
// 边界：ID 为正整数。
type IDResult struct {
	// ID 职责：新建帖子或举报的 ID；使用：后续导航或记录；边界：正整数。
	ID uint `json:"id"`
}

// SessionRequest 描述仅依赖已验证会话的查询请求。
// 职责：携带前端当前账号会话键给 Go 侧会话守卫核对。
// 使用：Categories 等无额外参数的查询方法调用。
// 边界：SessionKey 仅在本机核对，不发送到服务端。
type SessionRequest struct {
	// SessionKey 职责：前端当前已验证账号作用域键；使用：防止切号后错绑旧请求；边界：不透传远端。
	SessionKey string `json:"sessionKey"`
	// Force 职责：显式刷新分类；使用：Wails 调用方触发重新读取；边界：仅用于客户端请求策略。
	Force bool `json:"force,omitempty"`
}

// PostListRequest 描述帖子分页查询参数。
// 职责：携带会话键、可选分类 ID 与分页信息。
// 使用：ForumService.Posts 调用。
// 边界：CategoryID 为 0 时表示全部分类。
type PostListRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话隔离；边界：不透传远端。
	SessionKey string `json:"sessionKey"`
	// CategoryID 职责：可选分类筛选 ID；使用：0 时不传 query；边界：正整数有效。
	CategoryID uint `json:"categoryId"`
	// Page 职责：页码；使用：分页查询；边界：小于 1 时归一化为 1。
	Page int `json:"page"`
	// PageSize 职责：每页数量；使用：分页查询；边界：默认 20，最大 100。
	PageSize int `json:"pageSize"`
	// Force 职责：显式刷新分页；使用：Wails 调用方跳过内存结果；边界：不发送给论坛服务器。
	Force bool `json:"force,omitempty"`
}

// PostDetailRequest 描述单篇帖子详情查询参数。
// 职责：携带会话键和目标帖子 ID。
// 使用：ForumService.PostDetail 调用。
// 边界：PostID 必须大于 0。
type PostDetailRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话隔离；边界：不透传远端。
	SessionKey string `json:"sessionKey"`
	// PostID 职责：目标帖子 ID；使用：替换路径参数；边界：必须为正整数。
	PostID uint `json:"postId"`
	// Force 职责：显式核对最新详情；使用：资源状态变化后重新读取；边界：仅用于客户端请求策略。
	Force bool `json:"force,omitempty"`
}

// ReplyListRequest 描述回复分页查询参数。
// 职责：携带会话键、帖子 ID 和分页信息。
// 使用：ForumService.Replies 调用。
// 边界：PostID 必须大于 0。
type ReplyListRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话隔离；边界：不透传远端。
	SessionKey string `json:"sessionKey"`
	// PostID 职责：目标帖子 ID；使用：替换路径参数；边界：必须为正整数。
	PostID uint `json:"postId"`
	// Page 职责：页码；使用：分页查询；边界：小于 1 时归一化为 1。
	Page int `json:"page"`
	// PageSize 职责：每页数量；使用：分页查询；边界：默认 20，最大 100。
	PageSize int `json:"pageSize"`
}

// PublishBlueprintInput 描述发帖时以内联编码提交的蓝图信息。
// 职责：对应发帖 JSON 中可选的 blueprint 字段。
// 使用：用户选择以蓝图编码方式插入蓝图时传入。
// 边界：与 BlueprintFilePath 互斥。
type PublishBlueprintInput struct {
	// Name 职责：蓝图展示名称；使用：资源卡与导入命名；边界：非空。
	Name string `json:"name"`
	// BlueprintCode 职责：蓝图编码文本；使用：服务端校验与保存；边界：非空。
	BlueprintCode string `json:"blueprintCode"`
}

// PublishModInput 描述发帖时以 Thunderstore 市场引用提交的模组信息。
// 职责：对应发帖 JSON 中可选的 mod 字段。
// 使用：从市场选择或填写 Thunderstore 链接时传入。
// 边界：与 ModFilePath 互斥；传 URL 时不应同时传 Namespace/PackageName。
type PublishModInput struct {
	// Namespace 职责：Thunderstore 命名空间；使用：坐标提交；边界：与 URL 互斥。
	Namespace string `json:"namespace,omitempty"`
	// PackageName 职责：Thunderstore 包名；使用：坐标提交；边界：与 URL 互斥。
	PackageName string `json:"packageName,omitempty"`
	// URL 职责：Thunderstore 包页、版本页或官方下载链接；使用：链接提交；边界：必须为 thunderstore.io HTTPS 链接。
	URL string `json:"url,omitempty"`
	// Version 职责：可选指定版本号；使用：指定版本定位；边界：需满足 x.y.z 语义版本。
	Version string `json:"version,omitempty"`
}

// PublishMediaSlot 描述发帖或回复中绑定的单个本地媒体文件槽位。
// 职责：把正文标签键（如 image1、video）映射到本机文件路径。
// 使用：PublishPostRequest.Media 与 PublishReplyRequest.Images 传入。
// 边界：Go 侧按槽位名写入 multipart 字段并从磁盘流式读取。
type PublishMediaSlot struct {
	// Slot 职责：multipart 字段名及正文标签键（image1..image8 或 video）；使用：绑定正文标签；边界：不可重复。
	Slot string `json:"slot"`
	// Path 职责：用户通过本机对话框选择的绝对文件路径；使用：Go 侧流式读取上传；边界：需存在且符合大小与格式限制。
	Path string `json:"path"`
}

// PublishPostRequest 描述发布论坛帖子的完整客户端入参。
// 职责：聚合发帖元数据、内联资源、本机待上传文件路径及会话键。
// 使用：ForumService.PublishPost 接收前端草稿并构造 multipart 请求。
// 边界：同一草稿重试应复用 RequestID；不直接透传本机路径给服务端。
type PublishPostRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话守卫；边界：不发送到服务端。
	SessionKey string `json:"sessionKey"`
	// RequestID 职责：幂等请求 UUID；使用：失败重试防重复建帖；边界：必填。
	RequestID string `json:"requestId"`
	// CategoryID 职责：目标启用分类 ID；使用：服务端校验分类；边界：必须为正整数。
	CategoryID uint `json:"categoryId"`
	// Title 职责：帖子标题；使用：发帖表单提交；边界：非空且不超过 120 字符。
	Title string `json:"title"`
	// Body 职责：帖子正文；使用：含正文标签的 Markdown 或纯文本；边界：非空且不超过 10,000 字符。
	Body string `json:"body"`
	// BodyFormat 职责：正文格式，默认 markdown；使用：告知服务端启用标签绑定；边界：plain 或 markdown。
	BodyFormat string `json:"bodyFormat"`
	// Blueprint 职责：可选内联蓝图编码；使用：与 BlueprintFilePath 二选一；边界：不可同时提供。
	Blueprint *PublishBlueprintInput `json:"blueprint,omitempty"`
	// BlueprintFilePath 职责：可选本地 .txt 蓝图路径；使用：以 blueprintFile 字段流式上传；边界：最大 512 KiB。
	BlueprintFilePath string `json:"blueprintFilePath,omitempty"`
	// Mod 职责：可选 Thunderstore 市场引用；使用：与 ModFilePath 二选一；边界：不可同时提供。
	Mod *PublishModInput `json:"mod,omitempty"`
	// ModFilePath 职责：可选本地 .zip 模组路径；使用：以 modFile 字段流式上传；边界：最大 256 MiB。
	ModFilePath string `json:"modFilePath,omitempty"`
	// SaveFilePath 职责：可选本地 .dsv 存档路径；使用：以 save 字段流式上传；边界：最大 512 MiB。
	SaveFilePath string `json:"saveFilePath,omitempty"`
	// Media 职责：可选图片（image1..image8）和视频（video）槽位；使用：仅 markdown 格式下流式上传；边界：单图 10 MiB、视频 128 MiB。
	Media []PublishMediaSlot `json:"media,omitempty"`
}

// PublishReplyRequest 描述提交帖子回复的客户端入参。
// 职责：承载回复正文、幂等 UUID 与可选图片槽位（image1..image4）。
// 使用：ForumService.PublishReply 无图走 JSON，带图走 multipart。
// 边界：重试时复用同一 RequestID。
type PublishReplyRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话守卫；边界：不发送到服务端。
	SessionKey string `json:"sessionKey"`
	// PostID 职责：目标帖子 ID；使用：替换路径参数；边界：必须为正整数。
	PostID uint `json:"postId"`
	// RequestID 职责：幂等请求 UUID；使用：带图或无图重试防重；边界：必填。
	RequestID string `json:"requestId"`
	// Body 职责：回复正文；使用：Markdown 或纯文本；边界：非空且不超过 2,000 字符。
	Body string `json:"body"`
	// BodyFormat 职责：正文格式，默认 markdown；使用：带图时必须为 markdown；边界：plain 或 markdown。
	BodyFormat string `json:"bodyFormat"`
	// Images 职责：可选图片槽位（最多 image1..image4）；使用：带图时以 multipart 上传；边界：每张最多 10 MiB。
	Images []PublishMediaSlot `json:"images,omitempty"`
}

// ReportRequest 描述提交内容举报的客户端入参。
// 职责：举报帖子、回复或资源。
// 使用：ForumService.Report 接收前端举报弹窗提交。
// 边界：TargetType 仅允许 post、reply、resource。
type ReportRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话守卫；边界：不发送到服务端。
	SessionKey string `json:"sessionKey"`
	// TargetType 职责：举报目标类型（post/reply/resource）；使用：服务端定位目标；边界：必填枚举。
	TargetType string `json:"targetType"`
	// TargetID 职责：举报目标正整数 ID；使用：服务端定位目标；边界：必须为正整数。
	TargetID uint `json:"targetId"`
	// Reason 职责：举报理由；使用：版务审核依据；边界：非空。
	Reason string `json:"reason"`
}

// ResourceActionRequest 描述针对单个论坛资源的读取或导入请求。
// 职责：在获取或导入前按当前会话重新向服务端核对资源可用性。
// 使用：GetResource、FetchBlueprintCode、ImportBlueprint、ImportSave、ImportMod 调用。
// 边界：ResourceID 必须为正整数。
type ResourceActionRequest struct {
	// SessionKey 职责：前端当前账号会话键；使用：本机会话守卫；边界：不发送到服务端。
	SessionKey string `json:"sessionKey"`
	// ResourceID 职责：目标资源 ID；使用：复核可用性与下载；边界：必须为正整数。
	ResourceID uint `json:"resourceId"`
}

// BlueprintCodeResult 描述提取出的蓝图名称和编码。
// 职责：供前端复制内联或托管 .txt 蓝图编码到剪贴板。
// 使用：ForumService.FetchBlueprintCode 返回。
// 边界：托管文件已经过长度、SHA-256 与 UTF-8 校验。
type BlueprintCodeResult struct {
	// Name 职责：蓝图展示名；使用：提示文案；边界：非空。
	Name string `json:"name"`
	// BlueprintCode 职责：蓝图编码正文；使用：写入剪贴板；边界：UTF-8 文本。
	BlueprintCode string `json:"blueprintCode"`
}

// ResourceImportResult 描述蓝图、存档或模组导入本地环境的结果。
// 职责：向前端反馈导入成功提示或资源信息。
// 使用：ImportBlueprint、ImportSave、ImportMod 返回。
// 边界：冲突或校验失败通过错误返回并保留原本地文件。
type ResourceImportResult struct {
	// Message 职责：用户可读的结果说明；使用：界面提示；边界：非空中文消息。
	Message string `json:"message"`
	// Name 职责：导入的资源展示名；使用：界面反馈；边界：来自服务端资源名。
	Name string `json:"name"`
	// Type 职责：资源类型（blueprint/save/mod）；使用：前端区分后续跳转；边界：契约枚举。
	Type string `json:"type"`
	// Source 职责：资源来源；使用：诊断与提示；边界：契约枚举。
	Source string `json:"source"`
}

// SelectedLocalFile 描述用户通过系统对话框选中的单个待上传本地文件。
// 职责：向编辑器返回展示所需的元数据与受控本地预览地址。
// 使用：SelectImages、SelectVideo、SelectResourceFile 返回给编辑器。
// 边界：不包含文件内容字节，避免大文件跨 IPC 复制。
type SelectedLocalFile struct {
	// Path 职责：本地文件绝对路径；使用：发帖或回复时传回 Go 流式上传；边界：仅在桌面端本机使用。
	Path string `json:"path"`
	// Name 职责：文件基名；使用：编辑器展示；边界：非空。
	Name string `json:"name"`
	// Size 职责：文件字节数；使用：展示体积；边界：已通过类型上限校验。
	Size int64 `json:"size"`
	// Ext 职责：小写扩展名（含点号）；使用：界面展示；边界：如 .png、.mp4、.dsv。
	Ext string `json:"ext"`
	// ContentType 职责：识别到的 MIME 类型；使用：预览与辅助展示；边界：图片和视频非空。
	ContentType string `json:"contentType"`
	// PreviewURL 职责：仅图片和视频具备的受控本地预览相对 URL；使用：编辑实时预览；边界：需通过 ReleaseLocalPreviews 释放。
	PreviewURL string `json:"previewUrl,omitempty"`
	// PreviewToken 职责：本地预览令牌；使用：释放预览句柄；边界：无预览时为空。
	PreviewToken string `json:"previewToken,omitempty"`
}

// MediaSelectionResult 描述系统文件选择对话框的返回结果。
// 职责：统一表达取消选择或成功选中一个或多个文件。
// 使用：编辑器插入图片、视频、蓝图、模组或存档时读取。
// 边界：用户取消时 Canceled 为 true 且 Files 为空数组。
type MediaSelectionResult struct {
	// Files 职责：通过校验的本地文件列表；使用：绑定到编辑器槽位；边界：取消时为空数组。
	Files []SelectedLocalFile `json:"files"`
	// Canceled 职责：用户是否取消了对话框；使用：前端区分取消与失败；边界：无文件选中时为 true。
	Canceled bool `json:"canceled"`
}
