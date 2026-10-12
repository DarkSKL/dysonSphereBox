package p2p

// 文件职责：只管理 WS 授权的原生中继入口、预约续期与地址撤回。
// 使用：控制器 session.open 后 ConfigureRelay/RunReservations，Collect 读取有效地址。
// 边界：不发现公共中继、不发送 JWT、不决定房间身份；游戏准入仍由 Gater 与 forward 校验。

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	client "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	ma "github.com/multiformats/go-multiaddr"
)

// RelayState 保存预约的只读诊断。职责：展示限额和有效期；使用：前端投影；边界：失效预约不宣告地址。
type RelayState struct {
	// PeerID 职责：指定中继身份；使用：排障；边界：不代表游戏授权。
	PeerID string `json:"peerId"`
	// ExpiresAtMs 职责：预约到期；使用：诊断；边界：未成功为 0。
	ExpiresAtMs int64 `json:"expiresAtMs"`
	// LimitDurationSeconds 职责：电路持续限额；使用：诊断；边界：由中继响应。
	LimitDurationSeconds int64 `json:"limitDurationSeconds"`
	// LimitDataBytes 职责：每方向字节限额；使用：诊断；边界：由中继响应。
	LimitDataBytes uint64 `json:"limitDataBytes"`
	// Error 职责：最近预约失败；使用：诊断；边界：成功清空。
	Error string `json:"error"`
}

// relayPolicy 隔离服务端能力和预约状态。职责：同步限额及更新通知；使用：Host；边界：初始化一次，随 host 关闭。
type relayPolicy struct {
	mu           sync.RWMutex
	capabilities mpAPI.Capabilities
	infos        []peer.AddrInfo
	entries      map[peer.ID][]ma.Multiaddr
	states       map[peer.ID]RelayState
	updates      chan struct{}
	onReserve    func(RelayState, time.Duration)
	cancel       context.CancelFunc
	workers      sync.WaitGroup
	// expiry 职责：预约过期撤回通知；使用：成功预约后；边界：续约替换，关闭时取消。
	expiry map[peer.ID]*time.Timer
}

// IsCircuit 识别受限地址。职责：区分中继路径；使用：拨号和 stream 准入；边界：nil 不是中继。
func IsCircuit(addr ma.Multiaddr) bool {
	if addr == nil {
		return false
	}
	_, err := addr.ValueForProtocol(ma.P_CIRCUIT)
	return err == nil
}

