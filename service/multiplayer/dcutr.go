package multiplayer

// 文件职责：将库原生 DCUtR 事件转为 WS 诊断。使用：控制器会话注册 tracer。边界：不发旧 link.punch、不控制同步时刻。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"encoding/json"
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
	remoteUserID := int64(0)
	if c.room.view.Snapshot.Host.PeerID == evt.Remote.String() {
		remoteUserID = c.room.view.Snapshot.Host.UserID
	}
	for _, member := range c.room.view.Snapshot.Members {
		if member.PeerID == evt.Remote.String() {
			remoteUserID = member.UserID
		}
	}
	c.mu.Unlock()
	if remoteUserID == 0 {
		return
	}
	trace := mpAPI.LinkTraceRequest{RoomID: roomID, PeerUserID: remoteUserID, Stage: mpConstant.TraceDCUtR, Detail: evt.Type}
	switch detail := evt.Evt.(type) {
	case *holepunch.EndHolePunchEvt:
		trace.OK, trace.Error, trace.DurationMs = detail.Success, detail.Error, detail.EllapsedTime.Milliseconds()
	case *holepunch.DirectDialEvt:
		trace.OK, trace.Error, trace.DurationMs = detail.Success, detail.Error, detail.EllapsedTime.Milliseconds()
	case *holepunch.ProtocolErrorEvt:
		trace.Error = detail.Error
	case *holepunch.HolePunchAttemptEvt:
		trace.Round = detail.Attempt
	default:
		encoded, _ := json.Marshal(detail)
		trace.Detail += " " + string(encoded)
	}
	// Tracer 统一按 UTF-8 边界截断，避免本层切开多字节地址/错误。
	sess.tracer.Trace(trace)
}
