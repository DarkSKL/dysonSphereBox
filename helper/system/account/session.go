package account

// 文件职责：维护客户端后端持有的账号会话状态。
// 使用：账号请求附加 token、登录保存 token、续期替换 token、401/注销/切换服务器时清除会话。
// 边界：token 持久化在 Account/Token 配置中，只在后端与服务器之间传递，不返回给前端；强制改密标志只保存在进程内存。

import (
	"context"
	"errors"
	"strconv"
	"sync"

	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
)

// needChangePasswordState 保存当前会话是否需要先修改密码。
// 职责：在登录结果或 409/10001 响应后记录强制改密状态，供前端同步。
// 使用：setNeedChangePassword 写入，SessionState 读取。
// 边界：只在本进程内有效；重启后由服务端再次通过 409 告知。
var needChangePasswordState struct {
	sync.Mutex
	value bool
}

// sessionMutationState 职责：串行会话替换与响应副作用，记录登录/退出/切服代次。
// 使用：SaveToken、请求快照及响应接受检查；边界：静默续期不递增，避免误拒同会话并发请求。
var sessionMutationState struct {
	sync.Mutex
	generation uint64
	// subject 职责：保存服务端验证过的精确ID；使用：公告捕获；边界：不经JS数值转换。
	subject string
	// ctx/cancel 职责：取消旧代次在途请求；使用：替换会话；边界：取消不能撤回已送达写入。
	ctx    context.Context
	cancel context.CancelFunc
}

// Token 返回当前保存的账号 token。
// 职责：读取 Account/Token 配置。
// 使用：Request 附加鉴权头、Logout 判断是否需要请求服务端时调用。
// 边界：配置未初始化或读取失败时返回空字符串，视为未登录。
func Token() string {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.AccountToken == nil {
		return ""
	}

	token, err := core.GLOBAL.Configure.AccountToken.Get()
	if err != nil {
		return ""
	}
	return token
}

// SaveToken 保存服务端签发的账号 token。
// 职责：写入 Account/Token 配置，供后续请求与重启后恢复会话使用。
// 使用：登录成功或显式替换会话时调用；静默续期由 acceptAccountResponse 写入。
// 边界：递增会话代次以作废旧响应；配置未初始化时返回错误，空 token 等同于清除。
func SaveToken(token string) error {
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	sessionMutationState.generation++
	invalidateLocked()
	return saveTokenValue(token)
}

// saveTokenValue 职责：写入 token 配置；使用：持有会话锁的替换或续期流程；边界：不变更会话代次。
func saveTokenValue(token string) error {
	if core.GLOBAL.Configure == nil || core.GLOBAL.Configure.AccountToken == nil {
		return errors.New("configure account token is not initialized")
	}
	return core.GLOBAL.Configure.AccountToken.Set(token)
}

// ClearSession 清除本地账号会话。
// 职责：删除已保存 token 并重置强制改密状态。
// 使用：注销、收到 401、切换服务器地址时调用。
// 边界：只清除客户端状态，不通知服务端作废 token。
func ClearSession() error {
	sessionMutationState.Lock()
	sessionMutationState.generation++
	invalidateLocked()
	setNeedChangePassword(false)
	err := saveTokenValue("")
	sessionMutationState.Unlock()
	notifySessionCleared()
	return err
}

// notifySessionCleared 职责：在会话锁外通知连接清理；使用：主动注销或服务器401；边界：回调不得再次清除会话。
func notifySessionCleared() {
	sessionClearedHooks.Lock()
	hooks := append([]func(){}, sessionClearedHooks.list...)
	sessionClearedHooks.Unlock()
	for _, hook := range hooks {
		hook()
	}
}

// HandleAnnouncementAuthResponse按公告原会话处理鉴权副作用。
// 职责：安全应用401和409；使用：公告HTTP解析响应后调用。
// 边界：旧代次、旧token、旧client无副作用。
func HandleAnnouncementAuthResponse(generation uint64, token string, client *core.HTTPClient, refreshedToken string, unauthorized, needPassword bool) bool {
	return acceptAccountResponse(requestSnapshot{generation: generation, token: token, client: client}, refreshedToken, unauthorized, needPassword, 0, "", nil, false) == nil
}

