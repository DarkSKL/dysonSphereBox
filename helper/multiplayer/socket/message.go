package socket

// 文件职责：提取控制链路安全说明；使用：请求失败和连接拒绝反馈；边界：只解析契约字段，不推断错误码。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	"encoding/json"
)

// ReplyMessage 职责：优先提取管理扩展 data.message；使用：session.open 和业务请求失败；边界：缺失或非法 data 时回退原 msg。
func ReplyMessage(reply *mpAPI.ReplyFrame) string {
	if reply == nil {
		return ""
	}
	var detail struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(reply.Data, &detail) == nil && detail.Message != "" {
		return detail.Message
	}
	return reply.Msg
}
