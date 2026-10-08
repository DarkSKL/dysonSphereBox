package account

// 文件职责：维护客户端后端持有的账号会话状态。
// 使用：账号请求附加 token、登录保存 token、续期替换 token、401/注销/切换服务器时清除会话。
// 边界：token 持久化在 Account/Token 配置中，只在后端与服务器之间传递，不返回给前端；强制改密标志只保存在进程内存。

import (
	"errors"
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
// 使用：登录成功与响应头携带 new-token 时调用。
// 边界：配置未初始化时返回错误；空 token 等同于清除。
func SaveToken(token string) error {
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
	setNeedChangePassword(false)
	err := SaveToken("")
	sessionClearedHooks.Lock()
	hooks := append([]func(){}, sessionClearedHooks.list...)
	sessionClearedHooks.Unlock()
	for _, hook := range hooks {
		hook()
	}
	return err
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
	needChangePasswordState.Lock()
	defer needChangePasswordState.Unlock()

	return systemAPI.AccountSessionState{
		Authenticated:      Token() != "",
		NeedChangePassword: needChangePasswordState.value,
	}
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
