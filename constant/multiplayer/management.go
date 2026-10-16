package multiplayer

// 文件职责：管理扩展的事件和原因码；使用：客户端生命周期及提示；边界：与共享控制契约一致，不包含管理接口。

// 管理扩展常量职责：集中声明线上数值；使用：控制器分派；边界：仍使用协议 v2。
const (
	// EvSessionPolicyBefore 职责：策略变更准备；使用：事件回复；边界：不应用未提交值。
	EvSessionPolicyBefore = 2002
	// EvSessionPolicyAfter 职责：策略提交；使用：刷新动态能力；边界：只应用成功结果。
	EvSessionPolicyAfter = 3002
	// CodeAdmissionPaused 职责：服务暂停准入；使用：停止自动连接；边界：手动重试可重新读取策略。
	CodeAdmissionPaused = 20010
	// ReasonAdminDisconnect 职责：管理员断开会话；使用：停止重连；边界：不影响账号登录。
	ReasonAdminDisconnect = 20106
	// ReasonAdminClose 职责：管理员关闭房间；使用：退出提示；边界：保留大厅控制链路。
	ReasonAdminClose = 20225
	// ReasonMemberCap 职责：降低上限导致关房；使用：退出提示；边界：不会自动重建。
	ReasonMemberCap = 20226
	// ReasonAdminKick 职责：管理员移出成员；使用：退出提示；边界：不自动重新加入。
	ReasonAdminKick = 20235
)
