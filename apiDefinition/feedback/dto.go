package feedback

// 文件职责：定义星系桌面端 HTTP 与 Wails 共用的类型。
// 使用：Wails 方法收取请求 DTO 并将解码后的成功数据交给页面。
// 边界：ID 和版本始终为十进制字符串，令牌与后台审核数据不进入 DTO。

// MapRequest 职责：承载全景或视区查询；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type MapRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// MinX 表示范围左界，省略时采用服务端默认语义。
	MinX *float64 `json:"minX,omitempty"`
	// MinY 表示范围下界，省略时采用服务端默认语义。
	MinY *float64 `json:"minY,omitempty"`
	// MaxX 表示范围右界，省略时采用服务端默认语义。
	MaxX *float64 `json:"maxX,omitempty"`
	// MaxY 表示范围上界，省略时采用服务端默认语义。
	MaxY *float64 `json:"maxY,omitempty"`
	// Version 表示原视区快照版本，省略时采用服务端默认语义。
	Version string `json:"version,omitempty"`
}

// PreviewRequest 职责：承载单星预览查询；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type PreviewRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
}

// PreviewsRequest 承载一批代表星的预览查询。
// 职责：接收原始 ID 列表和本次调用的会话键。使用：页面调用 Previews。边界：会话键仅用于本机核对。
type PreviewsRequest struct {
	// StarIDs 保留调用方的顺序和十进制字符串精度。
	StarIDs []string `json:"starIds"`
	// SessionKey 在捕获已验证账号时核对，发往服务器的请求仅填写 StarIDs。
	SessionKey string `json:"sessionKey,omitempty"`
}

// DetailRequest 职责：承载详情与留言查询；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type DetailRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
	// Page 表示留言页码，省略时采用服务端默认语义。
	Page int `json:"page,omitempty"`
	// PageSize 表示留言分页大小，省略时采用服务端默认语义。
	PageSize int `json:"pageSize,omitempty"`
}

// PageRequest 职责：承载列表分页；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type PageRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// Page 表示页码，省略时采用服务端默认语义。
	Page int `json:"page,omitempty"`
	// PageSize 表示分页大小，省略时采用服务端默认语义。
	PageSize int `json:"pageSize,omitempty"`
}

// PublishRequest 职责：承载发布新星；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type PublishRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// RequestID 表示幂等请求 UUID。
	RequestID string `json:"requestId"`
	// Body 表示星星正文。
	Body string `json:"body"`
	// ShowName 表示是否公开署名。
	ShowName bool `json:"showName"`
}

// EditRequest 职责：承载编辑原文；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type EditRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// RequestID 表示幂等请求 UUID。
	RequestID string `json:"requestId"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
	// BodyVersion 表示所见正文版本。
	BodyVersion string `json:"bodyVersion"`
	// Body 表示替换后的正文。
	Body string `json:"body"`
}

// ReplyRequest 职责：承载提交留言或补充；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type ReplyRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// RequestID 表示幂等请求 UUID。
	RequestID string `json:"requestId"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
	// BodyVersion 表示所见正文版本。
	BodyVersion string `json:"bodyVersion"`
	// Kind 表示发言类型。
	Kind string `json:"kind"`
	// Body 表示发言正文。
	Body string `json:"body"`
}

// LikeRequest 职责：承载设置点赞状态；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type LikeRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
	// BodyVersion 表示所见正文版本。
	BodyVersion string `json:"bodyVersion"`
	// Liked 表示要设置的点赞状态。
	Liked bool `json:"liked"`
}

// WithdrawRequest 职责：承载撤回星星；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type WithdrawRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
}

// SeenRequest 职责：承载更新留言已读位置；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type SeenRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// StarID 表示目标星 ID。
	StarID string `json:"starId"`
	// ReviewVersion 表示已看到的留言审核版本。
	ReviewVersion string `json:"reviewVersion"`
}

// ReadNoticeRequest 职责：承载确认提醒已读；使用：页面构造 Wails 参数；边界：业务授权由服务端校验。
type ReadNoticeRequest struct {
	// SessionKey 对照页面捕获的账号会话键；由 Wails 调用方传入，可省略，仅供本机核对。
	SessionKey string `json:"sessionKey,omitempty"`
	// ID 表示提醒 ID。
	ID string `json:"id"`
}

// Bounds 职责：承载星图边界；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type Bounds struct {
	// MinX 表示星图左界。
	MinX float64 `json:"minX"`
	// MinY 表示星图下界。
	MinY float64 `json:"minY"`
	// MaxX 表示星图右界。
	MaxX float64 `json:"maxX"`
	// MaxY 表示星图上界。
	MaxY float64 `json:"maxY"`
}

