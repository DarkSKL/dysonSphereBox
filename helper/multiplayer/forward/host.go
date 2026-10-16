package forward

// 文件职责：房主侧 /dsbox/tcp/1 与 /dsbox/udp/1 stream handler：白名单与端口校验、目标地址拨号、UDP (PeerID,sid) 会话表与接回、资源上限。
// 使用：控制器在 room.create after 后 Start，room.close after 或控制链路断开时 Stop；成员事件调用 Block/Remove。
// 边界：只发往房主声明过的端口 G，目标地址只在本机候选中选择；成员只能传端口下标与偏移，不能指定地址。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

// udpKey 是房主 UDP 会话键。
// 职责：(远端 PeerID, sid)；使用：会话查找与接回；边界：其他成员猜中 sid 也接不走别人的会话。
type udpKey struct {
	peer peer.ID
	sid  string
}

// HostForwarder 是房主的数据链路转发器。
// 职责：注册 handler、维护每成员计数与 UDP 会话；使用：控制器每个房间一个；边界：Stop 后不可复用。
type HostForwarder struct {
	host    *p2p.Host
	ports   []mpAPI.PortDecl
	targets *targetCache

	mu       sync.Mutex
	blocked  map[peer.ID]bool
	closing  bool
	stopped  bool
	tcpCount map[peer.ID]int
	udpCount map[peer.ID]int
	sessions map[udpKey]*hostUDPSession
	streams  map[peer.ID]map[network.Stream]struct{}

	// IdleTimeout 职责：UDP 会话空闲关闭时限；使用：默认 60 秒；边界：测试可缩短。
	IdleTimeout time.Duration
	// OnTrace 职责：stream_accept / target_probe 记录回调；使用：控制器转成 link.trace；边界：Start 前设置，调用时不持锁，必须不阻塞。
	OnTrace func(TraceEvent)
	// OnTargets 职责：主动探测使目标状态变化时通知；使用：控制器推送新投影；边界：Start 前设置，调用时不持锁，必须不阻塞。
	OnTargets func()
	okSeen    map[firstOKKey]bool
	done    chan struct{}
}

// NewHostForwarder 创建房主转发器。
func NewHostForwarder(host *p2p.Host, ports []mpAPI.PortDecl) *HostForwarder {
	return &HostForwarder{
		host: host, ports: append([]mpAPI.PortDecl(nil), ports...), targets: newTargetCache(),
		blocked: map[peer.ID]bool{}, tcpCount: map[peer.ID]int{}, udpCount: map[peer.ID]int{},
		sessions: map[udpKey]*hostUDPSession{}, streams: map[peer.ID]map[network.Stream]struct{}{},
		IdleTimeout: mpConstant.UDPIdleTimeout, okSeen: map[firstOKKey]bool{}, done: make(chan struct{}),
	}
}

// traceAccept 记录 stream_accept：非 OK 每次都记，OK 只记每个成员每个端口的第一次。
func (forwarder *HostForwarder) traceAccept(remote peer.ID, status byte, proto string, header Header, started time.Time) {
	if forwarder.OnTrace == nil {
		return
	}
	if status == mpConstant.StatusOK {
		key := firstOKKey{peer: remote, proto: proto, port: header.Port}
		forwarder.mu.Lock()
		seen := forwarder.okSeen[key]
		forwarder.okSeen[key] = true
		forwarder.mu.Unlock()
		if seen {
			return
		}
	}
	forwarder.OnTrace(TraceEvent{Stage: mpConstant.TraceStreamAccept, Peer: remote, OK: status == mpConstant.StatusOK,
		DurationMs: time.Since(started).Milliseconds(), Detail: streamDetail(status, proto, header)})
}

// traceProbe 记录 target_probe（只在真正探测候选地址时）。
func (forwarder *HostForwarder) traceProbe(remote peer.ID, header Header, port int, probe targetProbe) {
	if forwarder.OnTrace == nil || !probe.probed {
		return
	}
	event := TraceEvent{Stage: mpConstant.TraceTargetProbe, Peer: remote, OK: probe.address != "",
		DurationMs: probe.duration.Milliseconds(), Detail: "tcp port=" + strconv.Itoa(header.Port) + " target=" + strconv.Itoa(port)}
	if probe.address != "" {
		event.Addr = net.JoinHostPort(probe.address, strconv.Itoa(port))
	}
	if probe.err != nil {
		event.Error = probe.err.Error()
	}
	forwarder.OnTrace(event)
}

