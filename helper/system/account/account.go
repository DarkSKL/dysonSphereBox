package account

// 文件职责：实现账号认证与资料相关的后端流程。
// 使用：service/system/AccountService 调用这些函数，前端账号弹窗经 Wails 绑定间接使用。
// 边界：只编排接口调用与会话维护；输入格式以服务端校验为准，不在这里重复密码策略。

import (
	"context"
	"fmt"
	"net/http"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// Captcha 获取一次性验证码。
// 职责：请求公开验证码接口。
// 使用：账号弹窗打开、切换模式或提交失败后刷新验证码。
// 边界：不携带 token。
func Captcha(ctx context.Context) (*systemAPI.AccountCaptcha, error) {
	captcha, err := request[systemAPI.AccountCaptcha](ctx, http.MethodPost, systemAPI.APIURLCaptcha, nil, false)
	if err != nil {
		return nil, err
	}
	return &captcha, nil
}

// Register 注册账号。
// 职责：转发注册请求并返回新建用户。
// 使用：账号弹窗注册模式提交时调用。
// 边界：注册成功不会登录，也不保存 token。
func Register(ctx context.Context, payload systemAPI.AccountRegisterRequest) (*systemAPI.AccountUser, error) {
	data, err := request[systemAPI.AccountRegisterData](ctx, http.MethodPost, systemAPI.APIURLRegister, payload, false)
	if err != nil {
		return nil, err
	}
	return &data.User, nil
}

// Login 登录并保存 token。
// 职责：转发登录请求，把 token 保存在后端并返回不含 token 的登录结果。
// 使用：账号弹窗登录模式提交时调用。
// 边界：token 保存失败视为登录失败，避免前端显示已登录但后续请求无 token。
func Login(ctx context.Context, payload systemAPI.AccountLoginRequest) (*systemAPI.AccountLoginResult, error) {
	data, err := request[systemAPI.AccountLoginData](ctx, http.MethodPost, systemAPI.APIURLLogin, payload, false)
	if err != nil {
		return nil, err
	}
	if err := SaveToken(data.Token); err != nil {
		return nil, fmt.Errorf("保存登录状态失败：%w", err)
	}
	setNeedChangePassword(data.NeedChangePassword)

	return &systemAPI.AccountLoginResult{
		User:               data.User,
		ExpiresAt:          data.ExpiresAt,
		NeedChangePassword: data.NeedChangePassword,
	}, nil
}

// CurrentUser 获取当前登录用户。
// 职责：携带 token 请求当前用户信息。
// 使用：启动恢复会话、登录后拉取资料时调用。
// 边界：401 时会话已在 request 中清除。
func CurrentUser(ctx context.Context) (*systemAPI.AccountUser, error) {
	data, err := request[systemAPI.AccountCurrentUserData](ctx, http.MethodGet, systemAPI.APIURLGetUserInfo, nil, true)
	if err != nil {
		return nil, err
	}
	return &data.UserInfo, nil
}

// SetSelfInfo 修改当前用户资料。
// 职责：转发资料修改请求。
// 使用：账号资料编辑时调用。
// 边界：空字段被服务端忽略；需要最新资料时由调用方重新获取。
func SetSelfInfo(ctx context.Context, payload systemAPI.AccountSelfInfoRequest) error {
	_, err := request[struct{}](ctx, http.MethodPut, systemAPI.APIURLSetSelfInfo, payload, true)
	return err
}

// SetSelfSetting 覆盖保存个人界面配置。
// 职责：转发个人设置保存请求。
// 使用：需要保存账号级界面配置时调用。
// 边界：服务端整体覆盖，调用方负责先合并。
func SetSelfSetting(ctx context.Context, settings map[string]interface{}) error {
	if settings == nil {
		settings = map[string]interface{}{}
	}
	_, err := request[struct{}](ctx, http.MethodPut, systemAPI.APIURLSetSelfSetting, settings, true)
	return err
}

// ChangePassword 修改当前账号密码。
// 职责：转发改密请求，成功后清除强制改密状态。
// 使用：账号弹窗改密模式提交时调用。
// 边界：不自动注销；调用方按接口约定随后调用 Logout。
func ChangePassword(ctx context.Context, payload systemAPI.AccountChangePasswordRequest) error {
	if _, err := request[struct{}](ctx, http.MethodPost, systemAPI.APIURLChangePassword, payload, true); err != nil {
		return err
	}
	setNeedChangePassword(false)
	return nil
}

// Logout 注销当前 token。
// 职责：请求服务端作废 token，并无论结果如何清除本地会话。
// 使用：用户退出登录或改密后调用。
// 边界：未持有 token 时只清除本地状态；服务端请求失败时仍返回该错误供界面提示。
func Logout(ctx context.Context) error {
	var requestErr error
	if Token() != "" {
		_, requestErr = request[struct{}](ctx, http.MethodPost, systemAPI.APIURLLogout, nil, true)
	}
	if err := ClearSession(); err != nil && requestErr == nil {
		return fmt.Errorf("清除登录状态失败：%w", err)
	}
	return requestErr
}
