package system

// 文件职责：定义应用账号服务的外部接口路径与请求/响应契约。
// 使用：helper/system/account 通过 core.GLOBAL.ServerClient 请求这些接口；service/system/AccountService 把响应 DTO 返回给前端。
// 边界：只描述接口形状，不发起 HTTP 请求、不处理 token；字段与行为以 docs/ex/auth-account-api.md 为准。

import "encoding/json"

// 账号服务接口路径常量组。
// 职责：集中保存账号服务相对路径，避免在 helper/service 中出现魔法字符串。
// 使用：helper/system/account 构造请求时引用，由 GLOBAL.ServerClient 拼接服务器基础地址。
// 边界：只保存路径，不决定请求方法、鉴权与错误处理策略。
const (
	// APIURLCaptcha 是获取验证码的接口路径（POST，公开）。
	// 职责：描述 `POST /base/captcha` 接口的相对路径。
	// 使用：账号验证码获取与服务器连接测试请求该接口。
	// 边界：接口无需鉴权且不会修改账号数据。
	APIURLCaptcha = "/base/captcha"
	// APIURLRegister 是注册账号的接口路径（POST，公开）。
	// 职责：描述 `POST /base/register` 接口的相对路径。
	// 使用：AccountService.Register 调用。
	// 边界：注册成功不会签发 token。
	APIURLRegister = "/base/register"
	// APIURLLogin 是登录的接口路径（POST，公开）。
	// 职责：描述 `POST /base/login` 接口的相对路径。
	// 使用：AccountService.Login 调用。
	// 边界：成功响应包含 token，由后端保存，不返回给前端。
	APIURLLogin = "/base/login"
	// APIURLGetUserInfo 是获取当前用户的接口路径（GET，需 x-token）。
	// 职责：描述 `GET /user/getUserInfo` 接口的相对路径。
	// 使用：AccountService.GetCurrentUser 调用。
	// 边界：只返回 token 对应的用户。
	APIURLGetUserInfo = "/user/getUserInfo"
	// APIURLSetSelfInfo 是修改个人资料的接口路径（PUT，需 x-token）。
	// 职责：描述 `PUT /user/setSelfInfo` 接口的相对路径。
	// 使用：AccountService.SetSelfInfo 调用。
	// 边界：仅更新四个资料字段；省略或 null 保持原值，空字符串可清除头像和联系方式。
	APIURLSetSelfInfo = "/user/setSelfInfo"
	// APIURLUploadAvatar 职责：描述头像上传并持久化的接口；使用：AccountService.UploadAvatar 发起 POST multipart file；边界：必须携带 x-token，成功返回 data.user。
	APIURLUploadAvatar = "/user/uploadAvatar"
	// APIURLSetSelfSetting 是保存个人界面配置的接口路径（PUT，需 x-token）。
	// 职责：描述 `PUT /user/setSelfSetting` 接口的相对路径。
	// 使用：AccountService.SetSelfSetting 调用。
	// 边界：服务端整体覆盖 originSetting，调用方负责合并。
	APIURLSetSelfSetting = "/user/setSelfSetting"
	// APIURLChangePassword 是修改密码的接口路径（POST，需 x-token）。
	// 职责：描述 `POST /user/changePassword` 接口的相对路径。
	// 使用：AccountService.ChangePassword 调用。
	// 边界：成功后应注销并重新登录。
	APIURLChangePassword = "/user/changePassword"
	// APIURLLogout 是注销 token 的接口路径（POST，需 x-token）。
	// 职责：描述 `POST /jwt/jsonInBlacklist` 接口的相对路径。
	// 使用：AccountService.Logout 调用。
	// 边界：无论接口结果如何，调用方都应清除本地 token。
	APIURLLogout = "/jwt/jsonInBlacklist"
)

// 账号服务约定的业务状态码与响应头常量组。
// 职责：描述统一响应结构中的成功码、强制改密码以及 token 相关请求/响应头。
// 使用：helper/system/account 解析响应与附加鉴权头时读取。
// 边界：只描述协议约定，不决定错误展示文案。
const (
	// AccountCodeSuccess 职责：标记业务成功；使用：判断响应是否成功；边界：成功只看 code，不只看 HTTP 状态。
	AccountCodeSuccess = 0
	// AccountCodeNeedChangePassword 职责：标记需要先修改密码（HTTP 409）；使用：设置会话的强制改密状态；边界：此时只能调用改密、获取信息与注销。
	AccountCodeNeedChangePassword = 10001
	// AccountTokenHeader 职责：请求鉴权头名称；使用：已登录请求附加 token；边界：`/base/*` 公开接口不附加。
	AccountTokenHeader = "x-token"
	// AccountNewTokenHeader 职责：静默续期响应头名称；使用：收到后替换已保存 token；边界：只在剩余有效期不足时出现。
	AccountNewTokenHeader = "new-token"
)