// Start 注册 TCP/UDP handler 并启动空闲清理。
func (forwarder *HostForwarder) Start() {
	forwarder.host.SetStreamHandler(mpConstant.StreamProtocolTCP, forwarder.handleTCP)
	forwarder.host.SetStreamHandler(mpConstant.StreamProtocolUDP, forwarder.handleUDP)
	go forwarder.janitor()
	go forwarder.prober()
}

// prober 周期性主动探测 TCP 端口声明的游戏监听。
// 职责：房主开房后无成员连入时也能展示“监听中 / 未监听”；使用：Start 启动，Stop 结束；
// 边界：只拨本机候选并立即关闭连接，不转发数据；UDP 声明无法确认监听，保持 unknown。
func (forwarder *HostForwarder) prober() {
	probe := func() {
		changed := false
		for index, decl := range forwarder.ports {
			if decl.Proto == mpConstant.ProtoUDP {
				continue
			}
			select {
			case <-forwarder.done:
				return
			default:
			}
			if forwarder.targets.probeTCP(index, decl.Port) {
				changed = true
			}
		}
		if changed && forwarder.OnTargets != nil {
			forwarder.OnTargets()
		}
	}
	probe()
	ticker := time.NewTicker(mpConstant.TargetProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-forwarder.done:
			return
		case <-ticker.C:
			probe()
		}
	}
}

// SetClosing 停止接受新 stream（room.close before）。
func (forwarder *HostForwarder) SetClosing() {
	forwarder.mu.Lock()
	forwarder.closing = true
	forwarder.mu.Unlock()
}

// Block 停止接受某成员的新 stream（room.member.leave before）。
func (forwarder *HostForwarder) Block(id peer.ID) {
	forwarder.mu.Lock()
	forwarder.blocked[id] = true
	forwarder.mu.Unlock()
}

// Unblock 撤销 Block（成员重新加入时以新 PeerID 进入，通常不需要）。
func (forwarder *HostForwarder) Unblock(id peer.ID) {
	forwarder.mu.Lock()
	delete(forwarder.blocked, id)
	forwarder.mu.Unlock()
}

// RemovePeer 关闭某成员的全部 stream 与 UDP 会话（room.member.leave after）。
// 职责：拆除该成员的转发状态；使用：控制器在移出白名单并 ClosePeer 时调用；边界：不改白名单。
func (forwarder *HostForwarder) RemovePeer(id peer.ID) {
	forwarder.mu.Lock()
	streams := forwarder.streams[id]
	delete(forwarder.streams, id)
	var sessions []*hostUDPSession
	for key, session := range forwarder.sessions {
		if key.peer == id {
			sessions = append(sessions, session)
			delete(forwarder.sessions, key)
		}
	}
	delete(forwarder.udpCount, id)
	forwarder.mu.Unlock()
	for stream := range streams {
		_ = stream.Reset()
	}
	for _, session := range sessions {
		session.close()
	}
}

// Stop 注销 handler 并关闭全部会话。
func (forwarder *HostForwarder) Stop() {
	forwarder.mu.Lock()
	if forwarder.stopped {
		forwarder.mu.Unlock()
		return
	}
	forwarder.stopped = true
	forwarder.closing = true
	peers := make([]peer.ID, 0, len(forwarder.streams))
	for id := range forwarder.streams {
		peers = append(peers, id)
	}
	for key := range forwarder.sessions {
		peers = append(peers, key.peer)
	}
	forwarder.mu.Unlock()
	close(forwarder.done)
	forwarder.host.RemoveStreamHandler(mpConstant.StreamProtocolTCP)
	forwarder.host.RemoveStreamHandler(mpConstant.StreamProtocolUDP)
	for _, id := range peers {
		forwarder.RemovePeer(id)
	}
}

// ClearTargets 清除目标地址缓存（网卡变化）。
func (forwarder *HostForwarder) ClearTargets() { forwarder.targets.clear() }

// Targets 返回目标地址探测结果。
func (forwarder *HostForwarder) Targets() []TargetState {
	return forwarder.targets.states(forwarder.ports)
}

