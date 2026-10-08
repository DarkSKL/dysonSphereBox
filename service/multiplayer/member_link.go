package multiplayer

// 文件职责：成员到房主的数据链路状态机（方案第 7.3/7.4 节）：connecting→connected→reconnecting→connected|failed，直连→打洞，3 分钟放弃，link.report 与 RTT 节流。
// 使用：控制器在加入提交后 newMemberLink 并 go run；事件处理通过 deliverPunch/updateEndpoint/wakeUp 非阻塞投递；房主收到 link.punch after 时调用 hostPunch 回拨。
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
	// natBlocked 职责：本次尝试因房主对称 NAT 跳过打洞（host/both）；使用：failed 直接给 20402、投影区分本机/房主；边界：每次尝试开始时清空。
	natBlocked string

	reportMu   sync.Mutex
	lastReport *mpAPI.LinkState
	lastRTTAt  time.Time

	wake         chan struct{}
	disconnected chan struct{}
	punch        chan mpAPI.LinkPunchEvent
	ctx          context.Context
	cancel       context.CancelFunc
	closeOnce    sync.Once
}

func newMemberLink(c *controller, sess *session, room *roomState, hostID peer.ID, endpoint mpAPI.HostEndpoint) *memberLink {
	ctx, cancel := context.WithCancel(context.Background())
	return &memberLink{c: c, sess: sess, room: room, roomID: room.view.Snapshot.RoomID, hostID: hostID,
		hostUserID: room.view.Snapshot.Host.UserID, endpoint: endpoint,
		state: mpAPI.LinkState{State: mpConstant.LinkConnecting},
		wake:  make(chan struct{}, 1), disconnected: make(chan struct{}, 1), punch: make(chan mpAPI.LinkPunchEvent, 8),
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

// deliverPunch 投递 link.punch after。
func (l *memberLink) deliverPunch(event mpAPI.LinkPunchEvent) {
	select {
	case l.punch <- event:
	default:
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
	notifiee := &network.NotifyBundle{DisconnectedF: func(_ network.Network, conn network.Conn) {
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
		punched, ok := l.establish()
		if l.ctx.Err() != nil {
			return
		}
		if ok {
			l.onConnected(punched)
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

// establish 依次执行直连与打洞；返回是否靠打洞连通与是否连通。
func (l *memberLink) establish() (bool, bool) {
	l.mu.Lock()
	endpoint := l.endpoint
	l.attemptFresh = true
	l.natBlocked = ""
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

	// 打洞前置判断（方案第 7.3 节第 3 步）：房主为对称 NAT 时打洞不可能成功，不发 link.punch
	if blocked := l.natPrecheck(endpoint); blocked != "" {
		if l.ctx.Err() != nil {
			return false, false
		}
		l.mu.Lock()
		l.natBlocked = blocked
		l.mu.Unlock()
		reason := "房主网络为对称 NAT（房主上报 natConsistent = false）"
		if blocked == mpConstant.NATBlockedBoth {
			reason = "房主与本机网络都是对称 NAT"
		}
		l.recordFailure("punch", 0, info.ID.String(), "跳过打洞："+reason+"，打洞不可能成功")
		l.trace(mpConstant.TracePunchRequest, 0, false, time.Time{}, "", "", "skipped: "+blocked+"_symmetric")
		return false, false
	}

	rounds := l.sess.info.Limits.PunchRounds
	if rounds <= 0 {
		rounds = mpConstant.DefaultPunchRounds
	}
	for round := 1; round <= rounds && l.ctx.Err() == nil; round++ {
		if round > 1 {
			l.waitRetry(mpConstant.PunchRoundInterval)
		}
		for drained := false; !drained; {
			select {
			case <-l.punch:
			default:
				drained = true
			}
		}
		requestStarted := time.Now()
		reply, err := l.sess.conn.Request(l.ctx, mpConstant.OpLinkPunch,
			mpAPI.LinkPunchRequest{RoomID: l.roomID, Round: round})
		if err != nil {
			if l.ctx.Err() == nil {
				l.recordFailure("punch", round, info.ID.String(), "打洞协调请求失败："+err.Error())
				l.trace(mpConstant.TracePunchRequest, round, false, requestStarted, "", err.Error(), "")
			}
			return false, false
		}
		l.trace(mpConstant.TracePunchRequest, round, reply.Code == 0, requestStarted, "", reply.Msg, "code="+strconv.Itoa(reply.Code))
		if reply.Code != 0 {
			l.recordFailure("punch", round, info.ID.String(), fmt.Sprintf("打洞协调被服务器拒绝（%d %s）", reply.Code, reply.Msg))
			// 20242 本轮中止可继续下一轮；限频或房主未上报地址时停止本次尝试
			if reply.Code == 20242 {
				continue
			}
			return false, false
		}
		var event mpAPI.LinkPunchEvent
		select {
		case event = <-l.punch:
		case <-time.After(mpConstant.PunchRoundTimeout):
			l.recordFailure("punch", round, info.ID.String(), "未收到打洞时刻（link.punch after）")
			continue
		case <-l.ctx.Done():
			return false, false
		}
		if event.Result != 0 || event.AtMs == 0 {
			l.recordFailure("punch", round, info.ID.String(), fmt.Sprintf("打洞协调中止（%d）", event.Result))
			continue
		}
		if event.Host != nil {
			if updated, err := link.AddrInfo(event.Host.PeerID, event.Host.Addrs); err == nil {
				info = updated
			}
		}
		at := l.sess.conn.ServerToLocal(event.AtMs)
		report := link.DialPunch(l.ctx, l.sess.host, info, true, at, mpConstant.PunchRoundTimeout)
		if report.Err == nil {
			l.traceDial(mpConstant.TracePunchDial, round, report)
			l.recordDialReport("punch", round, info.ID.String(), report)
			return true, true
		} else if l.ctx.Err() == nil {
			l.setDialErr(report.Err)
			l.recordDialReport("punch", round, info.ID.String(), report)
			l.traceDial(mpConstant.TracePunchDial, round, report)
		}
	}
	return false, false
}

// natPrecheck 返回打洞前置判断结果：只看房主 HostEndpoint.net.natConsistent。
// 房主为 false 时跳过打洞：成员也为 false 返回 both，否则返回 host；房主不是 false（含 null）返回空，照常打洞。
// 边界：只有成员为 false 时不跳过，打洞失败后由 Classify 归类为 20402。
func (l *memberLink) natPrecheck(endpoint mpAPI.HostEndpoint) string {
	if endpoint.Net == nil || endpoint.Net.NATConsistent == nil || *endpoint.Net.NATConsistent {
		return ""
	}
	l.c.mu.Lock()
	self := l.sess.report.Net.NATConsistent
	l.c.mu.Unlock()
	if self != nil && !*self {
		return mpConstant.NATBlockedBoth
	}
	return mpConstant.NATBlockedHost
}

// describeAddrs 返回 multiaddr 字符串带私网/公网归类的写法。
func describeAddrs(addrs []string) []string {
	result := make([]string, 0, len(addrs))
	for _, raw := range addrs {
		result = append(result, p2p.DescribeAddr(raw))
	}
	return result
}

// traceDial 上报 direct_dial / punch_dial：addr 为实际连上的地址，detail 为目标地址列表，error 为顶层错误 + 逐地址错误。
func (l *memberLink) traceDial(stage string, round int, report p2p.DialReport) {
	l.sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: l.roomID, PeerUserID: l.hostUserID, Stage: stage, Round: round,
		OK: report.Err == nil, DurationMs: report.Duration.Milliseconds(), Addr: report.TraceAddr(),
		Error: report.ErrorText(mpConstant.TraceErrorMax), Detail: report.TargetsDetail(mpConstant.TraceDetailMax)})
}

// recordDialReport 用拨号诊断结果记录一次直连/打洞。
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

// maxDetailLines 职责：links[].detail 保留的记录条数；使用：recordDial；边界：只保留本轮尝试（直连 + 各轮打洞）的最近几条，每条可含多行地址。
const maxDetailLines = 4

// recordFailure 记录一次没有真正拨号的失败（协调失败、未收到打洞时刻等）。
func (l *memberLink) recordFailure(path string, round int, peerID string, text string) {
	l.recordDial(link.DialRecord{Path: path, Round: round, PeerID: peerID, Err: text})
}

// recordDial 记录一次直连/打洞结果：写盒子日志；失败时把文本（含目标地址与逐地址错误）追加到本地投影 links[].detail。
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
	if l.state.State == mpConstant.LinkFailed && l.state.Error == mpConstant.LinkErrNATInconsistent {
		view.NATBlocked = l.natBlocked
	}
	return view
}

// onConnected 连通后设置路径并开放本地端口。
func (l *memberLink) onConnected(punched bool) {
	conns := l.sess.host.Network().ConnsToPeer(l.hostID)
	l.mu.Lock()
	hostNet := l.endpoint.Net
	l.everConnected = true
	l.mu.Unlock()
	path, family := mpConstant.PathDirect, ""
	if len(conns) > 0 {
		path, family = link.PathOf(conns[0], punched, hostNet)
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
			if len(l.sess.host.Network().ConnsToPeer(l.hostID)) == 0 {
				return
			}
		case <-l.wake:
		case <-ticker.C:
			if len(l.sess.host.Network().ConnsToPeer(l.hostID)) == 0 {
				return
			}
			l.measureRTT()
		}
	}
}

// measureRTT 用 libp2p ping 测 RTT，最多每 10 秒上报一次。
func (l *memberLink) measureRTT() {
	ctx, cancel := context.WithTimeout(network.WithNoDial(l.ctx, "rtt"), 3*time.Second)
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
	natBlocked := l.natBlocked
	l.mu.Unlock()
	// 本地已判定对称 NAT（跳过了打洞）时直接 20402，不经 Classify
	code := mpConstant.LinkErrNATInconsistent
	if natBlocked == "" {
		code = link.Classify(input)
	}
	l.room.local.SetAvailable(false)
	l.setState(func(state *mpAPI.LinkState) {
		*state = mpAPI.LinkState{State: mpConstant.LinkFailed, Error: code}
	})
	l.mu.Lock()
	detail := l.detail
	l.mu.Unlock()
	l.trace(mpConstant.TraceFailed, 0, false, time.Time{}, "", detail, "error="+strconv.Itoa(code))
}

// hostPunch 房主在 atMs 对应的本地时刻回拨成员，并上报 punch_dial、写盒子日志（成员地址列表、实际地址或逐地址错误）。
func (c *controller) hostPunch(sess *session, roomID string, memberUserID int64, round int, member mpAPI.PunchPeer, at time.Time) {
	record := link.DialRecord{Role: link.DialRoleHost, Path: "punch", Round: round, PeerID: member.PeerID,
		PeerUserID: memberUserID, RoomID: roomID}
	info, err := link.AddrInfo(member.PeerID, member.Addrs)
	if err != nil {
		record.Err, record.Targets = "成员地址无法解析："+err.Error(), describeAddrs(member.Addrs)
		go c.opts.logDial(record)
		sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, PeerUserID: memberUserID, Stage: mpConstant.TracePunchDial,
			Round: round, Error: record.Err, Detail: p2p.JoinLimited("目标地址：", record.Targets, ", ", mpConstant.TraceDetailMax)})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-sess.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	report := link.DialPunch(ctx, sess.host, info, false, at, mpConstant.PunchRoundTimeout)
	if ctx.Err() != nil && report.Err != nil {
		// 会话已结束，不再上报
		return
	}
	record.ApplyReport(report)
	go c.opts.logDial(record)
	sess.tracer.Trace(mpAPI.LinkTraceRequest{RoomID: roomID, PeerUserID: memberUserID, Stage: mpConstant.TracePunchDial, Round: round,
		OK: report.Err == nil, DurationMs: report.Duration.Milliseconds(), Addr: report.TraceAddr(),
		Error: report.ErrorText(mpConstant.TraceErrorMax), Detail: report.TargetsDetail(mpConstant.TraceDetailMax)})
}
