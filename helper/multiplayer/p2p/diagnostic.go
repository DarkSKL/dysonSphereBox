package p2p

// 文件职责：生成拨号和原生 DCUtR 的结构化证据；使用：service trace；边界：只复制真实输入，不推断未尝试传输的失败。

import (
	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	ma "github.com/multiformats/go-multiaddr"
	"time"
)

// AddrStrings 职责：复制 multiaddr 为诊断字符串；使用：监听和候选快照；边界：不探测可达性。
func AddrStrings(addrs []ma.Multiaddr) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}

// HolePunchTrace 职责：投影原生 DCUtR 时间、实际远端候选及进度；使用：控制器补房间身份后上报；边界：HostAddrs 只是库宣告，Start/Attempt 不算失败。
func HolePunchTrace(h *Host, evt *holepunch.Event) mpAPI.LinkTraceRequest {
	d := &mpAPI.TraceDiagnostic{Event: evt.Type, LocalPeerID: evt.Peer.String(), RemotePeerID: evt.Remote.String(),
		AtMs: evt.Timestamp / int64(time.Millisecond), ListenAddrs: AddrStrings(h.Network().ListenAddresses()), HostAddrs: AddrStrings(h.Addrs())}
	trace := mpAPI.LinkTraceRequest{Stage: mpConstant.TraceDCUtR, Detail: evt.Type, Result: "progress", Diagnostic: d}
	switch detail := evt.Evt.(type) {
	case *holepunch.StartHolePunchEvt:
		d.RemoteAddrs = append([]string{}, detail.RemoteAddrs...)
		d.RTTMs = float64(detail.RTT) / float64(time.Millisecond)
	case *holepunch.HolePunchAttemptEvt:
		trace.Round = detail.Attempt
	case *holepunch.EndHolePunchEvt:
		trace.OK, trace.Error, trace.DurationMs = detail.Success, detail.Error, detail.EllapsedTime.Milliseconds()
		trace.Result = "failure"
		if trace.OK {
			trace.Result = "success"
		}
	case *holepunch.DirectDialEvt:
		trace.OK, trace.Error, trace.DurationMs = detail.Success, detail.Error, detail.EllapsedTime.Milliseconds()
		trace.Result = "failure"
		if trace.OK {
			trace.Result = "success"
		}
	case *holepunch.ProtocolErrorEvt:
		trace.Error, trace.Result = detail.Error, "failure"
	}
	return trace
}

// DialDiagnostic 职责：保留逐地址拨号证据；使用：direct_dial/relay_dial；边界：最终由 Tracer 有界裁剪，不改变拨号。
func DialDiagnostic(report DialReport) *mpAPI.TraceDiagnostic {
	d := &mpAPI.TraceDiagnostic{Targets: AddrStrings(report.Targets), ConnectedAddr: report.ConnectedAddr()}
	for _, entry := range report.AddrErrors {
		d.Errors = append(d.Errors, mpAPI.TraceAddrError{Addr: entry.Addr.String(), Error: entry.Err})
	}
	return d
}