// 账号会话状态常量组。
// 职责：描述客户端会话快照的阶段，供前端全局账号状态和界面分支使用。
// 使用：AccountSessionState.Status 与 AccountSessionSnapshot 返回时读取。
// 边界：只描述客户端会话阶段，不替代服务端业务码；前端不得仅凭状态字符串推断用户权限。
const (
	// AccountSessionStatusInitializing 职责：客户端基础设施尚未就绪；使用：启动早期快照；边界：此时不能读取 token 或请求服务器。
	AccountSessionStatusInitializing = "initializing"
	// AccountSessionStatusAnonymous 职责：没有已保存 token；使用：未登录界面；边界：不代表服务器不可达。
	AccountSessionStatusAnonymous = "anonymous"
	// AccountSessionStatusPending 职责：已保存 token 但尚未完成用户验证；使用：等待统一同步；边界：公告、翻译等已验证能力不可用。
	AccountSessionStatusPending = "pending"
	// AccountSessionStatusAuthenticated 职责：token 已被服务器接受且用户资料可用；使用：已登录界面与已验证能力；边界：不保证 token 永久有效。
	AccountSessionStatusAuthenticated = "authenticated"
	// AccountSessionStatusPasswordRequired 职责：token 有效但服务端要求先修改密码；使用：强制改密界面；边界：此时保留 token，不按退出登录处理。
	AccountSessionStatusPasswordRequired = "passwordRequired"
	// AccountSessionStatusExpired 职责：服务器拒绝了原 token 且后端已清除本地会话；使用：提示重新登录；边界：必须与网络失败区分。
	AccountSessionStatusExpired = "expired"
	// AccountSessionStatusOffline 职责：token 仍保留但本次验证因网络或服务器失败未完成；使用：保留登录态并允许重试；边界：不得据此清除 token。
	AccountSessionStatusOffline = "offline"
)

// AccountResponse 是账号服务统一响应外壳 `{code,data,msg}`。
// 职责：承载业务状态码、原始数据与提示消息。
// 使用：helper/system/account 解码响应后按 code 判断成功，再把 Data 解码到具体 DTO。
// 边界：Data 保持原始 JSON，失败响应（如 401 的 null、409 的改密标志）不按成功结构解码。
type AccountResponse struct {
	// Code 职责：业务状态码；使用：0 表示成功；边界：HTTP 200 但 code 非 0 仍为失败。
	Code int `json:"code"`
	// Data 职责：业务数据原文；使用：成功时解码为具体 DTO；边界：失败或无数据接口可能为 null。
	Data json.RawMessage `json:"data"`
	// Msg 职责：服务端提示消息；使用：失败时直接展示给用户；边界：成功时仅作参考。
	Msg string `json:"msg"`
}

// AccountCaptcha 是验证码接口返回的数据。
// 职责：描述验证码图片、ID、位数与登录是否需要验证码。
// 使用：前端账号弹窗展示验证码并在登录/注册时回传 CaptchaID。
// 边界：验证码一次性，提交校验一次即作废。
type AccountCaptcha struct {
	// CaptchaID 职责：验证码 ID；使用：登录/注册提交时回传；边界：约 180 秒过期。
	CaptchaID string `json:"captchaId"`
	// PicPath 职责：验证码图片 data URL；使用：直接作为 img src；边界：不落盘。
	PicPath string `json:"picPath"`
	// CaptchaLength 职责：验证码位数；使用：限制输入框长度；边界：纯数字。
	CaptchaLength int `json:"captchaLength"`
	// OpenCaptcha 职责：登录是否需要验证码；使用：前端决定登录表单是否显示验证码；边界：注册始终需要验证码。
	OpenCaptcha bool `json:"openCaptcha"`
}

// AccountUser 是服务端 SysUser 中客户端需要的字段。
// 职责：描述当前用户的身份与资料信息。
// 使用：登录、注册与获取当前用户接口返回给前端展示。
// 边界：只声明客户端使用的字段，其余字段在解码时忽略；服务端不会返回 password。
type AccountUser struct {
	// ID 职责：用户主键；使用：标识当前用户；边界：服务端字段名为大写 ID。
	ID uint `json:"ID"`
	// UUID 职责：用户 UUID；使用：展示或关联；边界：可为空。
	UUID string `json:"uuid"`
	// UserName 职责：登录名；使用：展示账号名；边界：注意与登录请求的 username 大小写不同。
	UserName string `json:"userName"`
	// NickName 职责：昵称；使用：界面优先展示；边界：可为空。
	NickName string `json:"nickName"`
	// HeaderImg 职责：头像 URL；使用：展示头像；边界：可为空。
	HeaderImg string `json:"headerImg"`
	// Phone 职责：手机号；使用：资料展示；边界：可为空。
	Phone string `json:"phone"`
	// Email 职责：邮箱；使用：资料展示；边界：可为空。
	Email string `json:"email"`
	// AuthorityID 职责：当前角色 ID；使用：诊断权限问题；边界：注册用户为 1001。
	AuthorityID uint `json:"authorityId"`
	// Enable 职责：账号状态；使用：1 正常，2 冻结；边界：不在客户端做权限判断。
	Enable int `json:"enable"`
	// OriginSetting 职责：个人界面配置；使用：读取后合并再整体保存；边界：可为 null。
	OriginSetting map[string]interface{} `json:"originSetting"`
	// PasswordUpdatedAt 职责：密码最后修改时间文本；使用：资料展示；边界：可为 null，不解析时间格式。
	PasswordUpdatedAt *string `json:"passwordUpdatedAt"`
}

