package multiplayer

// 文件职责：把各上报点的 link.trace（1303）补上 roomId、peerUserId 后交给会话的 link.Tracer。
// 使用：成员链路状态机直接调用 memberLink.trace；转发层与 ConnectionGater 的回调经 go c.traceForward / c.traceGaterReject；addrLoop 调用 traceObserve。
// 边界：Tracer.Trace 本身不阻塞；需要取控制器锁换算 userId 的回调一律另起 goroutine，保证不阻塞事件 goroutine、libp2p 握手与转发路径。

import (
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/forward"
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/peer"
)

// peerUserLocked 返回当前房间中 PeerID 对应的 userId（持锁调用）；未知为 0。
func (c *controller) peerUserLocked(id peer.ID) int64 {
	if c.room == nil {
		return 0
	}
	text := id.String()
	snapshot := c.room.view.Snapshot
	if snapshot.Host.PeerID == text {
		return snapshot.Host.UserID
	}
	for _, member := range snapshot.Members {
		if member.PeerID == text {
			return member.UserID
		}
	}
	for userID, pending := range c.room.pendingJoin {
		if pending == id {
			return userID
		}
	}
	for userID, leaving := range c.room.leaving {
		if leaving == id {
			return userID
		}
	}
	return 0
}

// traceForward 上报转发层记录（stream_open / stream_accept / target_probe）。
func (c *controller) traceForward(sess *session, event forward.TraceEvent) {
	c.mu.Lock()
	if c.sess != sess || c.room == nil {
		c.mu.Unlock()
		return
	}
	roomID := c.room.view.Snapshot.RoomID
	peerUserID := c.peerUserLocked(event.Peer)
	c.mu.Unlock()
	sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, PeerUserID: peerUserID, Stage: event.Stage, OK: event.OK,
		DurationMs: event.DurationMs, Addr: event.Addr, Error: event.Error, Detail: event.Detail})
}

// traceGaterReject 上报房主 ConnectionGater 拒绝的入站 PeerID；不在房间或不是房主时不上报。
func (c *controller) traceGaterReject(sess *session, id peer.ID, remoteAddr string) {
	c.mu.Lock()
	if c.sess != sess || c.room == nil || c.room.hostForwarder == nil {
		c.mu.Unlock()
		return
	}
	roomID := c.room.view.Snapshot.RoomID
	peerUserID := c.peerUserLocked(id)
	c.mu.Unlock()
	sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, PeerUserID: peerUserID, Stage: mpConstant.TraceGaterReject,
		Addr: remoteAddr, Detail: "peer=" + id.String()})
}

// traceObserve 上报每个观测节点的结果；roomId 为当前房间（不在房间时为空）。
func (c *controller) traceObserve(sess *session, results []p2p.ObserverResult) {
	if len(results) == 0 {
		return
	}
	c.mu.Lock()
	if c.sess != sess {
		c.mu.Unlock()
		return
	}
	roomID := ""
	if c.room != nil {
		roomID = c.room.view.Snapshot.RoomID
	}
	c.mu.Unlock()
	for _, result := range results {
		sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, Stage: mpConstant.TraceObserve, OK: result.OK,
			Diagnostic: result.Diagnostic, DurationMs: result.Duration.Milliseconds(), Addr: result.Addr, Error: result.Err, Detail: result.Transport + " " + observeDetail(result)})
		if recovery := result.Recovery; recovery != nil {
			sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, Stage: mpConstant.TraceObserve, OK: recovery.OK,
				Diagnostic: recovery.Diagnostic, DurationMs: recovery.Duration.Milliseconds(), Addr: recovery.Addr,
				Error: recovery.Err, Detail: "恢复TCP观测连接 " + observeDetail(*recovery)})
		}
	}
}

// observeDetail 返回 observe trace 的 detail：成功时写 observed 地址的归类（非公网时注明原因方向），失败时写观测节点与拨号地址。
func observeDetail(result p2p.ObserverResult) string {
	observer := "observer=" + result.Observer
	if result.Observer == "" {
		observer = "observer=无"
	}
	if result.OK {
		kind := p2p.ClassifyAddr(result.Addr)
		detail := observer + " observed=" + p2p.DescribeAddr(result.Addr)
		if kind != p2p.AddrPublic {
			detail += "（非公网观测，不能据此判断公网入站可达性或 UDP 阻断）"
		}
		return detail
	}
	return p2p.JoinLimited(observer+" 拨号地址：", result.Targets, ", ", mpConstant.TraceDetailMax)
}

// trace 上报成员链路的一步（对端为房主）。
func (l *memberLink) trace(stage string, round int, ok bool, started time.Time, addr string, errText string, detail string) {
	var duration int64
	if !started.IsZero() {
		duration = time.Since(started).Milliseconds()
	}
	l.sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: l.roomID, PeerUserID: l.hostUserID, Stage: stage, Round: round, OK: ok,
		DurationMs: duration, Addr: addr, Error: errText, Detail: detail})
}
