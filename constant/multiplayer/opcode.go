package multiplayer

// 文件职责：集中定义联机控制链路协议的 opcode。
// 使用：helper/multiplayer/socket 编解码帧、service/multiplayer 分发事件与发起请求时引用。
// 边界：数值严格按协议设计 README 第 2 节，不得自行修改；点分名称只用于注释与日志。

// 请求 opcode（客户端 → 服务器，1xxx）。
// 职责：标识客户端主动请求；使用：socket.Conn.Request 的 opcode 参数；边界：只能由客户端发送。
const (
	// OpSessionOpen 职责：session.open 打开会话；使用：控制链路建立后的首帧；边界：每条 socket 只发一次。
	OpSessionOpen = 1001
	// OpPeerAddrs 职责：peer.addrs 上报本机地址与网络诊断；使用：会话打开后及地址变化时；边界：整体替换上一次内容。
	OpPeerAddrs = 1002
	// OpLobbyList 职责：lobby.list 查询大厅；使用：大厅页刷新；边界：只读。
	OpLobbyList = 1101
	// OpLobbyWatch 职责：lobby.watch 订阅大厅变化；使用：大厅页激活/停用；边界：socket 断开即取消。
	OpLobbyWatch = 1102
	// OpRoomSnapshot 职责：room.snapshot 拉取房间快照；使用：事件应用失败后自行纠正；边界：只读。
	OpRoomSnapshot = 1201
	// OpRoomCreate 职责：room.create 创建房间；使用：创建表单提交；边界：结果只经 after 事件下发。
	OpRoomCreate = 1202
	// OpRoomResolve 职责：room.resolve 预览房间；使用：加入前展示房间信息；边界：不占名额。
	OpRoomResolve = 1203
	// OpRoomJoin 职责：room.join 加入房间；使用：加入表单提交；边界：结果只经 after 事件下发。
	OpRoomJoin = 1204
	// OpRoomInvite 职责：room.invite 生成邀请码；使用：房主点击邀请；边界：只有房主可调用。
	OpRoomInvite = 1205
	// OpRoomLeave 职责：room.leave 退出房间；使用：成员点击退出；边界：房主须用 room.close。
	OpRoomLeave = 1206
	// OpRoomClose 职责：room.close 关闭房间；使用：房主点击关房；边界：只有房主可调用。
	OpRoomClose = 1207
	// OpRoomKick 职责：room.kick 踢出成员；使用：房主点击踢人；边界：不能踢自己。
	OpRoomKick = 1208
	// OpLinkPunch 职责：link.punch 请求打洞协调；使用：成员直连失败后；边界：每成员每分钟 10 次。
	OpLinkPunch = 1301
	// OpLinkReport 职责：link.report 上报链路状态；使用：成员链路状态变化与 RTT 节流上报；边界：只由成员发送。
	OpLinkReport = 1302
	// OpLinkTrace 职责：link.trace 上报数据链路建立过程的单步记录；使用：房主与成员在各 stage 结束时；边界：只用于排障，不产生事件，服务器不认识（20002）时本会话停止发送。
	OpLinkTrace = 1303
)

// 事件 opcode（服务器 → 客户端，before 2xxx / after 3xxx）。
// 职责：标识服务器事件；使用：service/multiplayer 事件分发；边界：after 固定等于 before + 1000。
const (
	// EvSessionTerminateBefore 职责：session.terminate before；使用：记录终止开始；边界：不等待回复。
	EvSessionTerminateBefore = 2001
	// EvLobbyChangeBefore 职责：lobby.change before；使用：无需准备；边界：每秒最多一对。
	EvLobbyChangeBefore = 2101
	// EvRoomCreateBefore 职责：room.create before；使用：无需准备；边界：只发给房主。
	EvRoomCreateBefore = 2201
	// EvRoomCloseBefore 职责：room.close before；使用：停止接收新连接；边界：不会中止。
	EvRoomCloseBefore = 2202
	// EvRoomInviteBefore 职责：room.invite before；使用：无需准备；边界：只发给房主。
	EvRoomInviteBefore = 2203
	// EvMemberJoinBefore 职责：room.member.join before；使用：房主预先放行 PeerID；边界：服务器等房主回复。
	EvMemberJoinBefore = 2211
	// EvMemberLeaveBefore 职责：room.member.leave before；使用：停止接受该成员新 stream；边界：不会中止。
	EvMemberLeaveBefore = 2212
	// EvMemberLinkBefore 职责：room.member.link before；使用：无需准备；边界：只在 state/path/error 变化时出现。
	EvMemberLinkBefore = 2213
	// EvHostAddrsBefore 职责：room.host.addrs before；使用：无需准备；边界：不改变 revision。
	EvHostAddrsBefore = 2221
	// EvLinkPunchBefore 职责：link.punch before；使用：确认 host 就绪；边界：服务器等双方回复。
	EvLinkPunchBefore = 2301

	// EvSessionTerminateAfter 职责：session.terminate after；使用：按 reason 决定是否自动重连；边界：之后连接被关闭。
	EvSessionTerminateAfter = 3001
	// EvLobbyChangeAfter 职责：lobby.change after；使用：合并大厅条目；边界：只发给订阅者。
	EvLobbyChangeAfter = 3101
	// EvRoomCreateAfter 职责：room.create after；使用：房主注册转发 handler；边界：携带完整快照。
	EvRoomCreateAfter = 3201
	// EvRoomCloseAfter 职责：room.close after；使用：拆除房间与数据链路；边界：room 为 null。
	EvRoomCloseAfter = 3202
	// EvRoomInviteAfter 职责：room.invite after；使用：展示邀请码；边界：不改变 revision。
	EvRoomInviteAfter = 3203
	// EvMemberJoinAfter 职责：room.member.join after；使用：加入者开本地端口并连房主；边界：result 非 0 表示中止。
	EvMemberJoinAfter = 3211
	// EvMemberLeaveAfter 职责：room.member.leave after；使用：房主移出白名单、离开者拆除；边界：不会中止。
	EvMemberLeaveAfter = 3212
	// EvMemberLinkAfter 职责：room.member.link after；使用：更新成员链路状态；边界：携带完整快照。
	EvMemberLinkAfter = 3213
	// EvHostAddrsAfter 职责：room.host.addrs after；使用：重连中的成员改用新地址；边界：不改变 revision。
	EvHostAddrsAfter = 3221
	// EvLinkPunchAfter 职责：link.punch after；使用：双方在 atMs 同时拨号；边界：按接收者区分 data。
	EvLinkPunchAfter = 3301
)
