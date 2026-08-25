package account

// 文件职责：定义账号自助接口的请求、响应和 Wails 传输 DTO。
// 使用：AccountService 解析国内服务器响应并向个人页返回稳定数据结构时引用。
// 边界：只表达数据形状，不发送 HTTP 请求、不持久化令牌、不决定页面交互。

// APIResponse 表示国内服务器统一业务响应外壳。
// 职责：承载业务码、消息和泛型数据字段。
// 使用：AccountService 解码验证码、登录、资料和上传接口响应时使用。
// 边界：HTTP 状态码判定仍由服务层负责，Code 非零不应被调用方视为成功。
type APIResponse[T any] struct {
	// Code 职责：保存服务器业务状态码；使用：服务层判定请求是否成功；边界：HTTP 成功不代表该字段必然为零。
	Code int `json:"code"`
	// Data 职责：保存接口成功时的数据；使用：服务层转换为前端 DTO；边界：失败响应中的零值不可作为有效业务数据。
	Data T `json:"data"`
	// Msg 职责：保存服务器消息；使用：服务层向前端返回可读错误；边界：不包含本地网络错误细节。
	Msg string `json:"msg"`
}

// CaptchaData 表示登录验证码数据。
// 职责：保存验证码标识、图片和是否需要验证的开关。
// 使用：个人页根据 OpenCaptcha 决定验证码控件是否展示。
// 边界：PicPath 可为 data URL，客户端不得尝试解析或存储验证码答案。
type CaptchaData struct {
	CaptchaID     string `json:"captchaId"`
	PicPath       string `json:"picPath"`
	CaptchaLength int    `json:"captchaLength"`
	OpenCaptcha   bool   `json:"openCaptcha"`
}

// LoginRequest 表示登录请求体。
// 职责：承载服务器要求的用户名、密码和验证码字段。
// 使用：个人页提交登录表单时传给 AccountService。
// 边界：验证码未开启时验证码字段可以为空，密码不得记录到日志或本地配置。
type LoginRequest struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	CaptchaID string `json:"captchaId"`
	Captcha   string `json:"captcha"`
}

// ChangePasswordRequest 表示当前用户改密请求体。
// 职责：承载当前密码与新密码。
// 使用：强制改密和个人页安全设置通过 AccountService 调用。
// 边界：确认密码仅由前端校验，不能放入该远端请求 DTO。
type ChangePasswordRequest struct {
	Password    string `json:"password"`
	NewPassword string `json:"newPassword"`
}

// SetSelfInfoRequest 表示当前用户可自行修改的资料字段。
// 职责：承载昵称、联系方式和头像地址。
// 使用：个人页保存资料或上传头像成功后调用。
// 边界：不包含角色、启用状态和其他用户 ID 等管理员字段。
type SetSelfInfoRequest struct {
	NickName  string `json:"nickName"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	HeaderImg string `json:"headerImg"`
}

// UserInfo 表示个人页使用的当前用户资料。
// 职责：保存登录、资料展示与鉴权所需的用户基础字段。
// 使用：服务端用户对象和用户信息接口响应都会解码为该结构。
// 边界：未声明的角色、部门和岗位复杂字段由后端保留，个人页不依赖其具体结构。
type UserInfo struct {
	ID          uint   `json:"id"`
	UUID        string `json:"uuid"`
	UserName    string `json:"userName"`
	NickName    string `json:"nickName"`
	HeaderImg   string `json:"headerImg"`
	AuthorityID uint   `json:"authorityId"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Enable      int    `json:"enable"`
}

// LoginData 表示登录成功返回的数据。
// 职责：承载用户资料、JWT、过期时间和强制改密状态。
// 使用：AccountService 保存登录态后将安全的 AccountSession 返回个人页。
// 边界：Token 只在服务端本地配置中保存，绝不通过 Wails 返回给前端。
type LoginData struct {
	User               UserInfo `json:"user"`
	Token              string   `json:"token"`
	ExpiresAt          int64    `json:"expiresAt"`
	NeedChangePassword bool     `json:"needChangePassword"`
}

// UserInfoData 表示获取当前用户信息接口的数据。
// 职责：包装最新用户资料。
// 使用：AccountService 每次刷新资料时从 UserInfo 字段读取。
// 边界：资料不存在时由服务层返回明确错误。
type UserInfoData struct {
	UserInfo UserInfo `json:"userInfo"`
}

// UploadData 表示文件上传响应的数据。
// 职责：包装服务器生成的文件信息。
// 使用：个人页头像上传成功后取 File.URL 保存到资料。
// 边界：上传成功不代表头像资料已更新，仍需调用 setSelfInfo。
type UploadData struct {
	File UploadedFile `json:"file"`
}

// UploadedFile 表示上传后返回的文件核心信息。
// 职责：保存可提交给用户资料接口的文件 URL。
// 使用：AccountService UploadAvatar 返回该 URL 给前端。
// 边界：不负责拼接 BaseURL，服务器相对路径按原样提交回服务器。
type UploadedFile struct {
	URL string `json:"url"`
}

// AccountSession 表示不暴露 JWT 的本地登录状态。
// 职责：向个人页提供当前登录、过期和强制改密状态及缓存用户资料。
// 使用：页面挂载时决定进入登录、改密或资料视图。
// 边界：不包含令牌、密码或验证码，缓存资料仍应由 GetCurrentUserInfo 刷新。
type AccountSession struct {
	Authenticated      bool     `json:"authenticated"`
	ExpiresAt          int64    `json:"expiresAt"`
	NeedChangePassword bool     `json:"needChangePassword"`
	User               UserInfo `json:"user"`
}
