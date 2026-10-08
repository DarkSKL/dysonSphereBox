package forward

// 文件职责：成员侧本地端口 M 的分配与 TCP/UDP 本地转发：同号 TCP/UDP、连续段保留偏移、UDP 每来源一个 sid、有界队列丢最旧、链路恢复后原 sid 接回。
// 使用：控制器在 room.member.join after（已提交）后 Open，链路状态变化时 SetAvailable，离房/关房时 SetClosing 与 Close。
// 边界：M 只监听 127.0.0.1；只在已有 libp2p 连接上开 stream，不负责拨号；链路不可用时新 TCP 连接直接关闭、UDP 报文丢弃。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// LocalPort 是成员的一个本地端口展示项。
// 职责：前端 localPorts；使用：用户把 address 填进游戏；边界：连续段显示为 127.0.0.1:M-N。
type LocalPort struct {
	// Label 职责：端口声明标签；使用：展示；边界：可为空。
	Label string `json:"label"`
	// Proto 职责：tcp / udp；使用：展示；边界：both 拆成两条。
	Proto string `json:"proto"`
	// Address 职责：本地地址；使用：复制；边界：只在 127.0.0.1。
	Address string `json:"address"`
}

// portGroup 是共用一个 M 起点的一组声明。
// 职责：同端口号同段长的 TCP/UDP 声明共用端口号；使用：AllocatePorts；边界：tcp/udp 至少一个为 true。
type portGroup struct {
	port, count int
	tcp, udp    bool
	indexes     []int
}

// groupPorts 把声明按 (port,count) 合并。
func groupPorts(ports []mpAPI.PortDecl) []*portGroup {
	groups := []*portGroup{}
	byKey := map[[2]int]*portGroup{}
	for index, decl := range ports {
		count := max(decl.Count, 1)
		key := [2]int{decl.Port, count}
		group := byKey[key]
		if group == nil {
			group = &portGroup{port: decl.Port, count: count}
			byKey[key] = group
			groups = append(groups, group)
		}
		group.indexes = append(group.indexes, index)
		switch decl.Proto {
		case mpConstant.ProtoTCP:
			group.tcp = true
		case mpConstant.ProtoUDP:
			group.udp = true
		default:
			group.tcp, group.udp = true, true
		}
	}
	return groups
}

// boundGroup 是一组已绑定的本地 socket。
type boundGroup struct {
	base int
	tcp  []*net.TCPListener
	udp  []*net.UDPConn
}

func (group *boundGroup) close() {
	for _, listener := range group.tcp {
		_ = listener.Close()
	}
	for _, conn := range group.udp {
		_ = conn.Close()
	}
}

// bindGroup 在 127.0.0.1 上绑定一组连续端口。
// 职责：随机起点、整段绑定、失败整段重抽；使用：AllocatePorts；边界：最多重试 LocalPortRetries 次。
func bindGroup(group *portGroup, pick func(span int) int) (*boundGroup, error) {
	var lastErr error
	for attempt := 0; attempt < mpConstant.LocalPortRetries; attempt++ {
		base := pick(group.count)
		bound := &boundGroup{base: base}
		ok := true
		for offset := 0; offset < group.count && ok; offset++ {
			address := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: base + offset}
			if group.tcp {
				listener, err := net.ListenTCP("tcp4", address)
				if err != nil {
					lastErr, ok = err, false
					break
				}
				bound.tcp = append(bound.tcp, listener)
			}
			if group.udp {
				conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: address.IP, Port: address.Port})
				if err != nil {
					lastErr, ok = err, false
					break
				}
				bound.udp = append(bound.udp, conn)
			}
		}
		if ok {
			return bound, nil
		}
		bound.close()
	}
	return nil, fmt.Errorf("本地端口分配失败：%w", lastErr)
}