// admit 校验握手并占用名额。
// 职责：白名单、阻止名单、关房、下标/协议/偏移、资源上限；使用：两个 handler；边界：返回 OK 时已计数，调用方须 release。
func (forwarder *HostForwarder) admit(stream network.Stream, header Header, udp bool) (byte, mpAPI.PortDecl) {
	remote := stream.Conn().RemotePeer()
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	if forwarder.closing || forwarder.blocked[remote] || !forwarder.host.Gater.Allowed(remote) ||
		((stream.Conn().Stat().Limited || p2p.IsCircuit(stream.Conn().RemoteMultiaddr())) && !forwarder.host.RelayFallback()) {
		return mpConstant.StatusUnauthorized, mpAPI.PortDecl{}
	}
	if header.Port < 0 || header.Port >= len(forwarder.ports) {
		return mpConstant.StatusBadPort, mpAPI.PortDecl{}
	}
	decl := forwarder.ports[header.Port]
	count := decl.Count
	if count <= 0 {
		count = 1
	}
	wantProto := mpConstant.ProtoTCP
	if udp {
		wantProto = mpConstant.ProtoUDP
	}
	if (decl.Proto != wantProto && decl.Proto != mpConstant.ProtoBoth) || header.Offset < 0 || header.Offset >= count {
		return mpConstant.StatusBadPort, mpAPI.PortDecl{}
	}
	if !udp {
		if forwarder.tcpCount[remote] >= mpConstant.MaxTCPStreamsPerPeer {
			return mpConstant.StatusHostBusy, mpAPI.PortDecl{}
		}
		forwarder.tcpCount[remote]++
	}
	if forwarder.streams[remote] == nil {
		forwarder.streams[remote] = map[network.Stream]struct{}{}
	}
	forwarder.streams[remote][stream] = struct{}{}
	return mpConstant.StatusOK, decl
}

// release 释放 stream 占用。
func (forwarder *HostForwarder) release(stream network.Stream, tcp bool) {
	remote := stream.Conn().RemotePeer()
	forwarder.mu.Lock()
	if tcp && forwarder.tcpCount[remote] > 0 {
		forwarder.tcpCount[remote]--
	}
	if set := forwarder.streams[remote]; set != nil {
		delete(set, stream)
	}
	forwarder.mu.Unlock()
}

// handleTCP 处理 /dsbox/tcp/1。
func (forwarder *HostForwarder) handleTCP(stream network.Stream) {
	started := time.Now()
	remote := stream.Conn().RemotePeer()
	_ = stream.SetDeadline(time.Now().Add(mpConstant.HandshakeTimeout))
	header, err := ReadHeader(stream)
	if err != nil {
		_, _ = stream.Write([]byte{mpConstant.StatusBadPort})
		_ = stream.Reset()
		forwarder.traceAccept(remote, mpConstant.StatusBadPort, mpConstant.ProtoTCP, Header{Port: -1}, started)
		return
	}
	status, decl := forwarder.admit(stream, header, false)
	if status != mpConstant.StatusOK {
		_, _ = stream.Write([]byte{status})
		_ = stream.Close()
		forwarder.traceAccept(remote, status, mpConstant.ProtoTCP, header, started)
		return
	}
	defer forwarder.release(stream, true)
	targetPort := decl.Port + header.Offset
	target, probe, err := forwarder.targets.dialTCP(header.Port, targetPort)
	forwarder.traceProbe(remote, header, targetPort, probe)
	if err != nil {
		_, _ = stream.Write([]byte{mpConstant.StatusTargetRefused})
		_ = stream.Close()
		forwarder.traceAccept(remote, mpConstant.StatusTargetRefused, mpConstant.ProtoTCP, header, started)
		return
	}
	if _, err := stream.Write([]byte{mpConstant.StatusOK}); err != nil {
		_ = target.Close()
		_ = stream.Reset()
		return
	}
	forwarder.traceAccept(remote, mpConstant.StatusOK, mpConstant.ProtoTCP, header, started)
	_ = stream.SetDeadline(time.Time{})
	Pipe(stream, target.(*net.TCPConn))
}

// Pipe 在 stream 与 TCP 连接之间双向对拷。
// 职责：一侧 EOF 时对另一侧 CloseWrite，两侧结束后关闭，出错时 Reset；使用：房主与成员 TCP 转发；边界：阻塞直到两侧都结束。
func Pipe(stream network.Stream, conn *net.TCPConn) {
	var wait sync.WaitGroup
	var failed sync.Once
	fail := func() { failed.Do(func() { _ = stream.Reset(); _ = conn.Close() }) }
	wait.Add(2)
	go func() {
		defer wait.Done()
		if _, err := io.Copy(conn, stream); err != nil {
			fail()
			return
		}
		_ = conn.CloseWrite()
	}()
	go func() {
		defer wait.Done()
		if _, err := io.Copy(stream, conn); err != nil {
			fail()
			return
		}
		_ = stream.CloseWrite()
	}()
	wait.Wait()
	_ = stream.Close()
	_ = conn.Close()
}

