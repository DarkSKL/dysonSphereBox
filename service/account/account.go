package account

// 文件职责：编排个人页的验证码、登录、资料、头像、改密和注销真实接口调用。
// 使用：个人页通过 Wails 绑定调用，服务复用 System/BaseURL 指向的国内服务器 HTTPClient。
// 边界：只处理当前用户自助账号流程，不暴露管理员账号接口，不向前端返回 JWT、密码或验证码原文。

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	accountDefinition "DysonSphereBox/apiDefinition/account"
	accountConstant "DysonSphereBox/constant/account"
	"DysonSphereBox/core"
)

// AccountService 提供当前用户的账号自助能力。
// 职责：以本地会话为边界编排国内服务器账号接口，并维护可恢复的登录状态。
// 使用：个人页登录、刷新资料、保存资料、上传头像、修改密码和退出登录时通过 Wails 调用。
// 边界：不提供管理员用户管理能力，不记录密码/JWT，不绕过服务器权限检查。
type AccountService struct{}

// GetCaptcha 获取服务器登录验证码。
// 职责：读取服务器当前验证码配置和图片数据。
// 使用：个人页首次进入登录态及用户点击刷新验证码时调用。
// 边界：验证码未开启时 OpenCaptcha 为 false；调用失败时不产生或伪造本地验证码。
func (service *AccountService) GetCaptcha() (accountDefinition.CaptchaData, error) {
	request, err := service.createRequest(http.MethodPost, accountDefinition.APIURLCaptcha, nil, false)
	if err != nil {
		return accountDefinition.CaptchaData{}, err
	}
	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return accountDefinition.CaptchaData{}, fmt.Errorf("获取验证码失败: %w", err)
	}
	defer response.Body.Close()
	var result accountDefinition.APIResponse[accountDefinition.CaptchaData]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return accountDefinition.CaptchaData{}, fmt.Errorf("解析验证码响应失败: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 {
		return accountDefinition.CaptchaData{}, service.responseError("获取验证码失败", response.StatusCode, result.Code, result.Msg)
	}
	return result.Data, nil
}

// Login 登录并保存当前用户会话。
// 职责：提交登录表单，保存服务器签发的 JWT、过期时间、强制改密状态和用户资料。
// 使用：个人页在客户端完成非空校验后调用，成功后根据返回的 AccountSession 切换视图。
// 边界：密码和 JWT 绝不返回前端；登录失败不会覆盖现有本地会话。
func (service *AccountService) Login(input accountDefinition.LoginRequest) (accountDefinition.AccountSession, error) {
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return accountDefinition.AccountSession{}, fmt.Errorf("请输入用户名和密码")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return accountDefinition.AccountSession{}, fmt.Errorf("序列化登录请求失败: %w", err)
	}
	request, err := service.createRequest(http.MethodPost, accountDefinition.APIURLLogin, bytes.NewReader(body), false)
	if err != nil {
		return accountDefinition.AccountSession{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return accountDefinition.AccountSession{}, fmt.Errorf("登录请求失败: %w", err)
	}
	defer response.Body.Close()
	var result accountDefinition.APIResponse[accountDefinition.LoginData]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return accountDefinition.AccountSession{}, fmt.Errorf("解析登录响应失败: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 {
		return accountDefinition.AccountSession{}, service.responseError("登录失败", response.StatusCode, result.Code, result.Msg)
	}
	if result.Data.Token == "" {
		return accountDefinition.AccountSession{}, fmt.Errorf("登录失败: 服务器未返回有效令牌")
	}
	session := accountDefinition.AccountSession{Authenticated: true, ExpiresAt: result.Data.ExpiresAt, NeedChangePassword: result.Data.NeedChangePassword, User: result.Data.User}
	if err := service.saveSession(session, result.Data.Token); err != nil {
		return accountDefinition.AccountSession{}, err
	}
	return session, nil
}

