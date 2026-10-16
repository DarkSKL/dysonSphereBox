package multiplayer

// 文件职责：定义联机协议的公共数据结构（端口声明、链路状态、成员、房间快照、大厅条目、房主连接信息、网络诊断）。
// 使用：请求/事件 data 结构、控制器本地状态与前端投影复用。
// 边界：字段名严格按协议设计消息定义第 1 节；不包含业务逻辑与校验。

// PortDecl 是房主声明的游戏端口。
// 职责：描述一个协议与连续端口段；使用：room.create 请求、RoomSnapshot、本地端口分配；边界：下标即数据链路握手的 port。
type PortDecl struct {
	// Proto 职责：tcp / udp / both；使用：协议匹配；边界：小写。
	Proto string `json:"proto"`
	// Port 职责：房主游戏端口起点；使用：目标端口 G 计算；边界：1–65535。
	Port int `json:"port"`
	// Count 职责：连续端口数；使用：段内偏移校验；边界：1–32。
	Count int `json:"count"`
	// Label 职责：界面显示名；使用：本地端口展示；边界：可选，最长 16 字符。
	Label string `json:"label"`
}

// LinkState 是成员到房主的数据链路状态。
// 职责：描述状态、路径、地址族、RTT 与失败码；使用：link.report 上报、成员列表与前端投影；边界：未连上时 path/ipFamily 为空。
type LinkState struct {
	// State 职责：connecting / connected / reconnecting / failed；使用：状态机；边界：取值见 constant。
	State string `json:"state"`
	// Path 职责：lan / direct / upnp / relay；使用：实际路径展示；边界：未连上为空，punch 仅历史解释。
	Path string `json:"path"`
	// IPFamily 职责：v4 / v6；使用：路径展示；边界：未连上为空。
	IPFamily string `json:"ipFamily"`
	// RTTMs 职责：往返时延毫秒；使用：延迟展示；边界：未知为 0。
	RTTMs int64 `json:"rttMs"`
	// Error 职责：数据链路失败码；使用：失败提示；边界：0 或 204xx。
	Error int `json:"error"`
}

// Member 是房间成员（不含房主）。
// 职责：描述成员身份、临时 PeerID 与链路；使用：RoomSnapshot.members 与 member.join 事件；边界：link 尚未上报时为 null。
type Member struct {
	// UserID 职责：账户 ID，房间内唯一；使用：踢人、链路投影；边界：number。
	UserID int64 `json:"userId"`
	// Nickname 职责：昵称；使用：界面展示；边界：只展示。
	Nickname string `json:"nickname"`
	// PeerID 职责：本次控制链路的临时 PeerID；使用：房主白名单；边界：随控制链路作废。
	PeerID string `json:"peerId"`
	// JoinedAtMs 职责：加入时间；使用：界面展示；边界：服务器时钟。
	JoinedAtMs int64 `json:"joinedAtMs"`
	// Link 职责：最近一次 link.report 的链路状态；使用：成员列表；边界：可为 null。
	Link *LinkState `json:"link"`
}

// RoomHost 是 RoomSnapshot 中的房主信息。
// 职责：描述房主身份；使用：界面展示与角色判断；边界：不含地址。
type RoomHost struct {
	// UserID 职责：房主账户 ID；使用：角色判断；边界：number。
	UserID int64 `json:"userId"`
	// Nickname 职责：房主昵称；使用：界面展示；边界：只展示。
	Nickname string `json:"nickname"`
	// PeerID 职责：房主临时 PeerID；使用：展示与校验；边界：地址经 HostEndpoint 下发。
	PeerID string `json:"peerId"`
}

// RoomSnapshot 是房间完整快照。
// 职责：房间状态的唯一来源；使用：after 事件与 room.snapshot 回复；边界：只应用 revision 更大的快照，不含任何网络地址。
type RoomSnapshot struct {
	// RoomID 职责：房间 ID；使用：房间请求参数；边界：r_ 前缀字符串。
	RoomID string `json:"roomId"`
	// Revision 职责：快照版本；使用：只应用更大的版本；边界：内容变化 +1。
	Revision int64 `json:"revision"`
	// Name 职责：房间名；使用：界面展示；边界：1–32 字符。
	Name string `json:"name"`
	// Visibility 职责：public / private；使用：界面展示；边界：private 只能邀请码加入。
	Visibility string `json:"visibility"`
	// HasPassword 职责：是否有密码；使用：界面展示；边界：不含密码本身。
	HasPassword bool `json:"hasPassword"`
	// MaxMembers 职责：人数上限（含房主）；使用：界面展示；边界：2–16。
	MaxMembers int `json:"maxMembers"`
	// Ports 职责：端口声明；使用：转发与本地端口；边界：创建后不可修改。
	Ports []PortDecl `json:"ports"`
	// CreatedAtMs 职责：创建时间；使用：界面展示；边界：服务器时钟。
	CreatedAtMs int64 `json:"createdAtMs"`
	// Host 职责：房主信息；使用：角色判断与展示；边界：不含地址。
	Host RoomHost `json:"host"`
	// Members 职责：成员列表（不含房主）；使用：白名单核对与展示；边界：可为空数组。
	Members []Member `json:"members"`
}

