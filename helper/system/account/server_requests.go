package account

// 文件职责：将账号状态机接到全局盒子服务器请求适配器。
// 使用：应用启动加载接口目录，各业务 helper 提供其 ApiDesc。
// 边界：全局 ServerClient 在每次调用中随原认证快照固定。

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
)

// serverRequests 共享请求执行器和内存缓存，传输来自捕获的全局客户端。
var serverRequests = request.NewAdapter(request.NewLoader(), captureServerSession)

// errLogoutWithoutToken 表示本地退出已完成且原会话没有远端凭据。
var errLogoutWithoutToken = errors.New("本地会话已清除")

// ServerRequests 职责：提供盒子服务器的请求入口；使用：各模块 helper 与应用启动；边界：全应用共享一个执行器。
func ServerRequests() *request.Adapter {
	return serverRequests
}

// ScopeKey 职责：生成与 Wails 账号键一致的作用域；使用：业务会话校验和缓存；边界：只包含服务器、主体和代次。
func ScopeKey(generation uint64, subject, server string) string {
	return strings.Join([]string{server, subject, strconv.FormatUint(generation, 10)}, "\n")
}

// captureServerSession 按接口认证门槛捕获原会话，登录与注销仍按账号状态机建立边界。
func captureServerSession(desc *api.ApiDesc, expectedKey string) (request.Session, error) {
	var snapshot requestSnapshot
	subject := ""
	switch {
	case desc.Auth == api.AuthVerified:
		g, user, _, token, ctx, client, ok := VerifiedSession()
		if !ok {
			if SessionState().NeedChangePassword {
				return request.Session{}, &request.Error{Kind: request.PasswordRequired}
			}
			return request.Session{}, &request.Error{Kind: request.AuthRequired}
		}
		subject = user
		snapshot = requestSnapshot{generation: g, token: token, ctx: ctx, client: client, verified: true}
	case desc.SessionAction == api.SessionLogin:
		snapshot = beginLoginRequest()
	case desc.SessionAction == api.SessionLogout:
		var err error
		snapshot, err = beginLogoutRequest()
		if err != nil {
			return request.Session{}, err
		}
		if snapshot.token == "" {
			return request.Session{}, errLogoutWithoutToken
		}
	default:
		snapshot = captureRequest()
	}
	if snapshot.client == nil {
		return request.Session{}, &request.Error{Kind: request.Unavailable}
	}
	key := ScopeKey(snapshot.generation, subject, snapshot.client.BaseURL())
	if expectedKey != "" && expectedKey != key {
		return request.Session{}, &request.Error{Kind: request.Stale}
	}
	session := request.Session{
		Mode: desc.Auth, Action: desc.SessionAction, Subject: subject, Generation: snapshot.generation, Transport: snapshot.client,
		Scope: requestcache.Scope{ID: key, Context: snapshot.ctx},
		Current: func() error {
			if err := ensureRequestCurrent(snapshot); err != nil {
				return &request.Error{Kind: request.Stale}
			}
			return nil
		},
	}
	if desc.Auth != api.AuthNone {
		session.Credential = snapshot.token
	}
	if desc.Auth != api.AuthNone || desc.SessionAction == api.SessionLogin {
		session.Accept = func(update request.AuthUpdate) error {
			return acceptServerAuth(snapshot, desc.SessionAction, update)
		}
	}
	return session, nil
}

// acceptServerAuth 将标准认证应答与登录、身份复验、改密的专属结果一起原子提交。
func acceptServerAuth(snapshot requestSnapshot, action api.SessionAction, update request.AuthUpdate) error {
	if action == api.SessionLogin && !update.Success {
		return nil
	}
	var user *systemAPI.AccountUser
	var token string
	var password *bool
	if update.Success {
		switch action {
		case api.SessionLogin:
			var login systemAPI.AccountLoginData
			if json.Unmarshal(update.Data, &login) != nil {
				return errors.New("登录响应无效")
			}
			user, token, password = &login.User, login.Token, &login.NeedChangePassword
		case api.SessionVerify:
			var data systemAPI.AccountCurrentUserData
			if json.Unmarshal(update.Data, &data) != nil || data.UserInfo.ID == 0 {
				return errors.New("账号验证未返回有效用户ID")
			}
			user = &data.UserInfo
		}
	}
	err := acceptAccountResponse(snapshot, update.Refreshed, update.Unauthorized, update.NeedPassword,
		user, token, password, update.Success && action == api.SessionChangePassword)
	if err != nil && snapshot.verified {
		return &request.Error{Kind: request.Stale}
	}
	return err
}

// projectProfile 应用服务端已确认的资料更新，锁内核对接收方仍为原账号和服务器。
func projectProfile(access request.Session, userID uint, apply func()) error {
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	if access.Generation != sessionMutationState.generation || access.Transport != core.GLOBAL.ServerClient ||
		access.Scope.Context.Err() != nil {
		return errors.New("账号会话或服务器已切换，请重试。")
	}
	if userID != 0 && sessionMutationState.subject != "" && sessionMutationState.subject != strconv.FormatUint(uint64(userID), 10) {
		return errors.New("账号资料与当前会话不匹配。")
	}
	apply()
	return nil
}
