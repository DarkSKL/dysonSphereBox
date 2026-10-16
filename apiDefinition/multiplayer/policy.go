package multiplayer

// 文件职责：定义运行期管理策略契约；使用：session.open 与 session.policy；边界：仅含玩家需要的有效策略，不暴露管理配置。

// SessionPolicy 职责：描述准入状态和有效人数上限；使用：客户端提示和表单约束；边界：旧服务缺失时保持原会话限制。
type SessionPolicy struct {
	// Revision 职责：策略版本；使用：丢弃过期事件；边界：同一服务进程递增。
	Revision int64 `json:"revision"`
	// AdmissionEnabled 职责：是否允许新建和加入；使用：禁用对应操作；边界：不禁止退出与关房。
	AdmissionEnabled bool `json:"admissionEnabled"`
	// Maintenance 职责：服务维护状态；使用：显示维护通知；边界：不在客户端自动恢复房间。
	Maintenance bool `json:"maintenance"`
	// Message 职责：服务端安全说明；使用：中文状态提示；边界：按纯文本渲染。
	Message string `json:"message"`
	// MaxMembers 职责：当前全局人数上限；使用：创建与加入约束；边界：含房主。
	MaxMembers int `json:"maxMembers"`
}

// SessionPolicyEvent 职责：整体替换当前会话策略和中继能力；使用：3002 提交后应用；边界：result 非零不替换。
type SessionPolicyEvent struct {
	EventBase
	// Policy 职责：有效策略；使用：按版本应用；边界：不能为空。
	Policy *SessionPolicy `json:"policy"`
	// Capabilities 职责：最新数据面授权；使用：替换而非合并；边界：缺失即撤销授权。
	Capabilities Capabilities `json:"capabilities"`
	// Relays 职责：最新受信入口；使用：重建原生预约；边界：旧 PeerID 不复用。
	Relays []string `json:"relays"`
}
