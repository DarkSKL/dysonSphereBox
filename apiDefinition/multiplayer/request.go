package multiplayer

// 文件职责：定义联机控制链路各请求的 data 与回复 data。
// 使用：service/multiplayer 发起请求、解析只读请求的回复；前端通过 Wails 绑定传入部分请求结构。
// 边界：字段名严格按协议设计消息定义第 2–5 节；变更类请求的结果只经 after 事件下发。

// SessionOpenRequest 是 session.open（1001）请求 data。
// 职责：上报协议版本、客户端版本与临时 PeerID；使用：控制链路首帧；边界：PeerID 被占用时服务器回复 20008。
type SessionOpenRequest struct {
	// ProtocolVersion 职责：协议版本；使用：服务器兼容判断；边界：当前为 1。
	ProtocolVersion int `json:"protocolVersion"`
	// ClientVersion 职责：盒子版本（去掉 v 前缀）；使用：最低版本校验；边界：开发构建为 0.0.0。
	ClientVersion string `json:"clientVersion"`
	// PeerID 职责：本次临时 PeerID；使用：服务器绑定 userId↔peerId；边界：随控制链路作废。
	PeerID string `json:"peerId"`
}

// SessionOpenReply 是 session.open 回复 data。
// 职责：返回会话、用户、观测节点与限制；使用：控制器保存会话信息并连接观测节点；边界：sessionId 只用于日志。
type SessionOpenReply struct {
	// SessionID 职责：会话 ID；使用：日志与排障；边界：后续请求不携带。
	SessionID string `json:"sessionId"`
	// User 职责：当前用户；使用：身份判断；边界：不含 token。
	User UserInfo `json:"user"`
	// Observers 职责：地址观测节点 multiaddr（带 /p2p）；使用：identify 获取 observed address；边界：可为空。
	Observers []string `json:"observers"`
	// Limits 职责：服务器限制；使用：表单与打洞；边界：以服务器为准。
	Limits SessionLimits `json:"limits"`
}

// PeerAddrsRequest 是 peer.addrs（1002）请求 data。
// 职责：上报本机可达地址与网络诊断；使用：会话打开后与地址变化时；边界：最多 16 条，单条最长 256 字符。
type PeerAddrsRequest struct {
	// Addrs 职责：可达 multiaddr；使用：房主地址或打洞地址；边界：整体替换上一次。
	Addrs []string `json:"addrs"`
	// Net 职责：网络诊断；使用：服务器原样转给房间成员；边界：不含地址。
	Net NetInfo `json:"net"`
}

// LobbyListRequest 是 lobby.list（1101）请求 data。
// 职责：分页查询公开房间；使用：大厅页刷新；边界：pageSize 1–50。
type LobbyListRequest struct {
	// Page 职责：页码；使用：分页；边界：从 1 开始。
	Page int `json:"page"`
	// PageSize 职责：每页条数；使用：分页；边界：1–50。
	PageSize int `json:"pageSize"`
	// Keyword 职责：名称关键字；使用：搜索；边界：可为空。
	Keyword string `json:"keyword"`
	// Sort 职责：createdAt / memberCount；使用：排序；边界：其他值由服务器拒绝。
	Sort string `json:"sort"`
	// Order 职责：asc / desc；使用：排序方向；边界：其他值由服务器拒绝。
	Order string `json:"order"`
}

// LobbyListReply 是 lobby.list 回复 data。
// 职责：返回总数与当前页；使用：大厅列表；边界：只含 public 且未满员的房间。
type LobbyListReply struct {
	// Total 职责：总数；使用：分页；边界：非负。
	Total int `json:"total"`
	// List 职责：当前页条目；使用：大厅列表；边界：不含地址。
	List []RoomSummary `json:"list"`
}

// LobbyWatchRequest 是 lobby.watch（1102）请求 data。
// 职责：开启或关闭大厅订阅；使用：大厅区域激活/停用；边界：socket 断开即取消。
type LobbyWatchRequest struct {
	// On 职责：是否订阅；使用：订阅开关；边界：重复开启无副作用。
	On bool `json:"on"`
}

// RoomSnapshotReply 是 room.snapshot（1201）回复 data。
// 职责：返回当前房间、角色与房主连接信息；使用：事件应用失败后纠正本地状态；边界：不在房间时三者为空。
type RoomSnapshotReply struct {
	// Room 职责：房间快照；使用：按 revision 规则应用；边界：可为 null。
	Room *RoomSnapshot `json:"room"`
	// Role 职责：host / member；使用：本地角色；边界：不在房间时为空。
	Role string `json:"role"`
	// Host 职责：房主连接信息；使用：成员建链；边界：可为 null。
	Host *HostEndpoint `json:"host"`
}

// RoomCreateRequest 是 room.create（1202）请求 data。
// 职责：描述新房间；使用：创建表单；边界：结果只经 room.create after 下发。
type RoomCreateRequest struct {
	// Name 职责：房间名；使用：展示；边界：1–32 字符。
	Name string `json:"name"`
	// Visibility 职责：public / private；使用：大厅可见性；边界：private 只能邀请码加入。
	Visibility string `json:"visibility"`
	// Password 职责：房间密码；使用：加入校验；边界：空表示无密码，否则 4–32 字符。
	Password string `json:"password"`
	// MaxMembers 职责：人数上限（含房主）；使用：服务器校验；边界：2–16。
	MaxMembers int `json:"maxMembers"`
	// Ports 职责：端口声明；使用：转发；边界：最多 8 条，同协议不重叠。
	Ports []PortDecl `json:"ports"`
}

