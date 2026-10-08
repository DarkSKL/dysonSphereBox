package multiplayer

// 文件职责：集中定义联机协议回复码、事件原因码、数据链路失败码与客户端本地码。
// 使用：socket、service/multiplayer 判断回复结果与重连策略，前端按同名码值映射文案。
// 边界：码值严格按协议设计错误码.md，不得新增线上码；本地码 209xx 永远不发往服务器。

// 回复与协议码。
// 职责：标识帧处理结果；使用：回复帧 code 与请求结果判断；边界：0 表示成功。
const (
	// CodeOK 职责：成功；使用：回复帧与事件回复；边界：唯一成功码。
	CodeOK = 0
	// CodeBadFrame 职责：帧无法解析；使用：收到非法帧时回复；边界：seqId/opcode 回 0。
	CodeBadFrame = 20001
	// CodeUnknownOpcode 职责：不认识的 opcode；使用：收到未知事件时回复；边界：不关闭连接。
	CodeUnknownOpcode = 20002
	// CodeProtocolVersion 职责：协议版本不支持；使用：session.open 失败时停止自动重连；边界：提示更新盒子。
	CodeProtocolVersion = 20006
	// CodeClientTooOld 职责：客户端版本过低；使用：session.open 失败时停止自动重连；边界：提示更新盒子。
	CodeClientTooOld = 20007
	// CodePeerIDInUse 职责：PeerID 已被占用；使用：session.open 失败时重新生成临时密钥；边界：只重试有限次。
	CodePeerIDInUse = 20008
)

// 会话终止原因。
// 职责：session.terminate 的 reason；使用：决定是否自动重连；边界：只有 20105 自动重连。
const (
	// ReasonTokenInvalid 职责：登录已失效；使用：停止重连并提示重新登录；边界：不自动重连。
	ReasonTokenInvalid = 20101
	// ReasonAccountDisabled 职责：账户被禁用；使用：停止重连；边界：不自动重连。
	ReasonAccountDisabled = 20102
	// ReasonNeedChangePassword 职责：需要修改密码；使用：停止重连并提示改密；边界：不自动重连。
	ReasonNeedChangePassword = 20103
	// ReasonReplaced 职责：账户在其他盒子上线；使用：停止重连；边界：不自动重连。
	ReasonReplaced = 20104
	// ReasonMaintenance 职责：服务器维护；使用：指数退避重连；边界：唯一自动重连的终止原因。
	ReasonMaintenance = 20105
)

// 客户端事件回复码。
// 职责：客户端回复事件时的失败码；使用：before 准备失败或 after 应用失败；边界：只出现在客户端回复帧。
const (
	// CodePrepareFailed 职责：before 准备失败；使用：房主无法放行成员等；边界：只用于 before。
	CodePrepareFailed = 20501
	// CodeApplyFailed 职责：after 应用失败；使用：随后发 room.snapshot 纠正；边界：只用于 after。
	CodeApplyFailed = 20502
)

// 数据链路失败码（LinkState.error）。
// 职责：成员本地判定的链路失败分类；使用：link.report 上报与前端提示；边界：不作为帧 code。
const (
	// LinkErrIPFamily 职责：双方 IP 协议族不匹配；使用：失败分类；边界：仅诊断提示。
	LinkErrIPFamily = 20401
	// LinkErrNATInconsistent 职责：NAT 映射随目标变化；使用：失败分类；边界：仅诊断提示。
	LinkErrNATInconsistent = 20402
	// LinkErrCGNAT 职责：运营商级 NAT；使用：失败分类；边界：仅诊断提示。
	LinkErrCGNAT = 20403
	// LinkErrPunchTimeout 职责：打洞超时；使用：其他失败的兜底分类；边界：仅诊断提示。
	LinkErrPunchTimeout = 20404
	// LinkErrHostFirewall 职责：房主防火墙拦截；使用：房主 firewallRule=false 时；边界：依赖 HostEndpoint.net。
	LinkErrHostFirewall = 20405
	// LinkErrRejected 职责：房主握手阶段拒绝；使用：拨号错误显示被 gater 拒绝时；边界：属于缺陷。
	LinkErrRejected = 20406
	// LinkErrTargetRefused 职责：房主游戏端口拒绝连接；使用：TCP 握手回 TARGET_REFUSED 时；边界：链路本身仍可用。
	LinkErrTargetRefused = 20407
)

// 客户端本地码。
// 职责：Go 后端与前端之间的失败码；使用：请求超时、断开、未连接；边界：永远不发往服务器。
const (
	// CodeRequestTimeout 职责：请求 10 秒内未收到回复；使用：socket.Conn.Request 超时；边界：不关闭连接。
	CodeRequestTimeout = 20901
	// CodeDisconnected 职责：控制链路已断开、请求未完成；使用：socket 断开时失败在途请求；边界：不跨 socket 重发。
	CodeDisconnected = 20902
	// CodeNotConnected 职责：控制链路未连接；使用：未在线时调用意图方法；边界：也用于握手被拒的本地提示。
	CodeNotConnected = 20903
)
