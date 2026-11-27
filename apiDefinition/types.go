package apiDefinition

// 文件职责：声明盒子服务器接口及其请求策略。
// 使用：业务模块定义接口，客户端加载器将声明装配为执行计划。
// 边界：声明中的路由相对于盒子服务器，调用参数、凭据和响应只在运行时产生。

import "time"

// AuthMode 职责：选择认证门槛；使用：接口声明明确选取；边界：零值表示未声明。
type AuthMode uint8

// 认证门槛分别服务公开接口、账号验证维护和已验证业务接口。
const (
	AuthNone AuthMode = iota + 1
	AuthToken
	AuthVerified
)

// SessionAction 职责：声明账号请求的会话副作用；使用：登录、验证、改密及注销；边界：普通接口使用零值。
type SessionAction uint8

// 账号专属动作由会话提供者处理，运行计划只核对声明及快照是否匹配。
const (
	SessionUnchanged SessionAction = iota
	SessionLogin
	SessionVerify
	SessionChangePassword
	SessionLogout
)

// CacheMode 职责：选择结果复用方式；使用：缓存计划；边界：零值关闭结果缓存。
type CacheMode uint8

// HTTPCache 遵循响应缓存指令，ViewSnapshot 仅用于显式声明的同会话业务视图。
const (
	CacheOff CacheMode = iota
	HTTPCache
	ViewSnapshot
)

// CacheSource 职责：绑定使当前查询失效的写接口；使用：读接口声明 HitSource；边界：MatchTarget 为真时匹配调用的 Target。
type CacheSource struct {
	API         string
	MatchTarget bool
}

// CachePolicy 职责：配置结果复用及写后失效；使用：加载器装配内存查询；边界：TTL 是复用时间上限。
type CachePolicy struct {
	Mode      CacheMode
	TTL       time.Duration
	HitSource []CacheSource
}

// ResponseMode 职责：选择响应适配；使用：适配器选择有界解码或流式消费；边界：零值为统一 JSON 外壳。
type ResponseMode uint8

// 流式模式分别校验文件、Range 和 SSE 的握手响应。
const (
	JSONResponse ResponseMode = iota
	FileResponse
	RangeResponse
	SSEResponse
)

// EnvelopePolicy 职责：声明统一 JSON 外壳的接口差异；使用：有界解码和鉴权响应；边界：业务码必须存在。
type EnvelopePolicy struct {
	// RequireMessage、RequireData 描述必须出现的外壳字段。ObjectData 要求成功数据是对象。
	RequireMessage bool
	RequireData    bool
	ObjectData     bool
	// PasswordFlag 要求改密业务码附带明确的 needChangePassword 标志。
	PasswordFlag bool
	// RenewOnHTTPError 允许从非 200 的有效协议响应接纳凭据续期。
	RenewOnHTTPError bool
}

// MiddlewareID 职责：引用已经注册的扩展中间件；使用：接口声明执行顺序；边界：名称来自 Go 端注册表。
type MiddlewareID string

// ApiDesc 职责：携带接口级 HTTP、认证、缓存和中间件配置；使用：每个盒子服务器接口声明一个实例；边界：加载后视为只读。
type ApiDesc struct {
	ID            string `json:"id"`
	URL           string `json:"api"`
	Method        string `json:"method"`
	Description   string `json:"description"`
	Auth          AuthMode
	SessionAction SessionAction
	Cache         CachePolicy
	// ShareRequest 合并同会话的等价在途读，可独立于结果缓存使用。
	ShareRequest bool
	// Timeout 约束普通请求或文件流。UploadTimeout 为 multipart 流式请求单独设置时限。
	Timeout       time.Duration
	UploadTimeout time.Duration
	// RequestLimit 约束 JSON 请求体。ResponseLimit 约束外壳，零值采用 4 MiB 上限。
	RequestLimit  int
	ResponseLimit int
	Response      ResponseMode
	Envelope      EnvelopePolicy
	// BeforeMiddleware 在请求参数归一化阶段执行。AfterMiddleware 只观察真实且校验成功的结果。
	BeforeMiddleware []MiddlewareID
	AfterMiddleware  []MiddlewareID
}