// AccountRegisterRequest 是注册接口请求体。
// 职责：描述注册所需用户名、密码与验证码。
// 使用：前端经 AccountService.Register 传入，后端原样转发。
// 边界：格式校验以服务端为准，前端本地校验只做提前提示。
type AccountRegisterRequest struct {
	// Username 职责：用户名；使用：注册登录名；边界：字段为全小写 username。
	Username string `json:"username"`
	// Password 职责：密码；使用：注册密码；边界：不记录日志。
	Password string `json:"password"`
	// Captcha 职责：验证码；使用：服务端校验；边界：必填。
	Captcha string `json:"captcha"`
	// CaptchaID 职责：验证码 ID；使用：服务端校验；边界：必填。
	CaptchaID string `json:"captchaId"`
	// NickName 职责：昵称；使用：缺省等于用户名；边界：可选。
	NickName string `json:"nickName,omitempty"`
}

// AccountRegisterData 是注册接口返回的数据。
// 职责：承载新建用户信息。
// 使用：helper 解码注册响应。
// 边界：注册成功不签发 token。
type AccountRegisterData struct {
	// User 职责：新建用户；使用：返回给前端；边界：不代表已登录。
	User AccountUser `json:"user"`
}

// AccountAvatarData 职责：承载头像提交后的当前用户；使用：解码上传响应 data.user；边界：缺少 user 视为协议错误。
type AccountAvatarData struct {
	// User 职责：服务器已保存的用户；使用：同步全局资料；边界：不得用本地图片地址替代。
	User AccountUser `json:"user"`
}

// AccountLoginRequest 是登录接口请求体。
// 职责：描述登录所需用户名、密码与可选验证码。
// 使用：前端经 AccountService.Login 传入，后端原样转发。
// 边界：openCaptcha 为 false 时验证码字段可省略。
type AccountLoginRequest struct {
	// Username 职责：用户名；使用：登录名；边界：字段为全小写 username。
	Username string `json:"username"`
	// Password 职责：密码；使用：登录密码；边界：不记录日志。
	Password string `json:"password"`
	// Captcha 职责：验证码；使用：服务端校验；边界：可选。
	Captcha string `json:"captcha,omitempty"`
	// CaptchaID 职责：验证码 ID；使用：服务端校验；边界：可选。
	CaptchaID string `json:"captchaId,omitempty"`
}

// AccountLoginData 是登录接口返回的原始数据。
// 职责：承载用户、token、过期时间与强制改密标志。
// 使用：helper 解码登录响应后保存 token，并转换为不含 token 的 AccountLoginResult 返回前端。
// 边界：Token 不离开后端。
type AccountLoginData struct {
	// User 职责：登录用户；使用：返回给前端；边界：无。
	User AccountUser `json:"user"`
	// Token 职责：会话 token；使用：后端保存并随后续请求发送；边界：不返回给前端。
	Token string `json:"token"`
	// ExpiresAt 职责：token 过期毫秒时间戳；使用：返回给前端参考；边界：单位是毫秒。
	ExpiresAt int64 `json:"expiresAt"`
	// NeedChangePassword 职责：是否需要先修改密码；使用：前端切换到改密模式；边界：为 true 时大部分接口返回 409。
	NeedChangePassword bool `json:"needChangePassword"`
}

// AccountLoginResult 是返回给前端的登录结果。
// 职责：在不暴露 token 的前提下告知登录用户、过期时间与强制改密状态。
// 使用：AccountService.Login 返回。
// 边界：token 由后端持有。
type AccountLoginResult struct {
	// User 职责：登录用户；使用：前端展示；边界：无。
	User AccountUser `json:"user"`
	// ExpiresAt 职责：token 过期毫秒时间戳；使用：前端参考；边界：静默续期后可能变化。
	ExpiresAt int64 `json:"expiresAt"`
	// NeedChangePassword 职责：是否需要先修改密码；使用：前端切换到改密模式；边界：无。
	NeedChangePassword bool `json:"needChangePassword"`
	// Session 职责：登录完成后的本地会话快照；使用：前端据此一次性更新已验证主体、用户和改密状态；边界：不包含 token。
	Session AccountSessionState `json:"session"`
}