// GetSession 返回当前本地保存的账号会话状态。
// 职责：恢复页面初始化所需的登录、过期和强制改密状态。
// 使用：个人页挂载时先调用，再按状态决定是否请求最新资料。
// 边界：过期或损坏的本地会话会被清理并返回未登录状态，不会尝试刷新 JWT。
func (service *AccountService) GetSession() (accountDefinition.AccountSession, error) {
	session, token, err := service.loadSession()
	if err != nil {
		return accountDefinition.AccountSession{}, err
	}
	if token == "" || (session.ExpiresAt > 0 && session.ExpiresAt <= time.Now().UnixMilli()) {
		if token != "" {
			if err := service.clearSession(); err != nil {
				return accountDefinition.AccountSession{}, err
			}
		}
		return accountDefinition.AccountSession{}, nil
	}
	session.Authenticated = true
	return session, nil
}

// GetCurrentUserInfo 获取并缓存服务器最新的当前用户资料。
// 职责：带当前 JWT 请求最新资料，并同步更新本地用户缓存。
// 使用：登录后、个人页进入资料页及资料保存后调用。
// 边界：401 会清理本地会话；409/10001 会保留会话并标记强制改密。
func (service *AccountService) GetCurrentUserInfo() (accountDefinition.UserInfo, error) {
	request, err := service.createRequest(http.MethodGet, accountDefinition.APIURLUserInfo, nil, true)
	if err != nil {
		return accountDefinition.UserInfo{}, err
	}
	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return accountDefinition.UserInfo{}, fmt.Errorf("获取用户资料失败: %w", err)
	}
	defer response.Body.Close()
	var result accountDefinition.APIResponse[accountDefinition.UserInfoData]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return accountDefinition.UserInfo{}, fmt.Errorf("解析用户资料响应失败: %w", err)
	}
	if response.StatusCode == 401 {
		_ = service.clearSession()
	}
	if response.StatusCode == 409 || result.Code == 10001 {
		_ = service.setNeedChangePassword(true)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 {
		return accountDefinition.UserInfo{}, service.responseError("获取用户资料失败", response.StatusCode, result.Code, result.Msg)
	}
	if err := service.setCachedUser(result.Data.UserInfo); err != nil {
		return accountDefinition.UserInfo{}, err
	}
	return result.Data.UserInfo, nil
}

// UpdateSelfInfo 保存当前用户可修改的资料。
// 职责：把个人页提交的昵称、联系方式和头像地址更新到服务器。
// 使用：用户保存账号资料或上传头像成功后调用。
// 边界：仅更新当前用户，成功后需由调用方重新读取资料；不接受管理员字段。
func (service *AccountService) UpdateSelfInfo(input accountDefinition.SetSelfInfoRequest) error {
	return service.sendJSON(http.MethodPut, accountDefinition.APIURLSetSelfInfo, input, "保存用户资料失败", true)
}

// UploadAvatar 上传头像文件并返回服务器文件地址。
// 职责：把前端编码的图片内容转换为 multipart/form-data 并调用上传接口。
// 使用：个人页选择头像后先调用，成功返回 URL 后再调用 UpdateSelfInfo。
// 边界：不自动覆盖现有头像；文件内容必须是合法 Base64，服务端上传失败时保留旧资料。
func (service *AccountService) UploadAvatar(fileName string, contentType string, contentBase64 string) (string, error) {
	if strings.TrimSpace(fileName) == "" || strings.TrimSpace(contentBase64) == "" {
		return "", fmt.Errorf("请选择头像文件")
	}
	content, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		return "", fmt.Errorf("头像文件编码无效: %w", err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("创建头像上传表单失败: %w", err)
	}
	if _, err = part.Write(content); err != nil {
		return "", fmt.Errorf("写入头像上传内容失败: %w", err)
	}
	if err = writer.Close(); err != nil {
		return "", fmt.Errorf("完成头像上传表单失败: %w", err)
	}
	request, err := service.createRequest(http.MethodPost, accountDefinition.APIURLUpload, &body, true)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("上传头像失败: %w", err)
	}
	defer response.Body.Close()
	var result accountDefinition.APIResponse[accountDefinition.UploadData]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析头像上传响应失败: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 || result.Data.File.URL == "" {
		return "", service.responseError("上传头像失败", response.StatusCode, result.Code, result.Msg)
	}
	return result.Data.File.URL, nil
}

