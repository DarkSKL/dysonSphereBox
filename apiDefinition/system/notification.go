package system

// 职责：定义桌面收件箱 HTTP 契约；使用：通知服务与请求封装；边界：路径相对于已配置的服务器前缀。

// 职责：声明桌面通知路由；使用：按查询、标读和长连接选择；边界：服务端按当前凭据决定账号。
const (
	// APIURLNotificationStream 职责：订阅收件箱变化；使用：应用级 SSE；边界：事件仅提示重新查询。
	APIURLNotificationStream = "/client/notify/stream"
	// APIURLNotificationUnread 职责：查询全局未读；使用：同步角标；边界：只统计桌面可见站内信。
	APIURLNotificationUnread = "/client/notify/inbox/getMyUnreadCount"
	// APIURLNotificationInbox 职责：查询收件箱；使用：消息面板分页；边界：查询不标为已读。
	APIURLNotificationInbox = "/client/notify/inbox/getMyList"
	// APIURLNotificationMarkRead 职责：写入站内信回执；使用：用户选择标读；边界：使用投递 ID。
	APIURLNotificationMarkRead = "/client/notify/inbox/markRead"
)

// NotificationUnread 职责：描述未读汇总；使用：全局角标和分类；边界：不包含旧公告。
type NotificationUnread struct {
	// Total 职责：全局未读条数；使用：标题栏角标；边界：仅当前账号。
	Total int64 `json:"total"`
	// Categories 职责：按分类提供未读数；使用：通知筛选；边界：没有未读的分类可以缺省。
	Categories map[string]int64 `json:"categories"`
}

// NotificationPage 职责：描述远端分页；使用：站内信列表解码；边界：查询参数由调用方校验。
type NotificationPage struct {
	// Page 职责：页码；使用：定位分页结果；边界：从 1 起。
	Page int `json:"page"`
	// PageSize 职责：单页容量；使用：计算页数；边界：由服务端返回实际值。
	PageSize int `json:"pageSize"`
	// Total 职责：可见条数；使用：分页；边界：可随撤回发生变化。
	Total int64 `json:"total"`
	// List 职责：当前页内容；使用：投影到 Wails；边界：投递 ID 保持整数精度。
	List []NotificationEntry `json:"list"`
}

// NotificationEntry 职责：描述桌面展示所需的远端投递；使用：收件箱解码；边界：正文按纯文本展示。
type NotificationEntry struct {
	// ID 职责：账号的投递 ID；使用：标读；边界：在 Go 内按原始整数解码。
	ID uint64 `json:"ID"`
	// Title 职责：消息标题；使用：收件箱列表；边界：普通文本。
	Title string `json:"title"`
	// Body 职责：消息正文；使用：收件箱列表；边界：普通文本。
	Body string `json:"body"`
	// BizType 职责：跳转类型；使用：匹配受支持的业务页面；边界：未知类型仍可阅读。
	BizType string `json:"bizType"`
	// BizID 职责：业务对象 ID；使用：路由参数；边界：与投递 ID 独立。
	BizID string `json:"bizID"`
	// ReadAt 职责：描述已读时间；使用：识别未读；边界：JSON null 解码为空字符串。
	ReadAt string `json:"readAt"`
	// RecalledAt 职责：描述撤回时间；使用：屏蔽撤回消息操作；边界：JSON null 解码为空字符串。
	RecalledAt string `json:"recalledAt"`
	// CreatedAt 职责：描述投递时间；使用：列表时间信息；边界：保留服务端时间格式。
	CreatedAt string `json:"createdAt"`
}

// NotificationReadRequest 职责：携带待确认投递；使用：markRead POST；边界：服务要求 JSON 整数。
type NotificationReadRequest struct {
	// IDs 职责：指定投递记录；使用：从列表 ID 取得；边界：服务端限制当前账号。
	IDs []uint64 `json:"IDs"`
}

// NotificationReadResult 职责：保留服务端写入结果；使用：按影响行数记录操作；边界：重复标读可返回零。
type NotificationReadResult struct {
	// Affected 职责：本次标读的行数；使用：判断是否产生新回执；边界：零可表示已读或撤回。
	Affected int64 `json:"affected"`
}
