package p2p

// 文件职责：在会话既有 host 上独立观测 TCP 和 QUIC 的实际映射与错误。
// 使用：ObserveDetailed 在入房前和换网时调用；结果交给 Collect 与会话 trace。
// 边界：只重建专用观测 peer，不改变游戏 ACL、协议或回退；断连会删除库对旧连接的 native observed，快照不等于原生打洞候选。

import (
	"context"
	"fmt"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	ma "github.com/multiformats/go-multiaddr"
)

// ObserveDetailed 职责：返回每节点 TCP、QUIC 的独立观测；使用：控制器地址循环；边界：轮次可取消，同 host 串行、不同观测 peer 并行，顺序按输入节点和 TCP/QUIC 固定。
// 逻辑流：分组入口 => 获取轮次锁 => 各节点按传输重建连接 => 匹配该连接的原生 identify => 保留快照和逐地址错误。
func ObserveDetailed(ctx context.Context, h *Host, observers []string, timeout time.Duration) ([]Observation, []ObserverResult) {
	// 原生 AutoNAT 已提供整体公网入站判断；仅保留第一个 observer 作为 identify 地址源，避免双观测映射被误读为打洞准入条件。
	observers = FirstObserverAddrs(observers)
	infos, invalid := groupObservers(observers)
	details := []ObserverResult{}
	if len(observers) == 0 {
		return nil, []ObserverResult{{Err: "服务器未下发观测节点（session.open observers 为空）"}}
	}
	for _, raw := range invalid {
		details = append(details, ObserverResult{Err: "观测入口无法解析（需带 /p2p/<peerId>）：" + raw})
	}
	select {
	case h.observeGate <- struct{}{}:
		defer func() { <-h.observeGate }()
	case <-ctx.Done():
		for _, info := range infos {
			for _, transport := range []string{"tcp", "quic"} {
				_, result := h.observeOne(ctx, info, transport, timeout)
				details = append(details, result)
			}
		}
		return nil, details
	}
	// 每个 goroutine 只写自己的固定槽位，完成顺序不影响返回顺序。
	observed := make([][]Observation, len(infos))
	results := make([][]ObserverResult, len(infos))
	var wg sync.WaitGroup
	for i, info := range infos {
		wg.Add(1)
		go func(index int, info peer.AddrInfo) {
			defer wg.Done()
			tcpOK := false
			for _, transport := range []string{"tcp", "quic"} {
				observation, result := h.observeOne(ctx, info, transport, timeout)
				if observation.Addr != nil {
					observed[index] = append(observed[index], observation)
				}
				if transport == "tcp" {
					tcpOK = result.OK
				} else if !result.OK && tcpOK && ctx.Err() == nil && len(result.Diagnostic.Targets) > 0 {
					// QUIC 重建已断开 TCP：失败后恢复既有传输的 native observed，保留 QUIC 原始失败。
					restored, recovery := h.observeOne(ctx, info, "tcp", timeout)
					result.Recovery = &recovery
					if recovery.OK {
						observed[index][0] = restored
					}
				}
				results[index] = append(results[index], result)
			}
		}(i, info)
	}
	wg.Wait()
	observations := []Observation{}
	for i := range infos {
		observations = append(observations, observed[i]...)
		details = append(details, results[i]...)
	}
	// TCP、QUIC 对同一 PeerID 的重建会关闭前一条连接。只保留
	// 当前仍有连接承载的原生候选，防止上报已失效的 TCP 映射。
	return h.LiveObservations(observations), details
}

// observerTransport 职责：识别直接 TCP 或 QUIC-v1；使用：入口与真实连接核对；边界：不接受 circuit 或其他 UDP 协议。
func observerTransport(addr ma.Multiaddr) string {
	if addr == nil || IsCircuit(addr) {
		return ""
	}
	if _, err := addr.ValueForProtocol(ma.P_QUIC_V1); err == nil {
		return "quic"
	}
	if _, err := addr.ValueForProtocol(ma.P_TCP); err == nil {
		return "tcp"
	}
	return ""
}

// protectedObserver 职责：拒绝观测和游戏/中继身份重叠；使用：重建观测连接前；边界：自拨也拒绝，不添加或撤销 ACL。
func (h *Host) protectedObserver(id peer.ID) bool {
	if id == h.ID() || h.Gater.Allowed(id) {
		return true
	}
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	return len(h.relay.entries[id]) > 0
}

// beginObservation 职责：限定专用观测节点本轮可拨号传输；使用：重建连接前；边界：在 gater 锁内再次拒绝游戏/中继身份，避免 identify 把其他传输地址写回后被拨号。
func (gater *Gater) beginObservation(id peer.ID, transport string) bool {
	gater.mu.Lock()
	defer gater.mu.Unlock()
	if _, allowed := gater.allowed[id]; allowed || len(gater.relays[id]) > 0 {
		return false
	}
	gater.observations[id] = transport
	return true
}

// endObservation 职责：移除本轮观测传输限制；使用：observeOne defer；边界：不改动 allowed/revoked 或中继策略。
func (gater *Gater) endObservation(id peer.ID) {
	gater.mu.Lock()
	defer gater.mu.Unlock()
	delete(gater.observations, id)
}