// RoomResolveRequest 是 room.resolve（1203）请求 data。
// 职责：预览房间；使用：加入前展示；边界：roomId 与 inviteCode 恰好一个非空。
type RoomResolveRequest struct {
	// RoomID 职责：房间 ID；使用：公开房间预览；边界：private 房间回复 20216。
	RoomID string `json:"roomId"`
	// InviteCode 职责：邀请码；使用：私密或公开房间预览；边界：去首尾空白。
	InviteCode string `json:"inviteCode"`
}

// RoomResolveReply 是 room.resolve 回复 data。
// 职责：返回房间摘要与是否需要密码；使用：加入表单；边界：不占名额。
type RoomResolveReply struct {
	// Room 职责：房间摘要；使用：预览展示；边界：不含地址。
	Room RoomSummary `json:"room"`
	// NeedPassword 职责：是否需要密码；使用：加入表单；边界：邀请码加入时由服务器决定。
	NeedPassword bool `json:"needPassword"`
}

// RoomJoinRequest 是 room.join（1204）请求 data。
// 职责：加入房间；使用：加入表单；边界：roomId 与 inviteCode 恰好一个非空，结果经 member.join after 下发。
type RoomJoinRequest struct {
	// RoomID 职责：房间 ID；使用：公开房间加入；边界：private 房间回复 20216。
	RoomID string `json:"roomId"`
	// InviteCode 职责：邀请码；使用：邀请加入；边界：去首尾空白。
	InviteCode string `json:"inviteCode"`
	// Password 职责：房间密码；使用：密码房间；边界：可为空。
	Password string `json:"password"`
}

// RoomIDRequest 是 room.invite / room.leave / room.close 请求 data。
// 职责：指定当前房间；使用：邀请、退出、关房；边界：与当前所在房间不一致时回复 20209。
type RoomIDRequest struct {
	// RoomID 职责：当前房间 ID；使用：请求参数；边界：必须是调用者所在房间。
	RoomID string `json:"roomId"`
}

// RoomKickRequest 是 room.kick（1208）请求 data。
// 职责：踢出成员；使用：房主操作；边界：踢自己回复 20214。
type RoomKickRequest struct {
	// RoomID 职责：当前房间 ID；使用：请求参数；边界：必须是调用者所在房间。
	RoomID string `json:"roomId"`
	// UserID 职责：被踢成员；使用：请求参数；边界：成员不存在回复 20215。
	UserID int64 `json:"userId"`
}

// LinkPunchRequest 是 link.punch（1301）请求 data。
// 职责：请求一轮打洞协调；使用：成员直连失败后；边界：round 为 1 到 limits.punchRounds。
type LinkPunchRequest struct {
	// RoomID 职责：当前房间 ID；使用：请求参数；边界：必须是调用者所在房间。
	RoomID string `json:"roomId"`
	// Round 职责：轮次；使用：服务器转发；边界：从 1 开始。
	Round int `json:"round"`
}

// LinkReportRequest 是 link.report（1302）请求 data。
// 职责：上报到房主的链路状态；使用：状态变化与 RTT 节流；边界：只有 state/path/error 变化产生事件。
type LinkReportRequest struct {
	// RoomID 职责：当前房间 ID；使用：请求参数；边界：必须是调用者所在房间。
	RoomID string `json:"roomId"`
	// Link 职责：链路状态；使用：服务器汇总进快照；边界：字段见 LinkState。
	Link LinkState `json:"link"`
}

// LinkTraceRequest 是 link.trace（1303）请求 data。
// 职责：上报数据链路建立过程中的一步；使用：房主与成员在各 stage 结束时；边界：服务器只记监控时间线，不产生事件；字段长度由 link.Tracer 截断。
type LinkTraceRequest struct {
	// RoomID 职责：调用者当前房间；使用：服务器按房间过滤；边界：只有 stage=observe 时可以为空。
	RoomID string `json:"roomId"`
	// PeerUserID 职责：对端；使用：成员填房主 userId，房主填该成员 userId；边界：observe 或未知时为 0。
	PeerUserID int64 `json:"peerUserId"`
	// Stage 职责：阶段名；使用：见 constant Trace*；边界：不认识的 stage 服务器回复 20003。
	Stage string `json:"stage"`
	// Round 职责：打洞轮次；使用：punch_request / punch_dial；边界：其余为 0。
	Round int `json:"round"`
	// OK 职责：这一步是否成功；使用：时间线着色；边界：无。
	OK bool `json:"ok"`
	// DurationMs 职责：这一步耗时；使用：排障；边界：未知为 0。
	DurationMs int64 `json:"durationMs"`
	// Addr 职责：相关地址（拨号成功的地址、observed 地址、目标地址）；使用：排障；边界：最长 256。
	Addr string `json:"addr"`
	// Error 职责：本地错误文本；使用：排障；边界：原样截断到 512。
	Error string `json:"error"`
	// Detail 职责：补充说明，例如握手状态、端口下标、回复码；使用：排障；边界：最长 256。
	Detail string `json:"detail"`
}
