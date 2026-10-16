package p2p

// 文件职责：拨号诊断：按 IP 给地址归类（私网/公网/回环/CGNAT），并在一次 Connect 中收集目标地址、实际连上的地址与逐地址错误。
// 使用：NewHost 把 dialRecorder 作为 swarm MetricsTracer 挂到 host；link 的直连/打洞与观测节点拨号调用 Host.ConnectReport；控制器投影用 ClassifyAddrs。
// 边界：只做诊断，不改变拨号行为。swarm 在外层 ctx 超时时直接返回 ctx.Err()、丢弃 swarm.DialError，
// 所以逐地址错误主要由 FailedDialing 回调按地址收集，errors.As 解出的 DialErrors 只作补充；回调在拨号 goroutine 中调用，必须立即返回。

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// 地址归类取值。
// 职责：标注地址能否从外网访问；使用：trace/日志文本与前端投影 kind；边界：只按 IP 判断，不探测可达性。
const (
	// AddrPrivate 职责：私网（RFC 1918、ULA、链路本地）；使用：外网不可达。
	AddrPrivate = "private"
	// AddrPublic 职责：公网；使用：外网可能可达。
	AddrPublic = "public"
	// AddrLoopback 职责：回环；使用：只在本机可达。
	AddrLoopback = "loopback"
	// AddrCGNAT 职责：运营商级 NAT 共享地址 100.64.0.0/10；使用：外网不可达。
	AddrCGNAT = "cgnat"
	// AddrUnknown 职责：无法解析出 IP（例如 DNS 地址）；使用：原样展示。
	AddrUnknown = "unknown"
	// AddrRelay 职责：指定中继地址；使用：诊断；边界：不以服务器 IP 误判房主公网。
	AddrRelay = "relay"
)

// dialSettle 职责：拨号失败后等待逐地址回调的最长时间；使用：ConnectReport；边界：只在失败时等待。
const dialSettle = 300 * time.Millisecond

// cgnatRange 是运营商级 NAT 共享地址段。
var cgnatRange = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// ClassifyAddr 返回 multiaddr 字符串的地址归类。
func ClassifyAddr(raw string) string {
	addr, err := ma.NewMultiaddr(strings.TrimSpace(raw))
	if err != nil {
		return AddrUnknown
	}
	return classify(addr)
}

func classify(addr ma.Multiaddr) string {
	if IsCircuit(addr) {
		return AddrRelay
	}
	ip, err := manet.ToIP(addr)
	if err != nil {
		return AddrUnknown
	}
	switch {
	case ip.IsLoopback():
		return AddrLoopback
	case cgnatRange.Contains(ip):
		return AddrCGNAT
	case ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified():
		return AddrPrivate
	default:
		return AddrPublic
	}
}

// KindText 返回地址归类的中文标注。
func KindText(kind string) string {
	switch kind {
	case AddrRelay:
		return "中继"
	case AddrPrivate:
		return "私网"
	case AddrPublic:
		return "公网"
	case AddrLoopback:
		return "回环"
	case AddrCGNAT:
		return "CGNAT"
	default:
		return "未知"
	}
}

// shortAddr 返回紧凑写法，例如 `quic 203.0.113.7:40123`；无法解析出 IP 与端口时原样返回。
func shortAddr(addr ma.Multiaddr) string {
	if IsCircuit(addr) {
		return addr.String()
	}
	ip, err := manet.ToIP(addr)
	if err != nil {
		return addr.String()
	}
	transport, port := "", ""
	if value, err := addr.ValueForProtocol(ma.P_TCP); err == nil {
		transport, port = "tcp", value
	} else if value, err := addr.ValueForProtocol(ma.P_UDP); err == nil {
		transport, port = "udp", value
		if _, err := addr.ValueForProtocol(ma.P_QUIC_V1); err == nil {
			transport = "quic"
		}
	}
	if port == "" {
		return addr.String()
	}
	return transport + " " + net.JoinHostPort(ip.String(), port)
}

// describe 返回带归类的紧凑写法，例如 `quic 203.0.113.7:40123 公网`。
func describe(addr ma.Multiaddr) string {
	return shortAddr(addr) + " " + KindText(classify(addr))
}

// DescribeAddr 返回 multiaddr 字符串带归类的紧凑写法；无法解析时原样返回并标注未知。
func DescribeAddr(raw string) string {
	addr, err := ma.NewMultiaddr(strings.TrimSpace(raw))
	if err != nil {
		return raw + " " + KindText(AddrUnknown)
	}
	return describe(addr)
}

