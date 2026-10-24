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
	loggingConstant "DysonSphereBox/constant/logging"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/core/diagnostics"
	"DysonSphereBox/helper/multiplayer/forward"
	"DysonSphereBox/helper/multiplayer/link"
	"DysonSphereBox/helper/multiplayer/p2p"
	"DysonSphereBox/helper/multiplayer/socket"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	ma "github.com/multiformats/go-multiaddr"
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
	conn *socket.Conn
	host *p2p.Host
	info mpAPI.SessionOpenReply
	port int
	stop chan struct{}
	// initialized 职责：阻止打开会话回复覆盖紧接着到达的策略；使用：事件分发等待初始化；边界：失败时由 stop 解除等待。
	initialized             chan struct{}
	firstReport             chan struct{}
	reportOnce              sync.Once
	terminate               int
	relayAdmissionRequested bool
	relayAdmissionEnabled   bool
	report                  p2p.Report
	observed                []p2p.Observation
	// autoNATReachability 职责：官方 AutoNAT 入站判断；使用：地址上报和本地诊断；边界：初始 unknown，事件只影响诊断，不筛除 observed 候选。
	autoNATReachability string
	// observeResults 职责：最近一次观测的逐节点结果；使用：投影中拿不到 observed 地址的原因；边界：每次重新观测整体替换。
	observeResults []p2p.ObserverResult
	// tracer 职责：本会话的 link.trace 上报器；使用：各 stage；边界：socket 建立后创建、endSession 关闭；nil 时上报为空操作。
	tracer *link.Tracer
	// relayLog 职责：中继预约最近一次记录的签名；使用：避免续约成功或同一失败重复写日志；边界：由控制器锁保护。
	relayLog map[string]string
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
	// lastMessage 职责：保留最近控制链路失败原因；使用：离线反馈；边界：写入前脱敏限长，新连接成功后清空。
	lastMessage string
	// notice 职责：保留最近房间管理原因；使用：回到大厅展示；边界：不驱动重连。
	notice         *NoticeView
	sess           *session
	lobby          LobbyView
	room           *roomState
	relayAdmission *RelayAdmissionView
	pending        map[string]int
	// localBases 职责：按房间记住上次成功的本地端口起点；使用：重进同一房间时复用，避免游戏里的本地地址变化；边界：只在本进程内有效，最多保留 32 个房间。
	localBases map[string][]int
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
		lobby: LobbyView{List: []mpAPI.RoomSummary{}}, localBases: map[string][]int{}}
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
	c.lastMessage = ""
	c.notice = nil
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

// startHost 用持久化端口启动 host，端口不可用时先重试原端口，仍失败才换号。
// 职责：建立后续联机所需的数据面 host；使用：runSession 每次控制链路拨号前调用。
// 边界：同一端口重试 ListenPortRetries 次；只有连续失败才换号并写回，避免一次瞬时占用永久改掉用户端口。
// 逻辑流：1. 读取持久化端口 => 2. 原端口退避重试 => 3. 仍失败则换号并记录 => 4. 返回真实末次错误。
func (c *controller) startHost() (*p2p.Host, int, error) {
	if c.opts.listenAddrs != nil {
		h, err := p2p.NewHost(0, c.opts.listenAddrs)
		return h, 0, err
	}
	port := p2p.ListenPort()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		for retry := 0; retry < mpConstant.ListenPortRetries; retry++ {
			h, err := p2p.NewHost(port, nil)
			if err == nil && h != nil && p2p.HasIPv4QUIC(h) {
				if retry > 0 {
					recordStateAsync(loggingConstant.Warn, "host.port", "联机监听端口重试成功", "原端口曾被短暂占用，重试后仍在原端口监听",
						map[string]interface{}{"listenPort": port, "retries": retry})
				}
				return h, port, nil
			}
			if h != nil {
				_ = h.Close()
			}
			lastErr = err
			if lastErr == nil {
				lastErr = errors.New("本机未能建立 IPv4 QUIC 监听")
			}
			if retry+1 < mpConstant.ListenPortRetries {
				time.Sleep(mpConstant.ListenPortRetryDelay)
			}
		}
		// 最后一次尝试不再换号：保留原端口，交给下一次重连继续复用
		if attempt+1 >= 3 {
			break
		}
		previous := port
		port = p2p.NewRandomListenPort()
		recordStateAsync(loggingConstant.Warn, "host.port", "联机监听端口已更换", "原端口连续绑定失败，换号并写回配置",
			map[string]interface{}{"previousPort": previous, "listenPort": port, "error": link.ErrText(lastErr)})
	}
	return nil, 0, fmt.Errorf("libp2p 监听失败：%w", lastErr)
}

