package multiplayer

// 文件职责：定义服务器事件（before/after）的 data 结构。
// 使用：service/multiplayer 按 opcode 解码事件；只列客户端需要的字段，其余字段按协议规则忽略。
// 边界：字段名严格按协议设计事件定义；不包含事件处理逻辑。

// EventActor 是事件的触发者。
// 职责：描述触发者 userId 与角色；使用：日志与展示；边界：system 表示服务器触发。
type EventActor struct {
	// UserID 职责：触发者账户；使用：日志；边界：system 时为 0。
	UserID int64 `json:"userId"`
	// Role 职责：host / member / system；使用：日志；边界：只读。
	Role string `json:"role"`
}

// EventBase 是每个事件 data 的公共字段。
// 职责：描述房间、revision、触发者、原因与结果；使用：所有事件结构嵌入；边界：result 只在 after 出现，非 0 表示中止。
type EventBase struct {
	// Message 职责：服务端管理原因说明；使用：玩家退出和维护提示；边界：纯文本，可为空。
	Message string `json:"message,omitempty"`
	// RoomID 职责：相关房间；使用：房间匹配；边界：与房间无关时为空。
	RoomID string `json:"roomId"`
	// Revision 职责：before 为变化前、after 为变化后的 revision；使用：快照应用；边界：中止时不变。
	Revision int64 `json:"revision"`
	// Actor 职责：触发者；使用：日志；边界：只读。
	Actor EventActor `json:"actor"`
	// Reason 职责：原因码；使用：关房、离开、终止原因提示；边界：不适用时为 0。
	Reason int `json:"reason"`
	// Result 职责：after 的结果；使用：0 已提交、非 0 已中止；边界：before 中不出现。
	Result int `json:"result"`
}

// RoomEvent 是只携带 room 的事件（room.create after、room.close after 等）。
// 职责：携带完整快照；使用：应用房间状态；边界：房间已不存在时 room 为 null。
type RoomEvent struct {
	EventBase
	// Room 职责：完整快照；使用：按 revision 应用；边界：可为 null。
	Room *RoomSnapshot `json:"room"`
}

// RoomInviteEvent 是 room.invite after data。
// 职责：携带新邀请码；使用：房主展示与复制；边界：邀请码不在快照中，不改变 revision。
type RoomInviteEvent struct {
	EventBase
	// InviteCode 职责：邀请码；使用：展示复制；边界：XXXX-XXXX。
	InviteCode string `json:"inviteCode"`
	// Link 职责：邀请链接；使用：展示复制；边界：dsbox://multiplayer/join?inviteCode=<码>。
	Link string `json:"link"`
	// ExpiresAtMs 职责：过期时间；使用：展示；边界：24 小时。
	ExpiresAtMs int64 `json:"expiresAtMs"`
	// Room 职责：完整快照；使用：按 revision 应用；边界：同 RoomEvent。
	Room *RoomSnapshot `json:"room"`
}

// MemberJoinEvent 是 room.member.join before/after data。
// 职责：携带加入者、快照与房主连接信息；使用：房主放行白名单、加入者建链；边界：before 只有 member，中止时 host 为 null。
type MemberJoinEvent struct {
	EventBase
	// Room 职责：完整快照；使用：after 应用；边界：before 中不出现。
	Room *RoomSnapshot `json:"room"`
	// Member 职责：加入者；使用：房主白名单与身份判断；边界：before 中 link 为 null。
	Member *Member `json:"member"`
	// Host 职责：房主连接信息；使用：加入者建链；边界：中止时为 null。
	Host *HostEndpoint `json:"host"`
}

// MemberLeaveEvent 是 room.member.leave before/after data。
// 职责：携带离开者与快照；使用：房主停收并移出白名单、离开者拆除；边界：不会中止。
type MemberLeaveEvent struct {
	EventBase
	// Room 职责：完整快照；使用：after 应用；边界：before 中不出现。
	Room *RoomSnapshot `json:"room"`
	// UserID 职责：离开者；使用：身份判断；边界：number。
	UserID int64 `json:"userId"`
	// PeerID 职责：离开者 PeerID；使用：房主停收新 stream；边界：只在 before 中出现。
	PeerID string `json:"peerId"`
}

// MemberLinkEvent 是 room.member.link before/after data。
// 职责：携带成员新链路状态；使用：更新成员列表；边界：只在 state/path/error 变化时出现。
type MemberLinkEvent struct {
	EventBase
	// Room 职责：完整快照；使用：after 应用；边界：before 中不出现。
	Room *RoomSnapshot `json:"room"`
	// UserID 职责：成员；使用：链路投影；边界：number。
	UserID int64 `json:"userId"`
	// Link 职责：新链路状态；使用：链路投影；边界：可为 null。
	Link *LinkState `json:"link"`
}

// HostAddrsEvent 是 room.host.addrs after data。
// 职责：携带房主新地址；使用：重连中或失败的成员改用新地址；边界：不改变 revision。
type HostAddrsEvent struct {
	EventBase
	// Host 职责：房主连接信息；使用：成员重连；边界：只下发给本房间成员。
	Host *HostEndpoint `json:"host"`
}

// LinkPunchEvent 是 link.punch before/after data。
// 职责：携带轮次、拨号时刻与对端地址；使用：双方在 atMs 同时拨号；边界：发给成员附 host，发给房主附 member，中止时 atMs 为 0 且不附地址。
type LinkPunchEvent struct {
	EventBase
	// UserID 职责：发起打洞的成员；使用：房主识别回拨对象；边界：number。
	UserID int64 `json:"userId"`
	// Round 职责：轮次；使用：日志与匹配；边界：从 1 开始。
	Round int `json:"round"`
	// AtMs 职责：拨号时刻（服务器时钟）；使用：换算本地时刻；边界：中止时为 0。
	AtMs int64 `json:"atMs"`
	// Host 职责：房主连接信息；使用：成员拨号；边界：只发给成员。
	Host *HostEndpoint `json:"host"`
	// Member 职责：成员地址；使用：房主回拨；边界：只发给房主。
	Member *PunchPeer `json:"member"`
}

// LobbyChangeEvent 是 lobby.change after data。
// 职责：携带合并后的大厅变化；使用：更新大厅列表；边界：满员房间放在 removed。
type LobbyChangeEvent struct {
	EventBase
	// Upserted 职责：新增或变化的条目；使用：按 roomId 覆盖；边界：只含公开未满房间。
	Upserted []RoomSummary `json:"upserted"`
	// Removed 职责：移除的 roomId；使用：从列表删除；边界：包含关闭与满员。
	Removed []string `json:"removed"`
}
