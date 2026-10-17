package system

// 文件职责：描述客户端公告读取与手动标读的HTTP契约。
// 使用：公告helper解析server统一响应并按稳定JSON字段映射。
// 边界：只声明协议路径和数据结构，不发请求、不持有本地会话。

import "encoding/json"

// 公告客户端路径常量描述相对ServerAddress路径，保留部署prefix。
// 职责：集中定义公告客户端两条只读/确认接口。
// 使用：helper/system/announcement构造请求。
// 边界：不包含/api假设或管理后台接口。
const (
	// APIURLClientNoticeGetLatestUnread描述最新未读查询路径。
	APIURLClientNoticeGetLatestUnread = "/clientNotice/getLatestUnread"
	// APIURLClientNoticeGetHistory描述分页历史查询路径。
	APIURLClientNoticeGetHistory = "/clientNotice/getHistory"
	// APIURLClientNoticeMarkRead描述用户手动确认路径。
	APIURLClientNoticeMarkRead = "/clientNotice/markRead"
)

// ClientNoticeEnvelope承载公告统一响应外壳。
// 职责：保留data原文以便严格解码必需字段。
// 使用：公告请求解析HTTP响应。
// 边界：业务错误码由调用流程判断。
type ClientNoticeEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  string          `json:"msg"`
}

// ClientNoticeLatestData承载最新查询主体与server时间。
// 职责：描述一次查询的身份及可选唯一公告。
// 使用：GET成功响应解码。
// 边界：Item字段缺失与显式null需要由helper区分。
type ClientNoticeLatestData struct {
	SubjectID  string          `json:"subjectId"`
	ServerTime string          `json:"serverTime"`
	Item       json.RawMessage `json:"item"`
}

// ClientNoticeItem描述server返回的一条公告版本。
// 职责：承载正文渲染和版本确认所需字段。
// 使用：客户端公告校验与显示。
// 边界：正文源码仍需前端解析和严格清洗。
type ClientNoticeItem struct {
	ID            string  `json:"id"`
	Revision      int64   `json:"revision"`
	Kind          string  `json:"kind"`
	Title         string  `json:"title"`
	ContentFormat string  `json:"contentFormat"`
	Content       string  `json:"content"`
	PublishedAt   string  `json:"publishedAt"`
	StartsAt      *string `json:"startsAt"`
	EndsAt        *string `json:"endsAt"`
}

// ClientNoticeMarkReadRequest仅包含公告ID和版本。
// 职责：描述手动已查阅POST正文。
// 使用：公告服务按保存的Go会话句柄构造。
// 边界：不接受userId、token、客户端时间或正文。
type ClientNoticeMarkReadRequest struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}

// ClientNoticeMarkReadReceipt承载server确认的首次已读回执。
// 职责：用于核对身份、公告版本及server时间。
// 使用：POST成功响应解码。
// 边界：仅匹配当前展示句柄才算成功。
type ClientNoticeMarkReadReceipt struct {
	SubjectID   string `json:"subjectId"`
	ID          string `json:"id"`
	Revision    int64  `json:"revision"`
	FirstReadAt string `json:"firstReadAt"`
}

// ClientNoticeHistoryData承载分页历史查询主体与server时间。
// 职责：描述一次分页查询的身份、列表及分页元数据。
// 使用：GET成功响应解码。
// 边界：数组可为空，页码和总数由helper校验。
type ClientNoticeHistoryData struct {
	// SubjectID 返回当前令牌所属主体，客户端须与捕获的账号核对。
	SubjectID string `json:"subjectId"`
	// ServerTime 是该页服务端查询快照时间。
	ServerTime string `json:"serverTime"`
	// List 暂存原始条目，供helper区分必需字段缺失和显式null。
	List []json.RawMessage `json:"list"`
	// Total 是满足历史查询过滤条件的公告总条数。
	Total int64 `json:"total"`
	// Page 是服务器回显的当前页码。
	Page int64 `json:"page"`
	// PageSize 是服务器回显的分页数量，最大为100。
	PageSize int64 `json:"pageSize"`
}

// ClientNoticeHistoryEntry描述历史列表中的公告条目，包含已读状态。
// 职责：承载列表显示的公告摘要和已读标记。
// 使用：客户端历史列表展示。
// 边界：严格校验时间窗口和已读标记。
type ClientNoticeHistoryEntry struct {
	// ID 使用十进制字符串保留64位及以上数据库ID精度。
	ID string `json:"id"`
	// Revision 标记当前公告内容版本。
	Revision int64 `json:"revision"`
	// Kind 是公告类型，供客户端选择显示标签。
	Kind string `json:"kind"`
	// Title 是列表及详情标题。
	Title string `json:"title"`
	// ContentFormat 决定正文进入HTML或Markdown清洗流程。
	ContentFormat string `json:"contentFormat"`
	// Content 是未经信任的正文源码，前端显示前必须清洗。
	Content string `json:"content"`
	// PublishedAt 是服务端发布时刻。
	PublishedAt string `json:"publishedAt"`
	// StartsAt 为空表示发布后立即生效。
	StartsAt *string `json:"startsAt"`
	// EndsAt 为空表示不设到期时间，已过期的历史项仍可查看。
	EndsAt *string `json:"endsAt"`
	// State 是服务端发布状态，客户端只接受published。
	State string `json:"state"`
	// Read 表示当前账号是否已确认此版本。
	Read bool `json:"read"`
	// ReadAt 是当前账号该版本首次确认时间，未读时为空。
	ReadAt *string `json:"readAt"`
}