// randomBase 职责：在 [LocalPortMin, LocalPortMax-span] 中随机起点；使用：bindGroup 默认；边界：使用 crypto/rand。
func randomBase(span int) int {
	limit := int64(mpConstant.LocalPortMax - mpConstant.LocalPortMin - span)
	value, err := rand.Int(rand.Reader, big.NewInt(limit))
	if err != nil {
		return mpConstant.LocalPortMin
	}
	return mpConstant.LocalPortMin + int(value.Int64())
}

// newSID 生成 16 位十六进制的随机 64 位会话标识。
func newSID() string {
	var raw [8]byte
	_, _ = rand.Read(raw[:])
	return hex.EncodeToString(raw[:])
}

// LocalForwarder 是成员侧本地端口转发器。
// 职责：持有全部 M socket、UDP 会话与链路可用标志；使用：控制器每个房间一个；边界：Close 后不可复用。
type LocalForwarder struct {
	host   *p2p.Host
	hostID peer.ID
	ports  []mpAPI.PortDecl

	// OnTCPStatus 职责：TCP 握手状态回调；使用：控制器据 TARGET_REFUSED 设置/清除 20407；边界：在转发 goroutine 中调用。
	OnTCPStatus func(status byte)
	// OnTrace 职责：stream_open 记录回调；使用：控制器转成 link.trace；边界：Open 前设置，调用时不持锁，必须不阻塞。
	OnTrace func(TraceEvent)

	mu        sync.Mutex
	groups    []*boundGroup
	available bool
	closing   bool
	closed    bool
	sessions  map[string]*localUDPSession
	local     []LocalPort
	pick      func(span int) int
	okSeen    map[firstOKKey]bool
}

// traceOpen 记录 stream_open：打开失败或非 OK 每次都记，OK 只记每个端口的第一次。
func (forwarder *LocalForwarder) traceOpen(proto string, header Header, status byte, err error, started time.Time) {
	if forwarder.OnTrace == nil {
		return
	}
	event := TraceEvent{Stage: mpConstant.TraceStreamOpen, Peer: forwarder.hostID, DurationMs: time.Since(started).Milliseconds()}
	if err != nil {
		event.Error = err.Error()
		event.Detail = proto + " port=" + strconv.Itoa(header.Port) + " offset=" + strconv.Itoa(header.Offset)
		forwarder.OnTrace(event)
		return
	}
	if status == mpConstant.StatusOK {
		key := firstOKKey{peer: forwarder.hostID, proto: proto, port: header.Port}
		forwarder.mu.Lock()
		seen := forwarder.okSeen[key]
		forwarder.okSeen[key] = true
		forwarder.mu.Unlock()
		if seen {
			return
		}
	}
	event.OK = status == mpConstant.StatusOK
	event.Detail = streamDetail(status, proto, header)
	forwarder.OnTrace(event)
}

// NewLocalForwarder 创建成员转发器。
func NewLocalForwarder(host *p2p.Host, hostID peer.ID, ports []mpAPI.PortDecl) *LocalForwarder {
	return &LocalForwarder{host: host, hostID: hostID, ports: append([]mpAPI.PortDecl(nil), ports...),
		sessions: map[string]*localUDPSession{}, pick: randomBase, okSeen: map[firstOKKey]bool{}}
}

