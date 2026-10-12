package multiplayer

// 文件职责：联机运行期控制器：控制链路会话生命周期与退避重连、地址上报、服务器事件处理、房主白名单与转发、成员本地端口与链路启动、状态投影。
// 使用：MultiplayerService 的意图方法委托给包级 controller；测试用 newController 注入替身服务器地址与 token。
// 边界：一个结构体 + 一把锁；事件处理在 socket 的事件 goroutine 中执行，只改本地状态，需要发请求的动作另起 goroutine，不等待请求回复。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/forward"
	"DysonSphereBox/helper/multiplayer/link"
	"DysonSphereBox/helper/multiplayer/p2p"
	"DysonSphereBox/helper/multiplayer/socket"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
)

// options 是控制器的可注入依赖。
// 职责：隔离服务器地址、token、监听地址与状态推送；使用：生产默认值见 service.go，测试注入替身；边界：函数必须并发安全。
type options struct {
	socketURL      func() (string, error)
	token          func() string
	listenAddrs    []string
	emit           func(State)
	firewall       func() bool
	logDial        func(link.DialRecord)
	observe        func(ctx context.Context, h *p2p.Host, observers []string, timeout time.Duration) ([]p2p.Observation, []p2p.ObserverResult)
	keepLoopback   bool
	clientVersion  string
	observeTimeout time.Duration
}

// session 是一次控制链路会话。
// 职责：持有 socket、临时密钥 host 与会话信息；使用：事件处理以指针比较判断是否仍是当前会话；边界：断开后整体丢弃。
type session struct {
	conn        *socket.Conn
	host        *p2p.Host
	info        mpAPI.SessionOpenReply
	port        int
	stop        chan struct{}
	firstReport chan struct{}
	reportOnce  sync.Once
	terminate   int
	report      p2p.Report
	observed    []p2p.Observation
	// observeResults 职责：最近一次观测的逐节点结果；使用：投影中拿不到 observed 地址的原因；边界：每次重新观测整体替换。
	observeResults []p2p.ObserverResult
	// tracer 职责：本会话的 link.trace 上报器；使用：各 stage；边界：socket 建立后创建、endSession 关闭；nil 时上报为空操作。
	tracer *link.Tracer
}

// roomState 是当前房间的运行期状态。
// 职责：快照、角色、房主连接信息与转发器；使用：事件处理与投影；边界：随离房/关房/断线整体拆除。
type roomState struct {
	view          RoomView
	endpoint      *mpAPI.HostEndpoint
	hostForwarder *forward.HostForwarder
	pendingJoin   map[int64]peer.ID
	leaving       map[int64]peer.ID
	local         *forward.LocalForwarder
	link          *memberLink
}

// controller 是联机运行期控制器。
type controller struct {
	opts options

	mu         sync.Mutex
	revision   int64
	wantOnline bool
	stopLoop   chan struct{}
	connState  string
	lastErr    int
	sess       *session
	lobby      LobbyView
	room       *roomState
	pending    map[string]int
}

func newController(opts options) *controller {
	if opts.observeTimeout <= 0 {
		opts.observeTimeout = 5 * time.Second
	}
	if opts.firewall == nil {
		opts.firewall = p2p.FirewallInboundAllowed
	}
	if opts.logDial == nil {
		opts.logDial = link.RecordDial
	}
	if opts.observe == nil {
		opts.observe = p2p.ObserveDetailed
	}
	return &controller{opts: opts, connState: mpConstant.ConnOffline, pending: map[string]int{},
		lobby: LobbyView{List: []mpAPI.RoomSummary{}}}
}

// ===================== 连接生命周期 =====================

// connect 开始保持在线（幂等）。
func (c *controller) connect() ActionResult {
	if c.opts.token() == "" {
		return ActionResult{Code: 7, Msg: "请先登录"}
	}
	c.mu.Lock()
	if c.wantOnline {
		c.mu.Unlock()
		return ActionResult{}
	}
	c.wantOnline = true
	c.lastErr = 0
	c.connState = mpConstant.ConnConnecting
	stop := make(chan struct{})
	c.stopLoop = stop
	c.mu.Unlock()
	c.emit()
	go c.runLoop(stop)
	return ActionResult{}
}