// handleUDP 处理 /dsbox/udp/1：新建或接回 (PeerID,sid) 会话。
func (forwarder *HostForwarder) handleUDP(stream network.Stream) {
	started := time.Now()
	remote := stream.Conn().RemotePeer()
	_ = stream.SetDeadline(time.Now().Add(mpConstant.HandshakeTimeout))
	header, err := ReadHeader(stream)
	if err != nil || header.SID == "" {
		_, _ = stream.Write([]byte{mpConstant.StatusBadPort})
		_ = stream.Reset()
		forwarder.traceAccept(remote, mpConstant.StatusBadPort, mpConstant.ProtoUDP, Header{Port: -1}, started)
		return
	}
	status, decl := forwarder.admit(stream, header, true)
	if status != mpConstant.StatusOK {
		_, _ = stream.Write([]byte{status})
		_ = stream.Close()
		forwarder.traceAccept(remote, status, mpConstant.ProtoUDP, header, started)
		return
	}
	key := udpKey{peer: stream.Conn().RemotePeer(), sid: header.SID}
	forwarder.mu.Lock()
	session := forwarder.sessions[key]
	if session == nil {
		if forwarder.udpCount[key.peer] >= mpConstant.MaxUDPSessionsPerPeer {
			forwarder.mu.Unlock()
			forwarder.release(stream, false)
			_, _ = stream.Write([]byte{mpConstant.StatusHostBusy})
			_ = stream.Close()
			forwarder.traceAccept(remote, mpConstant.StatusHostBusy, mpConstant.ProtoUDP, header, started)
			return
		}
		session = &hostUDPSession{port: decl.Port + header.Offset, candidates: forwarder.udpCandidates(header.Port, decl.Port),
			lastActive: time.Now()}
		if err := session.dial(); err != nil {
			forwarder.mu.Unlock()
			forwarder.release(stream, false)
			_, _ = stream.Write([]byte{mpConstant.StatusBadPort})
			_ = stream.Close()
			forwarder.traceAccept(remote, mpConstant.StatusBadPort, mpConstant.ProtoUDP, header, started)
			return
		}
		forwarder.sessions[key] = session
		forwarder.udpCount[key.peer]++
		go session.readTarget()
	}
	forwarder.mu.Unlock()
	if _, err := stream.Write([]byte{mpConstant.StatusOK}); err != nil {
		forwarder.release(stream, false)
		_ = stream.Reset()
		return
	}
	forwarder.traceAccept(remote, mpConstant.StatusOK, mpConstant.ProtoUDP, header, started)
	_ = stream.SetDeadline(time.Time{})
	session.attach(stream)
	session.readStream(stream)
	forwarder.release(stream, false)
}

// udpCandidates 返回 UDP 会话的目标地址顺序。
// 职责：同一端口号有 TCP 声明且已探测到地址时优先沿用，否则从 127.0.0.1 起；使用：handleUDP；边界：只含本机候选。
func (forwarder *HostForwarder) udpCandidates(index int, port int) []string {
	candidates := forwarder.targets.candidates()
	preferred := ""
	for tcpIndex, decl := range forwarder.ports {
		if decl.Proto == mpConstant.ProtoUDP || port < decl.Port || port >= decl.Port+max(decl.Count, 1) {
			continue
		}
		if address := forwarder.targets.get(tcpIndex); address != "" {
			preferred = address
			break
		}
	}
	if preferred == "" {
		return candidates
	}
	result := []string{preferred}
	for _, candidate := range candidates {
		if candidate != preferred {
			result = append(result, candidate)
		}
	}
	return result
}

// janitor 定时关闭空闲 UDP 会话。
func (forwarder *HostForwarder) janitor() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-forwarder.done:
			return
		case <-ticker.C:
		}
		var idle []*hostUDPSession
		forwarder.mu.Lock()
		for key, session := range forwarder.sessions {
			if session.idleFor() > forwarder.IdleTimeout {
				idle = append(idle, session)
				delete(forwarder.sessions, key)
				if forwarder.udpCount[key.peer] > 0 {
					forwarder.udpCount[key.peer]--
				}
			}
		}
		forwarder.mu.Unlock()
		for _, session := range idle {
			session.close()
		}
	}
}

// hostUDPSession 是房主侧一个 UDP 会话。
// 职责：独立本地 socket 发往目标:G，回包写回当前 stream；使用：handleUDP；边界：stream 断开时保留 socket，按空闲计时。
type hostUDPSession struct {
	port       int
	candidates []string

	mu         sync.Mutex
	index      int
	conn       *net.UDPConn
	stream     network.Stream
	lastActive time.Time
	closed     bool
}