// observeOne 职责：观测一个节点的一种传输；使用：ObserveDetailed；边界：订阅先于拨号，成功事件必须匹配实际连接 ID，旧 PeerID 事件不算结果。
func (h *Host) observeOne(ctx context.Context, info peer.AddrInfo, transport string, timeout time.Duration) (Observation, ObserverResult) {
	started := time.Now()
	target := peer.AddrInfo{ID: info.ID}
	for _, addr := range info.Addrs {
		if observerTransport(addr) == transport {
			target.Addrs = append(target.Addrs, addr)
		}
	}
	target.Addrs = ma.Unique(target.Addrs)
	d := &mpAPI.TraceDiagnostic{Transport: transport, LocalPeerID: h.ID().String(), RemotePeerID: info.ID.String(),
		AtMs: started.UnixMilli(), ListenAddrs: AddrStrings(h.Network().ListenAddresses()), HostAddrs: AddrStrings(h.Addrs()), Targets: AddrStrings(target.Addrs)}
	result := ObserverResult{Observer: info.ID.String(), Transport: transport, Targets: AddrStrings(target.Addrs), Diagnostic: d}
	finish := func(reason string) (Observation, ObserverResult) {
		result.Err, result.Duration = reason, time.Since(started)
		return Observation{}, result
	}
	if ctx.Err() != nil {
		return finish(ctx.Err().Error())
	}
	if h.protectedObserver(info.ID) {
		return finish("观测节点与本机/游戏/中继身份重复，拒绝重建该连接")
	}
	if len(target.Addrs) == 0 {
		return finish("服务器未下发 " + transport + " 观测入口，未尝试")
	}
	ids, ok := h.Host.(interface{ IDService() identify.IDService })
	if !ok {
		return finish("本机 host 不提供连接级原生 identify")
	}
	sub, err := h.EventBus().Subscribe(new(event.EvtPeerIdentificationCompleted))
	if err != nil {
		return finish("订阅 identify 事件失败：" + err.Error())
	}
	defer sub.Close()
	if !h.Gater.beginObservation(info.ID, transport) {
		return finish("观测节点与游戏/中继身份重复，拒绝重建该连接")
	}
	defer h.Gater.endObservation(info.ID)
	// 原生 Connect/DialPeer 会复用连接；仅清理这个专用观测 peer，防止 QUIC 误用 TCP。
	if err := h.Network().ClosePeer(info.ID); err != nil {
		return finish(fmt.Sprintf("关闭旧观测连接失败：%v", err))
	}
	h.Peerstore().ClearAddrs(info.ID)
	if sw, ok := h.Network().(*swarm.Swarm); ok {
		sw.Backoff().Clear(info.ID)
	}
	dialCtx, cancel := context.WithTimeout(network.WithForceDirectDial(ctx, "observer-"+transport), timeout)
	defer cancel()
	conn, report := h.dialObserver(dialCtx, target)
	d.Errors = DialDiagnostic(report).Errors
	d.Targets, d.ConnectedAddr = AddrStrings(report.Targets), report.ConnectedAddr()
	if report.Err != nil {
		return finish("连接观测节点失败：" + report.ErrorText(512))
	}
	if observerTransport(conn.RemoteMultiaddr()) != transport || observerTransport(conn.LocalMultiaddr()) != transport {
		return finish("实际连接传输与独立探测不一致")
	}
	identified := ids.IDService().IdentifyWait(conn)
	consume := func(raw interface{}) (Observation, ObserverResult, bool) {
		evt, ok := raw.(event.EvtPeerIdentificationCompleted)
		if !ok || evt.Peer != info.ID || evt.Conn == nil || evt.Conn.ID() != conn.ID() {
			return Observation{}, result, false
		}
		if evt.ObservedAddr == nil {
			observation, detail := finish("identify 已完成，但观测节点没有返回 observed 地址")
			return observation, detail, true
		}
		d.ObservedAddr = evt.ObservedAddr.String()
		if observerTransport(evt.ObservedAddr) != transport {
			observation, detail := finish("identify 返回的传输与独立探测不一致")
			return observation, detail, true
		}
		result.OK, result.Addr, result.Duration = true, evt.ObservedAddr.String(), time.Since(started)
		return Observation{Observer: info.ID, ConnID: conn.ID(), Addr: evt.ObservedAddr}, result, true
	}
	for {
		select {
		case raw, open := <-sub.Out():
			if !open {
				return finish("identify 事件订阅已关闭")
			}
			if observation, detail, done := consume(raw); done {
				return observation, detail
			}
		case <-identified:
			// IdentifyWait 在成功事件发出后关闭；排空已入队事件，否则视作该连接的 identify 未成功。
			for {
				select {
				case raw, open := <-sub.Out():
					if !open {
						return finish("identify 事件订阅已关闭")
					}
					if observation, detail, done := consume(raw); done {
						return observation, detail
					}
				default:
					return finish("该连接的 identify 未成功（无匹配连接的完成事件）")
				}
			}
		case <-dialCtx.Done():
			return finish("等待该连接的 identify：" + dialCtx.Err().Error())
		}
	}
}

// dialObserver 职责：返回原生拨号的具体连接和逐地址证据；使用：observeOne；边界：复用现有 transport，identify 仍由库执行，不新建 host 或发送自定义协议。
func (h *Host) dialObserver(ctx context.Context, info peer.AddrInfo) (network.Conn, DialReport) {
	// identify 会清除所有 TempAddrTTL，使用独立短 TTL 防止旧连接的异步更新删除本轮候选。
	h.Peerstore().AddAddrs(info.ID, info.Addrs, time.Minute)
	targets := ma.Unique(append(append([]ma.Multiaddr{}, info.Addrs...), h.Peerstore().Addrs(info.ID)...))
	watch := h.dials.watch(targets)
	started := time.Now()
	conn, err := h.Network().DialPeer(ctx, info.ID)
	report := DialReport{Targets: targets, Err: err, Duration: time.Since(started)}
	if err != nil {
		return nil, h.finishDialFailure(watch, report)
	}
	h.dials.unwatch(watch)
	report.Connected = conn.RemoteMultiaddr()
	return conn, report
}
