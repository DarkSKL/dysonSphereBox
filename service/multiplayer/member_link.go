package multiplayer

// 文件职责：成员到房主的数据链路状态机（方案第 7.3/7.4 节）：connecting→connected→reconnecting→connected|failed，直连→原生中继/DCUtR，3 分钟放弃，link.report 与 RTT 节流。
// 使用：控制器在加入提交后 newMemberLink 并 go run；事件处理通过 updateEndpoint/wakeUp 非阻塞投递；原生 DCUtR 由 libp2p 管理。
// 边界：在独立 goroutine 中发控制链路请求，不阻塞事件 goroutine；锁顺序固定为 controller.mu → memberLink.mu，持有 memberLink.mu 时不取 controller.mu。

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/link"
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/ping"
)

// memberLink 是成员的一条到房主的数据链路。
type memberLink struct {
	c      *controller
	sess   *session
	room   *roomState
	roomID string
	hostID peer.ID
	// hostUserID 职责：房主 userId；使用：link.trace peerUserId；边界：加入时的快照值。
	hostUserID int64

	mu            sync.Mutex
	endpoint      mpAPI.HostEndpoint
	state         mpAPI.LinkState
	targetRefused bool
	lastDialErr   error
	detail        string
	detailLines   []string
	attemptFresh  bool
	everConnected bool

	reportMu   sync.Mutex
	lastReport *mpAPI.LinkState
	lastRTTAt  time.Time

	wake chan struct{}
	// connections 职责：原生连接变化；使用：更新路径；边界：不触发 failed 状态手动重试。
	connections  chan struct{}
	disconnected chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
}

func newMemberLink(c *controller, sess *session, room *roomState, hostID peer.ID, endpoint mpAPI.HostEndpoint) *memberLink {
	ctx, cancel := context.WithCancel(context.Background())
	return &memberLink{c: c, sess: sess, room: room, roomID: room.view.Snapshot.RoomID, hostID: hostID,
		hostUserID: room.view.Snapshot.Host.UserID, endpoint: endpoint,
		state: mpAPI.LinkState{State: mpConstant.LinkConnecting},
		wake:  make(chan struct{}, 1), connections: make(chan struct{}, 1), disconnected: make(chan struct{}, 1),
		ctx: ctx, cancel: cancel}
}

// current 返回当前链路状态。
func (l *memberLink) current() mpAPI.LinkState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state
}

// close 停止状态机（持有控制器锁时调用，不阻塞）。
func (l *memberLink) close() { l.closeOnce.Do(l.cancel) }

// wakeUp 唤醒 failed / reconnecting 中的等待，立即重试。
func (l *memberLink) wakeUp() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// updateEndpoint 收到 room.host.addrs after：更新房主地址，reconnecting/failed 时立即重试。
func (l *memberLink) updateEndpoint(endpoint mpAPI.HostEndpoint) {
	l.mu.Lock()
	l.endpoint = endpoint
	state := l.state.State
	l.mu.Unlock()
	if state == mpConstant.LinkReconnecting || state == mpConstant.LinkFailed {
		l.wakeUp()
	}
}

// onTCPStatus 根据 TCP 握手结果设置或清除 20407（链路仍 connected）。
func (l *memberLink) onTCPStatus(status byte) {
	l.mu.Lock()
	refused := status == mpConstant.StatusTargetRefused
	if refused == l.targetRefused || l.state.State != mpConstant.LinkConnected {
		l.mu.Unlock()
		return
	}
	l.targetRefused = refused
	if refused {
		l.state.Error = mpConstant.LinkErrTargetRefused
	} else {
		l.state.Error = 0
	}
	l.mu.Unlock()
	go l.changed()
}

// setState 修改链路状态并上报、推送投影。
func (l *memberLink) setState(update func(state *mpAPI.LinkState)) {
	l.mu.Lock()
	update(&l.state)
	l.mu.Unlock()
	l.changed()
}

// changed 上报（state/path/error 变化时）并推送投影。
func (l *memberLink) changed() {
	l.report(false)
	l.c.emit()
}

