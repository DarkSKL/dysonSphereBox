package forum

// 文件职责：记录社区论坛写入与本地资源导入的关键操作日志。
// 使用：ForumService 在发帖、回复、举报或资源导入完成后调用。
// 边界：仅记录操作类型、资源 ID 等摘要维度，不记录会话令牌、完整正文或蓝图编码。

import (
	"DysonSphereBox/constant/logging"
	systemHelper "DysonSphereBox/helper/system"
)

// RecordAction 记录社区论坛用户操作日志。
// 职责：通过系统统一日志入口写入可诊断的结构化审计记录。
// 使用：ForumService 状态变更和本地导入工作流调用。
// 边界：日志失败不影响主流程返回；metadata 不包含敏感凭据或大文本。
func RecordAction(event, action, reason string, metadata map[string]interface{}) {
	systemHelper.RecordLogSilently(systemHelper.LogEntry{
		Action:    action,
		Level:     logging.Info,
		Source:    "backend.forum",
		Reason:    reason,
		Namespace: "community.forum." + event,
		Handler:   logging.User,
		Data:      metadata,
	})
}
