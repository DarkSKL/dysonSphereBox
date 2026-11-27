package forum

// 文件职责：维护社区论坛的桌面客户端相对接口路径模板。
// 使用：由论坛 helper 结合 utils.ResolveAPIPath 构造请求路径。
// 边界：仅声明玩家侧相对路径，不包含后台版务路由或服务器基址。

// 社区论坛玩家接口路径仅用于已验证账号会话。
const (
	// PathCategories 查询启用的论坛分类列表。
	PathCategories = "/client/forum/categories"
	// PathPosts 分页查询可见帖子或发布新帖子。
	PathPosts = "/client/forum/posts"
	// PathPostDetail 查询单篇帖子正文、资源块与可用资源。
	PathPostDetail = "/client/forum/posts/{{ID}}"
	// PathPostReplies 分页查询或提交指定帖子的回复。
	PathPostReplies = "/client/forum/posts/{{ID}}/replies"
	// PathReports 提交对帖子、回复或资源的举报。
	PathReports = "/client/forum/reports"
	// PathResource 查询并复核单个可见资源的最新元数据。
	PathResource = "/client/forum/resources/{{ID}}"
	// PathResourceDownload 经鉴权下载托管蓝图、模组或存档文件。
	PathResourceDownload = "/client/forum/resources/{{ID}}/download"
	// PathResourceContent 经鉴权读取正文图片或支持 Range 的 MP4 视频。
	PathResourceContent = "/client/forum/resources/{{ID}}/content"
)

// IDPathParams 描述仅包含单个正整数资源标识的路径模板参数。
// 职责：为 utils.ResolveAPIPath 提供与 {{ID}} 匹配的导出字段。
// 使用：解析帖子详情、回复列表、资源查询、下载与媒体路径时传入。
// 边界：调用方需先校验 ID 大于 0。
type IDPathParams struct {
	// ID 职责：保存帖子或资源的正整数 ID；使用：替换路径模板中的 {{ID}}；边界：必须为正整数。
	ID uint
}