// VerifiedSession 职责：捕获已验证主体、HTTP目标、token及可取消代次；使用：公告GET/POST；边界：token仅留Go内存。
func VerifiedSession() (generation uint64, subject, serverKey, token string, ctx context.Context, client *core.HTTPClient, ok bool) {
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	client = core.GLOBAL.ServerClient
	token = Token()
	if client == nil || token == "" || sessionMutationState.subject == "" || needPasswordLocked() {
		return
	}
	if sessionMutationState.ctx == nil {
		sessionMutationState.ctx, sessionMutationState.cancel = context.WithCancel(context.Background())
	}
	return sessionMutationState.generation, sessionMutationState.subject, client.BaseURL(), token, sessionMutationState.ctx, client, true
}

// needPasswordLocked 职责：读取改密标志；使用：持会话锁捕获凭据；边界：不改变状态。
func needPasswordLocked() bool {
	needChangePasswordState.Lock()
	defer needChangePasswordState.Unlock()
	return needChangePasswordState.value
}

// sessionClearedHooks 保存会话清除后的回调。
// 职责：让依赖账号会话的长连接（联机控制链路）在注销、401、切换服务器时主动断开。
// 使用：OnSessionCleared 注册，ClearSession 调用。
// 边界：回调不得再调用 ClearSession；只在本进程内有效。
var sessionClearedHooks struct {
	sync.Mutex
	list []func()
}

// OnSessionCleared 注册会话清除后的回调。
// 职责：解耦账号会话与联机控制链路；使用：service/multiplayer 注册服务时调用一次；边界：回调需快速返回。
func OnSessionCleared(hook func()) {
	sessionClearedHooks.Lock()
	sessionClearedHooks.list = append(sessionClearedHooks.list, hook)
	sessionClearedHooks.Unlock()
}

// SessionState 返回前端可见的会话状态。
// 职责：汇总是否持有 token 与是否需要先修改密码。
// 使用：AccountService.GetSessionState 返回给前端。
// 边界：Authenticated 不保证 token 仍被服务端接受。
func SessionState() systemAPI.AccountSessionState {
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	needChangePasswordState.Lock()
	defer needChangePasswordState.Unlock()

	return systemAPI.AccountSessionState{
		Authenticated:      Token() != "",
		NeedChangePassword: needChangePasswordState.value,
		SubjectID:          sessionMutationState.subject,
		Generation:         strconv.FormatUint(sessionMutationState.generation, 10),
		ServerKey:          core.GLOBAL.ServerClient.BaseURL(),
	}
}

// invalidateLocked 职责：作废可信身份并取消旧请求；使用：持锁的代次切换；边界：不写配置。
func invalidateLocked() {
	if sessionMutationState.cancel != nil {
		sessionMutationState.cancel()
	}
	sessionMutationState.subject = ""
	sessionMutationState.ctx, sessionMutationState.cancel = context.WithCancel(context.Background())
}

// SwapServer 职责：原子切换HTTP目标及会话；使用：配置写入成功后；边界：同基址不清会话、不换客户端。
func SwapServer(client *core.HTTPClient) error {
	sessionMutationState.Lock()
	if core.GLOBAL.ServerClient.BaseURL() == client.BaseURL() {
		sessionMutationState.Unlock()
		return nil
	}
	core.GLOBAL.ServerClient = client
	sessionMutationState.generation++
	invalidateLocked()
	setNeedChangePassword(false)
	err := saveTokenValue("")
	sessionMutationState.Unlock()
	notifySessionCleared()
	return err
}

// setNeedChangePassword 更新强制改密状态。
// 职责：并发安全地写入进程内改密标志。
// 使用：登录结果、409/10001 响应与清除会话时调用。
// 边界：不持久化。
func setNeedChangePassword(value bool) {
	needChangePasswordState.Lock()
	needChangePasswordState.value = value
	needChangePasswordState.Unlock()
}
