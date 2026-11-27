package notification

// 文件职责：固定一次通知连接所有的可信会话。
// 使用：通知重连循环每次建连前捕获。
// 边界：前端只收到去凭据的会话键和事件。

import (
	"context"

	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// Session 职责：保存一次通知连接的账号作用域；使用：重连循环和 SSE 读取；边界：旧作用域不能改绑新账号。
type Session struct {
	Key     string
	Context context.Context
	access  request.Session
}

// Capture 职责：取得当前可信会话；使用：通知连接观察者；边界：未验证、改密和会话切换都会中断连接。
func Capture(expectedKey string) (Session, error) {
	access, err := account.ServerRequests().Capture(systemAPI.NotificationStreamAPI, expectedKey)
	if err != nil {
		return Session{}, notificationFailure(err)
	}
	return Session{Key: access.Scope.ID, Context: access.Scope.Context, access: access}, nil
}
