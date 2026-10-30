package account

// 文件职责：提供账号会话的统一同步入口，把本地 token 验证、用户资料缓存和失败阶段收敛为一次快照。
// 使用：service/system/AccountService.SyncSession 调用；前端启动引导、账号弹窗打开、登录后和资料更新后统一使用。
// 边界：只负责会话验证与快照组装，不决定前端弹窗、公告排队或页面跳转；网络失败保留 token，不用离线错误退出登录。

import (
	"context"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// SyncSession 同步当前账号会话并返回前端可直接消费的完整快照。
// 职责：在基础设施就绪后验证已保存 token，拉取并缓存用户资料，或返回未登录、强制改密、过期、离线等明确阶段。
// 使用：前端全局账号 store 的唯一刷新入口；force 为 true 时即使已有缓存用户也重新请求服务器。
// 边界：网络或服务器异常时保留 token 与缓存用户并标记 offline；只有服务器明确拒绝 token 时才返回 expired。
// 逻辑流：1. 读取本地会话阶段 => 2. 未就绪或未登录直接返回 => 3. 已验证且无需强制刷新时复用缓存 => 4. 请求当前用户并建立可信身份 => 5. 失败时按 token、改密标志归类为过期或离线。
func SyncSession(ctx context.Context, force bool) (*systemAPI.AccountSessionSnapshot, error) {
	state := SessionState()
	if !state.Ready {
		return &systemAPI.AccountSessionSnapshot{
			Session: state,
			Message: "客户端正在初始化账号会话，请稍候重试。",
		}, nil
	}
	if !state.Authenticated {
		return &systemAPI.AccountSessionSnapshot{Session: state}, nil
	}
	if state.NeedChangePassword {
		return &systemAPI.AccountSessionSnapshot{
			Session: state,
			Message: "请先修改账号密码。",
		}, nil
	}
	if state.Verified && state.User != nil && !force {
		return &systemAPI.AccountSessionSnapshot{Session: state}, nil
	}

	if _, err := CurrentUser(ctx); err != nil {
		return sessionSnapshotAfterFailure(err), nil
	}
	return &systemAPI.AccountSessionSnapshot{Session: SessionState()}, nil
}

// sessionSnapshotAfterFailure 职责：把验证失败归类为可保留登录态的离线或必须重新登录的过期；使用：SyncSession 请求失败分支；边界：不自行清除 token，清理由账号请求的 401 处理完成。
func sessionSnapshotAfterFailure(cause error) *systemAPI.AccountSessionSnapshot {
	state := SessionState()
	message := ""
	if cause != nil {
		message = cause.Error()
	}
	switch {
	case !state.Authenticated:
		// 401 已在统一请求层清除 token；这里只把结果标记为过期，供前端提示重新登录。
		state.Status = systemAPI.AccountSessionStatusExpired
		state.Verified = false
		state.User = nil
	case state.NeedChangePassword:
		state.Status = systemAPI.AccountSessionStatusPasswordRequired
		state.Verified = false
	default:
		// 网络中断、服务器 5xx 或响应解析失败都保留原 token 与已验证身份，允许稍后重试；
		// 本次刷新失败不再让公告、翻译等已验证能力突然消失。
		state.Status = systemAPI.AccountSessionStatusOffline
	}
	return &systemAPI.AccountSessionSnapshot{Session: state, Message: message}
}