// StarPoint 职责：承载星图代表点；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type StarPoint struct {
	// StarID 表示真实代表星 ID。
	StarID string `json:"starId"`
	// PreviewVersion 是代表星当前生效的审核记录 ID，旧服务器省略时交给页面使用地图版本。
	PreviewVersion string `json:"previewVersion,omitempty"`
	// X 表示横坐标。
	X float64 `json:"x"`
	// Y 表示纵坐标。
	Y float64 `json:"y"`
	// Z 表示星点沿盘面法向的高度，旧服务端省略该字段时为 0。
	Z int `json:"z"`
	// Progress 表示开发进展。
	Progress string `json:"progress"`
	// Brightness 表示归一化亮度。
	Brightness float64 `json:"brightness"`
	// Count 表示聚合点覆盖的星数，单星可省略。
	Count *int `json:"count,omitempty"`
}

// MapResult 职责：承载视区快照；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type MapResult struct {
	// Version 表示视区版本。
	Version string `json:"version"`
	// Bounds 表示星图边界。
	Bounds Bounds `json:"bounds"`
	// Unchanged 表示是否复用已有快照。
	Unchanged bool `json:"unchanged"`
	// Points 表示视区代表点。
	Points []StarPoint `json:"points"`
}

// PreviewResult 职责：承载单星预览；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type PreviewResult struct {
	// StarID 表示星星 ID。
	StarID string `json:"starId"`
	// Preview 表示首行预览。
	Preview string `json:"preview"`
	// DisplayName 来自服务器的可见署名，旧响应缺省时留空。
	DisplayName string `json:"displayName,omitempty"`
}

// PreviewsResult 承载批量预览的可见条目。
// 职责：投影服务器返回的首行与署名。使用：预取公共星摘要。边界：跳过的星不会补成空条目。
type PreviewsResult struct {
	// Items 复用单条预览的展示字段，空数组表示本批没有可读的星。
	Items []PreviewResult `json:"items"`
}

// Entry 职责：承载可见发言；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type Entry struct {
	// ID 表示发言 ID。
	ID string `json:"id"`
	// Kind 表示发言类型。
	Kind string `json:"kind"`
	// Body 表示纯文本正文。
	Body string `json:"body"`
	// DisplayName 表示可公开显示的署名。
	DisplayName string `json:"displayName"`
	// CreatedAt 表示UTC 创建时间。
	CreatedAt string `json:"createdAt"`
	// ReviewState 表示审核状态。
	ReviewState string `json:"reviewState"`
	// BodyVersion 表示正文版本。
	BodyVersion string `json:"bodyVersion"`
	// ReviewVersion 表示审核版本。
	ReviewVersion string `json:"reviewVersion"`
	// ReviewReason 用于展示该发言的退回原因，由服务端仅向提交人提供。
	ReviewReason string `json:"reviewReason,omitempty"`
}

// EntryPage 职责：承载发言分页；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type EntryPage struct {
	// Page 表示页码。
	Page int `json:"page"`
	// PageSize 表示分页大小。
	PageSize int `json:"pageSize"`
	// Total 表示总条数。
	Total int64 `json:"total"`
	// List 表示当前页发言。
	List []Entry `json:"list"`
}

// StarDetail 职责：承载星星详情；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type StarDetail struct {
	// StarID 表示星星 ID。
	StarID string `json:"starId"`
	// X 表示横坐标。
	X float64 `json:"x"`
	// Y 表示纵坐标。
	Y float64 `json:"y"`
	// Z 表示详情中星体的盘面法向高度，旧服务端省略该字段时为 0。
	Z int `json:"z"`
	// Original 表示星星原文。
	Original Entry `json:"original"`
	// Progress 表示开发进展。
	Progress string `json:"progress"`
	// ReleaseVersion 表示实现版本。
	ReleaseVersion string `json:"releaseVersion"`
	// LikeCount 表示点赞数。
	LikeCount int64 `json:"likeCount"`
	// Liked 表示当前账号点赞状态。
	Liked bool `json:"liked"`
	// Owned 表示是否属于当前账号。
	Owned bool `json:"owned"`
	// CanEdit 表示当前账号能否编辑。
	CanEdit bool `json:"canEdit"`
	// Withdrawn 表示是否撤回。
	Withdrawn bool `json:"withdrawn"`
	// ReviewReason 表示作者可见的审核原因。
	ReviewReason string `json:"reviewReason"`
	// Replies 表示发言分页。
	Replies EntryPage `json:"replies"`
	// OfficialReplies 表示近期官方回应。
	OfficialReplies []Entry `json:"officialReplies"`
	// Supplements 表示近期作者补充。
	Supplements []Entry `json:"supplements"`
}

