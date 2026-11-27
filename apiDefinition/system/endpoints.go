package system

// 文件职责：声明账号、公告及通知中心的客户端请求策略。
// 使用：系统 helper 依据接口描述发送请求。
// 边界：结果实时读取，账号快照和公告显示绑定由各自业务维护。

import (
	"net/http"
	"time"

	api "DysonSphereBox/apiDefinition"
)

// 账号接口区分公开凭证获取和持有 Token 的会话建立、维护。
var (
	CaptchaAPI        = accountAPI("account.captcha", "获取验证码", http.MethodPost, APIURLCaptcha, api.AuthNone, api.SessionUnchanged)
	RegisterAPI       = accountAPI("account.register", "注册账号", http.MethodPost, APIURLRegister, api.AuthNone, api.SessionUnchanged)
	LoginAPI          = accountAPI("account.login", "登录账号", http.MethodPost, APIURLLogin, api.AuthNone, api.SessionLogin)
	CurrentUserAPI    = accountAPI("account.currentUser", "验证并读取账号", http.MethodGet, APIURLGetUserInfo, api.AuthToken, api.SessionVerify)
	SetSelfInfoAPI    = accountAPI("account.setSelfInfo", "修改账号资料", http.MethodPut, APIURLSetSelfInfo, api.AuthToken, api.SessionUnchanged)
	UploadAvatarAPI   = accountAPI("account.uploadAvatar", "上传头像", http.MethodPost, APIURLUploadAvatar, api.AuthToken, api.SessionUnchanged)
	SetSelfSettingAPI = accountAPI("account.setSelfSetting", "保存个人配置", http.MethodPut, APIURLSetSelfSetting, api.AuthToken, api.SessionUnchanged)
	ChangePasswordAPI = accountAPI("account.changePassword", "修改账号密码", http.MethodPost, APIURLChangePassword, api.AuthToken, api.SessionChangePassword)
	LogoutAPI         = accountAPI("account.logout", "注销账号", http.MethodPost, APIURLLogout, api.AuthToken, api.SessionLogout)
)

// ProbeAPI 职责：公开探测候选地址的账号协议；使用：设置页连接测试；边界：短时限、小响应、无凭据。
var ProbeAPI = &api.ApiDesc{
	ID: "account.probe", Description: "探测候选服务器", URL: APIURLCaptcha, Method: http.MethodPost,
	Auth: api.AuthNone, Timeout: 6 * time.Second, ResponseLimit: 256 << 10,
	Envelope: api.EnvelopePolicy{RenewOnHTTPError: true},
}

// 公告查询和确认要求已验证会话，确认成功由公告协议另行核验。
var (
	LatestNoticeAPI   = noticeAPI("notice.getLatestUnread", "查询最新未读公告", http.MethodGet, APIURLClientNoticeGetLatestUnread)
	NoticeHistoryAPI  = noticeAPI("notice.history", "查询公告历史", http.MethodGet, APIURLClientNoticeGetHistory)
	MarkNoticeReadAPI = noticeAPI("notice.markRead", "确认已读公告", http.MethodPost, APIURLClientNoticeMarkRead)
)

// 消息收件箱和 SSE 共享已验证会话，事件连接时限由流式业务管理。
var (
	NotificationUnreadAPI = notificationAPI("notification.unread", "查询未读消息数", http.MethodGet, APIURLNotificationUnread)
	NotificationInboxAPI  = notificationAPI("notification.inbox", "查询收件箱", http.MethodGet, APIURLNotificationInbox)
	NotificationReadAPI   = notificationAPI("notification.markRead", "确认已读消息", http.MethodPost, APIURLNotificationMarkRead)
	NotificationStreamAPI = &api.ApiDesc{
		ID: "notification.stream", Description: "订阅站内通知", URL: APIURLNotificationStream, Method: http.MethodGet,
		Auth: api.AuthVerified, Response: api.SSEResponse,
	}
)

// accountAPI 为账号接口复用统一外壳和可信认证应答策略。
func accountAPI(id, description, method, path string, auth api.AuthMode, action api.SessionAction) *api.ApiDesc {
	return &api.ApiDesc{
		ID: id, Description: description, URL: path, Method: method, Auth: auth, SessionAction: action,
		Envelope: api.EnvelopePolicy{RenewOnHTTPError: true},
	}
}

// noticeAPI 固定公告的完整外壳和改密标志要求。
func noticeAPI(id, description, method, path string) *api.ApiDesc {
	return &api.ApiDesc{
		ID: id, Description: description, URL: path, Method: method, Auth: api.AuthVerified, Timeout: 10 * time.Second,
		Envelope: api.EnvelopePolicy{RequireMessage: true, RequireData: true, PasswordFlag: true, RenewOnHTTPError: true},
	}
}

// notificationAPI 固定收件箱成功响应的对象形状。
func notificationAPI(id, description, method, path string) *api.ApiDesc {
	return &api.ApiDesc{
		ID: id, Description: description, URL: path, Method: method, Auth: api.AuthVerified,
		Envelope: api.EnvelopePolicy{ObjectData: true},
	}
}

// Endpoints 职责：提供系统请求目录；使用：应用启动加载；边界：返回独立列表。
func Endpoints() []*api.ApiDesc {
	return []*api.ApiDesc{
		CaptchaAPI, RegisterAPI, LoginAPI, CurrentUserAPI, SetSelfInfoAPI, UploadAvatarAPI, SetSelfSettingAPI, ChangePasswordAPI, LogoutAPI, ProbeAPI,
		LatestNoticeAPI, NoticeHistoryAPI, MarkNoticeReadAPI,
		NotificationUnreadAPI, NotificationInboxAPI, NotificationReadAPI, NotificationStreamAPI,
	}
}
