package feedback

// 文件职责：维护反馈星系的桌面相对接口路径。
// 使用：由星系绑定发起固定目标请求。
// 边界：不包含后台路由或服务器基址。

// 星系玩家接口路径仅用于已验证账号。
const (
	// PathMap 用于请求星图接口，路径相对于已配置服务器。
	PathMap = "/client/feedback/map"
	// PathPreview 用于请求预览接口，路径相对于已配置服务器。
	PathPreview = "/client/feedback/preview"
	// PathPreviews 定位已验证账号的批量查询，供桌面服务 POST 只读的星星 ID 列表。
	PathPreviews = "/client/feedback/previews"
	// PathDetail 用于请求详情接口，路径相对于已配置服务器。
	PathDetail = "/client/feedback/detail"
	// PathMine 用于请求我的星星接口，路径相对于已配置服务器。
	PathMine = "/client/feedback/mine"
	// PathPublish 用于请求发布接口，路径相对于已配置服务器。
	PathPublish = "/client/feedback/publish"
	// PathEdit 用于请求编辑接口，路径相对于已配置服务器。
	PathEdit = "/client/feedback/edit"
	// PathReply 用于请求回复接口，路径相对于已配置服务器。
	PathReply = "/client/feedback/reply"
	// PathLike 用于请求点赞接口，路径相对于已配置服务器。
	PathLike = "/client/feedback/like"
	// PathWithdraw 用于请求撤回接口，路径相对于已配置服务器。
	PathWithdraw = "/client/feedback/withdraw"
	// PathSeen 用于请求留言已读接口，路径相对于已配置服务器。
	PathSeen = "/client/feedback/seen"
	// PathNotices 用于请求提醒列表接口，路径相对于已配置服务器。
	PathNotices = "/client/feedback/notices"
	// PathReadNotice 用于请求提醒已读接口，路径相对于已配置服务器。
	PathReadNotice = "/client/feedback/readNotice"
)