// MyStar 职责：承载作者列表项；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type MyStar struct {
	// StarID 表示我的星星 ID。
	StarID string `json:"starId"`
	// X 表示横坐标。
	X float64 `json:"x"`
	// Y 表示纵坐标。
	Y float64 `json:"y"`
	// Z 表示作者列表中星体的盘面法向高度，旧服务端省略该字段时为 0。
	Z int `json:"z"`
	// Preview 表示正文预览。
	Preview string `json:"preview"`
	// DisplayName 复用与详情一致的可见署名，旧服务器省略时留空。
	DisplayName string `json:"displayName,omitempty"`
	// ReviewState 表示审核状态。
	ReviewState string `json:"reviewState"`
	// BodyVersion 表示正文版本。
	BodyVersion string `json:"bodyVersion"`
	// ReviewReason 表示作者可见的审核原因。
	ReviewReason string `json:"reviewReason"`
	// Progress 表示开发进展。
	Progress string `json:"progress"`
	// Withdrawn 表示是否撤回。
	Withdrawn bool `json:"withdrawn"`
	// NewComment 表示是否有新留言。
	NewComment bool `json:"newComment"`
	// CreatedAt 表示UTC 创建时间。
	CreatedAt string `json:"createdAt"`
}

// MineResult 职责：承载我的星星分页；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type MineResult struct {
	// Page 表示页码。
	Page int `json:"page"`
	// PageSize 表示分页大小。
	PageSize int `json:"pageSize"`
	// Total 表示总条数。
	Total int64 `json:"total"`
	// List 表示当前页的我的星星。
	List []MyStar `json:"list"`
}

// WriteStarResult 职责：承载发布或编辑回执；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type WriteStarResult struct {
	// StarID 表示写入的星星 ID。
	StarID string `json:"starId"`
	// X 表示横坐标。
	X float64 `json:"x"`
	// Y 表示纵坐标。
	Y float64 `json:"y"`
	// Z 表示发布或编辑后星体的盘面法向高度，旧服务端省略该字段时为 0。
	Z int `json:"z"`
	// ReviewState 表示当前审核状态。
	ReviewState string `json:"reviewState"`
	// BodyVersion 表示写入后的正文版本。
	BodyVersion string `json:"bodyVersion"`
	// Withdrawn 表示是否撤回。
	Withdrawn bool `json:"withdrawn"`
}

// ReplyResult 职责：承载留言或补充回执；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type ReplyResult struct {
	// ID 表示新发言 ID。
	ID string `json:"id"`
	// ReviewState 表示当前审核状态。
	ReviewState string `json:"reviewState"`
	// BodyVersion 表示写入后的正文版本。
	BodyVersion string `json:"bodyVersion"`
}

// LikeResult 职责：承载点赞回执；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type LikeResult struct {
	// Liked 表示服务端确认后的点赞状态。
	Liked bool `json:"liked"`
	// LikeCount 表示点赞总数。
	LikeCount int64 `json:"likeCount"`
}

// WithdrawResult 职责：承载星星撤回回执；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type WithdrawResult struct {
	// StarID 表示星星 ID。
	StarID string `json:"starId"`
	// Withdrawn 表示撤回后状态。
	Withdrawn bool `json:"withdrawn"`
}

// OKResult 职责：承载已读回执；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type OKResult struct {
	// OK 表示操作已接纳。
	OK bool `json:"ok"`
}

// Notice 职责：承载作者提醒；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type Notice struct {
	// ID 表示提醒 ID。
	ID string `json:"id"`
	// Kind 表示提醒类型。
	Kind string `json:"kind"`
	// StarID 表示关联星星 ID。
	StarID string `json:"starId"`
	// ContentID 表示关联内容 ID。
	ContentID string `json:"contentId"`
	// CreatedAt 表示UTC 创建时间。
	CreatedAt string `json:"createdAt"`
	// ReadAt 表示UTC 已读时间或 null。
	ReadAt *string `json:"readAt" ts_type:"string | null"`
}

// NoticesResult 职责：承载提醒分页；使用：绑定接收 HTTP 成功数据；边界：不包含账号凭据。
type NoticesResult struct {
	// Page 表示页码。
	Page int `json:"page"`
	// PageSize 表示分页大小。
	PageSize int `json:"pageSize"`
	// Total 表示总条数。
	Total int64 `json:"total"`
	// List 表示当前页作者提醒。
	List []Notice `json:"list"`
	// UnreadCount 表示未读提醒总数。
	UnreadCount int64 `json:"unreadCount"`
}
