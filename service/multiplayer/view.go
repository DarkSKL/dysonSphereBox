package multiplayer

// 文件职责：定义 multiplayer:state 前端投影与意图方法返回值（方案第 6.6 节）。
// 使用：控制器每次状态变化生成 State 推送给前端；MultiplayerService 的意图方法返回 ActionResult。
// 边界：投影只含前端需要展示的信息，不含账户 token、私钥与任何成员地址。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	"DysonSphereBox/helper/multiplayer/forward"
	"DysonSphereBox/helper/multiplayer/p2p"
)

// ActionResult 是意图方法的返回值。
// 职责：携带结果码、提示与只读请求的数据；使用：前端按 code 映射文案；边界：code 为线上回复码或本地码 209xx，0 表示成功。
type ActionResult struct {
	// Code 职责：结果码；使用：前端文案映射；边界：0 成功。
	Code int `json:"code"`
	// Msg 职责：服务器提示或本地说明；使用：未收录的码直接展示；边界：可为空。
	Msg string `json:"msg"`
	// Data 职责：只读请求结果；使用：大厅列表、房间预览；边界：变更类请求为 nil。
	Data any `json:"data"`
}

// State 是 multiplayer:state 投影。
// 职责：前端渲染联机页面的唯一状态来源；使用：EventsEmit 推送与 GetState 恢复；边界：stateRevision 每次变化 +1。
type State struct {
	// StateRevision 职责：投影版本；使用：前端丢弃旧投影；边界：进程内单调递增。
	StateRevision int64 `json:"stateRevision"`
	// Connection 职责：控制链路状态；使用：连接区块；边界：见 ConnectionView。
	Connection ConnectionView `json:"connection"`
	// Account 职责：当前账户；使用：展示昵称；边界：不含 token。
	Account AccountView `json:"account"`
	// Session 职责：当前会话；使用：展示与限制；边界：离线时为 nil。
	Session *SessionView `json:"session"`
	// Lobby 职责：大厅；使用：大厅列表；边界：只含公开未满房间。
	Lobby LobbyView `json:"lobby"`
	// Room 职责：当前房间；使用：房间区块；边界：不在房间时为 nil。
	Room *RoomView `json:"room"`
	// Links 职责：数据链路；使用：成员链路展示；边界：房主为每个成员一条，成员只有自己一条。
	Links []LinkView `json:"links"`
	// LocalPorts 职责：成员本地端口；使用：复制给游戏；边界：房主为空。
	LocalPorts []forward.LocalPort `json:"localPorts"`
	// Network 职责：本机网络诊断；使用：连接失败建议；边界：targets 只在房主侧有值。
	Network NetworkView `json:"network"`
	// Pending 职责：进行中的意图；使用：按钮禁用；边界：取值为意图名。
	Pending []string `json:"pending"`
}

// ConnectionView 是控制链路状态。
// 职责：描述 offline/connecting/online/reconnecting 与最近错误；使用：前端连接提示；边界：lastErrorCode 可为握手 code（7/10001）、HTTP 404、会话码或终止原因。
type ConnectionView struct {
	// State 职责：连接状态；使用：展示；边界：取值见 constant。
	State string `json:"state"`
	// LastErrorCode 职责：最近一次导致断开或拒绝的码；使用：文案映射；边界：0 表示无。
	LastErrorCode int `json:"lastErrorCode"`
}

// AccountView 是当前账户。
type AccountView struct {
	// LoggedIn 职责：本地是否持有 token；使用：提示登录；边界：不保证 token 有效。
	LoggedIn bool `json:"loggedIn"`
	// UserID 职责：账户 ID；使用：识别自己；边界：会话打开前为 0。
	UserID int64 `json:"userId"`
	// Nickname 职责：昵称；使用：展示；边界：会话打开前为空。
	Nickname string `json:"nickname"`
}

// SessionView 是当前会话。
type SessionView struct {
	// SessionID 职责：会话 ID；使用：排障；边界：只读。
	SessionID string `json:"sessionId"`
	// PeerID 职责：本次临时 PeerID；使用：排障；边界：随控制链路作废。
	PeerID string `json:"peerId"`
	// ListenPort 职责：本机 libp2p 监听端口 H；使用：防火墙/UPnP 提示；边界：持久化端口号。
	ListenPort int `json:"listenPort"`
	// Limits 职责：服务器限制；使用：创建表单；边界：以服务器为准。
	Limits mpAPI.SessionLimits `json:"limits"`
	// Capabilities 职责：服务端数据面能力；使用：网络诊断；边界：缺失不授权。
	Capabilities mpAPI.Capabilities `json:"capabilities"`
	// Relays 职责：预约期限与资源限额；使用：中继诊断；边界：不包含凭证。
	Relays []p2p.RelayState `json:"relays"`
}