// runSession 建立一次控制链路并阻塞到断开。
// 返回：是否曾经在线、断开/拒绝码、是否不应自动重连。
func (c *controller) runSession(stop chan struct{}) (bool, int, bool) {
	url, err := c.opts.socketURL()
	if err != nil {
		c.setConnectionMessage(stop, "联机服务器地址无效："+err.Error())
		return false, mpConstant.CodeNotConnected, true
	}
	for attempt := 0; attempt < 3; attempt++ {
		h, port, err := c.startHost()
		if err != nil {
			c.setConnectionMessage(stop, "启动本机联机监听失败："+err.Error())
			return false, mpConstant.CodeNotConnected, false
		}
		sess := &session{host: h, port: port, stop: make(chan struct{}), initialized: make(chan struct{}), firstReport: make(chan struct{}), autoNATReachability: "unknown"}
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
				message := strings.TrimSpace(handshake.Msg)
				if handshake.Status != 0 {
					message = fmt.Sprintf("WebSocket 握手被拒绝（HTTP %d）", handshake.Status)
					if handshake.Code != 0 {
						message += fmt.Sprintf("，服务器码 %d", handshake.Code)
					}
					if strings.TrimSpace(handshake.Msg) != "" {
						message += "：" + strings.TrimSpace(handshake.Msg)
					}
				}
				if message == "" {
					message = "WebSocket 握手未完成"
				}
				c.setConnectionMessage(stop, message)
				return false, code, handshake.Permanent()
			}
			c.setConnectionMessage(stop, "连接联机服务器失败："+err.Error())
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

		sess.relayAdmissionRequested = true
		reply, err := conn.Request(context.Background(), mpConstant.OpSessionOpen, mpAPI.SessionOpenRequest{
			ProtocolVersion: mpConstant.ProtocolVersion, ClientVersion: c.opts.clientVersion, PeerID: h.ID().String(), ManagementEvents: true,
			RelayAdmissionEvents: sess.relayAdmissionRequested})
		if err == nil && reply.Code == mpConstant.CodePeerIDInUse {
			c.endSession(sess)
			continue
		}
		if err != nil || reply.Code != 0 {
			code := mpConstant.CodeDisconnected
			message := ""
			if err == nil {
				code = reply.Code
				message = socket.ReplyMessage(reply)
			} else {
				var localErr *socket.LocalError
				if errors.As(err, &localErr) {
					code = localErr.Code
					switch localErr.Code {
					case mpConstant.CodeRequestTimeout:
						message = "服务器未在规定时间内响应会话握手"
					case mpConstant.CodeDisconnected:
						message = "发送会话握手请求时控制链路已断开"
					}
				}
				if message == "" {
					message = err.Error()
				}
			}
			c.setConnectionMessage(stop, message)
			c.endSession(sess)
			permanent := code == mpConstant.CodeProtocolVersion || code == mpConstant.CodeClientTooOld || code == mpConstant.CodeAdmissionPaused
			return false, code, permanent
		}
		c.mu.Lock()
		_ = json.Unmarshal(reply.Data, &sess.info)
		sess.relayAdmissionEnabled = negotiatedRelayAdmissionEvents(sess.relayAdmissionRequested, sess.info.Capabilities)
		c.relayAdmission = nil
		c.clearSessionNotice()
		if sess.info.Policy != nil && sess.info.Policy.MaxMembers > 0 {
			sess.info.Limits.MaxMembers = sess.info.Policy.MaxMembers
		}
		c.mu.Unlock()
		// 官方 AutoNAT 回拨使用独立临时 PeerID；准入仅限 observer
		// 入口与服务端明确配置的公网出口，不授予游戏协议权限。
		h.Gater.SetAutoNATSources(p2p.AutoNATSources(context.Background(), sess.info.Observers, sess.info.AutoNATEgressIPs))
		if err := h.ConfigureRelay(sess.info.Capabilities, sess.info.Relays, func(state p2p.RelayState, duration time.Duration) {
			sess.tracer.Trace(mpAPI.LinkTraceRequest{Stage: mpConstant.TraceRelayReserve, OK: state.ExpiresAtMs > time.Now().UnixMilli(),
				DurationMs: duration.Milliseconds(), Error: state.Error, Detail: fmt.Sprintf("relay=%s expires=%d duration=%ds data=%d", state.PeerID, state.ExpiresAtMs, state.LimitDurationSeconds, state.LimitDataBytes)})
			c.logRelayReserve(sess, state, duration)
		}); err != nil {
			sess.tracer.Trace(mpAPI.LinkTraceRequest{Stage: mpConstant.TraceRelayReserve, Error: err.Error()})
			recordStateAsync(loggingConstant.Warn, "relay.policy", "中继策略应用失败", "session.open 下发的入口无法应用，本会话不使用中继",
				map[string]interface{}{"error": link.ErrText(err)})
		}
		h.SetDCUtRTracer(func(evt *holepunch.Event) { go c.traceDCUtR(sess, evt) })
		h.RunReservations()

		c.mu.Lock()
		c.connState, c.lastErr, c.lastMessage = mpConstant.ConnOnline, 0, ""
		close(sess.initialized)
		c.mu.Unlock()
		recordStateAsync(loggingConstant.Info, "session.online", "联机会话已建立", "session.open 成功，控制链路在线",
			map[string]interface{}{
				"sessionId": sess.info.SessionID, "peerId": h.ID().String(), "listenPort": port,
				"nativeDcutr": sess.info.Capabilities.NativeDCUtR, "relayV2": sess.info.Capabilities.RelayV2,
				"relayFallback": sess.info.Capabilities.RelayFallback, "relays": sess.info.Relays,
			})
		c.emit()
		go c.addrLoop(sess)

		openedAt := time.Now()
		select {
		case <-conn.Done():
		case <-stop:
			conn.Close()
		}
		connectionErr := conn.Err()
		c.endSession(sess)
		c.mu.Lock()
		reason := sess.terminate
		c.mu.Unlock()
		level := loggingConstant.Warn
		if reason == 0 {
			level = loggingConstant.Info
		}
		detail := ""
		if connectionErr != nil {
			detail = connectionErr.Error()
		}
		recordStateAsync(level, "session.closed", "联机会话已断开", "控制链路读循环结束或被服务端断开",
			map[string]interface{}{
				"sessionId": sess.info.SessionID, "reason": reason, "permanent": reason != 0,
				"error": detail, "onlineMs": durationMs(openedAt),
			})
		if reason != 0 {
			return true, reason, permanentTerminationReason(reason)
		}
		select {
		case <-stop:
			return true, 0, false
		default:
		}
		if reason == 0 {
			reason = mpConstant.CodeDisconnected
		}
		message := "控制链路已断开"
		if connectionErr != nil {
			message += "：" + connectionErr.Error()
		}
		c.setConnectionMessage(stop, message)
		return true, reason, false
	}
	return false, mpConstant.CodePeerIDInUse, false
}