// ParseRelays 校验服务器入口。职责：只接受带唯一终端 PeerID 的单跳 TCP/QUIC；使用：ConfigureRelay；边界：任一非法入口整体拒绝，最多 8 条各 256 字节。
func ParseRelays(raw []string) ([]peer.AddrInfo, map[peer.ID][]ma.Multiaddr, error) {
	if len(raw) > 8 {
		return nil, nil, errors.New("中继入口数量超限")
	}
	entries := map[peer.ID][]ma.Multiaddr{}
	infos := []peer.AddrInfo{}
	indexes := map[peer.ID]int{}
	for _, text := range raw {
		addr, err := ma.NewMultiaddr(text)
		if err != nil || len(text) > 256 || IsCircuit(addr) {
			return nil, nil, fmt.Errorf("中继入口无效：%s", text)
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil || len(info.Addrs) != 1 {
			return nil, nil, fmt.Errorf("中继入口缺少终端 PeerID：%s", text)
		}
		transport := info.Addrs[0]
		protocols := transport.Protocols()
		valid := len(protocols) == 2 && protocols[1].Code == ma.P_TCP || len(protocols) == 3 && protocols[1].Code == ma.P_UDP && protocols[2].Code == ma.P_QUIC_V1
		if !valid || (protocols[0].Code != ma.P_IP4 && protocols[0].Code != ma.P_IP6 && protocols[0].Code != ma.P_DNS && protocols[0].Code != ma.P_DNS4 && protocols[0].Code != ma.P_DNS6) {
			return nil, nil, fmt.Errorf("中继入口传输无效：%s", text)
		}
		entries[info.ID] = ma.Unique(append(entries[info.ID], addr))
		if index, exists := indexes[info.ID]; exists {
			infos[index].Addrs = ma.Unique(append(infos[index].Addrs, transport))
		} else {
			indexes[info.ID] = len(infos)
			infos = append(infos, *info)
		}
	}
	return infos, entries, nil
}

// ConfigureRelay 安装本次 WS 能力。职责：配置唯一受信任中继列表；使用：session.open 后调用一次；边界：缺失能力/地址时不预约，不允许 circuit。
func (h *Host) ConfigureRelay(caps mpAPI.Capabilities, raw []string, onReserve func(RelayState, time.Duration)) error {
	if !caps.NativeDCUtR || !caps.RelayV2 || len(raw) == 0 {
		return nil
	}
	infos, entries, err := ParseRelays(raw)
	if err != nil {
		return err
	}
	h.relay.mu.Lock()
	h.relay.capabilities, h.relay.infos, h.relay.entries, h.relay.onReserve = caps, infos, entries, onReserve
	h.relay.mu.Unlock()
	h.Gater.setRelays(entries)
	return nil
}

// RelayEnabled 返回明确授权结果。职责：禁止默认使用中继；使用：成员建链；边界：能力与入口均须有效。
func (h *Host) RelayEnabled() bool {
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	return h.relay.capabilities.NativeDCUtR && h.relay.capabilities.RelayV2 && len(h.relay.infos) > 0
}

// RelayFallback 返回游戏流回退授权。职责：双侧有限流准入；使用：forward；边界：没有可用策略时 false。
func (h *Host) RelayFallback() bool {
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	return len(h.relay.infos) > 0 && h.relay.capabilities.RelayFallback
}

// RelayUpdates 返回预约变化通知。职责：触发地址重报；使用：addrLoop；边界：有界合并，不关闭通道。
func (h *Host) RelayUpdates() <-chan struct{} { return h.relay.updates }

// signalRelay 合并变化。职责：唤醒地址报告；使用：预约更新；边界：不阻塞网络 goroutine。
func (h *Host) signalRelay() {
	select {
	case h.relay.updates <- struct{}{}:
	default:
	}
}

// RelayStates 返回诊断副本。职责：暴露预约限额和错误；使用：状态投影；边界：不暴露私钥或凭证。
func (h *Host) RelayStates() []RelayState {
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	result := []RelayState{}
	for _, info := range h.relay.infos {
		state := h.relay.states[info.ID]
		state.PeerID = info.ID.String()
		result = append(result, state)
	}
	return result
}

// CircuitAddrs 返回尚未过期的成功预约地址。职责：生成 server 指定单跳路径；使用：peer.addrs；边界：不采用预约响应中的任意外部入口。
func (h *Host) CircuitAddrs() []string {
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	result := []string{}
	for _, info := range h.relay.infos {
		if h.relay.states[info.ID].ExpiresAtMs <= time.Now().UnixMilli() {
			continue
		}
		for _, entry := range h.relay.entries[info.ID] {
			result = append(result, entry.String()+"/p2p-circuit")
		}
	}
	return result
}

// TrustedCircuit 校验单跳入口和可选目标。职责：阻止未知中继和冒用 PeerID；使用：gater/成员拨号；边界：省略目标时由拨号上下文确定。
func trustedCircuit(addr ma.Multiaddr, target peer.ID, entries map[peer.ID][]ma.Multiaddr) bool {
	if !IsCircuit(addr) {
		return false
	}
	text := addr.String()
	parts := strings.Split(text, "/p2p-circuit")
	if len(parts) != 2 || (parts[1] != "" && parts[1] != "/p2p/"+target.String()) {
		return false
	}
	for _, values := range entries {
		for _, entry := range values {
			if parts[0] == entry.String() {
				return true
			}
		}
	}
	return false
}

// trustedCircuitConnection 校验握手后的中继身份。职责：兼容 DNS 入口已解析为 IP 的运行时地址；使用：安全连接 gater；边界：只准唯一已授权 relay PeerID，拨号入口仍须精确匹配。
func trustedCircuitConnection(addr ma.Multiaddr, entries map[peer.ID][]ma.Multiaddr) bool {
	if !IsCircuit(addr) {
		return false
	}
	parts := strings.Split(addr.String(), "/p2p-circuit")
	if len(parts) != 2 || parts[1] != "" {
		return false
	}
	prefix, err := ma.NewMultiaddr(parts[0])
	if err != nil {
		return false
	}
	info, err := peer.AddrInfoFromP2pAddr(prefix)
	if err != nil || len(info.Addrs) != 1 {
		return false
	}
	for _, proto := range info.Addrs[0].Protocols() {
		if proto.Code == ma.P_P2P || proto.Code == ma.P_CIRCUIT {
			return false
		}
	}
	return len(entries[info.ID]) > 0
}

// TrustedCircuit 返回当前 WS 授权的地址判断。职责：验证房主路径；使用：link；边界：未授权时一律 false。
func (h *Host) TrustedCircuit(addr ma.Multiaddr, target peer.ID) bool {
	h.relay.mu.RLock()
	defer h.relay.mu.RUnlock()
	return trustedCircuit(addr, target, h.relay.entries)
}

// RunReservations 启动每个指定 relay 的预约续约。职责：原生 Reserve、到期撤回与失败重试；使用：session.open 后；边界：无能力不产生任何网络请求，Host.Close 取消并等待。
func (h *Host) RunReservations() {
	h.relay.mu.Lock()
	if h.relay.cancel != nil || len(h.relay.infos) == 0 {
		h.relay.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	h.relay.cancel = cancel
	for _, info := range h.relay.infos {
		h.relay.workers.Add(1)
		go h.reserveLoop(ctx, info)
	}
	h.relay.mu.Unlock()
}

// reserveLoop 管理一个原生预约。职责：按响应到期前续约，失效时撤回；使用：RunReservations；边界：中继断开立即撤回，不重试公共节点。
func (h *Host) reserveLoop(ctx context.Context, info peer.AddrInfo) {
	defer h.relay.workers.Done()
	disconnected := make(chan struct{}, 1)
	notifiee := &network.NotifyBundle{DisconnectedF: func(n network.Network, conn network.Conn) {
		if conn.RemotePeer() == info.ID && len(n.ConnsToPeer(info.ID)) == 0 {
			select {
			case disconnected <- struct{}{}:
			default:
			}
		}
	}}
	h.Network().Notify(notifiee)
	defer h.Network().StopNotify(notifiee)
	defer h.storeReservation(RelayState{PeerID: info.ID.String()}, 0)
	for ctx.Err() == nil {
		if sw, ok := h.Network().(*swarm.Swarm); ok {
			sw.Backoff().Clear(info.ID)
		}
		started := time.Now()
		dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := h.Connect(dialCtx, info)
		var reservation *client.Reservation
		if err == nil {
			reservation, err = client.Reserve(dialCtx, reservationHost{Host: h.Host}, info)
		}
		cancel()
		state := RelayState{PeerID: info.ID.String()}
		delay := 10 * time.Second
		if err != nil {
			state.Error = err.Error()
		} else {
			state.ExpiresAtMs = reservation.Expiration.UnixMilli()
			state.LimitDurationSeconds, state.LimitDataBytes = int64(reservation.LimitDuration/time.Second), reservation.LimitData
			delay = max(time.Until(reservation.Expiration)*3/4, 100*time.Millisecond)
		}
		h.storeReservation(state, time.Since(started))
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-disconnected:
			timer.Stop()
			h.storeReservation(RelayState{PeerID: info.ID.String(), Error: "中继连接已断开"}, 0)
		}
	}
}

// storeReservation 原子更新预约。职责：限额/错误诊断和地址撤回；使用：续约 worker；边界：回调不持锁且不得阻塞。
func (h *Host) storeReservation(state RelayState, elapsed time.Duration) {
	id, _ := peer.Decode(state.PeerID)
	h.relay.mu.Lock()
	if timer := h.relay.expiry[id]; timer != nil {
		timer.Stop()
	}
	h.relay.states[id] = state
	if state.ExpiresAtMs > time.Now().UnixMilli() {
		h.relay.expiry[id] = time.AfterFunc(time.Until(time.UnixMilli(state.ExpiresAtMs)), h.signalRelay)
	}
	callback := h.relay.onReserve
	h.relay.mu.Unlock()
	h.signalRelay()
	if callback != nil {
		callback(state, elapsed)
	}
}

// Close 拆除整个临时数据面。职责：取消续约、关闭原生 host、等待 worker；使用：WS 结束；边界：幂等，不保留旧预约地址。
func (h *Host) Close() error {
	h.closeOnce.Do(func() {
		h.relay.mu.Lock()
		cancel := h.relay.cancel
		for _, timer := range h.relay.expiry {
			timer.Stop()
		}
		h.relay.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		h.closeErr = h.Host.Close()
		h.relay.workers.Wait()
	})
	return h.closeErr
}

// reservationHost 将应用 context 截止传入官方 Reserve stream。
// 职责：避免官方一分钟默认 stream deadline 阻塞会话拆除；使用：预约 worker；边界：不改协议数据。
type reservationHost struct{ host.Host }

// NewStream 职责：包装预约 stream 的截止时间；使用：官方 Reserve；边界：不改变其他 host 能力。
func (h reservationHost) NewStream(ctx context.Context, id peer.ID, ids ...protocol.ID) (network.Stream, error) {
	stream, err := h.Host.NewStream(ctx, id, ids...)
	if err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return stream, nil
	}
	return &reservationStream{Stream: stream, deadline: deadline}, nil
}

// reservationStream 职责：约束官方 Reserve 的 stream deadline；使用：预约读取；边界：不改变协议包。
type reservationStream struct {
	network.Stream
	deadline time.Time
}

// SetDeadline 职责：保留应用更早截止；使用：官方 Reserve；边界：不允许默认一分钟覆盖应用窗口。
func (stream *reservationStream) SetDeadline(deadline time.Time) error {
	if deadline.IsZero() || stream.deadline.Before(deadline) {
		deadline = stream.deadline
	}
	return stream.Stream.SetDeadline(deadline)
}

// BestConn 优先选择直接连接。职责：新 stream 选路与路径投影；使用：link/controller；边界：仅返回未关闭连接，受限回退可禁止。
func BestConn(h *Host, id peer.ID, fallback bool) network.Conn {
	var relayed network.Conn
	for _, conn := range h.Network().ConnsToPeer(id) {
		if conn.IsClosed() {
			continue
		}
		if !conn.Stat().Limited && !IsCircuit(conn.RemoteMultiaddr()) {
			return conn
		}
		if fallback {
			relayed = conn
		}
	}
	return relayed
}