// LobbyView 是大厅状态。
type LobbyView struct {
	// Watching 职责：是否订阅大厅变化；使用：展示；边界：断线后重连自动恢复。
	Watching bool `json:"watching"`
	// Total 职责：总数；使用：分页；边界：合并 lobby.change 后近似值。
	Total int `json:"total"`
	// List 职责：房间条目；使用：大厅列表；边界：不含地址。
	List []mpAPI.RoomSummary `json:"list"`
}

// RoomView 是当前房间。
type RoomView struct {
	// Snapshot 职责：房间快照；使用：成员列表与端口表；边界：只应用 revision 更大的快照。
	Snapshot mpAPI.RoomSnapshot `json:"snapshot"`
	// Role 职责：host / member；使用：按钮显示；边界：小写。
	Role string `json:"role"`
	// Invite 职责：最近一次邀请码；使用：房主复制；边界：只有房主有。
	Invite *InviteView `json:"invite"`
}

// InviteView 是邀请码。
type InviteView struct {
	// Code 职责：邀请码；使用：复制；边界：XXXX-XXXX。
	Code string `json:"code"`
	// Link 职责：邀请链接；使用：复制；边界：原样展示。
	Link string `json:"link"`
	// ExpiresAtMs 职责：过期时间；使用：展示；边界：服务器时钟。
	ExpiresAtMs int64 `json:"expiresAtMs"`
}

// LinkView 是一条数据链路。
type LinkView struct {
	// UserID 职责：成员；使用：对应成员列表；边界：成员侧为自己。
	UserID int64 `json:"userId"`
	mpAPI.LinkState
	// Detail 职责：最近一次直连/打洞失败的错误文本；使用：链路失败提示下方展示真实原因；边界：只在成员自己的链路上有值，只存在本地投影，不进 link.report。
	Detail string `json:"detail"`
	// NATBlocked 职责：保留旧前端字段；使用：兼容已有投影解码；边界：v2 不因 NAT 观测跳过原生 DCUtR，始终为空。
	NATBlocked string `json:"natBlocked"`
}

// NetworkView 是本机网络诊断。
type NetworkView struct {
	// UPnP 职责：是否拿到端口映射；使用：诊断；边界：失败不阻塞。
	UPnP bool `json:"upnp"`
	// IPv6 职责：是否有全局 IPv6；使用：诊断；边界：只看本机。
	IPv6 bool `json:"ipv6"`
	// NATConsistent 职责：两个观测节点看到的 QUIC 映射是否一致；使用：诊断、对称 NAT 提示；边界：拿不到两个节点的 QUIC 映射时为 null。
	NATConsistent *bool `json:"natConsistent"`
	// NATMappings 职责：各观测节点看到的 QUIC 映射 `ip:port`；使用：对称 NAT 提示中显示两个映射；边界：可为空。
	NATMappings []string `json:"natMappings"`
	// ObservedAddrs 职责：观测节点看到的地址；使用：诊断；边界：可为空。
	ObservedAddrs []string `json:"observedAddrs"`
	// ReportedAddrs 职责：本机最近一次 peer.addrs 上报的地址（带私网/公网归类）；使用：网络诊断区；边界：离线为空。
	ReportedAddrs []p2p.ClassifiedAddr `json:"reportedAddrs"`
	// Observed 职责：observed 地址（带归类）；使用：网络诊断区；边界：可为空。
	Observed []p2p.ClassifiedAddr `json:"observed"`
	// ObserveErrors 职责：拿不到 observed 地址的原因（每个观测节点一条）；使用：网络诊断区；边界：成功的节点不出现。
	ObserveErrors []string `json:"observeErrors"`
	// HostAddrs 职责：房主下发的地址（HostEndpoint.addrs，带归类）；使用：成员网络诊断区；边界：只在成员侧有值。
	HostAddrs []p2p.ClassifiedAddr `json:"hostAddrs"`
	// HostNoPublic 职责：成员侧房主地址里没有公网地址；使用：提示检查 observer-public-ip、安全组与 UPnP；边界：房主侧恒为 false。
	HostNoPublic bool `json:"hostNoPublic"`
	// FirewallRule 职责：入站不会被 Windows 防火墙拦截；使用：诊断；边界：每个会话检测一次，检测失败为 true。
	FirewallRule bool `json:"firewallRule"`
	// Targets 职责：房主目标地址探测；使用：房主界面；边界：成员为空。
	Targets []forward.TargetState `json:"targets"`
}