// disconnect 主动断开并停止重连（幂等）。
func (c *controller) disconnect() ActionResult {
	c.mu.Lock()
	if !c.wantOnline {
		c.mu.Unlock()
		return ActionResult{}
	}
	c.wantOnline = false
	close(c.stopLoop)
	sess := c.sess
	c.mu.Unlock()
	if sess != nil {
		sess.conn.Close()
	}
	return ActionResult{}
}

// runLoop 维持控制链路：断开后按 1s→30s 指数退避重连，不可重连的原因直接停止。
func (c *controller) runLoop(stop chan struct{}) {
	backoff := time.Second
	for {
		online, code, permanent := c.runSession(stop)
		select {
		case <-stop:
			c.setOffline(stop, 0)
			return
		default:
		}
		if permanent {
			c.mu.Lock()
			if c.stopLoop == stop {
				c.wantOnline = false
			}
			c.mu.Unlock()
			c.setOffline(stop, code)
			return
		}
		if online {
			backoff = time.Second
		}
		c.mu.Lock()
		if c.stopLoop == stop {
			c.connState, c.lastErr = mpConstant.ConnReconnecting, code
		}
		c.mu.Unlock()
		c.emit()
		select {
		case <-stop:
			c.setOffline(stop, 0)
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, mpConstant.ReconnectMaxBackoff)
	}
}

// setOffline 把连接状态置为 offline；stop 已不是当前循环（断开后又重新连接）时不覆盖新状态。
func (c *controller) setOffline(stop chan struct{}, code int) {
	c.mu.Lock()
	if c.stopLoop != stop {
		c.mu.Unlock()
		return
	}
	c.connState = mpConstant.ConnOffline
	if code != 0 {
		c.lastErr = code
	}
	c.mu.Unlock()
	c.emit()
}

// startHost 用持久化端口启动 host，端口不可用时换号重试。
func (c *controller) startHost() (*p2p.Host, int, error) {
	if c.opts.listenAddrs != nil {
		h, err := p2p.NewHost(0, c.opts.listenAddrs)
		return h, 0, err
	}
	port := p2p.ListenPort()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		h, err := p2p.NewHost(port, nil)
		if err == nil && p2p.HasIPv4QUIC(h) {
			return h, port, nil
		}
		if h != nil {
			_ = h.Close()
		}
		lastErr = err
		port = p2p.NewRandomListenPort()
	}
	return nil, 0, fmt.Errorf("libp2p 监听失败：%v", lastErr)
}