// permanentTerminationReason 仅对协议指定的维护原因保留自动退避重连。
func permanentTerminationReason(reason int) bool {
	return reason != mpConstant.ReasonMaintenance
}

// negotiatedRelayAdmissionEvents 仅在请求与响应能力都明确为 true 时启用事件。
func negotiatedRelayAdmissionEvents(requested bool, capabilities mpAPI.Capabilities) bool {
	return requested && capabilities.RelayAdmissionEvents
}

// clearSessionNotice 清除上一会话遗留的房间/管理通知。
func (c *controller) clearSessionNotice() {
	c.notice = nil
}

// setConnectionMessage 保存可向用户展示的失败原因。
// 职责：脱敏并限制最近连接错误摘要；使用：控制链路拨号、握手和断线失败时调用。
// 边界：只更新当前重连循环，旧循环不能覆盖新连接状态。
func (c *controller) setConnectionMessage(stop chan struct{}, message string) {
	message = c.cleanConnectionMessage(message)
	c.mu.Lock()
	if c.stopLoop == stop {
		c.lastMessage = message
	}
	c.mu.Unlock()
}

// cleanConnectionMessage 清洗并限制控制链路错误摘要。
// 职责：避免前端投影携带凭据或过长服务端文本；使用：保存拨号、握手和服务端断开原因。
// 边界：保留可用于排障的网络错误内容，不将摘要写入服务器协议。
func (c *controller) cleanConnectionMessage(message string) string {
	message = strings.TrimSpace(diagnostics.CleanText(message))
	if runes := []rune(message); len(runes) > 512 {
		message = string(runes[:512]) + "…"
	}
	return message
}

