package forum

// 文件职责：声明论坛页面读取、公开写入和资源传输的接口策略。
// 使用：论坛 helper 与应用启动的请求目录。
// 边界：回复只合并在途读取，发布后的可见性由论坛回执认定。

import (
	"net/http"
	"time"

	api "DysonSphereBox/apiDefinition"
)

// 策略名称供接口及其失效来源引用，扩展中间件由论坛 helper 注册。
const (
	PublishPostID                        = "forum.publishPost"
	PublishReplyID                       = "forum.publishReply"
	NormalizePage       api.MiddlewareID = "forum.normalizePage"
	InvalidatePublished api.MiddlewareID = "forum.invalidatePublished"
	JSONRequestLimit                     = 1 << 20
	transferTimeout                      = 15 * time.Minute
)

// 读取策略固定已上线的分类、列表、详情 TTL 和回复在途合并。
var (
	CategoriesAPI = forumQuery("forum.categories", "查询论坛分类", PathCategories, 5*time.Minute, false)
	PostsAPI      = forumQuery("forum.posts", "查询帖子分页", PathPosts, 30*time.Second, true,
		api.CacheSource{API: PublishPostID, MatchTarget: true}, api.CacheSource{API: PublishReplyID})
	PostDetailAPI = forumQuery("forum.postDetail", "读取帖子详情", PathPostDetail, time.Minute, false,
		api.CacheSource{API: PublishReplyID, MatchTarget: true})
	RepliesAPI = forumQuery("forum.replies", "查询回复分页", PathPostReplies, 0, true,
		api.CacheSource{API: PublishReplyID, MatchTarget: true})
)

// 发帖、回复和举报使用实时写请求，公开发帖和回复产生定向失效。
var (
	PublishPostAPI  = forumPublication(PublishPostID, "发布帖子", PathPosts)
	PublishReplyAPI = forumPublication(PublishReplyID, "发布回复", PathPostReplies)
	ReportAPI       = forumJSON("forum.report", "提交举报", http.MethodPost, PathReports)
	ReviewAPI       = forumJSON("forum.review", "读取作者审核记录", http.MethodGet, PathPublicationReview)
	ResourceAPI     = forumJSON("forum.resource", "复核资源可用性", http.MethodGet, PathResource)
	DownloadAPI     = forumStream("forum.download", "下载托管文件", http.MethodGet, PathResourceDownload, api.FileResponse, transferTimeout)
	MediaAPI        = forumStream("forum.media", "读取媒体内容", http.MethodGet, PathResourceContent, api.RangeResponse, 0)
	MediaHeadAPI    = forumStream("forum.mediaHead", "读取媒体响应头", http.MethodHead, PathResourceContent, api.RangeResponse, 0)
)

// forumJSON 复用论坛普通 JSON 的时限、大小和外壳校验。
func forumJSON(id, description, method, path string) *api.ApiDesc {
	return &api.ApiDesc{
		ID: id, Description: description, URL: path, Method: method, Auth: api.AuthVerified,
		Timeout: 15 * time.Second, RequestLimit: JSONRequestLimit, ResponseLimit: 8 << 20,
		Envelope: api.EnvelopePolicy{RequireMessage: true},
	}
}

// forumQuery 显式声明视图快照、分页准备和写后失效来源。
func forumQuery(id, description, path string, ttl time.Duration, paged bool, sources ...api.CacheSource) *api.ApiDesc {
	desc := forumJSON(id, description, http.MethodGet, path)
	desc.ShareRequest = true
	desc.Cache.HitSource = sources
	if ttl > 0 {
		desc.Cache.Mode, desc.Cache.TTL = api.ViewSnapshot, ttl
	}
	if paged {
		desc.BeforeMiddleware = []api.MiddlewareID{NormalizePage}
	}
	return desc
}

// forumPublication 为 JSON 和 multipart 提交共用可见性回执，上传时读取独立长时限。
func forumPublication(id, description, path string) *api.ApiDesc {
	desc := forumJSON(id, description, http.MethodPost, path)
	desc.UploadTimeout = transferTimeout
	desc.AfterMiddleware = []api.MiddlewareID{InvalidatePublished}
	return desc
}

// forumStream 复用论坛鉴权外壳，仅将合法流式响应交给文件或媒体处理。
func forumStream(id, description, method, path string, mode api.ResponseMode, timeout time.Duration) *api.ApiDesc {
	desc := forumJSON(id, description, method, path)
	desc.Response, desc.Timeout = mode, timeout
	return desc
}

// Endpoints 职责：提供论坛请求目录；使用：应用启动加载；边界：返回独立列表。
func Endpoints() []*api.ApiDesc {
	return []*api.ApiDesc{
		CategoriesAPI, PostsAPI, PostDetailAPI, RepliesAPI, PublishPostAPI, PublishReplyAPI,
		ReportAPI, ReviewAPI, ResourceAPI, DownloadAPI, MediaAPI, MediaHeadAPI,
	}
}