// runSession 建立一次控制链路并阻塞到断开。
// 返回：是否曾经在线、断开/拒绝码、是否不应自动重连。
func (c *controller) runSession(stop chan struct{}) (bool, int, bool) {
	url, err := c.opts.socketURL()
	if err != nil {
		return false, mpConstant.CodeNotConnected, true
	}
	for attempt := 0; attempt < 3; attempt++ {
		h, port, err := c.startHost()
		if err != nil {
			return false, mpConstant.CodeNotConnected, false
		}
		sess := &session{host: h, port: port, stop: make(chan struct{}), firstReport: make(chan struct{})}
		conn, err := socket.Dial(context.Background(), socket.Config{URL: url, Token: c.opts.token()},
			func(frame *mpAPI.ActiveFrame) (int, string) { return c.handleEvent(sess, frame) })
		if err != nil {
			_ = h.Close()
			var handshake *socket.HandshakeError
			if errors.As(err, &handshake) {
				code := handshake.Code
				if code == 0 {
					code = handshake.Status
				}
				return false, code, handshake.Permanent()
			}
			return false, mpConstant.CodeNotConnected, false
		}
		sess.conn = conn
		sess.tracer = link.NewTracer(conn)
		// gater 回调在 libp2p 握手路径中，换算 userId 需要控制器锁，另起 goroutine
		h.Gater.SetOnReject(func(id peer.ID, remoteAddr string) { go c.traceGaterReject(sess, id, remoteAddr) })
		c.mu.Lock()
		select {
		case <-stop:
			c.mu.Unlock()
			sess.tracer.Close()
			conn.Close()
			_ = h.Close()
			return false, 0, false
		default:
		}
		c.sess = sess
		c.mu.Unlock()

		reply, err := conn.Request(context.Background(), mpConstant.OpSessionOpen, mpAPI.SessionOpenRequest{
			ProtocolVersion: mpConstant.ProtocolVersion, ClientVersion: c.opts.clientVersion, PeerID: h.ID().String()})
		if err == nil && reply.Code == mpConstant.CodePeerIDInUse {
			c.endSession(sess)
			continue
		}
		if err != nil || reply.Code != 0 {
			code := mpConstant.CodeDisconnected
			if err == nil {
				code = reply.Code
			}
			c.endSession(sess)
			permanent := code == mpConstant.CodeProtocolVersion || code == mpConstant.CodeClientTooOld
			return false, code, permanent
		}
		_ = json.Unmarshal(reply.Data, &sess.info)
		if err := h.ConfigureRelay(sess.info.Capabilities, sess.info.Relays, func(state p2p.RelayState, duration time.Duration) {
			sess.tracer.Trace(mpAPI.LinkTraceRequest{Stage: mpConstant.TraceRelayReserve, OK: state.ExpiresAtMs > time.Now().UnixMilli(),
				DurationMs: duration.Milliseconds(), Error: state.Error, Detail: fmt.Sprintf("relay=%s expires=%d duration=%ds data=%d", state.PeerID, state.ExpiresAtMs, state.LimitDurationSeconds, state.LimitDataBytes)})
		}); err != nil {
			sess.tracer.Trace(mpAPI.LinkTraceRequest{Stage: mpConstant.TraceRelayReserve, Error: err.Error()})
		}
		h.SetDCUtRTracer(func(evt *holepunch.Event) { go c.traceDCUtR(sess, evt) })
		h.RunReservations()

		c.mu.Lock()
		c.connState, c.lastErr = mpConstant.ConnOnline, 0
		watching := c.lobby.Watching
		c.mu.Unlock()
		c.emit()
		go c.addrLoop(sess)
		if watching {
			go func() {
				_, _ = conn.Request(context.Background(), mpConstant.OpLobbyWatch, mpAPI.LobbyWatchRequest{On: true})
			}()
		}

		select {
		case <-conn.Done():
		case <-stop:
			conn.Close()
		}
		c.endSession(sess)
		reason := sess.terminate
		if reason >= mpConstant.ReasonTokenInvalid && reason <= mpConstant.ReasonReplaced {
			return true, reason, true
		}
		if reason == 0 {
			reason = mpConstant.CodeDisconnected
		}
		return true, reason, false
	}
	return false, mpConstant.CodePeerIDInUse, false
}

// endSession 拆除会话：房间、转发、host 全部销毁。
func (c *controller) endSession(sess *session) {
	c.mu.Lock()
	if c.sess != sess {
		c.mu.Unlock()
		return
	}
	c.sess = nil
	room := c.room
	c.room = nil
	c.mu.Unlock()
	close(sess.stop)
	sess.tracer.Close()
	sess.conn.Close()
	teardownRoom(sess, room)
	_ = sess.host.Close()
	c.emit()
}

// teardownRoom 拆除房间的转发与链路。
func teardownRoom(sess *session, room *roomState) {
	if room == nil {
		return
	}
	if room.hostForwarder != nil {
		room.hostForwarder.Stop()
	}
	if room.link != nil {
		room.link.close()
	}
	if room.local != nil {
		room.local.Close()
	}
	if sess != nil {
		ids := sess.host.Gater.List()
		sess.host.Gater.Clear()
		// ClosePeer 可能触发 libp2p 通知回调，放到独立 goroutine，避免在持有控制器锁时阻塞
		go func() {
			for _, id := range ids {
				_ = sess.host.Network().ClosePeer(id)
			}
		}()
	}
}

// ===================== 地址上报 =====================