// Open 分配全部 M 并开始接收。
// 职责：按第 3 节规则绑定端口、启动 accept/读循环；使用：加入提交后立即调用；边界：任一组分配失败时关闭已绑定的端口并返回错误。
func (forwarder *LocalForwarder) Open() ([]LocalPort, error) {
	groups := groupPorts(forwarder.ports)
	bound := make([]*boundGroup, 0, len(groups))
	local := []LocalPort{}
	for _, group := range groups {
		item, err := bindGroup(group, forwarder.pick)
		if err != nil {
			for _, done := range bound {
				done.close()
			}
			return nil, err
		}
		bound = append(bound, item)
		address := "127.0.0.1:" + strconv.Itoa(item.base)
		if group.count > 1 {
			address += "-" + strconv.Itoa(item.base+group.count-1)
		}
		for _, index := range group.indexes {
			decl := forwarder.ports[index]
			protos := []string{decl.Proto}
			if decl.Proto == mpConstant.ProtoBoth {
				protos = []string{mpConstant.ProtoTCP, mpConstant.ProtoUDP}
			}
			for _, proto := range protos {
				local = append(local, LocalPort{Label: decl.Label, Proto: proto, Address: address})
			}
		}
		for _, index := range group.indexes {
			decl := forwarder.ports[index]
			for offset := 0; offset < group.count; offset++ {
				if decl.Proto != mpConstant.ProtoUDP && len(item.tcp) > 0 {
					go forwarder.acceptTCP(item.tcp[offset], index, offset)
				}
			}
		}
		// UDP socket 只能有一个读循环：同组多条 UDP 声明时归属第一条
		if len(item.udp) > 0 {
			udpIndex := -1
			for _, index := range group.indexes {
				if forwarder.ports[index].Proto != mpConstant.ProtoTCP {
					udpIndex = index
					break
				}
			}
			for offset := 0; offset < group.count; offset++ {
				go forwarder.readUDP(item.udp[offset], udpIndex, offset)
			}
		}
	}
	forwarder.mu.Lock()
	forwarder.groups = bound
	forwarder.local = local
	forwarder.mu.Unlock()
	return local, nil
}

// LocalPorts 返回已分配的本地端口。
func (forwarder *LocalForwarder) LocalPorts() []LocalPort {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	return append([]LocalPort(nil), forwarder.local...)
}

// SetAvailable 设置数据链路是否可用。
// 职责：不可用时拒绝新连接并断开 UDP stream；恢复时对 60 秒内活跃的 UDP 会话用原 sid 重开 stream；使用：链路状态机；边界：关闭后无效。
func (forwarder *LocalForwarder) SetAvailable(available bool) {
	forwarder.mu.Lock()
	if forwarder.closed || forwarder.available == available {
		forwarder.mu.Unlock()
		return
	}
	forwarder.available = available
	sessions := make([]*localUDPSession, 0, len(forwarder.sessions))
	for _, session := range forwarder.sessions {
		sessions = append(sessions, session)
	}
	forwarder.mu.Unlock()
	for _, session := range sessions {
		if !available {
			session.dropStream(nil)
			continue
		}
		if session.idleFor() < mpConstant.UDPIdleTimeout {
			go session.ensureStream()
		}
	}
}

// SetClosing 停止接收新的游戏连接（room.close / room.member.leave before）。
func (forwarder *LocalForwarder) SetClosing() {
	forwarder.mu.Lock()
	forwarder.closing = true
	forwarder.mu.Unlock()
}

// Close 关闭全部 M 与 UDP 会话。
func (forwarder *LocalForwarder) Close() {
	forwarder.mu.Lock()
	if forwarder.closed {
		forwarder.mu.Unlock()
		return
	}
	forwarder.closed, forwarder.closing, forwarder.available = true, true, false
	groups := forwarder.groups
	sessions := forwarder.sessions
	forwarder.sessions = map[string]*localUDPSession{}
	forwarder.mu.Unlock()
	for _, group := range groups {
		group.close()
	}
	for _, session := range sessions {
		session.close()
	}
}

// usable 判断当前是否接收新流量。
func (forwarder *LocalForwarder) usable() bool {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	return forwarder.available && !forwarder.closing
}