// ChangePassword 修改当前用户密码。
// 职责：将旧密码和新密码发送至服务器，并在成功后清理本地登录状态。
// 使用：强制改密页和账号安全设置通过该方法调用，前端先校验两次新密码一致。
// 边界：不接收确认密码，不保存密码；改密成功后必须重新登录获取新的会话。
func (service *AccountService) ChangePassword(input accountDefinition.ChangePasswordRequest) error {
	if input.Password == "" || input.NewPassword == "" {
		return fmt.Errorf("请输入当前密码和新密码")
	}
	if err := service.sendJSON(http.MethodPost, accountDefinition.APIURLChangePassword, input, "修改密码失败", true); err != nil {
		return err
	}
	return service.clearSession()
}

// Logout 注销服务器 JWT 并清理本地登录状态。
// 职责：请求服务器将当前 JWT 加入黑名单，再清除本地缓存。
// 使用：个人页用户点击退出登录时调用。
// 边界：网络或服务器注销失败时仍清理本地会话，避免继续使用可能失效的令牌。
func (service *AccountService) Logout() error {
	request, err := service.createRequest(http.MethodPost, accountDefinition.APIURLLogout, nil, true)
	if err != nil {
		return err
	}
	response, requestErr := core.GLOBAL.HTTPClient.Do(request)
	if response != nil {
		defer response.Body.Close()
	}
	clearErr := service.clearSession()
	if clearErr != nil {
		return clearErr
	}
	if requestErr != nil {
		return fmt.Errorf("服务端注销失败，本地登录状态已清理: %w", requestErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("服务端注销失败，本地登录状态已清理: %s", response.Status)
	}
	var result accountDefinition.APIResponse[json.RawMessage]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析服务端注销响应失败，本地登录状态已清理: %w", err)
	}
	if result.Code != 0 {
		return service.responseError("服务端注销失败，本地登录状态已清理", response.StatusCode, result.Code, result.Msg)
	}
	return nil
}

// createRequest 创建账号接口请求并按需要附加当前用户鉴权头。
// 职责：统一使用全局国内服务器 HTTPClient，读取本地会话并设置 x-token/x-user-id。
// 使用：AccountService 的所有远端接口方法在发送前调用。
// 边界：不发送请求、不解析响应；私有接口没有有效会话时直接返回错误。
func (service *AccountService) createRequest(method string, path string, body io.Reader, requiresAuthentication bool) (*http.Request, error) {
	if core.GLOBAL.HTTPClient == nil {
		return nil, fmt.Errorf("国内服务器客户端尚未初始化")
	}
	request, err := core.GLOBAL.HTTPClient.NewRequest(context.Background(), method, path, body)
	if err != nil {
		return nil, err
	}
	if !requiresAuthentication {
		return request, nil
	}
	session, token, err := service.loadSession()
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, fmt.Errorf("当前未登录")
	}
	if session.ExpiresAt > 0 && session.ExpiresAt <= time.Now().UnixMilli() {
		_ = service.clearSession()
		return nil, fmt.Errorf("登录已过期，请重新登录")
	}
	request.Header.Set("x-token", token)
	if session.User.ID > 0 {
		request.Header.Set("x-user-id", strconv.FormatUint(uint64(session.User.ID), 10))
	}
	return request, nil
}

// sendJSON 发送需要 JSON 请求体的账号接口并校验统一响应。
// 职责：减少资料保存和改密流程的重复编码、请求和业务码校验。
// 使用：UpdateSelfInfo 与 ChangePassword 调用。
// 边界：只处理无业务数据响应，不适用于登录、验证码、资料读取和文件上传。
func (service *AccountService) sendJSON(method string, path string, input interface{}, prefix string, requiresAuthentication bool) error {
	body, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}
	request, err := service.createRequest(method, path, bytes.NewReader(body), requiresAuthentication)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := core.GLOBAL.HTTPClient.Do(request)
	if err != nil {
		return fmt.Errorf("%s: %w", prefix, err)
	}
	defer response.Body.Close()
	var result accountDefinition.APIResponse[json.RawMessage]
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if response.StatusCode == 401 {
		_ = service.clearSession()
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.Code != 0 {
		return service.responseError(prefix, response.StatusCode, result.Code, result.Msg)
	}
	return nil
}