// addrLoop 观测地址、上报 peer.addrs，并在网卡或 libp2p 地址变化时重报。
// 防火墙每个会话（新的 libp2p host）检测一次，与首次观测并行执行。
func (c *controller) addrLoop(sess *session) {
	firewallCh := make(chan bool, 1)
	go func() { firewallCh <- c.opts.firewall() }()
	firewall, firewallKnown := true, false
	var last p2p.Report
	reported := false
	report := func(observe bool) {
		if observe {
			observed, results := c.opts.observe(context.Background(), sess.host, sess.info.Observers, c.opts.observeTimeout)
			c.mu.Lock()
			sess.observed = observed
			sess.observeResults = results
			c.mu.Unlock()
			c.traceObserve(sess, results)
		}
		if !firewallKnown {
			select {
			case firewall = <-firewallCh:
			case <-sess.stop:
				return
			}
			firewallKnown = true
		}
		c.mu.Lock()
		observed := sess.observed
		c.mu.Unlock()
		current := p2p.Collect(sess.host, observed, firewall, c.opts.keepLoopback)
		c.mu.Lock()
		sess.report = current
		c.mu.Unlock()
		if reported && slices.Equal(current.Addrs, last.Addrs) && sameNet(current.Net, last.Net) {
			return
		}
		reply, err := sess.conn.Request(context.Background(), mpConstant.OpPeerAddrs,
			mpAPI.PeerAddrsRequest{Addrs: current.Addrs, Net: current.Net})
		if err == nil && reply.Code == 0 {
			last, reported = current, true
			sess.reportOnce.Do(func() { close(sess.firstReport) })
		}
		c.emit()
	}
	report(true)

	sub, err := sess.host.EventBus().Subscribe(new(event.EvtLocalAddressesUpdated))
	var updates <-chan interface{}
	if err == nil {
		defer sub.Close()
		updates = sub.Out()
	}
	signature := p2p.InterfaceSignature()
	ticker := time.NewTicker(mpConstant.NetworkCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-sess.stop:
			return
		case <-updates:
			report(false)
		case <-sess.host.RelayUpdates():
			report(false)
		case <-ticker.C:
			next := p2p.InterfaceSignature()
			if next != signature {
				signature = next
				c.mu.Lock()
				if c.room != nil && c.room.hostForwarder != nil {
					c.room.hostForwarder.ClearTargets()
				}
				c.mu.Unlock()
				report(true)
			} else if !reported {
				report(false)
			}
		}
	}
}

func sameNet(a, b mpAPI.NetInfo) bool {
	return a.UPnP == b.UPnP && a.IPv6 == b.IPv6 && a.FirewallRule == b.FirewallRule &&
		(a.NATConsistent == nil) == (b.NATConsistent == nil) &&
		(a.NATConsistent == nil || *a.NATConsistent == *b.NATConsistent)
}

// ===================== 意图方法 =====================

// request 发一个请求并转换为 ActionResult。
func (c *controller) request(name string, opcode int, data any, out any) ActionResult {
	c.mu.Lock()
	sess := c.sess
	online := c.connState == mpConstant.ConnOnline
	if sess == nil || !online {
		c.mu.Unlock()
		return ActionResult{Code: mpConstant.CodeNotConnected, Msg: "控制链路未连接"}
	}
	c.pending[name]++
	c.mu.Unlock()
	c.emit()
	defer func() {
		c.mu.Lock()
		if c.pending[name]--; c.pending[name] <= 0 {
			delete(c.pending, name)
		}
		c.mu.Unlock()
		c.emit()
	}()

	reply, err := sess.conn.Request(context.Background(), opcode, data)
	if err != nil {
		var local *socket.LocalError
		if errors.As(err, &local) {
			return ActionResult{Code: local.Code}
		}
		return ActionResult{Code: mpConstant.CodeDisconnected, Msg: err.Error()}
	}
	result := ActionResult{Code: reply.Code, Msg: reply.Msg}
	if reply.Code == 0 && out != nil {
		if err := json.Unmarshal(reply.Data, out); err == nil {
			result.Data = out
		}
	} else if reply.Code != 0 && len(reply.Data) > 2 {
		result.Data = reply.Data
	}
	return result
}

func (c *controller) currentRoomID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room == nil {
		return ""
	}
	return c.room.view.Snapshot.RoomID
}

func (c *controller) lobbyList(req mpAPI.LobbyListRequest) ActionResult {
	var reply mpAPI.LobbyListReply
	result := c.request("lobbyList", mpConstant.OpLobbyList, req, &reply)
	if result.Code == 0 {
		c.mu.Lock()
		c.lobby.Total = reply.Total
		c.lobby.List = append([]mpAPI.RoomSummary{}, reply.List...)
		c.mu.Unlock()
		c.emit()
	}
	return result
}