// endSession 拆除会话：房间、转发、host 全部销毁。
func (c *controller) endSession(sess *session) {
	c.mu.Lock()
	if c.sess != sess {
		c.mu.Unlock()
		return
	}
	c.sess = nil
	c.relayAdmission = nil
	// 新 socket 必未订阅（消息定义：socket 断开即取消订阅），复位后由前端在重连后对齐，避免重连多发一帧。
	c.lobby.Watching = false
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

// ===================== 状态日志与本地端口复用 =====================

// logRelayReserve 记录中继预约结果的变化。
// 职责：把预约限额和错误写入本地日志；使用：ConfigureRelay 回调；边界：相同结果只记一次，成功续约不重复刷屏。
func (c *controller) logRelayReserve(sess *session, state p2p.RelayState, duration time.Duration) {
	ok := state.ExpiresAtMs > time.Now().UnixMilli()
	signature := "ok"
	if !ok {
		signature = "error:" + state.Error
	}
	c.mu.Lock()
	if c.sess != sess {
		c.mu.Unlock()
		return
	}
	if sess.relayLog == nil {
		sess.relayLog = map[string]string{}
	}
	previous, seen := sess.relayLog[state.PeerID]
	sess.relayLog[state.PeerID] = signature
	c.mu.Unlock()
	if seen && previous == signature {
		return
	}
	level := loggingConstant.Info
	if !ok {
		level = loggingConstant.Warn
	}
	recordState(level, "relay.reserve", "中继预约结果", "原生 Circuit Relay v2 预约成功、续约或失败",
		map[string]interface{}{
			"relayPeerId": state.PeerID, "expiresAtMs": state.ExpiresAtMs,
			"limitDurationSeconds": state.LimitDurationSeconds, "limitDataBytes": state.LimitDataBytes,
			"error": state.Error, "durationMs": duration.Milliseconds(),
		})
}

// localPortKey 生成本地端口复用键。
// 职责：同一房主的同一房间复用上次起点；使用：startMember；边界：换房主或换房间自然换键。
func localPortKey(roomID, hostPeerID string) string { return roomID + "|" + hostPeerID }

// rememberLocalBases 记住本次分配的本地端口起点。
// 职责：供重进同一房间时复用，避免游戏里的本地地址变化；使用：startMember 成功后（持控制器锁）；
// 边界：最多保留 32 个房间，超出后整体清空重新记录。
func (c *controller) rememberLocalBases(key string, bases []int) {
	if len(bases) == 0 {
		return
	}
	if c.localBases == nil || len(c.localBases) >= 32 {
		c.localBases = map[string][]int{}
	}
	c.localBases[key] = append([]int(nil), bases...)
}

// ===================== 地址上报 =====================

// addrLoop 观测地址、上报 peer.addrs，并在网卡或 libp2p 地址变化时重报。
// 防火墙每个会话（新的 libp2p host）检测一次，与首次观测并行执行。
func (c *controller) addrLoop(sess *session) {
	// 会话关闭立即取消观测和地址上报，避免旧 host 继续拨号或覆盖新会话诊断。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-sess.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	firewallCh := make(chan bool, 1)
	go func() { firewallCh <- c.opts.firewall() }()
	firewall, firewallKnown := true, false
	var last p2p.Report
	reported := false
	// lastObserve 限制断线事件触发的观测重建频率。观测轮次会主动关闭并重建
	// observer 连接，不能在一连串 connectedness 事件中递归重建。
	var lastObserve time.Time
	report := func(observe bool) {
		if observe {
			if !lastObserve.IsZero() && time.Since(lastObserve) < 15*time.Second {
				observe = false
			} else {
				lastObserve = time.Now()
			}
		}
		if observe {
			observed, results := c.opts.observe(ctx, sess.host, sess.info.Observers, c.opts.observeTimeout)
			sess.host.SetObservedAddrs(sess.info.Observers, observed)
			c.mu.Lock()
			if c.sess != sess || ctx.Err() != nil {
				c.mu.Unlock()
				return
			}
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
		current := p2p.Collect(sess.host, observed, firewall, c.opts.keepLoopback, sess.autoNATReachability)
		c.mu.Lock()
		sess.report = current
		c.mu.Unlock()
		if reported && slices.Equal(current.Addrs, last.Addrs) && sameNet(current.Net, last.Net) {
			return
		}
		reply, err := sess.conn.Request(ctx, mpConstant.OpPeerAddrs,
			mpAPI.PeerAddrsRequest{Addrs: current.Addrs, Net: current.Net})
		if err == nil && reply.Code == 0 {
			last, reported = current, true
			sess.reportOnce.Do(func() { close(sess.firstReport) })
		}
		c.emit()
	}
	report(true)

	sub, err := sess.host.EventBus().Subscribe([]interface{}{new(event.EvtLocalAddressesUpdated), new(event.EvtLocalReachabilityChanged), new(event.EvtPeerConnectednessChanged)})
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
		case raw, open := <-updates:
			if !open {
				updates = nil
				continue
			}
			if evt, ok := raw.(event.EvtLocalReachabilityChanged); ok {
				sess.autoNATReachability = autoNATText(evt.Reachability)
			} else if evt, ok := raw.(event.EvtPeerConnectednessChanged); ok && evt.Connectedness == network.NotConnected && observerPeer(sess.info.Observers, evt.Peer) {
				// observedaddrs.Manager 在连接断开时立即移除该连接贡献的
				// observed 地址。仅在 observer 断线时重观测，且由 15s 冷却
				// 保证有界，避免高频重建冲掉最后候选。
				report(true)
			}
			if _, ok := raw.(event.EvtPeerConnectednessChanged); !ok {
				report(false)
			}
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
			} else if observerDisconnected(sess.host, sess.info.Observers) {
				report(true)
			} else if time.Since(lastObserve) >= 5*time.Minute {
				// NAT 映射变化未必伴随网卡事件；低频刷新 identify 的
				// observed 地址，避免长期使用旧映射。
				report(true)
			} else if !reported {
				report(false)
			}
		}
	}
}