// loadSession 读取本地保存的账号会话及其 JWT。
// 职责：从配置存储恢复会话字段和用户资料缓存。
// 使用：GetSession 与所有私有接口创建请求前调用。
// 边界：缓存 JSON 损坏时返回错误，不猜测用户身份或令牌内容。
func (service *AccountService) loadSession() (accountDefinition.AccountSession, string, error) {
	token, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionTokenKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	expires, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionExpiresAtKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	needChange, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionNeedChangePasswordKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	userText, err := core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionUserKey).Get()
	if err != nil {
		return accountDefinition.AccountSession{}, "", err
	}
	session := accountDefinition.AccountSession{NeedChangePassword: needChange == "true"}
	if expires != "" {
		session.ExpiresAt, err = strconv.ParseInt(expires, 10, 64)
		if err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("本地登录过期时间无效: %w", err)
		}
	}
	if userText != "" {
		if err := json.Unmarshal([]byte(userText), &session.User); err != nil {
			return accountDefinition.AccountSession{}, "", fmt.Errorf("本地用户资料缓存无效: %w", err)
		}
	}
	return session, token, nil
}

// saveSession 保存成功登录后的本地会话。
// 职责：将 JWT、过期时间、改密状态和用户资料写入 SQLite 配置。
// 使用：Login 校验服务器响应成功后调用。
// 边界：写入失败时返回错误，不把部分会话视为可用。
func (service *AccountService) saveSession(session accountDefinition.AccountSession, token string) error {
	userJSON, err := json.Marshal(session.User)
	if err != nil {
		return fmt.Errorf("序列化本地用户资料失败: %w", err)
	}
	values := map[string]string{accountConstant.SessionTokenKey: token, accountConstant.SessionExpiresAtKey: strconv.FormatInt(session.ExpiresAt, 10), accountConstant.SessionNeedChangePasswordKey: strconv.FormatBool(session.NeedChangePassword), accountConstant.SessionUserKey: string(userJSON)}
	for key, value := range values {
		if err := core.InitConfigUnit(accountConstant.SessionConfigGroup, key).Set(value); err != nil {
			return fmt.Errorf("保存本地登录状态失败: %w", err)
		}
	}
	return nil
}

// clearSession 清除本地账号会话。
// 职责：覆盖保存的 JWT、过期时间、强制改密状态和用户资料缓存。
// 使用：注销、令牌过期、401 和改密成功后调用。
// 边界：只清理本地状态，不替代服务器 JWT 黑名单调用。
func (service *AccountService) clearSession() error {
	for _, key := range []string{accountConstant.SessionTokenKey, accountConstant.SessionExpiresAtKey, accountConstant.SessionNeedChangePasswordKey, accountConstant.SessionUserKey} {
		if err := core.InitConfigUnit(accountConstant.SessionConfigGroup, key).Set(""); err != nil {
			return fmt.Errorf("清理本地登录状态失败: %w", err)
		}
	}
	return nil
}

// setCachedUser 更新本地用户资料缓存。
// 职责：保存服务器返回的最新资料，供后续鉴权用户 ID 与页面冷启动使用。
// 使用：GetCurrentUserInfo 成功后调用。
// 边界：只写用户资料，不改动 JWT、过期时间或强制改密状态。
func (service *AccountService) setCachedUser(user accountDefinition.UserInfo) error {
	content, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionUserKey).Set(string(content))
}

// setNeedChangePassword 更新本地强制改密状态。
// 职责：在服务器拒绝普通私有接口时保存需要改密标识。
// 使用：GetCurrentUserInfo 收到 409 或业务码 10001 时调用。
// 边界：不修改 JWT 或用户资料，也不绕过服务端对改密前操作的限制。
func (service *AccountService) setNeedChangePassword(value bool) error {
	return core.InitConfigUnit(accountConstant.SessionConfigGroup, accountConstant.SessionNeedChangePasswordKey).Set(strconv.FormatBool(value))
}

// responseError 构造包含 HTTP 与业务码的账号接口错误。
// 职责：把远端失败状态归一为适合前端展示的错误消息。
// 使用：各账号接口在 HTTP 或业务码失败时调用。
// 边界：不包含密码、JWT、请求体或服务器敏感响应内容。
func (service *AccountService) responseError(prefix string, httpStatus int, businessCode int, message string) error {
	if message == "" {
		message = "服务器未返回错误说明"
	}
	return fmt.Errorf("%s: %s (HTTP %d，业务码 %d)", prefix, message, httpStatus, businessCode)
}
