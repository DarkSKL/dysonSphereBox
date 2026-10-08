package multiplayer

// 文件职责：集中定义联机控制链路与数据链路的稳定常量（时长、协议 ID、握手状态、状态名、前端事件名）。
// 使用：helper/multiplayer/* 与 service/multiplayer 引用，避免魔法值散落。
// 边界：只定义常量，不包含逻辑；时长按方案与协议设计取值，测试通过各组件的可注入参数缩短。

import "time"

// 控制链路时长。
// 职责：心跳、读超时、请求与事件回复时限；使用：socket.Conn 默认参数；边界：两端阈值相同。
const (
	// ProtocolVersion 职责：session.open 的 protocolVersion；使用：打开会话；边界：协议升级时修改。
	ProtocolVersion = 1
	// PingInterval 职责：WS ping 间隔；使用：socket 心跳；边界：协议规定 5 秒。
	PingInterval = 5 * time.Second
	// ReadTimeout 职责：读超时判定断线；使用：socket 读截止；边界：协议规定 15 秒。
	ReadTimeout = 15 * time.Second
	// RequestTimeout 职责：请求等待回复时限；使用：超时以 20901 失败；边界：不关闭连接。
	RequestTimeout = 10 * time.Second
	// MaxFrameBytes 职责：单帧大小上限；使用：socket 读限制；边界：64 KiB。
	MaxFrameBytes = 64 * 1024
	// ReconnectMaxBackoff 职责：自动重连最大退避；使用：控制器断线重连；边界：从 1 秒倍增到该值。
	ReconnectMaxBackoff = 30 * time.Second
)

// 数据链路时长与限额。
// 职责：直连、打洞、重连、UDP 会话与资源上限；使用：helper/multiplayer/forward、link 与控制器；边界：按方案第 7 节取值。
const (
	// DirectDialTimeout 职责：直连超时；使用：成员直连房主；边界：5 秒。
	DirectDialTimeout = 5 * time.Second
	// PunchRoundTimeout 职责：每轮打洞超时；使用：同时拨号；边界：5 秒。
	PunchRoundTimeout = 5 * time.Second
	// PunchRoundInterval 职责：打洞轮间隔；使用：成员发起下一轮前等待；边界：1 秒。
	PunchRoundInterval = time.Second
	// DefaultPunchRounds 职责：打洞轮数默认值；使用：session.open 未下发 limits 时；边界：以服务器 limits.punchRounds 为准。
	DefaultPunchRounds = 3
	// ReconnectGiveUp 职责：数据链路重连放弃时限；使用：reconnecting 超过后进入 failed；边界：3 分钟。
	ReconnectGiveUp = 3 * time.Minute
	// RTTReportInterval 职责：connected 时 RTT 上报节流；使用：成员链路；边界：最多每 10 秒一次。
	RTTReportInterval = 10 * time.Second
	// HandshakeTimeout 职责：数据链路握手超时；使用：TCP/UDP stream 握手头；边界：5 秒。
	HandshakeTimeout = 5 * time.Second
	// TargetProbeTimeout 职责：目标地址单个候选拨号上限；使用：房主目标地址探测；边界：500 毫秒。
	TargetProbeTimeout = 500 * time.Millisecond
	// UDPIdleTimeout 职责：UDP 会话空闲关闭时限；使用：房主与成员 UDP 会话表；边界：60 秒。
	UDPIdleTimeout = 60 * time.Second
	// UDPQueuePackets 职责：UDP 发送队列报文数上限；使用：成员会话有界队列；边界：超出丢最旧。
	UDPQueuePackets = 64
	// UDPQueueBytes 职责：UDP 发送队列字节上限；使用：成员会话有界队列；边界：256 KiB。
	UDPQueueBytes = 256 * 1024
	// MaxUDPPayload 职责：UDP 报文最大长度；使用：报文帧编解码；边界：65507 字节。
	MaxUDPPayload = 65507
	// MaxTCPStreamsPerPeer 职责：每成员并发 TCP stream 上限；使用：房主 handler；边界：超限回 HOST_BUSY。
	MaxTCPStreamsPerPeer = 64
	// MaxUDPSessionsPerPeer 职责：每成员 UDP 会话上限；使用：房主 handler；边界：超限回 HOST_BUSY。
	MaxUDPSessionsPerPeer = 32
	// MaxRoomMembers 职责：房间人数上限（含房主）；使用：rcmgr 限额计算；边界：协议规定 16。
	MaxRoomMembers = 16
	// LocalPortMin 职责：成员本地端口 M 与 libp2p 监听端口随机下界；使用：端口分配；边界：避开常用低位端口。
	LocalPortMin = 20000
	// LocalPortMax 职责：随机端口上界；使用：端口分配；边界：不含该值。
	LocalPortMax = 60000
	// LocalPortRetries 职责：本地端口段绑定重试次数；使用：M 分配；边界：方案规定 20 次。
	LocalPortRetries = 20
	// NetworkCheckInterval 职责：网卡与地址变化检测间隔；使用：控制器地址重报与目标缓存清理；边界：只在线时运行。
	NetworkCheckInterval = 10 * time.Second
)

