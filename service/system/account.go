package system

// 文件职责：提供账号认证与资料相关的 Wails 服务。
// 使用：前端账号弹窗与主布局通过生成绑定调用；请求由客户端后端经 core.GLOBAL.ServerClient 发往应用服务器。
// 边界：只暴露账号能力并编排 helper；token 由后端保存，不返回给前端，前端不直接请求服务器。

import (
	"context"

	systemAPI "DysonSphereBox/apiDefinition/system"
	accountHelper "DysonSphereBox/helper/system/account"
)

// AccountService 提供前端可调用的账号能力。
// 职责：转发验证码、注册、登录、资料、改密与注销请求，并暴露本地会话状态。
// 使用：RegisterServices 绑定给前端 services/account.ts。
// 边界：注册时不访问数据库或网络；服务器地址来自全局 HTTP 客户端。
type AccountService struct{}

// GetCaptcha 获取一次性验证码。
// 职责：返回验证码图片、ID 与登录是否需要验证码。
// 使用：账号弹窗打开、切换模式或提交后刷新。
// 边界：公开接口，不携带 token。
func (service *AccountService) GetCaptcha() (*systemAPI.AccountCaptcha, error) {
	return accountHelper.Captcha(context.Background())
}

// Register 注册账号。
// 职责：创建账号并返回新用户。
// 使用：账号弹窗注册提交时调用。
// 边界：注册成功不会自动登录。
func (service *AccountService) Register(payload systemAPI.AccountRegisterRequest) (*systemAPI.AccountUser, error) {
	return accountHelper.Register(context.Background(), payload)
}

// Login 登录账号。
// 职责：登录并在后端保存 token，返回用户与强制改密状态。
// 使用：账号弹窗登录提交时调用。
// 边界：返回值不包含 token。
func (service *AccountService) Login(payload systemAPI.AccountLoginRequest) (*systemAPI.AccountLoginResult, error) {
	return accountHelper.Login(context.Background(), payload)
}

// GetCurrentUser 获取当前登录用户。
// 职责：用已保存 token 拉取用户资料。
// 使用：启动恢复会话、登录后刷新资料时调用。
// 边界：token 失效（401）时后端清除会话并返回错误。
func (service *AccountService) GetCurrentUser() (*systemAPI.AccountUser, error) {
	return accountHelper.CurrentUser(context.Background())
}

// SetSelfInfo 修改当前用户资料。
// 职责：更新昵称、头像、手机、邮箱等非空字段。
// 使用：账号资料编辑时调用。
// 边界：空字符串字段被服务端忽略。
func (service *AccountService) SetSelfInfo(payload systemAPI.AccountSelfInfoRequest) error {
	return accountHelper.SetSelfInfo(context.Background(), payload)
}

// SetSelfSetting 覆盖保存个人界面配置。
// 职责：整体保存 originSetting。
// 使用：需要保存账号级界面配置时调用。
// 边界：调用方负责先读取并合并。
func (service *AccountService) SetSelfSetting(settings map[string]interface{}) error {
	return accountHelper.SetSelfSetting(context.Background(), settings)
}

// ChangePassword 修改当前账号密码。
// 职责：校验旧密码并设置新密码。
// 使用：账号弹窗改密提交时调用。
// 边界：成功后调用方应调用 Logout 并重新登录。
func (service *AccountService) ChangePassword(payload systemAPI.AccountChangePasswordRequest) error {
	return accountHelper.ChangePassword(context.Background(), payload)
}

// Logout 注销当前账号。
// 职责：作废服务端 token 并清除本地会话。
// 使用：用户退出登录或改密后调用。
// 边界：服务端请求失败时本地会话仍已清除。
func (service *AccountService) Logout() error {
	return accountHelper.Logout(context.Background())
}

// GetSessionState 返回本地会话状态。
// 职责：告知前端是否持有 token 与是否需要先修改密码。
// 使用：前端每次账号调用后同步登录态。
// 边界：不访问网络。
func (service *AccountService) GetSessionState() systemAPI.AccountSessionState {
	return accountHelper.SessionState()
}