// report 发送 link.report：state/path/error 变化即报；force 用于 RTT 节流上报。
func (l *memberLink) report(force bool) {
	l.reportMu.Lock()
	defer l.reportMu.Unlock()
	current := l.current()
	if !force && l.lastReport != nil && l.lastReport.State == current.State && l.lastReport.Path == current.Path &&
		l.lastReport.Error == current.Error {
		return
	}
	if l.ctx.Err() != nil {
		return
	}
	reply, err := l.sess.conn.Request(l.ctx, mpConstant.OpLinkReport,
		mpAPI.LinkReportRequest{RoomID: l.roomID, Link: current})
	if err == nil && reply.Code == 0 {
		l.lastReport = &current
	}
}

// run 是状态机主循环。
func (l *memberLink) run() {
	notifiee := &network.NotifyBundle{ConnectedF: func(_ network.Network, conn network.Conn) {
		if conn.RemotePeer() == l.hostID {
			select {
			case l.connections <- struct{}{}:
			default:
			}
		}
	}, DisconnectedF: func(_ network.Network, conn network.Conn) {
		if conn.RemotePeer() == l.hostID {
			select {
			case l.disconnected <- struct{}{}:
			default:
			}
		}
	}}
	l.sess.host.Network().Notify(notifiee)
	defer l.sess.host.Network().StopNotify(notifiee)
	l.changed()

	var reconnectSince time.Time
	for l.ctx.Err() == nil {
		_, ok := l.establish()
		if l.ctx.Err() != nil {
			return
		}
		if ok {
			l.onConnected()
			connectedAt := time.Now()
			l.holdConnection()
			if l.ctx.Err() != nil {
				return
			}
			l.trace(mpConstant.TraceDisconnected, 0, false, connectedAt, "", "", "")
			l.room.local.SetAvailable(false)
			reconnectSince = time.Now()
			l.setState(func(state *mpAPI.LinkState) {
				*state = mpAPI.LinkState{State: mpConstant.LinkReconnecting}
			})
			continue
		}

		l.mu.Lock()
		state := l.state.State
		l.mu.Unlock()
		if state == mpConstant.LinkReconnecting && time.Since(reconnectSince) < mpConstant.ReconnectGiveUp {
			l.waitRetry(time.Second)
			continue
		}
		l.fail()
		// failed：等待手动重试或房主新地址
		select {
		case <-l.ctx.Done():
			return
		case <-l.wake:
		}
		reconnectSince = time.Now()
		l.setState(func(linkState *mpAPI.LinkState) {
			next := mpConstant.LinkConnecting
			if l.everConnected {
				next = mpConstant.LinkReconnecting
			}
			*linkState = mpAPI.LinkState{State: next}
		})
	}
}

// waitRetry 等待 delay 或被唤醒。
func (l *memberLink) waitRetry(delay time.Duration) {
	select {
	case <-l.ctx.Done():
	case <-l.wake:
	case <-time.After(delay):
	}
}

// establish 依次直连和指定中继；职责：有界等待原生升级；使用：状态机；边界：false 回退禁止游戏经中继。
func (l *memberLink) establish() (bool, bool) {
	l.mu.Lock()
	endpoint := l.endpoint
	l.attemptFresh = true
	l.mu.Unlock()
	info, err := link.AddrInfo(endpoint.PeerID, endpoint.Addrs)
	if err != nil {
		l.recordDial(link.DialRecord{Path: "direct", PeerID: endpoint.PeerID, Err: "房主地址无法解析：" + err.Error(),
			Targets: describeAddrs(endpoint.Addrs)})
		return false, false
	}
	report := link.DialDirect(l.ctx, l.sess.host, info, mpConstant.DirectDialTimeout)
	if report.Err == nil {
		l.traceDial(mpConstant.TraceDirectDial, 0, report)
		l.recordDialReport("direct", 0, info.ID.String(), report)
		return false, true
	} else if l.ctx.Err() == nil {
		l.setDialErr(report.Err)
		l.recordDialReport("direct", 0, info.ID.String(), report)
		l.traceDial(mpConstant.TraceDirectDial, 0, report)
	}

	if l.ctx.Err() != nil || !l.sess.host.RelayEnabled() {
		return false, false
	}
	report = link.DialRelay(l.ctx, l.sess.host, info, mpConstant.DirectDialTimeout)
	l.traceDial(mpConstant.TraceRelayDial, 0, report)
	l.recordDialReport("relay", 0, info.ID.String(), report)
	if report.Err != nil {
		l.setDialErr(report.Err)
		return false, false
	}
	if l.sess.host.RelayFallback() {
		return false, true
	}
	// 仅协商模式等待库建立直接连接；窗口到期拆除本次受限连接，绝不开放游戏流。
	timer := time.NewTimer(mpConstant.DCUtRUpgradeTimeout)
	defer timer.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if p2p.BestConn(l.sess.host, l.hostID, false) != nil {
			return false, true
		}
		select {
		case <-l.ctx.Done():
			return false, false
		case <-timer.C:
			for _, conn := range l.sess.host.Network().ConnsToPeer(l.hostID) {
				if conn.Stat().Limited || p2p.IsCircuit(conn.RemoteMultiaddr()) {
					_ = conn.Close()
				}
			}
			l.setDialErr(fmt.Errorf("原生 DCUtR 升级等待超时，服务器禁止中继游戏回退"))
			l.recordFailure("relay", 0, info.ID.String(), "原生 DCUtR 升级等待超时，服务器禁止中继游戏回退")
			return false, false
		case <-ticker.C:
		}
	}
}

