package feedback

// 文件职责：声明反馈星系请求及其大小、时限和认证契约。
// 使用：反馈 helper 和应用请求目录。
// 边界：结果实时获取，批量查询使用服务端既有的 POST 接口。

import (
	"net/http"
	"time"

	api "DysonSphereBox/apiDefinition"
)

// 反馈查询和写入复用一致的有界 JSON 策略，均要求已验证会话。
var (
	MapAPI        = feedbackAPI("feedback.map", "查询反馈星图", http.MethodGet, PathMap)
	PreviewAPI    = feedbackAPI("feedback.preview", "查询反馈预览", http.MethodGet, PathPreview)
	PreviewsAPI   = feedbackAPI("feedback.previews", "批量查询反馈预览", http.MethodPost, PathPreviews)
	DetailAPI     = feedbackAPI("feedback.detail", "查询反馈详情", http.MethodGet, PathDetail)
	MineAPI       = feedbackAPI("feedback.mine", "查询我的反馈", http.MethodGet, PathMine)
	PublishAPI    = feedbackAPI("feedback.publish", "发布反馈", http.MethodPost, PathPublish)
	EditAPI       = feedbackAPI("feedback.edit", "编辑反馈", http.MethodPost, PathEdit)
	ReplyAPI      = feedbackAPI("feedback.reply", "回复反馈", http.MethodPost, PathReply)
	LikeAPI       = feedbackAPI("feedback.like", "更新反馈点赞", http.MethodPost, PathLike)
	WithdrawAPI   = feedbackAPI("feedback.withdraw", "撤回反馈", http.MethodPost, PathWithdraw)
	SeenAPI       = feedbackAPI("feedback.seen", "确认查看反馈", http.MethodPost, PathSeen)
	NoticesAPI    = feedbackAPI("feedback.notices", "查询反馈提醒", http.MethodGet, PathNotices)
	ReadNoticeAPI = feedbackAPI("feedback.readNotice", "确认反馈提醒", http.MethodPost, PathReadNotice)
)

// feedbackAPI 固定反馈接口的认证、响应形状及资源上限。
func feedbackAPI(id, description, method, path string) *api.ApiDesc {
	return &api.ApiDesc{
		ID: id, Description: description, URL: path, Method: method, Auth: api.AuthVerified,
		Timeout: 10 * time.Second, RequestLimit: 16 << 10,
		Envelope: api.EnvelopePolicy{RequireMessage: true, RequireData: true, ObjectData: true},
	}
}

// Endpoints 职责：提供反馈请求目录；使用：应用启动加载；边界：返回独立列表。
func Endpoints() []*api.ApiDesc {
	return []*api.ApiDesc{MapAPI, PreviewAPI, PreviewsAPI, DetailAPI, MineAPI, PublishAPI,
		EditAPI, ReplyAPI, LikeAPI, WithdrawAPI, SeenAPI, NoticesAPI, ReadNoticeAPI}
}