// ClassifiedAddr 是一条带归类的地址。
// 职责：前端网络诊断展示；使用：multiplayer:state network；边界：Addr 为原始 multiaddr。
type ClassifiedAddr struct {
	// Addr 职责：原始 multiaddr；使用：复制与排障；边界：只读。
	Addr string `json:"addr"`
	// Display 职责：紧凑写法（传输 IP:端口）；使用：展示；边界：无法解析时同 Addr。
	Display string `json:"display"`
	// Kind 职责：归类 private/public/loopback/cgnat/unknown；使用：前端标注与提示；边界：只按 IP 判断。
	Kind string `json:"kind"`
}

// ClassifyAddrs 给地址列表逐条归类。
func ClassifyAddrs(addrs []string) []ClassifiedAddr {
	result := make([]ClassifiedAddr, 0, len(addrs))
	for _, raw := range addrs {
		item := ClassifiedAddr{Addr: raw, Display: raw, Kind: AddrUnknown}
		if addr, err := ma.NewMultiaddr(strings.TrimSpace(raw)); err == nil {
			item.Display, item.Kind = shortAddr(addr), classify(addr)
		}
		result = append(result, item)
	}
	return result
}

// HasPublic 判断列表中是否有公网地址。
func HasPublic(addrs []ClassifiedAddr) bool {
	for _, item := range addrs {
		if item.Kind == AddrPublic {
			return true
		}
	}
	return false
}

// AddrError 是一个地址的拨号错误。
type AddrError struct {
	// Addr 职责：目标地址；使用：逐地址展示；边界：只读。
	Addr ma.Multiaddr
	// Err 职责：该地址的错误文本；使用：逐地址展示；边界：非空。
	Err string
}

// DialReport 是一次拨号的诊断结果。
// 职责：携带目标地址、实际连上的地址、顶层错误与逐地址错误；使用：trace、盒子日志与 links[].detail；边界：成功时也可保留已收集的其他候选错误，不声称记录全部实际尝试。
type DialReport struct {
	// Targets 职责：本次拨号的目标地址（下发地址 ∪ peerstore 已有地址）；使用：诊断；边界：已去重。
	Targets []ma.Multiaddr
	// Connected 职责：实际连上的远端地址；使用：成功时展示；边界：失败为 nil。
	Connected ma.Multiaddr
	// Err 职责：Connect 返回的错误；使用：失败分类与顶层错误；边界：成功为 nil。
	Err error
	// AddrErrors 职责：逐地址错误，按 Targets 顺序；使用：定位哪个地址为什么失败；边界：没有结果的目标不出现。
	AddrErrors []AddrError
	// Duration 职责：Connect 本身耗时（不含收集回调的等待）；使用：trace durationMs；边界：无。
	Duration time.Duration
}

// ConnectedAddr 返回实际连上的地址字符串；失败为空。
func (report DialReport) ConnectedAddr() string {
	if report.Connected == nil {
		return ""
	}
	return report.Connected.String()
}

// ConnectedText 返回实际连上的地址带归类写法；失败为空。
func (report DialReport) ConnectedText() string {
	if report.Connected == nil {
		return ""
	}
	return describe(report.Connected)
}

// TargetTexts 返回每个目标地址带归类的写法。
func (report DialReport) TargetTexts() []string {
	result := make([]string, 0, len(report.Targets))
	for _, addr := range report.Targets {
		result = append(result, describe(addr))
	}
	return result
}

// TopError 返回顶层错误：DialError 只取 Cause，避免与逐地址错误重复；成功为空。
func (report DialReport) TopError() string {
	if report.Err == nil {
		return ""
	}
	var dialErr *swarm.DialError
	if errors.As(report.Err, &dialErr) {
		if dialErr.Cause != nil {
			return "failed to dial: " + dialErr.Cause.Error()
		}
		return "failed to dial: " + swarm.ErrAllDialsFailed.Error()
	}
	return report.Err.Error()
}

// AddrLines 返回每个目标地址的结果一行：错误文本，或“无结果”（未拨号、被 swarm 跳过或未在超时前返回）；全部无结果时只有一行“全部 N 个地址在 Xs 内无响应”。
func (report DialReport) AddrLines() []string {
	if report.Err == nil {
		return nil
	}
	if report.NoResponse() {
		return []string{report.noResponseText()}
	}
	errs := map[string]string{}
	for _, item := range report.AddrErrors {
		errs[item.Addr.String()] = item.Err
	}
	lines := make([]string, 0, len(report.Targets))
	for _, addr := range report.Targets {
		text, ok := errs[addr.String()]
		switch {
		case !ok:
			text = "无结果（未拨号或未在超时前返回）"
		case strings.Contains(text, "deadline exceeded") || strings.Contains(strings.ToLower(text), "timeout"):
			// 超时说明对端没有任何响应：地址不可达、端口未映射或 UDP 被拦截；拒绝连接则会给出具体错误
			text = "超时无响应（" + text + "）"
		}
		lines = append(lines, describe(addr)+": "+text)
	}
	return lines
}

