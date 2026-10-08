package forward

// 文件职责：转发层的 link.trace 记录（stream_open / stream_accept / target_probe）与握手状态名。
// 使用：HostForwarder / LocalForwarder 在握手与目标探测结束时调用 OnTrace，由控制器补上 roomId、peerUserId 后交给 link.Tracer。
// 边界：只描述事件，不发送；回调在转发 goroutine 中调用且调用时不持有转发器锁，回调必须不阻塞。

import (
	"strconv"

	mpConstant "DysonSphereBox/constant/multiplayer"

	"github.com/libp2p/go-libp2p/core/peer"
)

// TraceEvent 是转发层的一次 link.trace 记录。
// 职责：携带 stage 与结果；使用：OnTrace 回调；边界：Peer 为对端 PeerID，由控制器换算为 userId。
type TraceEvent struct {
	// Stage 职责：stream_open / stream_accept / target_probe；使用：LinkTraceRequest.stage；边界：取值见 constant。
	Stage string
	// Peer 职责：对端 PeerID；使用：换算 peerUserId；边界：成员侧为房主。
	Peer peer.ID
	// OK 职责：是否成功；使用：LinkTraceRequest.ok；边界：无。
	OK bool
	// DurationMs 职责：耗时；使用：LinkTraceRequest.durationMs；边界：未知为 0。
	DurationMs int64
	// Addr 职责：相关地址（目标地址）；使用：LinkTraceRequest.addr；边界：可为空。
	Addr string
	// Error 职责：错误文本；使用：LinkTraceRequest.error；边界：可为空。
	Error string
	// Detail 职责：握手状态、协议与端口下标；使用：LinkTraceRequest.detail；边界：可为空。
	Detail string
}

// StatusName 返回握手状态字节的名称。
func StatusName(status byte) string {
	switch status {
	case mpConstant.StatusOK:
		return "OK"
	case mpConstant.StatusUnauthorized:
		return "UNAUTHORIZED"
	case mpConstant.StatusBadPort:
		return "BAD_PORT"
	case mpConstant.StatusTargetRefused:
		return "TARGET_REFUSED"
	case mpConstant.StatusHostBusy:
		return "HOST_BUSY"
	}
	return "STATUS_" + strconv.Itoa(int(status))
}

// streamDetail 生成 detail：`<状态> <协议> port=<下标> offset=<偏移>`。
func streamDetail(status byte, proto string, header Header) string {
	return StatusName(status) + " " + proto + " port=" + strconv.Itoa(header.Port) + " offset=" + strconv.Itoa(header.Offset)
}

// firstOKKey 是“每个对端每个端口第一次 OK”的去重键。
type firstOKKey struct {
	peer  peer.ID
	proto string
	port  int
}