// describeAddrs 返回 multiaddr 字符串带私网/公网归类的写法。
func describeAddrs(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	for _, raw := range addrs {
		result = append(result, p2p.DescribeAddr(raw))
	}
	return result
}

// traceDial 上报 direct_dial / relay_dial：addr 为实际连上的地址，detail 为目标地址列表，error 为顶层错误 + 逐地址错误。
func (l *memberLink) traceDial(stage string, round int, report p2p.DialReport) {
	l.sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: l.roomID, PeerUserID: l.hostUserID, Stage: stage, Round: round,
		OK: report.Err == nil, DurationMs: report.Duration.Milliseconds(), Addr: report.TraceAddr(),
		Error: report.ErrorText(mpConstant.TraceErrorMax), Detail: report.TargetsDetail(mpConstant.TraceDetailMax)})
}

// recordDialReport 用拨号诊断结果记录一次直连/中继。
func (l *memberLink) recordDialReport(path string, round int, peerID string, report p2p.DialReport) {
	record := link.DialRecord{Path: path, Round: round, PeerID: peerID}
	record.ApplyReport(report)
	l.recordDial(record)
}

func (l *memberLink) setDialErr(err error) {
	l.mu.Lock()
	l.lastDialErr = err
	l.mu.Unlock()
}

// maxDetailLines 职责：links[].detail 保留的记录条数；使用：recordDial；边界：只保留本轮尝试（直连 + 中继）的最近几条，每条可含多行地址。
const maxDetailLines = 4

// recordFailure 记录一次没有真正拨号的失败（能力关闭或升级失败等）。
func (l *memberLink) recordFailure(path string, round int, peerID string, text string) {
	l.recordDial(link.DialRecord{Path: path, Round: round, PeerID: peerID, Err: text})
}

// recordDial 记录一次直连/中继结果：写盒子日志；失败时把文本（含目标地址与逐地址错误）追加到本地投影 links[].detail。
// 边界：detail 只进前端投影，不进 link.report；一轮尝试的失败逐条累积，新一轮的第一条失败替换旧内容；成功只写日志、不改 detail；日志写入放到独立 goroutine，不阻塞状态机。
func (l *memberLink) recordDial(record link.DialRecord) {
	record.Role, record.RoomID, record.PeerUserID = link.DialRoleMember, l.roomID, l.hostUserID
	go l.c.opts.logDial(record)
	if record.OK {
		return
	}
	l.mu.Lock()
	if l.attemptFresh {
		l.detailLines, l.attemptFresh = nil, false
	}
	l.detailLines = append(l.detailLines, record.Text())
	if len(l.detailLines) > maxDetailLines {
		l.detailLines = l.detailLines[len(l.detailLines)-maxDetailLines:]
	}
	l.detail = strings.Join(l.detailLines, "\n")
	l.mu.Unlock()
	l.c.emit()
}

// view 返回投影用的链路：协议字段 + 本地 detail。
func (l *memberLink) view(userID int64) LinkView {
	l.mu.Lock()
	defer l.mu.Unlock()
	view := LinkView{UserID: userID, LinkState: l.state, Detail: l.detail}
	return view
}