// KindSummary 返回地址归类计数，例如 `公网=1 私网=2`，按 公网/私网/CGNAT/回环/未知 顺序，只列非零项。
func KindSummary(addrs []ma.Multiaddr) string {
	counts := map[string]int{}
	for _, addr := range addrs {
		counts[classify(addr)]++
	}
	parts := []string{}
	for _, kind := range []string{AddrPublic, AddrPrivate, AddrCGNAT, AddrLoopback, AddrUnknown} {
		if counts[kind] > 0 {
			parts = append(parts, KindText(kind)+"="+strconv.Itoa(counts[kind]))
		}
	}
	return strings.Join(parts, " ")
}

// FirstTarget 返回第一个目标地址；没有目标时为空。
func (report DialReport) FirstTarget() string {
	if len(report.Targets) == 0 {
		return ""
	}
	return report.Targets[0].String()
}

// TraceAddr 返回 trace addr：成功为实际连上的地址，失败为第一个目标地址。
func (report DialReport) TraceAddr() string {
	if report.Connected != nil {
		return report.Connected.String()
	}
	return report.FirstTarget()
}

// TargetsDetail 返回 trace detail：归类计数 + 目标地址列表，按条截断到 max 字节。
func (report DialReport) TargetsDetail(max int) string {
	if len(report.Targets) == 0 {
		return "目标地址：无"
	}
	return JoinLimited("目标地址 "+KindSummary(report.Targets)+"：", report.TargetTexts(), ", ", max)
}

// NoResponse 职责：判断失败是否缺少逐地址证据；使用：诊断展示；边界：不证明全部候选已尝试或网络无响应。
func (report DialReport) NoResponse() bool {
	return report.Err != nil && len(report.Targets) > 0 && len(report.AddrErrors) == 0
}

// noResponseText 职责：描述逐地址证据缺失；使用：ErrorText/AddrLines；边界：不能把候选误称为实际拨号失败。
func (report DialReport) noResponseText() string {
	return "全部 " + strconv.Itoa(len(report.Targets)) + " 个候选缺少逐地址结果（未尝试或尚未完成，耗时 " +
		strconv.FormatFloat(report.Duration.Seconds(), 'f', 1, 64) + "s）"
}

// ErrorText 职责：组合顶层及逐地址错误；使用：trace error；边界：证据缺失明确注明，按条限制 max 字节，成功为空。
func (report DialReport) ErrorText(max int) string {
	if report.Err == nil {
		return ""
	}
	if report.NoResponse() {
		return JoinLimited(report.TopError()+" | "+report.noResponseText()+"：", report.TargetTexts(), ", ", max)
	}
	return JoinLimited(report.TopError()+" | ", report.AddrLines(), " | ", max)
}

// JoinLimited 把 prefix 与条目拼接，超出 max 字节时停在完整条目处并注明剩余条数。
// 职责：trace 字段截断不切开单条地址；使用：TargetsDetail/ErrorText；边界：单条本身超长时由 Tracer 再按字节截断。
func JoinLimited(prefix string, items []string, sep string, max int) string {
	var builder strings.Builder
	builder.WriteString(prefix)
	for index, item := range items {
		piece := item
		if index > 0 {
			piece = sep + item
		}
		rest := len(items) - index
		suffix := "…等 " + strconv.Itoa(rest) + " 条"
		// 不是最后一条时为省略说明留位置
		limit := max
		if rest > 1 {
			limit = max - len(suffix)
		}
		if index > 0 && builder.Len()+len(piece) > limit {
			builder.WriteString(suffix)
			return builder.String()
		}
		builder.WriteString(piece)
	}
	return builder.String()
}

// ConnectReport 连接对端并收集拨号诊断。
// 职责：记录目标地址，Connect 成功时取实际连接地址；失败时最多等待 dialSettle 收集逐地址回调，并合并 errors.As 解出的 DialErrors；
// 使用：直连、打洞与观测节点拨号；边界：拨号行为与 h.Connect 完全相同。
func (h *Host) ConnectReport(ctx context.Context, info peer.AddrInfo) DialReport {
	targets := ma.Unique(append(append([]ma.Multiaddr{}, info.Addrs...), h.Peerstore().Addrs(info.ID)...))
	watch := h.dials.watch(targets)
	started := time.Now()
	err := h.Connect(ctx, info)
	report := DialReport{Targets: targets, Err: err, Duration: time.Since(started)}
	if err == nil {
		errs := h.dials.unwatch(watch)
		for _, addr := range targets {
			if text, ok := errs[addr.String()]; ok {
				report.AddrErrors = append(report.AddrErrors, AddrError{Addr: addr, Err: text})
			}
		}
		limited, _ := network.GetAllowLimitedConn(ctx)
		if conn := BestConn(h, info.ID, limited); conn != nil {
			report.Connected = conn.RemoteMultiaddr()
		}
		return report
	}
	return h.finishDialFailure(watch, report)
}