// acceptTCP 接收游戏 TCP 连接并转发。
func (forwarder *LocalForwarder) acceptTCP(listener *net.TCPListener, index int, offset int) {
	for {
		conn, err := listener.AcceptTCP()
		if err != nil {
			return
		}
		if !forwarder.usable() {
			_ = conn.Close()
			continue
		}
		go func() {
			started := time.Now()
			header := Header{V: mpConstant.HandshakeVersion, Port: index, Offset: offset}
			stream, status, err := openStream(context.Background(), forwarder.host, forwarder.hostID,
				mpConstant.StreamProtocolTCP, header)
			forwarder.traceOpen(mpConstant.ProtoTCP, header, status, err, started)
			if err != nil {
				_ = conn.Close()
				return
			}
			if forwarder.OnTCPStatus != nil {
				forwarder.OnTCPStatus(status)
			}
			if status != mpConstant.StatusOK {
				_ = stream.Close()
				_ = conn.Close()
				return
			}
			Pipe(stream, conn)
		}()
	}
}

// readUDP 读取游戏 UDP 报文并按来源分配会话。
func (forwarder *LocalForwarder) readUDP(conn *net.UDPConn, index int, offset int) {
	buffer := make([]byte, mpConstant.MaxUDPPayload)
	for {
		size, source, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if forwarder.isClosed() {
				return
			}
			continue
		}
		if !forwarder.usable() {
			continue
		}
		key := strconv.Itoa(index) + "/" + strconv.Itoa(offset) + "/" + source.String()
		forwarder.mu.Lock()
		session := forwarder.sessions[key]
		if session == nil {
			session = newLocalUDPSession(forwarder, conn, source, Header{V: mpConstant.HandshakeVersion,
				Port: index, Offset: offset, SID: newSID()})
			forwarder.sessions[key] = session
			go session.run(func() {
				forwarder.mu.Lock()
				if forwarder.sessions[key] == session {
					delete(forwarder.sessions, key)
				}
				forwarder.mu.Unlock()
			})
		}
		forwarder.mu.Unlock()
		session.push(append([]byte(nil), buffer[:size]...))
	}
}

func (forwarder *LocalForwarder) isClosed() bool {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	return forwarder.closed
}

// PacketQueue 是 UDP 发送有界队列。
// 职责：超过报文数或字节上限时丢最旧，保持“宁可丢包不要延迟”；使用：成员 UDP 会话；边界：非并发安全，由会话锁保护。
type PacketQueue struct {
	packets  [][]byte
	bytes    int
	maxCount int
	maxBytes int
}

// NewPacketQueue 创建有界队列。
func NewPacketQueue(maxCount, maxBytes int) *PacketQueue {
	return &PacketQueue{maxCount: maxCount, maxBytes: maxBytes}
}

// Push 追加报文，超限时丢最旧，返回丢弃数。
func (queue *PacketQueue) Push(packet []byte) int {
	queue.packets = append(queue.packets, packet)
	queue.bytes += len(packet)
	dropped := 0
	for len(queue.packets) > 1 && (len(queue.packets) > queue.maxCount || queue.bytes > queue.maxBytes) {
		queue.bytes -= len(queue.packets[0])
		queue.packets = queue.packets[1:]
		dropped++
	}
	return dropped
}

// Pop 取出最旧报文。
func (queue *PacketQueue) Pop() ([]byte, bool) {
	if len(queue.packets) == 0 {
		return nil, false
	}
	packet := queue.packets[0]
	queue.packets = queue.packets[1:]
	queue.bytes -= len(packet)
	return packet, true
}

// Len 返回队列报文数。
func (queue *PacketQueue) Len() int { return len(queue.packets) }

// Reset 清空队列。
func (queue *PacketQueue) Reset() { queue.packets, queue.bytes = nil, 0 }

// localUDPSession 是成员侧一个 UDP 会话（一个游戏来源端点）。
// 职责：有界队列写入 stream、stream 回包发回来源；使用：readUDP；边界：空闲 60 秒关闭，stream 断开后可用原 sid 重开。
type localUDPSession struct {
	forwarder *LocalForwarder
	conn      *net.UDPConn
	source    *net.UDPAddr
	header    Header

	mu         sync.Mutex
	queue      *PacketQueue
	stream     network.Stream
	lastActive time.Time
	closed     bool
	signal     chan struct{}
	done       chan struct{}
	openMu     sync.Mutex
}