// AccountCurrentUserData 是获取当前用户接口返回的数据。
// 职责：承载 userInfo 外壳。
// 使用：helper 解码当前用户响应。
// 边界：无。
type AccountCurrentUserData struct {
	// UserInfo 职责：当前用户；使用：返回给前端；边界：无。
	UserInfo AccountUser `json:"userInfo"`
}

// AccountSelfInfoRequest 是修改个人资料接口请求体。
// 职责：描述可修改的资料字段。
// 使用：前端经 AccountService.SetSelfInfo 传入。
// 边界：省略或 null 保持原值；显式空字符串清除头像、手机、邮箱，昵称不能清空；不接受身份或权限字段。
type AccountSelfInfoRequest struct {
	// NickName 职责：昵称；使用：更新资料；边界：可选。
	NickName *string `json:"nickName,omitempty"`
	// HeaderImg 职责：头像 URL；使用：更新资料；边界：可选。
	HeaderImg *string `json:"headerImg,omitempty"`
	// Phone 职责：手机号；使用：更新资料；边界：可选。
	Phone *string `json:"phone,omitempty"`
	// Email 职责：邮箱；使用：更新资料；边界：可选。
	Email *string `json:"email,omitempty"`
}

// AccountChangePasswordRequest 是修改密码接口请求体。
// 职责：描述旧密码与新密码。
// 使用：前端经 AccountService.ChangePassword 传入。
// 边界：不记录日志。
type AccountChangePasswordRequest struct {
	// Password 职责：当前密码；使用：服务端校验；边界：必填。
	Password string `json:"password"`
	// NewPassword 职责：新密码；使用：服务端按安全策略校验；边界：必填。
	NewPassword string `json:"newPassword"`
}

// AccountSessionState 是返回给前端的本地会话状态。
// 职责：汇总基础设施就绪、token、用户验证、强制改密、账号身份和缓存用户资料，作为前端全局账号 store 的唯一数据源。
// 使用：AccountService.GetSessionState 返回本地快照，AccountService.SyncSession 返回远端验证后的完整快照。
// 边界：不包含 token；Authenticated 只表示后端保存了 token，Verified 才表示 token 已被服务器接受且用户资料可用。
type AccountSessionState struct {
	// Ready 职责：数据库配置与服务器客户端是否已初始化；使用：前端启动引导判断是否可发起验证；边界：false 时其余字段可能都是默认值。
	Ready bool `json:"ready"`
	// Authenticated 职责：是否持有 token；使用：前端展示登录态；边界：401 或退出后会被清除。
	Authenticated bool `json:"authenticated"`
	// Verified 职责：token 是否已被服务器接受且已取得用户身份；使用：公告、翻译和联机等已验证能力判断可用性；边界：网络失败时可能为 false，但 token 仍保留。
	Verified bool `json:"verified"`
	// NeedChangePassword 职责：是否需要先修改密码；使用：前端切换到改密模式；边界：只在本进程内有效，重启后由服务端 409 再次告知。
	NeedChangePassword bool `json:"needChangePassword"`
	// Status 职责：会话阶段，取值为 AccountSessionStatus* 常量；使用：前端统一分支展示；边界：只描述本地快照阶段，不替代服务端业务码。
	Status string `json:"status"`
	// SubjectID、Generation、ServerKey 职责：标识已验证会话；使用：本机公告请求绑定与前端作用域判断；边界：不包含 token。
	SubjectID  string `json:"subjectId"`
	Generation string `json:"generation"`
	ServerKey  string `json:"serverKey"`
	// User 职责：后端缓存的当前已验证用户资料；使用：前端同步后立即渲染头像与昵称；边界：只在本进程内有效，重启后需由 SyncSession 重新获取，不包含凭据。
	User *AccountUser `json:"user,omitempty"`
}

// AccountSessionSnapshot 是统一会话同步结果。
// 职责：一次返回会话状态、缓存用户资料和可读失败原因，替代前端把恢复拆成两次独立调用。
// 使用：AccountService.SyncSession 返回给前端全局账号 store；启动引导、弹窗打开、登录和资料刷新统一读取。
// 边界：不包含 token；Message 只用于前端提示，鉴权结论以 Session.Status、Authenticated 和 Verified 为准。
type AccountSessionSnapshot struct {
	// Session 职责：同步后的会话快照；使用：前端原子替换全局账号状态；边界：网络失败时可能保留 authenticated=true 且 verified=false。
	Session AccountSessionState `json:"session"`
	// Message 职责：本次同步的失败或提示文本；使用：前端展示离线、改密或过期原因；边界：成功且无需提示时为空字符串。
	Message string `json:"message,omitempty"`
}
