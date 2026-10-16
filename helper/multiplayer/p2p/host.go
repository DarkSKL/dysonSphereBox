package p2p

// 文件职责：构建联机数据面的 libp2p host：临时 Ed25519 密钥、QUIC+TCP 双栈监听、NATPortMap、显式 rcmgr 限额与白名单 ConnectionGater。
// 使用：service/multiplayer 控制器每次控制链路建立时调用 NewHost，控制链路断开时 Close。
// 边界：密钥只在内存中，不落盘；原生 relay/DCUtR 预装但由 WS 策略授权；不启用中继服务、DHT、mDNS，不注册游戏协议。

import (
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"time"

	mpConstant "DysonSphereBox/constant/multiplayer"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/control"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// Host 是一次控制链路生命周期内的 libp2p host。
// 职责：持有 host 与白名单 gater；使用：forward 注册 handler、link 拨号；边界：Close 后不可复用。
type Host struct {
	host.Host
	// Gater 职责：入站白名单；使用：房主按成员事件维护，成员只放行房主；边界：出站一律放行。
	Gater *Gater
	// Port 职责：本次监听端口号；使用：日志与持久化；边界：测试监听 0 端口时为 0。
	Port int
	// dials 职责：逐地址拨号错误收集；使用：ConnectReport；边界：只做诊断。
	dials *dialRecorder
	// relay 职责：指定中继授权和预约；使用：session.open 配置；边界：默认禁止 circuit。
	relay *relayPolicy
	// punches 职责：原生 DCUtR 事件；使用：控制器 trace；边界：回调不得阻塞。
	punches *punchTracer
	// closeOnce 职责：幂等拆除；使用：Close；边界：关闭后不可复用。
	closeOnce sync.Once
	// closeErr 职责：首次关闭结果；使用：重复 Close 返回；边界：由 closeOnce 同步。
	closeErr error
	// observeGate 职责：串行化同一 host 的观测轮次；使用：ObserveDetailed 可取消获取；边界：不锁住游戏或中继拨号。
	observeGate chan struct{}
	// observedFactory 职责：将指定 observer 证实的当前公网地址提供给官方 AutoNAT 与地址宣告；使用：观测结果更新；边界：仅在对应原生连接存活时生效。
	observedFactory *observedAddrFactory
}

// ListenAddrs 返回监听端口 port 的 IPv4/IPv6 QUIC 与 TCP 监听地址。
// 职责：生成 H 端口监听 multiaddr；使用：NewHost 默认参数；边界：监听所有网卡。
func ListenAddrs(port int) []string {
	return []string{
		fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", port),
		fmt.Sprintf("/ip6/::/udp/%d/quic-v1", port),
		fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port),
		fmt.Sprintf("/ip6/::/tcp/%d", port),
	}
}