// onConnected 连通后设置实际路径并开放本地端口；职责：投影可用游戏链路；使用：状态机；边界：仅协商电路不调用。
func (l *memberLink) onConnected() {
	conn := p2p.BestConn(l.sess.host, l.hostID, l.sess.host.RelayFallback())
	l.mu.Lock()
	hostNet := l.endpoint.Net
	l.everConnected = true
	l.mu.Unlock()
	path, family := mpConstant.PathDirect, ""
	if conn != nil {
		path, family = link.PathOf(conn, hostNet)
	}
	l.room.local.SetAvailable(true)
	l.setState(func(state *mpAPI.LinkState) {
		errorCode := 0
		if l.targetRefused {
			errorCode = mpConstant.LinkErrTargetRefused
		}
		*state = mpAPI.LinkState{State: mpConstant.LinkConnected, Path: path, IPFamily: family, Error: errorCode}
	})
	l.trace(mpConstant.TraceConnected, 0, true, time.Time{}, l.remoteAddr(), "", "path="+path+" ipFamily="+family)
	l.measureRTT()
}

// remoteAddr 返回当前到房主的第一条连接的远端地址；没有连接时为空。
func (l *memberLink) remoteAddr() string {
	conns := l.sess.host.Network().ConnsToPeer(l.hostID)
	if len(conns) == 0 {
		return ""
	}
	return conns[0].RemoteMultiaddr().String()
}

// holdConnection 在连通期间测 RTT，直到与房主的全部连接断开。
func (l *memberLink) holdConnection() {
	ticker := time.NewTicker(mpConstant.RTTReportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-l.ctx.Done():
			return
		case <-l.disconnected:
			if p2p.BestConn(l.sess.host, l.hostID, l.sess.host.RelayFallback()) == nil {
				return
			}
		case <-l.connections:
			l.refreshPath()
		case <-l.wake:
			l.refreshPath()
		case <-ticker.C:
			if p2p.BestConn(l.sess.host, l.hostID, l.sess.host.RelayFallback()) == nil {
				return
			}
			l.refreshPath()
			l.measureRTT()
		}
	}
}

// measureRTT 用 libp2p ping 测 RTT，最多每 10 秒上报一次。
func (l *memberLink) measureRTT() {
	pingCtx := network.WithNoDial(l.ctx, "rtt")
	if l.sess.host.RelayFallback() {
		pingCtx = network.WithAllowLimitedConn(pingCtx, "relay-rtt")
	}
	ctx, cancel := context.WithTimeout(pingCtx, 3*time.Second)
	defer cancel()
	result, ok := <-ping.Ping(ctx, l.sess.host, l.hostID)
	if !ok || result.Error != nil {
		return
	}
	l.mu.Lock()
	l.state.RTTMs = result.RTT.Milliseconds()
	l.mu.Unlock()
	l.reportMu.Lock()
	due := time.Since(l.lastRTTAt) >= mpConstant.RTTReportInterval
	if due {
		l.lastRTTAt = time.Now()
	}
	l.reportMu.Unlock()
	if due {
		l.report(true)
	}
	l.c.emit()
}

// fail 进入 failed 并归类失败原因。
func (l *memberLink) fail() {
	l.c.mu.Lock()
	selfReport := l.sess.report
	l.c.mu.Unlock()
	l.mu.Lock()
	input := link.ClassifyInput{DialErr: l.lastDialErr, TargetRefused: l.targetRefused, SelfAddrs: selfReport.Addrs,
		SelfNet: selfReport.Net, SelfObserved: selfReport.Observed, SelfUPnP: selfReport.UPnPAddrs,
		HostAddrs: l.endpoint.Addrs, HostNet: l.endpoint.Net}
	l.mu.Unlock()
	code := link.Classify(input)
	l.room.local.SetAvailable(false)
	l.setState(func(state *mpAPI.LinkState) {
		*state = mpAPI.LinkState{State: mpConstant.LinkFailed, Error: code}
	})
	l.mu.Lock()
	detail := l.detail
	l.mu.Unlock()
	l.trace(mpConstant.TraceFailed, 0, false, time.Time{}, "", detail, "error="+strconv.Itoa(code))
}

// refreshPath 投影原生多连接变化。职责：优先直接路径；使用：通知和轮询；边界：不重置已有游戏 stream。
func (l *memberLink) refreshPath() {
	conn := p2p.BestConn(l.sess.host, l.hostID, l.sess.host.RelayFallback())
	if conn == nil {
		return
	}
	l.mu.Lock()
	path, family := link.PathOf(conn, l.endpoint.Net)
	changed := path != l.state.Path || family != l.state.IPFamily
	l.state.Path, l.state.IPFamily = path, family
	l.mu.Unlock()
	if changed {
		l.changed()
	}
}
