package account

// 文件职责：捕获请求凭据并串行接纳账号响应。
// 使用：账号HTTP请求及公告复用这里的同步边界。
// 边界：token仅留Go内存；失效请求只能触达原目标，不可提交新会话副作用。

import (
	"context"
	"errors"
	"strconv"

	"DysonSphereBox/core"
)

// requestSnapshot 职责：保存请求发起时的目标、凭据、代次和取消信号；使用：HTTP全生命周期；边界：字段不返回Wails。
type requestSnapshot struct {
	generation uint64
	token      string
	client     *core.HTTPClient
	ctx        context.Context
	detached   bool
}

// captureRequestLocked 职责：原子捕获凭据；使用：持会话锁的认证/注销流程；边界：只初始化取消上下文。
func captureRequestLocked() requestSnapshot {
	if sessionMutationState.ctx == nil {
		sessionMutationState.ctx, sessionMutationState.cancel = context.WithCancel(context.Background())
	}
	return requestSnapshot{generation: sessionMutationState.generation, token: Token(), client: core.GLOBAL.ServerClient, ctx: sessionMutationState.ctx}
}

// captureRequest 职责：取得不可变请求快照；使用：普通账号请求；边界：不更改代次。
func captureRequest() requestSnapshot {
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	return captureRequestLocked()
}

// ensureRequestCurrent 职责：拒绝已退出、切号或切服请求的迟到结果；使用：账号响应准备返回前；边界：注销请求只允许触达固定旧凭据。
func ensureRequestCurrent(s requestSnapshot) error {
	if s.detached {
		return nil
	}
	sessionMutationState.Lock()
	defer sessionMutationState.Unlock()
	if sessionMutationState.generation != s.generation || core.GLOBAL.ServerClient != s.client {
		return errors.New("账号会话或服务器已切换，已丢弃旧响应。")
	}
	return nil
}

// beginLoginRequest 职责：开始登录并捕获同一代次目标；使用：Login发送前；边界：先清旧会话，失败不恢复旧账号。
func beginLoginRequest() requestSnapshot {
	sessionMutationState.Lock()
	sessionMutationState.generation++
	invalidateLocked()
	setNeedChangePassword(false)
	_ = saveTokenValue("")
	snapshot := captureRequestLocked()
	sessionMutationState.Unlock()
	notifySessionCleared()
	return snapshot
}

// beginLogoutRequest 职责：立即清本机会话并保留原token注销；使用：Logout发送前；边界：响应不会再作用于任何新会话。
func beginLogoutRequest() (requestSnapshot, error) {
	sessionMutationState.Lock()
	snapshot := captureRequestLocked()
	sessionMutationState.generation++
	invalidateLocked()
	setNeedChangePassword(false)
	err := saveTokenValue("")
	snapshot.ctx = context.Background()
	snapshot.detached = true
	sessionMutationState.Unlock()
	notifySessionCleared()
	return snapshot, err
}

// acceptAccountResponse 职责：原子提交当前请求的续期、鉴权及可信用户；使用：完整解析响应后；边界：同代次旧凭据只能返回结果，不覆盖较新凭据。
func acceptAccountResponse(s requestSnapshot, refreshed string, unauthorized, needPassword bool, verifiedID uint, loginToken string, loginPassword *bool, clearPassword bool) error {
	if s.detached {
		return nil
	}
	sessionMutationState.Lock()
	if sessionMutationState.generation != s.generation || core.GLOBAL.ServerClient != s.client {
		sessionMutationState.Unlock()
		return errors.New("账号会话或服务器已切换，请重试。")
	}
	currentCredential := Token() == s.token
	if (unauthorized || needPassword) && !currentCredential {
		sessionMutationState.Unlock()
		return errors.New("账号凭据已续期，已丢弃旧鉴权响应。")
	}
	if verifiedID != 0 && !currentCredential {
		sessionMutationState.Unlock()
		return errors.New("账号凭据已续期，已丢弃旧身份响应。")
	}
	if verifiedID != 0 && sessionMutationState.subject != "" && sessionMutationState.subject != strconv.FormatUint(uint64(verifiedID), 10) {
		sessionMutationState.Unlock()
		return errors.New("账号验证主体与当前会话不匹配。")
	}
	if unauthorized && currentCredential {
		sessionMutationState.generation++
		invalidateLocked()
		setNeedChangePassword(false)
		err := saveTokenValue("")
		sessionMutationState.Unlock()
		notifySessionCleared()
		return err
	}
	defer sessionMutationState.Unlock()
	if loginPassword != nil {
		if loginToken == "" {
			return errors.New("服务器未返回登录凭据")
		}
		if err := saveTokenValue(loginToken); err != nil {
			return err
		}
		setNeedChangePassword(*loginPassword)
	} else {
		if currentCredential && refreshed != "" {
			if err := saveTokenValue(refreshed); err != nil {
				return err
			}
		}
		if currentCredential && needPassword {
			setNeedChangePassword(true)
		}
		if currentCredential && clearPassword {
			setNeedChangePassword(false)
		}
		if verifiedID != 0 {
			sessionMutationState.subject = strconv.FormatUint(uint64(verifiedID), 10)
		}
	}
	return nil
}