// observerPeer 判断断线事件是否来自 session.open 指定的观测节点。
// 职责：限定 observed 地址重建触发源；使用：addrLoop；边界：非法入口忽略，不影响游戏成员连接。
func observerPeer(observers []string, id peer.ID) bool {
	for _, raw := range p2p.FirstObserverAddrs(observers) {
		addr, err := ma.NewMultiaddr(raw)
		if err != nil {
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err == nil && info.ID == id {
			return true
		}
	}
	return false
}

// observerDisconnected 检查指定观测节点是否仍有连接承载原生 observed 地址。
// 职责：兜底丢失的断线事件；使用：addrLoop 的 10s 检查；边界：无合法 observer 时不触发重试。
func observerDisconnected(h *p2p.Host, observers []string) bool {
	observers = p2p.FirstObserverAddrs(observers)
	if len(observers) == 0 {
		return false
	}
	addr, err := ma.NewMultiaddr(observers[0])
	if err != nil {
		return false
	}
	info, err := peer.AddrInfoFromP2pAddr(addr)
	return err == nil && h.Network().Connectedness(info.ID) != network.Connected
}

func sameNet(a, b mpAPI.NetInfo) bool {
	return a.UPnP == b.UPnP && a.IPv6 == b.IPv6 && a.FirewallRule == b.FirewallRule && a.AutoNATReachability == b.AutoNATReachability
}

// autoNATText 职责：把官方 AutoNAT 状态转换成控制链路诊断枚举；使用：reachability 事件；边界：未知枚举保守上报 unknown。
func autoNATText(reachability network.Reachability) string {
	switch reachability {
	case network.ReachabilityPublic:
		return "public"
	case network.ReachabilityPrivate:
		return "private"
	default:
		return "unknown"
	}
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
	if policy := sess.info.Policy; policy != nil && (!policy.AdmissionEnabled || policy.Maintenance) && (opcode == mpConstant.OpRoomCreate || opcode == mpConstant.OpRoomJoin) {
		c.mu.Unlock()
		return ActionResult{Code: mpConstant.CodeAdmissionPaused, Msg: policy.Message}
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
	if reply.Code != 0 {
		result.Msg = socket.ReplyMessage(reply)
	}
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
		changed := c.lobby.Watching != on
		c.lobby.Watching = on
		c.mu.Unlock()
		// 说明：订阅态实际变化才推送；request() 两端的 emit 保留，pending/按钮态依赖它。
		if changed {
			c.emit()
		}
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
	if c.relayAdmission != nil && c.relayAdmission.RoomID == c.room.view.Snapshot.RoomID {
		c.relayAdmission.Retrying = true
	}
	c.room.link.wakeUp()
	go c.emit()
	return ActionResult{}
}

// ===================== 事件处理 =====================

// handleEvent 处理一个服务器事件并返回回复码。
func (c *controller) handleEvent(sess *session, frame *mpAPI.ActiveFrame) (int, string) {
	if sess.initialized != nil {
		select {
		case <-sess.initialized:
		case <-sess.stop:
			return 0, ""
		}
	}
	if frame.Opcode == mpConstant.EvSessionPolicyAfter {
		return c.applyPolicy(sess, frame)
	}
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
	case mpConstant.EvSessionRelayAdmissionBefore:
		if !sess.relayAdmissionEnabled {
			return mpConstant.CodeUnknownOpcode, "unknown opcode", false
		}
		var data mpAPI.RelayAdmissionEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodePrepareFailed, "bad data", false
		}
		if data.SessionID != sess.info.SessionID || c.sess != sess || c.room == nil || data.RoomID == "" || data.RoomID != c.room.view.Snapshot.RoomID {
			return 0, "", false
		}
		return 0, "", false

	case mpConstant.EvSessionPolicyBefore, mpConstant.EvLobbyChangeBefore, mpConstant.EvRoomCreateBefore,
		mpConstant.EvRoomInviteBefore, mpConstant.EvMemberLinkBefore, mpConstant.EvHostAddrsBefore:
		return 0, "", false

	case mpConstant.EvSessionTerminateBefore:
		// before 只告知终止即将发生，等 after 再拆除数据面，避免过早关闭事件回复所需的控制链路。
		return 0, "", false

	case mpConstant.EvSessionTerminateAfter:
		var data mpAPI.EventBase
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		sess.terminate = data.Reason
		c.lastErr, c.lastMessage = data.Reason, c.cleanConnectionMessage(data.Message)
		c.notice = &NoticeView{Code: data.Reason, Message: data.Message}
		c.relayAdmission = nil
		teardownRoom(sess, c.room)
		c.room = nil
		c.connState = mpConstant.ConnReconnecting
		// 控制链路仍回复事件；数据面立即撤销，随后由 endSession 完成会话释放。
		go sess.host.Close()
		return 0, "", true

	case mpConstant.EvLobbyChangeAfter:
		var data mpAPI.LobbyChangeEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		c.mergeLobby(data)
		return 0, "", true

	case mpConstant.EvSessionRelayAdmissionAfter:
		if !sess.relayAdmissionEnabled {
			return mpConstant.CodeUnknownOpcode, "unknown opcode", false
		}
		var data mpAPI.RelayAdmissionEvent
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if data.SessionID != sess.info.SessionID || c.sess != sess || c.room == nil || data.RoomID == "" || data.RoomID != c.room.view.Snapshot.RoomID {
			return 0, "", false
		}
		if data.Stage != "connect" || data.Status != "member_limit" || data.ErrorCode != mpConstant.RelayAdmissionServerFull {
			return 0, "", false
		}
		c.relayAdmission = &RelayAdmissionView{SessionID: data.SessionID, RoomID: data.RoomID, Stage: data.Stage,
			Status: data.Status, ErrorCode: data.ErrorCode, RelayPeerID: data.RelayPeerID, AtMs: data.AtMs}
		return 0, "", true

	case mpConstant.EvRoomCreateAfter:
		var data mpAPI.RoomEvent
		if json.Unmarshal(frame.Data, &data) != nil || data.Result != 0 || data.Room == nil {
			return 0, "", false
		}
		teardownRoom(sess, c.room)
		forwarder := forward.NewHostForwarder(sess.host, data.Room.Ports)
		forwarder.OnTrace = func(event forward.TraceEvent) { go c.traceForward(sess, event) }
		// 主动探测发现游戏开始/停止监听时推送新投影；emit 需取控制器锁，另起 goroutine 避免阻塞探测循环
		forwarder.OnTargets = func() { go c.emit() }
		forwarder.Start()
		c.room = &roomState{view: RoomView{Snapshot: *data.Room, Role: mpConstant.RoleHost},
			hostForwarder: forwarder, pendingJoin: map[int64]peer.ID{}, leaving: map[int64]peer.ID{}}
		c.relayAdmission = nil
		c.notice = nil
		return 0, "", true

	case mpConstant.EvRoomCloseBefore:
		var data mpAPI.EventBase
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodePrepareFailed, "bad data", false
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID {
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
		if json.Unmarshal(frame.Data, &data) != nil {
			return mpConstant.CodeApplyFailed, "bad data", false
		}
		if data.Result != 0 {
			return 0, "", false
		}
		if c.room != nil && data.RoomID == c.room.view.Snapshot.RoomID {
			teardownRoom(sess, c.room)
			c.room = nil
			c.relayAdmission = nil
			c.notice = &NoticeView{Code: data.Reason, Message: data.Message}
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
			c.notice = &NoticeView{Code: data.Result, Message: data.Message}
			return 0, "", true
		}
		if data.Member.UserID == selfID && c.room == nil && data.Room != nil && data.Host != nil {
			c.relayAdmission = nil
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
			c.relayAdmission = nil
			c.notice = &NoticeView{Code: data.Reason, Message: data.Message}
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
			data.Host.Addrs = sess.host.FilterRelayAddrs(data.Host.Addrs, c.room.link.hostID)
			c.room.endpoint = data.Host
			c.room.link.updateEndpoint(*data.Host)
		}
		return 0, "", true

	}
	return mpConstant.CodeUnknownOpcode, "unknown opcode", false
}

// startMember 加入已提交：开本地端口 M 并启动链路。
func (c *controller) startMember(sess *session, data mpAPI.MemberJoinEvent) (int, string, bool) {
	c.notice = nil
	c.relayAdmission = nil
	hostID, err := peer.Decode(data.Host.PeerID)
	if err != nil {
		return mpConstant.CodeApplyFailed, "bad host peerId", false
	}
	data.Host.Addrs = sess.host.FilterRelayAddrs(data.Host.Addrs, hostID)
	room := &roomState{view: RoomView{Snapshot: *data.Room, Role: mpConstant.RoleMember}, endpoint: data.Host,
		pendingJoin: map[int64]peer.ID{}, leaving: map[int64]peer.ID{}}
	// 打洞时房主回拨，成员只放行房主
	sess.host.Gater.Allow(hostID)
	// 重进同一房间时复用上次端口起点，游戏里的本地地址才不会变
	key := localPortKey(data.Room.RoomID, data.Host.PeerID)
	room.local = forward.NewLocalForwarder(sess.host, hostID, data.Room.Ports, c.localBases[key])
	room.local.OnTrace = func(event forward.TraceEvent) { go c.traceForward(sess, event) }
	opened := time.Now()
	local, bases, err := room.local.Open()
	portsTrace := mpAPI.LinkTraceRequest{RoomID: data.Room.RoomID, PeerUserID: data.Room.Host.UserID,
		Stage: mpConstant.TracePortsOpen, OK: err == nil, DurationMs: time.Since(opened).Milliseconds(), Error: link.ErrText(err)}
	if err == nil {
		addrs := make([]string, 0, len(local))
		for _, item := range local {
			addrs = append(addrs, item.Proto+" "+item.Address)
		}
		portsTrace.Detail = strings.Join(addrs, ", ")
		c.rememberLocalBases(key, bases)
		recordStateAsync(loggingConstant.Info, "room.ports", "本地转发端口已分配", "member.join 提交后绑定本地端口",
			map[string]interface{}{"roomId": data.Room.RoomID, "hostPeerId": data.Host.PeerID, "bases": bases, "ports": addrs})
	} else {
		recordStateAsync(loggingConstant.Warn, "room.ports", "本地转发端口分配失败", "member.join 提交后本地端口无法绑定",
			map[string]interface{}{"roomId": data.Room.RoomID, "error": link.ErrText(err)})
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

// clearRelayAdmission 清理与指定会话/房间关联的准入诊断。
func (c *controller) clearRelayAdmission(sess *session, roomID string) {
	c.mu.Lock()
	if c.relayAdmission != nil && c.sess == sess && c.relayAdmission.SessionID == sess.info.SessionID && c.relayAdmission.RoomID == roomID {
		c.relayAdmission = nil
	}
	c.mu.Unlock()
}

// finishRelayAdmissionRetry 结束本轮重试标记但保留服务端容量原因，供用户再次操作。
func (c *controller) finishRelayAdmissionRetry(sess *session, roomID string) {
	c.mu.Lock()
	if c.sess == sess && c.relayAdmission != nil && c.relayAdmission.SessionID == sess.info.SessionID &&
		c.relayAdmission.RoomID == roomID && c.relayAdmission.Retrying {
		c.relayAdmission.Retrying = false
	}
	c.mu.Unlock()
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
		Connection:    ConnectionView{State: c.connState, LastErrorCode: c.lastErr, LastErrorMessage: c.lastMessage},
		Notice:        c.notice,
		Account:       AccountView{LoggedIn: c.opts.token() != ""},
		Lobby:         LobbyView{Watching: c.lobby.Watching, Total: c.lobby.Total, List: append([]mpAPI.RoomSummary{}, c.lobby.List...)},
		Links:         []LinkView{},
		LocalPorts:    []forward.LocalPort{},
		Network: NetworkView{ObservedAddrs: []string{}, Targets: []forward.TargetState{}, ReportedAddrs: []p2p.ClassifiedAddr{},
			Observed: []p2p.ClassifiedAddr{}, ObserveErrors: []string{}, HostAddrs: []p2p.ClassifiedAddr{}},
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
				ListenPort: sess.port, Limits: sess.info.Limits, Policy: sess.info.Policy, Capabilities: sess.info.Capabilities, Relays: sess.host.RelayStates()}
		}
		state.Network.UPnP = sess.report.Net.UPnP
		state.Network.IPv6 = sess.report.Net.IPv6
		state.Network.AutoNATReachability = sess.report.Net.AutoNATReachability
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
	if c.relayAdmission != nil && c.room != nil && c.relayAdmission.SessionID == c.sess.info.SessionID && c.relayAdmission.RoomID == c.room.view.Snapshot.RoomID {
		admission := *c.relayAdmission
		state.RelayAdmission = &admission
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
