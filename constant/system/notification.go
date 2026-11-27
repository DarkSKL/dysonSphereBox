package system

// 职责：集中收件箱事件名；使用：服务端 SSE 解析与 Wails 全局同步；边界：只携带刷新提示。

// 职责：区分全局通知变化；使用：流读取器与应用级事件桥；边界：事件不能作为站内信内容。
const (
	// NotificationRuntimeEvent 职责：命名 Wails 事件；使用：全局会话监听；边界：只发作用域和原因。
	NotificationRuntimeEvent = "system:notifications"
	// NotificationConnected 职责：表示收到建连首帧；使用：首次和每次重连后校准；边界：每条连接一次。
	NotificationConnected = "connected"
	// NotificationInvalidated 职责：通知前端可信状态已撤销；使用：更新本地会话快照；边界：仅旧作用域接收。
	NotificationInvalidated = "invalidated"
	// NotificationMessage 职责：处理站内信写入；使用：SSE 帧分发；边界：刷新时重新读取 HTTP。
	NotificationMessage = "notify:message"
	// NotificationRecall 职责：处理站内信撤回；使用：SSE 帧分发；边界：刷新时重新读取 HTTP。
	NotificationRecall = "notify:recall"
)