func (c *controller) lobbyWatch(on bool) ActionResult {
	result := c.request("lobbyWatch", mpConstant.OpLobbyWatch, mpAPI.LobbyWatchRequest{On: on}, nil)
	if result.Code == 0 {
		c.mu.Lock()
		c.lobby.Watching = on
		c.mu.Unlock()
		c.emit()
	}
	return result
}

func (c *controller) createRoom(req mpAPI.RoomCreateRequest) ActionResult {
	c.mu.Lock()
	sess := c.sess
	c.mu.Unlock()
	if sess != nil {
		// 创建前需要至少上报过一次 peer.addrs（否则 20304），首次观测最多等 8 秒
		select {
		case <-sess.firstReport:
		case <-time.After(8 * time.Second):
		}
	}
	return c.request("createRoom", mpConstant.OpRoomCreate, req, nil)
}

func (c *controller) resolveRoom(req mpAPI.RoomResolveRequest) ActionResult {
	req.RoomID, req.InviteCode = strings.TrimSpace(req.RoomID), strings.TrimSpace(req.InviteCode)
	var reply mpAPI.RoomResolveReply
	return c.request("resolveRoom", mpConstant.OpRoomResolve, req, &reply)
}

func (c *controller) joinRoom(req mpAPI.RoomJoinRequest) ActionResult {
	req.RoomID, req.InviteCode = strings.TrimSpace(req.RoomID), strings.TrimSpace(req.InviteCode)
	return c.request("joinRoom", mpConstant.OpRoomJoin, req, nil)
}

func (c *controller) roomAction(name string, opcode int) ActionResult {
	roomID := c.currentRoomID()
	if roomID == "" {
		return ActionResult{Code: 20209, Msg: "不在房间中"}
	}
	return c.request(name, opcode, mpAPI.RoomIDRequest{RoomID: roomID}, nil)
}

func (c *controller) kick(userID int64) ActionResult {
	roomID := c.currentRoomID()
	if roomID == "" {
		return ActionResult{Code: 20209, Msg: "不在房间中"}
	}
	return c.request("kick", mpConstant.OpRoomKick, mpAPI.RoomKickRequest{RoomID: roomID, UserID: userID}, nil)
}

func (c *controller) retryLink() ActionResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.room == nil || c.room.link == nil {
		return ActionResult{Code: 20209, Msg: "当前没有成员链路"}
	}
	c.room.link.wakeUp()
	return ActionResult{}
}

// ===================== 事件处理 =====================

// handleEvent 处理一个服务器事件并返回回复码。
func (c *controller) handleEvent(sess *session, frame *mpAPI.ActiveFrame) (int, string) {
	c.mu.Lock()
	if c.sess != sess {
		// 已拆除的旧会话上的残余事件不再应用
		c.mu.Unlock()
		return 0, ""
	}
	code, msg, changed := c.applyEvent(sess, frame)
	c.mu.Unlock()
	if changed {
		c.emit()
	}
	return code, msg
}