// finishDialFailure 职责：合并 swarm 逐地址回调和类型化错误；使用：普通拨号及观测拨号；边界：只收集实际错误，不将未拨号候选描述为失败。
func (h *Host) finishDialFailure(watch *dialWatch, report DialReport) DialReport {
	timer := time.NewTimer(dialSettle)
	select {
	case <-watch.done:
	case <-timer.C:
	}
	timer.Stop()
	errs := h.dials.unwatch(watch)
	var dialErr *swarm.DialError
	if errors.As(report.Err, &dialErr) {
		for _, item := range dialErr.DialErrors {
			key := item.Address.String()
			if _, ok := errs[key]; !ok && item.Cause != nil {
				errs[key] = item.Cause.Error()
			}
			if !containsAddr(report.Targets, item.Address) {
				report.Targets = append(report.Targets, item.Address)
			}
		}
	}
	for _, addr := range report.Targets {
		if text, ok := errs[addr.String()]; ok {
			report.AddrErrors = append(report.AddrErrors, AddrError{Addr: addr, Err: text})
		}
	}
	return report
}

func containsAddr(list []ma.Multiaddr, addr ma.Multiaddr) bool {
	for _, item := range list {
		if item.Equal(addr) {
			return true
		}
	}
	return false
}

// dialWatch 是一次 ConnectReport 关注的目标地址。
type dialWatch struct {
	targets   map[string]bool
	errs      map[string]string
	remaining int
	done      chan struct{}
}

// dialRecorder 是只关心 FailedDialing 的 swarm MetricsTracer。
// 职责：把逐地址拨号错误分发给正在进行的 ConnectReport；使用：NewHost 经 swarm.WithMetricsTracer 挂载；边界：其余回调为空操作，不做指标统计。
type dialRecorder struct {
	mu      sync.Mutex
	watches map[*dialWatch]struct{}
}

func newDialRecorder() *dialRecorder { return &dialRecorder{watches: map[*dialWatch]struct{}{}} }

func (recorder *dialRecorder) watch(targets []ma.Multiaddr) *dialWatch {
	watch := &dialWatch{targets: map[string]bool{}, errs: map[string]string{}, done: make(chan struct{})}
	for _, addr := range targets {
		watch.targets[addr.String()] = true
	}
	watch.remaining = len(watch.targets)
	if watch.remaining == 0 {
		close(watch.done)
	}
	if recorder == nil {
		return watch
	}
	recorder.mu.Lock()
	recorder.watches[watch] = struct{}{}
	recorder.mu.Unlock()
	return watch
}

// unwatch 结束关注并返回已收集的逐地址错误副本。
func (recorder *dialRecorder) unwatch(watch *dialWatch) map[string]string {
	result := map[string]string{}
	if recorder == nil {
		return result
	}
	recorder.mu.Lock()
	delete(recorder.watches, watch)
	for key, value := range watch.errs {
		result[key] = value
	}
	recorder.mu.Unlock()
	return result
}

// FailedDialing 记录一个地址的拨号错误（swarm 对每个真正发起的地址拨号失败或被取消时调用）。
func (recorder *dialRecorder) FailedDialing(addr ma.Multiaddr, dialErr error, cause error) {
	if addr == nil || dialErr == nil {
		return
	}
	text := dialErr.Error()
	if cause != nil && !errors.Is(dialErr, cause) && !strings.Contains(text, cause.Error()) {
		text += "（" + cause.Error() + "）"
	}
	key := addr.String()
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	for watch := range recorder.watches {
		if !watch.targets[key] {
			continue
		}
		if _, seen := watch.errs[key]; seen {
			continue
		}
		watch.errs[key] = text
		if watch.remaining--; watch.remaining == 0 {
			close(watch.done)
		}
	}
}

func (*dialRecorder) OpenedConnection(network.Direction, crypto.PubKey, network.ConnectionState, ma.Multiaddr) {
}
func (*dialRecorder) ClosedConnection(network.Direction, time.Duration, network.ConnectionState, ma.Multiaddr) {
}
func (*dialRecorder) CompletedHandshake(time.Duration, network.ConnectionState, ma.Multiaddr) {}
func (*dialRecorder) DialCompleted(bool, int, time.Duration)                                  {}
func (*dialRecorder) DialRankingDelay(time.Duration)                                          {}
func (*dialRecorder) UpdatedBlackHoleSuccessCounter(string, swarm.BlackHoleState, int, float64) {
}

var _ swarm.MetricsTracer = (*dialRecorder)(nil)