// dial 按当前候选创建已 connect 的 UDP socket。
func (session *hostUDPSession) dial() error {
	var lastErr error = errors.New("no candidate")
	for session.index < len(session.candidates) {
		address := net.JoinHostPort(session.candidates[session.index], strconv.Itoa(session.port))
		remote, err := net.ResolveUDPAddr("udp", address)
		if err == nil {
			conn, dialErr := net.DialUDP("udp", nil, remote)
			if dialErr == nil {
				enableConnReset(conn)
				session.conn = conn
				return nil
			}
			err = dialErr
		}
		lastErr = err
		session.index++
	}
	return lastErr
}

// attach 把新 stream 挂到会话（接回）。
func (session *hostUDPSession) attach(stream network.Stream) {
	session.mu.Lock()
	old := session.stream
	session.stream = stream
	session.lastActive = time.Now()
	session.mu.Unlock()
	if old != nil && old != stream {
		_ = old.Reset()
	}
}

// idleFor 返回空闲时长。
func (session *hostUDPSession) idleFor() time.Duration {
	session.mu.Lock()
	defer session.mu.Unlock()
	return time.Since(session.lastActive)
}

// readStream 把 stream 上的报文写到游戏。
func (session *hostUDPSession) readStream(stream network.Stream) {
	for {
		payload, err := ReadPacket(stream)
		if err != nil {
			session.mu.Lock()
			if session.stream == stream {
				session.stream = nil
			}
			session.mu.Unlock()
			_ = stream.Reset()
			return
		}
		session.mu.Lock()
		session.lastActive = time.Now()
		conn := session.conn
		session.mu.Unlock()
		if conn != nil {
			_, _ = conn.Write(payload)
		}
	}
}

// readTarget 把游戏回包写回当前 stream；目标端口不可达时换下一个候选。
func (session *hostUDPSession) readTarget() {
	buffer := make([]byte, mpConstant.MaxUDPPayload)
	for {
		session.mu.Lock()
		conn := session.conn
		session.mu.Unlock()
		if conn == nil {
			return
		}
		size, err := conn.Read(buffer)
		if err != nil {
			session.mu.Lock()
			if session.closed {
				session.mu.Unlock()
				return
			}
			if isConnReset(err) && session.index+1 < len(session.candidates) {
				_ = session.conn.Close()
				session.index++
				if session.dial() == nil {
					session.mu.Unlock()
					continue
				}
			}
			session.mu.Unlock()
			if isConnReset(err) {
				// 全部候选不可达：保留会话，稍后游戏启动后仍可能收到回包
				time.Sleep(200 * time.Millisecond)
				continue
			}
			return
		}
		session.mu.Lock()
		session.lastActive = time.Now()
		stream := session.stream
		session.mu.Unlock()
		if stream != nil {
			if err := WritePacket(stream, buffer[:size]); err != nil {
				_ = stream.Reset()
			}
		}
	}
}

// close 关闭会话 socket 与 stream。
func (session *hostUDPSession) close() {
	session.mu.Lock()
	session.closed = true
	conn, stream := session.conn, session.stream
	session.conn, session.stream = nil, nil
	session.mu.Unlock()
	if conn != nil {
		_ = conn.Close()
	}
	if stream != nil {
		_ = stream.Reset()
	}
}

// openStream 在已有连接上打开转发 stream 并完成握手。
// 职责：成员侧写握手头、读状态字节；使用：LocalForwarder；边界：不主动拨号（WithNoDial），链路由 link 包维护。
func openStream(ctx context.Context, host *p2p.Host, hostID peer.ID, protocol string, header Header) (network.Stream, byte, error) {
	if !host.Gater.Allowed(hostID) || p2p.BestConn(host, hostID, host.RelayFallback()) == nil {
		return nil, mpConstant.StatusUnauthorized, fmt.Errorf("游戏链路未授权或仅允许直接连接")
	}
	if host.RelayFallback() {
		ctx = network.WithAllowLimitedConn(ctx, "relay-game")
	}
	ctx, cancel := context.WithTimeout(network.WithNoDial(ctx, "forward"), mpConstant.HandshakeTimeout)
	defer cancel()
	stream, err := host.NewStream(ctx, hostID, protocolID(protocol))
	if err != nil {
		return nil, 0, err
	}
	_ = stream.SetDeadline(time.Now().Add(mpConstant.HandshakeTimeout))
	if err := WriteHeader(stream, header); err != nil {
		_ = stream.Reset()
		return nil, 0, err
	}
	var status [1]byte
	if _, err := io.ReadFull(stream, status[:]); err != nil {
		_ = stream.Reset()
		return nil, 0, err
	}
	_ = stream.SetDeadline(time.Time{})
	return stream, status[0], nil
}