// 数据链路协议 ID 与握手状态。
// 职责：libp2p stream 协议与握手状态字节；使用：forward 包；边界：按方案第 7.6/7.7 节。
const (
	// StreamProtocolTCP 职责：TCP 转发协议 ID；使用：房主注册 handler、成员开 stream；边界：v1。
	StreamProtocolTCP = "/dsbox/tcp/1"
	// StreamProtocolUDP 职责：UDP 转发协议 ID；使用：房主注册 handler、成员开 stream；边界：v1。
	StreamProtocolUDP = "/dsbox/udp/1"
	// HandshakeVersion 职责：握手头 v 字段；使用：数据链路握手；边界：只接受 1。
	HandshakeVersion = 1

	// StatusOK 职责：握手成功；使用：房主回复状态字节；边界：之后进入数据阶段。
	StatusOK byte = 0
	// StatusUnauthorized 职责：远端不在白名单；使用：房主回复；边界：随后关闭 stream。
	StatusUnauthorized byte = 1
	// StatusBadPort 职责：端口下标、偏移或协议不合法；使用：房主回复；边界：随后关闭 stream。
	StatusBadPort byte = 2
	// StatusTargetRefused 职责：目标地址全部拒绝；使用：TCP 房主回复；边界：UDP 不会出现。
	StatusTargetRefused byte = 3
	// StatusHostBusy 职责：超过资源上限；使用：房主回复；边界：随后关闭 stream。
	StatusHostBusy byte = 4
)

// 端口声明协议、链路状态与路径名称。
// 职责：协议字段取值；使用：DTO 与状态机；边界：取值按消息定义第 1 节。
const (
	// ProtoTCP 职责：TCP 端口声明；使用：PortDecl.proto 与本地端口展示；边界：小写。
	ProtoTCP = "tcp"
	// ProtoUDP 职责：UDP 端口声明；使用：同上；边界：小写。
	ProtoUDP = "udp"
	// ProtoBoth 职责：TCP 与 UDP 同时声明；使用：PortDecl.proto；边界：本地端口展示拆为 tcp/udp 两条。
	ProtoBoth = "both"

	// LinkConnecting 职责：首次建链中；使用：LinkState.state；边界：内部依次直连、打洞。
	LinkConnecting = "connecting"
	// LinkConnected 职责：已连通；使用：LinkState.state；边界：可携带 rttMs。
	LinkConnected = "connected"
	// LinkReconnecting 职责：断开后重连中；使用：LinkState.state；边界：3 分钟后进入 failed。
	LinkReconnecting = "reconnecting"
	// LinkFailed 职责：建链失败；使用：LinkState.state；边界：error 给出分类。
	LinkFailed = "failed"

	// PathLAN 职责：局域网路径；使用：LinkState.path；边界：远端为私网/链路本地地址。
	PathLAN = "lan"
	// PathDirect 职责：公网直连；使用：LinkState.path；边界：非 UPnP 的公网地址。
	PathDirect = "direct"
	// PathUPnP 职责：UPnP 映射直连；使用：LinkState.path；边界：房主 net.upnp=true 的公网 IPv4。
	PathUPnP = "upnp"
	// PathPunch 职责：打洞连通；使用：LinkState.path；边界：同时拨号成功。
	PathPunch = "punch"

	// RoleHost 职责：房主角色；使用：room.snapshot 与本地角色；边界：小写。
	RoleHost = "host"
	// RoleMember 职责：成员角色；使用：同上；边界：小写。
	RoleMember = "member"
)

