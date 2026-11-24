package feedback

// 文件职责：记录星系用户写入的成功回执。
// 使用：Wails 写入方法收到当前会话的成功结果后调用。
// 边界：只记录操作名称，不保存会话键、用户资料或反馈正文。

import (
	"DysonSphereBox/constant/logging"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordWrite 记录星系写入成功，用于本机诊断，日志写入不改变远端回执。
func RecordWrite(event, action string) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Action:    action,
		Level:     logging.Info,
		Source:    "backend.feedback",
		Reason:    "反馈服务确认本次操作",
		Namespace: "community.feedback." + event,
		Handler:   logging.User,
	})
}