// NewHost 用新的临时密钥启动 host。
// 职责：按方案第 7.1 节配置 libp2p；使用：控制器拨号前；边界：listen 为空时监听 ListenAddrs(port)，全部监听失败返回错误由调用方换端口重试。
func NewHost(port int, listen []string) (*Host, error) {
	key, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	if len(listen) == 0 {
		listen = ListenAddrs(port)
	}
	manager, err := newResourceManager()
	if err != nil {
		return nil, err
	}
	gater := NewGater()
	// DisableMetrics 只关闭 prometheus 指标；逐地址拨号错误另由 dialRecorder 经 swarm MetricsTracer 收集
	dials := newDialRecorder()
	punches := &punchTracer{}
	observedFactory := &observedAddrFactory{}
	h, err := libp2p.New(
		libp2p.SwarmOpts(swarm.WithMetricsTracer(dials)),
		libp2p.Identity(key),
		libp2p.ListenAddrStrings(listen...),
		libp2p.Transport(libp2pquic.NewTransport),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.NATPortMap(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(holepunch.WithTracer(punches)),
		libp2p.AddrsFactory(observedFactory.addrs),
		libp2p.ResourceManager(manager),
		libp2p.ConnectionGater(gater),
		libp2p.DisableMetrics(),
	)
	if err != nil {
		_ = manager.Close()
		return nil, err
	}
	observedFactory.setNetwork(h.Network())
	return &Host{Host: h, Gater: gater, Port: port, dials: dials, punches: punches, observeGate: make(chan struct{}, 1),
		observedFactory: observedFactory,
		relay:           &relayPolicy{entries: map[peer.ID][]ma.Multiaddr{}, states: map[peer.ID]RelayState{}, expiry: map[peer.ID]*time.Timer{}, updates: make(chan struct{}, 1)}}, nil
}

// newResourceManager 构建显式限额的资源管理器。
// 职责：按支持的人数硬上界 × 每人 64 TCP + 32 UDP stream 计算容量；使用：NewHost；边界：实际准入由服务端动态限制，不使用自动缩放默认值。
func newResourceManager() (network.ResourceManager, error) {
	perPeerStreams := mpConstant.MaxTCPStreamsPerPeer + mpConstant.MaxUDPSessionsPerPeer + 16
	peers := mpConstant.MaxRoomMembers + 8 // 成员 + 观测节点与余量
	limits := rcmgr.PartialLimitConfig{
		System: rcmgr.ResourceLimits{
			Conns: rcmgr.LimitVal(peers * 4), ConnsInbound: rcmgr.LimitVal(peers * 4), ConnsOutbound: rcmgr.LimitVal(peers * 4),
			Streams: rcmgr.LimitVal(peers * perPeerStreams), StreamsInbound: rcmgr.LimitVal(peers * perPeerStreams),
			StreamsOutbound: rcmgr.LimitVal(peers * perPeerStreams), FD: rcmgr.LimitVal(peers * 4),
			Memory: rcmgr.LimitVal64(512 << 20),
		},
		Transient: rcmgr.ResourceLimits{
			Conns: rcmgr.LimitVal(peers * 2), ConnsInbound: rcmgr.LimitVal(peers * 2), ConnsOutbound: rcmgr.LimitVal(peers * 2),
			Streams: rcmgr.LimitVal(peers * 4), StreamsInbound: rcmgr.LimitVal(peers * 4), StreamsOutbound: rcmgr.LimitVal(peers * 4),
			FD: rcmgr.LimitVal(peers * 2), Memory: rcmgr.LimitVal64(64 << 20),
		},
		PeerDefault: rcmgr.ResourceLimits{
			Conns: 8, ConnsInbound: 8, ConnsOutbound: 8,
			Streams: rcmgr.LimitVal(perPeerStreams), StreamsInbound: rcmgr.LimitVal(perPeerStreams),
			StreamsOutbound: rcmgr.LimitVal(perPeerStreams), FD: 8, Memory: rcmgr.LimitVal64(64 << 20),
		},
	}
	concrete := limits.Build(rcmgr.DefaultLimits.AutoScale())
	return rcmgr.NewResourceManager(rcmgr.NewFixedLimiter(concrete))
}

// Gater 是按白名单放行入站连接的 ConnectionGater。
// 职责：入站只放行白名单 PeerID，出站一律放行；使用：Host.Gater；边界：白名单只由控制链路事件维护。
type Gater struct {
	mu      sync.RWMutex
	allowed map[peer.ID]struct{}
	// revoked 职责：禁止原生后台重新拨被撤销成员；使用：Remove/Clear；边界：重新 Allow 时恢复。
	revoked map[peer.ID]bool
	// observations 职责：专用观测 peer 当前允许探测的传输；使用：独立探测期间过滤 identify 并发写回的异传输地址；边界：不变更游戏白名单。
	observations map[peer.ID]string
	// autoNATSources 职责：session.open 指定观测节点的公网来源 IP；使用：放行其独立临时 PeerID 的 AutoNAT 回拨基础连接；边界：不授予游戏 Allowed，随会话 host 销毁。
	autoNATSources map[string]struct{}
	// relays 职责：基础设施入口；使用：握手准入；边界：不进入游戏 Allowed/List。
	relays   map[peer.ID][]ma.Multiaddr
	onReject func(id peer.ID, remoteAddr string)
}

// SetOnReject 设置入站拒绝回调。
// 职责：gater_reject trace；使用：控制器在 host 建立后设置；边界：回调在 libp2p 握手路径中调用，必须立即返回（自行另起 goroutine）。
func (gater *Gater) SetOnReject(callback func(id peer.ID, remoteAddr string)) {
	gater.mu.Lock()
	gater.onReject = callback
	gater.mu.Unlock()
}

// NewGater 创建空白名单 gater。
func NewGater() *Gater {
	return &Gater{allowed: map[peer.ID]struct{}{}, revoked: map[peer.ID]bool{}, relays: map[peer.ID][]ma.Multiaddr{}, observations: map[peer.ID]string{}, autoNATSources: map[string]struct{}{}}
}

// Allow 把 PeerID 加入白名单。
func (gater *Gater) Allow(id peer.ID) {
	gater.mu.Lock()
	gater.allowed[id] = struct{}{}
	delete(gater.revoked, id)
	gater.mu.Unlock()
}

// Remove 把 PeerID 移出白名单。
func (gater *Gater) Remove(id peer.ID) {
	gater.mu.Lock()
	delete(gater.allowed, id)
	gater.revoked[id] = true
	gater.mu.Unlock()
}

// Clear 清空白名单。
func (gater *Gater) Clear() {
	gater.mu.Lock()
	for id := range gater.allowed {
		gater.revoked[id] = true
	}
	gater.allowed = map[peer.ID]struct{}{}
	gater.mu.Unlock()
}

// List 返回白名单快照。
func (gater *Gater) List() []peer.ID {
	gater.mu.RLock()
	defer gater.mu.RUnlock()
	result := make([]peer.ID, 0, len(gater.allowed))
	for id := range gater.allowed {
		result = append(result, id)
	}
	return result
}

// Allowed 判断 PeerID 是否在白名单。
func (gater *Gater) Allowed(id peer.ID) bool {
	gater.mu.RLock()
	defer gater.mu.RUnlock()
	_, ok := gater.allowed[id]
	return ok
}

// InterceptPeerDial 出站拨号一律放行。
func (gater *Gater) InterceptPeerDial(id peer.ID) bool {
	gater.mu.RLock()
	defer gater.mu.RUnlock()
	return !gater.revoked[id]
}

// InterceptAddrDial 出站拨号一律放行。
func (gater *Gater) InterceptAddrDial(id peer.ID, addr ma.Multiaddr) bool {
	gater.mu.RLock()
	defer gater.mu.RUnlock()
	if gater.revoked[id] {
		return false
	}
	if transport := gater.observations[id]; transport != "" && observerTransport(addr) != transport {
		return false
	}
	if !IsCircuit(addr) {
		return true
	}
	_, allowed := gater.allowed[id]
	return allowed && trustedCircuit(addr, id, gater.relays)
}

// InterceptAccept 入站连接在握手后再判断。
func (gater *Gater) InterceptAccept(network.ConnMultiaddrs) bool { return true }

// InterceptSecured 握手完成后入站只放行白名单；拒绝时通知 onReject。
func (gater *Gater) InterceptSecured(direction network.Direction, id peer.ID, addrs network.ConnMultiaddrs) bool {
	gater.mu.RLock()
	_, ok := gater.allowed[id]
	if gater.revoked[id] {
		ok = false
	} else if addrs != nil && IsCircuit(addrs.RemoteMultiaddr()) {
		ok = ok && trustedCircuitConnection(addrs.RemoteMultiaddr(), gater.relays)
	} else if direction == network.DirOutbound || len(gater.relays[id]) > 0 || direction == network.DirInbound && addrs != nil && gater.autoNATSource(addrs.RemoteMultiaddr()) {
		ok = true
	}
	onReject := gater.onReject
	gater.mu.RUnlock()
	if !ok && onReject != nil {
		remote := ""
		if addrs != nil && addrs.RemoteMultiaddr() != nil {
			remote = addrs.RemoteMultiaddr().String()
		}
		onReject(id, remote)
	}
	return ok
}

// InterceptUpgraded 升级后的连接一律放行。
func (gater *Gater) InterceptUpgraded(conn network.Conn) (bool, control.DisconnectReason) {
	gater.mu.RLock()
	defer gater.mu.RUnlock()
	if gater.revoked[conn.RemotePeer()] {
		return false, 0
	}
	if IsCircuit(conn.RemoteMultiaddr()) {
		_, allowed := gater.allowed[conn.RemotePeer()]
		return allowed && trustedCircuitConnection(conn.RemoteMultiaddr(), gater.relays), 0
	}
	if conn.Stat().Direction == network.DirInbound && gater.autoNATSource(conn.RemoteMultiaddr()) {
		return true, 0
	}
	return true, 0
}

// SetAutoNATSources 职责：原子替换允许官方回拨的来源 IP；使用：session.open 下发指定 observer 后；边界：只接收公网 IP，不扩大游戏白名单。
func (gater *Gater) SetAutoNATSources(ips []net.IP) {
	gater.mu.Lock()
	defer gater.mu.Unlock()
	gater.autoNATSources = map[string]struct{}{}
	for _, ip := range ips {
		if ip != nil {
			gater.autoNATSources[ip.String()] = struct{}{}
		}
	}
}

// autoNATSource 职责：比对入站连接远端 IP；使用：secured/upgraded；边界：调用方已持 gater 锁，relay 路径不得进入。
func (gater *Gater) autoNATSource(addr ma.Multiaddr) bool {
	if addr == nil || IsCircuit(addr) {
		return false
	}
	ip, err := manet.ToIP(addr)
	if err != nil {
		return false
	}
	_, ok := gater.autoNATSources[ip.String()]
	return ok
}

// setRelays 分离基础设施准入。职责：接入 WS 指定入口；使用：ConfigureRelay；边界：不授予游戏 Allowed。
func (gater *Gater) setRelays(entries map[peer.ID][]ma.Multiaddr) {
	gater.mu.Lock()
	gater.relays = entries
	gater.mu.Unlock()
}