// link.trace（1303）阶段名与限制。
// 职责：trace 的 stage 取值、字段长度上限与本地限频；使用：link.Tracer 与各上报点；边界：取值按消息定义第 5 节 1303。
const (
	// TraceObserve 职责：连接观测节点完成 identify 或失败；使用：任意一方；边界：roomId 可为空，peerUserId 为 0。
	TraceObserve = "observe"
	// TracePortsOpen 职责：本地端口 M 全部打开或失败；使用：成员。
	TracePortsOpen = "ports_open"
	// TraceDirectDial 职责：直连结束；使用：成员。
	TraceDirectDial = "direct_dial"
	// TracePunchRequest 职责：link.punch 请求结束；使用：成员；边界：detail 写回复码；房主对称 NAT 跳过打洞时 ok=false、detail 为 "skipped: host_symmetric" 或 "skipped: both_symmetric"。
	TracePunchRequest = "punch_request"
	// TracePunchDial 职责：按 atMs 同时拨号结束；使用：双方。
	TracePunchDial = "punch_dial"
	// TraceConnected 职责：链路连通；使用：成员。
	TraceConnected = "connected"
	// TraceDisconnected 职责：链路断开；使用：成员。
	TraceDisconnected = "disconnected"
	// TraceFailed 职责：链路进入 failed；使用：成员。
	TraceFailed = "failed"
	// TraceStreamOpen 职责：数据链路握手非 OK，或每个端口第一次 OK；使用：成员。
	TraceStreamOpen = "stream_open"
	// TraceStreamAccept 职责：握手回复非 OK，或每个成员每个端口第一次 OK；使用：房主。
	TraceStreamAccept = "stream_accept"
	// TraceTargetProbe 职责：目标地址探测结果；使用：房主；边界：addr 为连通的目标地址。
	TraceTargetProbe = "target_probe"
	// TraceGaterReject 职责：ConnectionGater 拒绝入站 PeerID；使用：房主。
	TraceGaterReject = "gater_reject"

	// TraceAddrMax 职责：addr 最长字节数；使用：截断；边界：256。
	TraceAddrMax = 256
	// TraceErrorMax 职责：error 最长字节数；使用：原样截断；边界：512。
	TraceErrorMax = 512
	// TraceDetailMax 职责：detail 最长字节数；使用：截断；边界：256。
	TraceDetailMax = 256
	// TracePerMinute 职责：每会话每分钟最多发送的 trace 条数；使用：本地限频，超出直接丢弃；边界：与服务器每 socket 限额一致。
	TracePerMinute = 120
)

// 对称 NAT 打洞前置判断结果。
// 职责：标记跳过打洞的原因；使用：成员链路、link.trace detail、前端 links[].natBlocked；
// 边界：只有房主 natConsistent = false 才跳过；只有成员为 false 时照常打洞，不产生这些值；只在本地使用，不是协议字段。
const (
	// NATBlockedHost 职责：房主为对称 NAT、成员不是；使用：trace detail "skipped: host_symmetric"。
	NATBlockedHost = "host"
	// NATBlockedBoth 职责：房主与成员都是对称 NAT；使用：trace detail "skipped: both_symmetric"。
	NATBlockedBoth = "both"
)

// 前端连接状态与事件名。
// 职责：multiplayer:state 投影取值；使用：view 包与前端；边界：只在 Go 与前端之间使用。
const (
	// StateEventName 职责：状态投影事件名；使用：runtime.EventsEmit；边界：前端只监听该事件。
	StateEventName = "multiplayer:state"
	// ConnOffline 职责：未连接；使用：connection.state；边界：不自动重连。
	ConnOffline = "offline"
	// ConnConnecting 职责：首次连接中；使用：connection.state；边界：会话打开前。
	ConnConnecting = "connecting"
	// ConnOnline 职责：已在线；使用：connection.state；边界：session.open 成功后。
	ConnOnline = "online"
	// ConnReconnecting 职责：断线退避重连中；使用：connection.state；边界：终止原因不允许时不会进入。
	ConnReconnecting = "reconnecting"
)
