package multiplayer

// 文件职责：将库原生 DCUtR 事件转为 WS 诊断。使用：控制器会话注册 tracer。边界：不发旧 link.punch、不控制同步时刻。

import (
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

// traceDCUtR 投影实际协议事件。职责：关联当前房间身份和原生结果；使用：host tracer 回调异步调用；边界：旧会话或未知游戏对端丢弃，事件不可包含 JWT。
func (c *controller) traceDCUtR(sess *session, evt *holepunch.Event) {
	c.mu.Lock()
	if c.sess != sess || c.room == nil {
		c.mu.Unlock()
		return
	}
	roomID := c.room.view.Snapshot.RoomID
	remoteUserID := c.peerUserLocked(evt.Remote)
	c.mu.Unlock()
	if remoteUserID == 0 {
		return
	}
	trace := p2p.HolePunchTrace(sess.host, evt)
	trace.RoomID, trace.PeerUserID = roomID, remoteUserID
	sess.tracer.Trace(trace)
}