func newLocalUDPSession(forwarder *LocalForwarder, conn *net.UDPConn, source *net.UDPAddr, header Header) *localUDPSession {
	return &localUDPSession{forwarder: forwarder, conn: conn, source: source, header: header,
		queue: NewPacketQueue(mpConstant.UDPQueuePackets, mpConstant.UDPQueueBytes), lastActive: time.Now(),
		signal: make(chan struct{}, 1), done: make(chan struct{})}
}

// push 入队并唤醒写循环。
func (session *localUDPSession) push(packet []byte) {
	session.mu.Lock()
	session.queue.Push(packet)
	session.lastActive = time.Now()
	session.mu.Unlock()
	select {
	case session.signal <- struct{}{}:
	default:
	}
}

func (session *localUDPSession) idleFor() time.Duration {
	session.mu.Lock()
	defer session.mu.Unlock()
	return time.Since(session.lastActive)
}

// run 是写循环与空闲检测；结束时调用 onDone。
func (session *localUDPSession) run(onDone func()) {
	defer onDone()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-session.done:
			return
		case <-ticker.C:
			if session.idleFor() > mpConstant.UDPIdleTimeout {
				session.close()
				return
			}
			continue
		case <-session.signal:
		}
		for {
			session.mu.Lock()
			packet, ok := session.queue.Pop()
			session.mu.Unlock()
			if !ok {
				break
			}
			stream := session.ensureStream()
			if stream == nil {
				session.mu.Lock()
				session.queue.Reset()
				session.mu.Unlock()
				break
			}
			if err := WritePacket(stream, packet); err != nil {
				session.dropStream(stream)
			}
		}
	}
}

// ensureStream 返回当前 stream，没有时按原 sid 打开。
func (session *localUDPSession) ensureStream() network.Stream {
	session.openMu.Lock()
	defer session.openMu.Unlock()
	session.mu.Lock()
	if session.stream != nil || session.closed {
		stream := session.stream
		session.mu.Unlock()
		return stream
	}
	session.mu.Unlock()
	if !session.forwarder.usable() {
		return nil
	}
	started := time.Now()
	stream, status, err := openStream(context.Background(), session.forwarder.host, session.forwarder.hostID,
		mpConstant.StreamProtocolUDP, session.header)
	session.forwarder.traceOpen(mpConstant.ProtoUDP, session.header, status, err, started)
	if err != nil {
		return nil
	}
	if status != mpConstant.StatusOK {
		_ = stream.Close()
		return nil
	}
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		_ = stream.Reset()
		return nil
	}
	session.stream = stream
	session.mu.Unlock()
	go session.readStream(stream)
	return stream
}

// readStream 把房主回包发回游戏来源端点。
func (session *localUDPSession) readStream(stream network.Stream) {
	for {
		payload, err := ReadPacket(stream)
		if err != nil {
			session.dropStream(stream)
			return
		}
		session.mu.Lock()
		session.lastActive = time.Now()
		session.mu.Unlock()
		_, _ = session.conn.WriteToUDP(payload, session.source)
	}
}

// dropStream 断开 stream（nil 表示断开当前任意 stream）。
func (session *localUDPSession) dropStream(stream network.Stream) {
	session.mu.Lock()
	current := session.stream
	if stream == nil || current == stream {
		session.stream = nil
	}
	session.mu.Unlock()
	if current != nil && (stream == nil || current == stream) {
		_ = current.Reset()
	}
}

// close 关闭会话。
func (session *localUDPSession) close() {
	session.mu.Lock()
	if session.closed {
		session.mu.Unlock()
		return
	}
	session.closed = true
	stream := session.stream
	session.stream = nil
	session.mu.Unlock()
	close(session.done)
	if stream != nil {
		_ = stream.Reset()
	}
}

// protocolID 把字符串转换为 libp2p 协议 ID。
func protocolID(value string) protocol.ID { return protocol.ID(value) }