// RoomSummary 是大厅与预览中的房间条目。
// 职责：描述可加入房间的摘要；使用：lobby.list、lobby.change、room.resolve；边界：不含任何地址。
type RoomSummary struct {
	// RoomID 职责：房间 ID；使用：加入参数；边界：字符串。
	RoomID string `json:"roomId"`
	// Name 职责：房间名；使用：展示；边界：只展示。
	Name string `json:"name"`
	// HostNickname 职责：房主昵称；使用：展示；边界：只展示。
	HostNickname string `json:"hostNickname"`
	// MemberCount 职责：当前人数（含房主）；使用：展示；边界：大厅只列未满房间。
	MemberCount int `json:"memberCount"`
	// MaxMembers 职责：人数上限；使用：展示；边界：含房主。
	MaxMembers int `json:"maxMembers"`
	// HasPassword 职责：是否需要密码；使用：加入表单；边界：只展示。
	HasPassword bool `json:"hasPassword"`
	// Ports 职责：端口声明；使用：展示；边界：同 RoomSnapshot。
	Ports []PortDecl `json:"ports"`
	// CreatedAtMs 职责：创建时间；使用：展示；边界：服务器时钟。
	CreatedAtMs int64 `json:"createdAtMs"`
}

// NetInfo 是 peer.addrs 上报的网络诊断。
// 职责：描述 UPnP、IPv6、AutoNAT 与防火墙规则；使用：peer.addrs 请求、HostEndpoint.net、前端诊断；边界：AutoNAT unknown 不表示不可达。
type NetInfo struct {
	// UPnP 职责：是否拿到 UPnP/NAT-PMP 映射；使用：路径判定与诊断；边界：失败不阻塞。
	UPnP bool `json:"upnp"`
	// IPv6 职责：是否有全局 IPv6 地址；使用：诊断；边界：只看本机地址。
	IPv6 bool `json:"ipv6"`
	// AutoNATReachability 职责：官方 AutoNAT 对本机公网入站的聚合判断；使用：peer.addrs 与本地诊断；边界：unknown 不代表不可达，public 不保证每种传输均可达。
	AutoNATReachability string `json:"autoNatReachability"`
	// FirewallRule 职责：入站不会被 Windows 防火墙拦截（生效配置文件全关，或有允许当前 exe 的入站规则）；使用：20405 分类与诊断；边界：检测失败视为 true。
	FirewallRule bool `json:"firewallRule"`
}

// HostEndpoint 是房主连接信息。
// 职责：下发房主 PeerID、multiaddr 与网络诊断；使用：成员直连与打洞；边界：只下发给本房间成员，net 未上报时省略。
type HostEndpoint struct {
	// PeerID 职责：房主临时 PeerID；使用：拨号目标；边界：随房主控制链路作废。
	PeerID string `json:"peerId"`
	// Addrs 职责：房主最近一次 peer.addrs 的地址；使用：直连与打洞；边界：multiaddr 字符串。
	Addrs []string `json:"addrs"`
	// Net 职责：房主最近一次 peer.addrs 的 net；使用：失败分类；边界：可选。
	Net *NetInfo `json:"net,omitempty"`
}

// PunchPeer 是 link.punch after 发给房主的成员地址。
// 职责：描述被回拨成员的 PeerID 与地址；使用：房主同时拨号；边界：只在 link.punch after 中发给房主。
type PunchPeer struct {
	// PeerID 职责：成员临时 PeerID；使用：拨号目标；边界：必须在白名单中。
	PeerID string `json:"peerId"`
	// Addrs 职责：成员最近一次 peer.addrs 的地址；使用：拨号；边界：multiaddr 字符串。
	Addrs []string `json:"addrs"`
}

// UserInfo 是 session.open 回复中的当前用户。
// 职责：描述账户身份；使用：识别自己是否是事件中的成员或房主；边界：昵称取自 token。
type UserInfo struct {
	// UserID 职责：账户 ID；使用：身份判断；边界：number。
	UserID int64 `json:"userId"`
	// Nickname 职责：昵称；使用：展示；边界：只展示。
	Nickname string `json:"nickname"`
}

// SessionLimits 是 session.open 回复中的限制。
// 职责：描述人数、端口数与打洞轮数上限；使用：表单校验与打洞轮数；边界：以服务器下发为准。
type SessionLimits struct {
	// MaxMembers 职责：人数上限；使用：创建表单；边界：含房主。
	MaxMembers int `json:"maxMembers"`
	// MaxPorts 职责：端口声明条数上限；使用：创建表单；边界：协议规定 8。
	MaxPorts int `json:"maxPorts"`
	// PunchRounds 职责：打洞轮数；使用：成员链路；边界：0 时使用默认 3。
	PunchRounds int `json:"punchRounds"`
}
