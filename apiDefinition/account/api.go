package account

// 文件职责：定义账号自助流程使用的国内服务器接口路径。
// 使用：AccountService 通过这些相对路径调用与 Mod 市场相同的 System/BaseURL 服务。
// 边界：只描述远端契约路径，不发送请求、不保存登录态、不包含管理员账号管理接口。

const (
	// APIURLCaptcha 职责：保存登录验证码接口路径；使用：登录前刷新验证码时调用；边界：无需鉴权且不处理验证码展示。
	APIURLCaptcha = "/base/captcha"
	// APIURLLogin 职责：保存账号登录接口路径；使用：个人页提交用户名、密码和验证码时调用；边界：不在此处保存 JWT。
	APIURLLogin = "/base/login"
	// APIURLUserInfo 职责：保存当前用户资料读取接口路径；使用：登录后和个人页初始化时调用；边界：需要有效 JWT。
	APIURLUserInfo = "/user/getUserInfo"
	// APIURLSetSelfInfo 职责：保存当前用户资料更新接口路径；使用：个人页保存昵称、邮箱、手机号和头像时调用；边界：不允许更新其他用户资料。
	APIURLSetSelfInfo = "/user/setSelfInfo"
	// APIURLChangePassword 职责：保存当前用户改密接口路径；使用：强制改密和账号安全设置调用；边界：确认密码只在客户端校验，不能提交给服务端。
	APIURLChangePassword = "/user/changePassword"
	// APIURLLogout 职责：保存当前 JWT 注销接口路径；使用：用户主动退出时调用；边界：服务端请求失败时仍由服务层清理本地状态。
	APIURLLogout = "/jwt/jsonInBlacklist"
	// APIURLUpload 职责：保存头像文件上传接口路径；使用：个人页选择头像后先上传文件；边界：只上传文件，不修改用户资料。
	APIURLUpload = "/fileUploadAndDownload/upload"
)