// applyEvent 在持锁状态下应用事件。返回回复码、提示与投影是否变化。
func (c *controller) applyEvent(sess *session, frame *mpAPI.ActiveFrame) (int, string, bool) {
	selfID := sess.info.User.UserID
	switch frame.Opcode {
	case mpConstant.EvSessionTerminateBefore, mpConstant.EvLobbyChangeBefore, mpConstant.EvRoomCreateBefore,
		mpConstant.EvRoomInviteBefore, mpConstant.EvMemberLinkBefore, mpConstant.EvHostAddrsBefore:
		return 0, "", false

	case mpConstant.EvSessionTerminateAfter:
		var data mpAPI.EventBase
		_ = json.Unmarshal(frame.Data, &data)
		sess.terminate = data.Reason
		return 0, "", false

	case mpConstant.EvLobbyChangeAfter:
		var data mpAPI.LobbyChangeEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		c.mergeLobby(data)
		return 0, "", true

	case mpConstant.EvRoomCreateAfter:
		var data mpAPI.RoomEvent
		if json.Unmarshal(frame.Data, &data) != nil || data.Result != 0 || data.Room == nil {
			return 0, "", false
		}
		teardownRoom(sess, c.room)
		forwarder := forward.NewHostForwarder(sess.host, data.Room.Ports)
		forwarder.OnTrace = func(event forward.TraceEvent) { go c.traceForward(sess, event) }
		forwarder.Start()
		c.room = &roomState{view: RoomView{Snapshot: *data.Room, Role: mpConstant.RoleHost},
			hostForwarder: forwarder, pendingJoin: map[int64]peer.ID{}, leaving: map[int64]peer.ID{}}
		return 0, "", true

	case mpConstant.EvRoomCloseBefore:
		if c.room != nil {
			if c.room.hostForwarder != nil {
				c.room.hostForwarder.SetClosing()
			}
			if c.room.local != nil {
				c.room.local.SetClosing()
			}
		}
		return 0, "", false

	case mpConstant.EvRoomCloseAfter:
		var data mpAPI.EventBase
		_ = json.Unmarshal(frame.Data, &data)
		if c.room != nil && (data.RoomID == "" || data.RoomID == c.room.view.Snapshot.RoomID) {
			teardownRoom(sess, c.room)
			c.room = nil
			return 0, "", true
		}
		return 0, "", false

	case mpConstant.EvRoomInviteAfter:
		var data mpAPI.RoomInviteEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID {
			c.room.view.Invite = &InviteView{Code: data.InviteCode, Link: data.Link, ExpiresAtMs: data.ExpiresAtMs}
			c.applySnapshot(data.Room)
		}
		return 0, "", true

	case mpConstant.EvMemberJoinBefore:
		var data mpAPI.MemberJoinEvent
		if json.Unmarshal(frame.Data, &data) != nil || data.Member == nil {
			return mpConstant.CodePrepareFailed, "bad data", false
		}
		if c.isHost(data.RoomID) {
			id, err := peer.Decode(data.Member.PeerID)
			if err != nil {
				return mpConstant.CodePrepareFailed, "bad peerId", false
			}
			// 待定白名单：before 放行，after 已提交转正、已中止移除
			sess.host.Gater.Allow(id)
			c.room.hostForwarder.Unblock(id)
			c.room.pendingJoin[data.Member.UserID] = id
		}
		return 0, "", false

	case mpConstant.EvMemberJoinAfter:
		var data mpAPI.MemberJoinEvent
		if json.Unmarshal(frame.Data, &data) != nil || data.Member == nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if c.isHost(data.RoomID) {
			id, pending := c.room.pendingJoin[data.Member.UserID]
			delete(c.room.pendingJoin, data.Member.UserID)
			if data.Result != 0 {
				if pending {
					sess.host.Gater.Remove(id)
					c.room.hostForwarder.RemovePeer(id)
					go func() { _ = sess.host.Network().ClosePeer(id) }()
				}
				return 0, "", false
			}
			c.applySnapshot(data.Room)
			return 0, "", true
		}
		if data.Result != 0 {
			return 0, "", false
		}
		if data.Member.UserID == selfID && c.room == nil && data.Room != nil && data.Host != nil {
			return c.startMember(sess, data)
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID {
			c.applySnapshot(data.Room)
			return 0, "", true
		}
		return 0, "", false

	case mpConstant.EvMemberLeaveBefore:
		var data mpAPI.MemberLeaveEvent
		_ = json.Unmarshal(frame.Data, &data)
		if c.room == nil || data.RoomID != c.room.view.Snapshot.RoomID {
			return 0, "", false
		}
		if c.room.hostForwarder != nil {
			if id, err := peer.Decode(data.PeerID); err == nil {
				c.room.hostForwarder.Block(id)
				c.room.leaving[data.UserID] = id
			}
		}
		if data.UserID == selfID && c.room.local != nil {
			c.room.local.SetClosing()
		}
		return 0, "", false

	case mpConstant.EvMemberLeaveAfter:
		var data mpAPI.MemberLeaveEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if c.room == nil || data.RoomID != c.room.view.Snapshot.RoomID {
			return 0, "", false
		}
		if data.UserID == selfID {
			teardownRoom(sess, c.room)
			c.room = nil
			return 0, "", true
		}
		if c.room.hostForwarder != nil {
			id, ok := c.room.leaving[data.UserID]
			delete(c.room.leaving, data.UserID)
			if !ok {
				id, ok = c.memberPeer(data.UserID)
			}
			if ok {
				sess.host.Gater.Remove(id)
				c.room.hostForwarder.RemovePeer(id)
				go func() { _ = sess.host.Network().ClosePeer(id) }()
			}
		}
		c.applySnapshot(data.Room)
		return 0, "", true

	case mpConstant.EvMemberLinkAfter:
		var data mpAPI.MemberLinkEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID {
			c.applySnapshot(data.Room)
			return 0, "", true
		}
		return 0, "", false

	case mpConstant.EvHostAddrsAfter:
		var data mpAPI.HostAddrsEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID && data.Host != nil && c.room.link != nil {
			c.room.endpoint = data.Host
			c.room.link.updateEndpoint(*data.Host)
		}
		return 0, "", false

	}
	return mpConstant.CodeUnknownOpcode, "unknown opcode", false
}

// startMember 加入已提交：开本地端口 M 并启动链路。
func (c *controller) startMember(sess *session, data mpAPI.MemberJoinEvent) (int, string, bool) {
	hostID, err := peer.Decode(data.Host.PeerID)
	if err != nil {
		return mpConstant.CodeApplyFailed, "bad host peerId", false
	}
	room := &roomState{view: RoomView{Snapshot: *data.Room, Role: mpConstant.RoleMember}, endpoint: data.Host,
		pendingJoin: map[int64]peer.ID{}, leaving: map[int64]peer.ID{}}
	// 打洞时房主回拨，成员只放行房主
	sess.host.Gater.Allow(hostID)
	room.local = forward.NewLocalForwarder(sess.host, hostID, data.Room.Ports)
	room.local.OnTrace = func(event forward.TraceEvent) { go c.traceForward(sess, event) }
	opened := time.Now()
	local, err := room.local.Open()
	portsTrace := mpAPI.LinkTraceRequest{RoomID: data.Room.RoomID, PeerUserID: data.Room.Host.UserID,
		Stage: mpConstant.TracePortsOpen, OK: err == nil, DurationMs: time.Since(opened).Milliseconds(), Error: link.ErrText(err)}
	if err == nil {
		addrs := make([]string, 0, len(local))
		for _, item := range local {
			addrs = append(addrs, item.Proto+" "+item.Address)
		}
		portsTrace.Detail = strings.Join(addrs, ", ")
	}
	sess.tracer.Trace(portsTrace)
	if err != nil {
		c.room = room
		return mpConstant.CodeApplyFailed, err.Error(), true
	}
	room.link = newMemberLink(c, sess, room, hostID, *data.Host)
	room.local.OnTCPStatus = room.link.onTCPStatus
	c.room = room
	go room.link.run()
	return 0, "", true
}

// isHost 判断自己是否是 roomID 的房主（持锁调用）。
func (c *controller) isHost(roomID string) bool {
	return c.room != nil && c.room.hostForwarder != nil && c.room.view.Snapshot.RoomID == roomID
}

// memberPeer 从当前快照查找成员 PeerID（持锁调用）。
func (c *controller) memberPeer(userID int64) (peer.ID, bool) {
	for _, member := range c.room.view.Snapshot.Members {
		if member.UserID == userID {
			id, err := peer.Decode(member.PeerID)
			return id, err == nil
		}
	}
	return "", false
}

// applySnapshot 只应用 revision 更大的快照（持锁调用）。
func (c *controller) applySnapshot(snapshot *mpAPI.RoomSnapshot) {
	if snapshot == nil || c.room == nil || snapshot.RoomID != c.room.view.Snapshot.RoomID {
		return
	}
	if snapshot.Revision <= c.room.view.Snapshot.Revision {
		return
	}
	c.room.view.Snapshot = *snapshot
}

// mergeLobby 合并 lobby.change（持锁调用）。
func (c *controller) mergeLobby(data mpAPI.LobbyChangeEvent) {
	removed := map[string]bool{}
	for _, id := range data.Removed {
		removed[id] = true
	}
	next := make([]mpAPI.RoomSummary, 0, len(c.lobby.List)+len(data.Upserted))
	upserted := map[string]mpAPI.RoomSummary{}
	for _, item := range data.Upserted {
		upserted[item.RoomID] = item
	}
	for _, item := range data.Upserted {
		if !slices.ContainsFunc(c.lobby.List, func(old mpAPI.RoomSummary) bool { return old.RoomID == item.RoomID }) {
			next = append(next, item)
			c.lobby.Total++
		}
	}
	for _, item := range c.lobby.List {
		if removed[item.RoomID] {
			c.lobby.Total--
			continue
		}
		if updated, ok := upserted[item.RoomID]; ok {
			item = updated
		}
		next = append(next, item)
	}
	c.lobby.Total = max(c.lobby.Total, len(next))
	c.lobby.List = next
}

// ===================== 状态投影 =====================

// snapshotState 生成投影（持锁调用）。
func (c *controller) snapshotState() State {
	c.revision++
	state := State{
		StateRevision: c.revision,
		Connection:    ConnectionView{State: c.connState, LastErrorCode: c.lastErr},
		Account:       AccountView{LoggedIn: c.opts.token() != ""},
		Lobby:         LobbyView{Watching: c.lobby.Watching, Total: c.lobby.Total, List: append([]mpAPI.RoomSummary{}, c.lobby.List...)},
		Links:         []LinkView{},
		LocalPorts:    []forward.LocalPort{},
		Network: NetworkView{ObservedAddrs: []string{}, Targets: []forward.TargetState{}, ReportedAddrs: []p2p.ClassifiedAddr{},
			Observed: []p2p.ClassifiedAddr{}, ObserveErrors: []string{}, HostAddrs: []p2p.ClassifiedAddr{}, NATMappings: []string{}},
		Pending: []string{},
	}
	for name := range c.pending {
		state.Pending = append(state.Pending, name)
	}
	slices.Sort(state.Pending)
	if sess := c.sess; sess != nil {
		state.Account.UserID, state.Account.Nickname = sess.info.User.UserID, sess.info.User.Nickname
		if sess.info.SessionID != "" {
			state.Session = &SessionView{SessionID: sess.info.SessionID, PeerID: sess.host.ID().String(),
				ListenPort: sess.port, Limits: sess.info.Limits, Capabilities: sess.info.Capabilities, Relays: sess.host.RelayStates()}
		}
		state.Network.UPnP = sess.report.Net.UPnP
		state.Network.IPv6 = sess.report.Net.IPv6
		state.Network.NATConsistent = sess.report.Net.NATConsistent
		state.Network.NATMappings = append(state.Network.NATMappings, sess.report.NATMappings...)
		state.Network.FirewallRule = sess.report.Net.FirewallRule
		state.Network.ObservedAddrs = append(state.Network.ObservedAddrs, sess.report.Observed...)
		state.Network.ReportedAddrs = p2p.ClassifyAddrs(sess.report.Addrs)
		state.Network.Observed = p2p.ClassifyAddrs(sess.report.Observed)
		for _, result := range sess.observeResults {
			if !result.OK && result.Err != "" {
				state.Network.ObserveErrors = append(state.Network.ObserveErrors, observeErrorText(result))
			}
		}
	}
	if room := c.room; room != nil {
		view := room.view
		state.Room = &view
		if room.hostForwarder == nil && room.endpoint != nil {
			state.Network.HostAddrs = p2p.ClassifyAddrs(room.endpoint.Addrs)
			state.Network.HostNoPublic = !p2p.HasPublic(state.Network.HostAddrs)
		}
		if room.hostForwarder != nil {
			state.Network.Targets = room.hostForwarder.Targets()
			for _, member := range room.view.Snapshot.Members {
				link := mpAPI.LinkState{State: mpConstant.LinkConnecting}
				if member.Link != nil {
					link = *member.Link
				}
				state.Links = append(state.Links, LinkView{UserID: member.UserID, LinkState: link})
			}
		}
		if room.link != nil {
			state.Links = append(state.Links, room.link.view(state.Account.UserID))
		}
		if room.local != nil {
			state.LocalPorts = room.local.LocalPorts()
		}
	}
	return state
}

// observeErrorText 返回一个观测节点拿不到 observed 地址的说明。
func observeErrorText(result p2p.ObserverResult) string {
	if result.Observer == "" {
		return result.Err
	}
	return "观测节点 " + result.Observer + "：" + result.Err
}

// getState 返回当前投影。
func (c *controller) getState() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotState()
}

// emit 推送投影。
func (c *controller) emit() {
	if c.opts.emit == nil {
		return
	}
	c.mu.Lock()
	state := c.snapshotState()
	c.mu.Unlock()
	c.opts.emit(state)
}
