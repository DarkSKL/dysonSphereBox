package p2p

// 文件职责：连接地址观测节点拿 observed address，汇总本机可达地址与网络诊断（upnp/ipv6/natConsistent），提供网卡变化签名。
// 使用：控制器在 session.open 后与网卡变化时调用 Observe + Collect，结果用于 peer.addrs 与前端 network 投影。
// 边界：观测连接被服务器关闭不视为错误；NAT 判断只是诊断提示；不访问控制链路。

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"

	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// maxReportAddrs 职责：peer.addrs 地址条数上限；使用：Collect 截断；边界：协议规定 16 条。
const maxReportAddrs = 16

// Observation 是一个观测节点看到的本机地址。
// 职责：记录观测节点 PeerID 与 observed address；使用：natConsistent 计算与诊断展示；边界：失败的观测节点不出现。
type Observation struct {
	// Observer 职责：观测节点 PeerID；使用：按节点区分映射（两个节点可以同 IP 不同端口）；边界：无。
	Observer peer.ID
	// Addr 职责：observed address；使用：peer.addrs 与 CGNAT 判断；边界：不含 /p2p。
	Addr ma.Multiaddr
}

// ObserverResult 是一个观测节点的观测结果（成功或失败）。
// 职责：observe trace 与前端“拿不到 observed 地址的原因”；使用：控制器上报 link.trace；边界：每个观测节点一条；未下发或地址无法解析时为一条 Observer 为空的失败。
type ObserverResult struct {
	// Observer 职责：观测节点 PeerID；使用：trace detail；边界：字符串形式，无法确定节点时为空。
	Observer string
	// Targets 职责：观测节点的拨号地址（带归类）；使用：trace detail；边界：可为空。
	Targets []string
	// OK 职责：是否拿到 observed address；使用：trace ok；边界：无。
	OK bool
	// Addr 职责：observed address；使用：trace addr；边界：失败为空。
	Addr string
	// Duration 职责：从拨号到 identify 完成或失败的耗时；使用：trace durationMs；边界：无。
	Duration time.Duration
	// Err 职责：失败原因；使用：trace error；边界：成功为空。
	Err string
}

// Observe 连接全部观测节点并等待 identify 给出 observed address。
// 职责：按 /p2p 分组观测地址、每个节点最多等待 timeout；使用：测试；边界：单个节点失败只跳过。
func Observe(ctx context.Context, h *Host, observers []string, timeout time.Duration) []Observation {
	observations, _ := ObserveDetailed(ctx, h, observers, timeout)
	return observations
}

// ObserveDetailed 与 Observe 相同，另外返回每个观测节点的结果（含失败）。
// 职责：observed address 与 observe trace；使用：控制器；边界：结果顺序按完成先后。
// 拿不到 observed 地址时 Err 写原因：未下发观测节点、观测地址无法解析、拨号失败（逐地址）、identify 失败、identify 完成但没有 observed 地址、超时。
func ObserveDetailed(ctx context.Context, h *Host, observers []string, timeout time.Duration) ([]Observation, []ObserverResult) {
	if len(observers) == 0 {
		return nil, []ObserverResult{{Err: "服务器未下发观测节点（session.open observers 为空）"}}
	}
	infos, invalid := groupObservers(observers)
	details := []ObserverResult{}
	if len(invalid) > 0 {
		details = append(details, ObserverResult{Err: "观测地址无法解析（需为带 /p2p/<peerId> 的 multiaddr）：" + strings.Join(invalid, ", ")})
	}
	if len(infos) == 0 {
		return nil, details
	}
	sub, err := h.EventBus().Subscribe([]interface{}{new(event.EvtPeerIdentificationCompleted), new(event.EvtPeerIdentificationFailed)})
	if err != nil {
		return nil, append(details, ObserverResult{Err: "订阅 identify 事件失败：" + err.Error()})
	}
	defer sub.Close()

	type dialResult struct {
		id     peer.ID
		report DialReport
	}
	started := time.Now()
	dialed := make(chan dialResult, len(infos))
	waiting := map[peer.ID]bool{}
	targets := map[peer.ID][]string{}
	for _, info := range infos {
		waiting[info.ID] = true
		for _, addr := range info.Addrs {
			targets[info.ID] = append(targets[info.ID], describe(addr))
		}
		go func(info peer.AddrInfo) {
			dialCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			dialed <- dialResult{id: info.ID, report: h.ConnectReport(dialCtx, info)}
		}(info)
	}
	result := func(id peer.ID, ok bool, addr string, reason string) ObserverResult {
		return ObserverResult{Observer: id.String(), Targets: targets[id], OK: ok, Addr: addr, Duration: time.Since(started), Err: reason}
	}
	results := []Observation{}
	// 观测结果按 observers 下发顺序排列，诊断区的映射顺序稳定
	order := map[peer.ID]int{}
	for index, info := range infos {
		order[info.ID] = index
	}
	sorted := func() []Observation {
		sort.SliceStable(results, func(i, j int) bool { return order[results[i].Observer] < order[results[j].Observer] })
		return results
	}
	finish := func(reason string) ([]Observation, []ObserverResult) {
		for id := range waiting {
			details = append(details, result(id, false, "", reason))
		}
		return sorted(), details
	}
	deadline := time.NewTimer(timeout + time.Second)
	defer deadline.Stop()
	for len(waiting) > 0 {
		select {
		case raw := <-sub.Out():
			switch evt := raw.(type) {
			case event.EvtPeerIdentificationCompleted:
				if !waiting[evt.Peer] {
					continue
				}
				delete(waiting, evt.Peer)
				if evt.ObservedAddr == nil {
					details = append(details, result(evt.Peer, false, "", "identify 已完成，但观测节点没有返回 observed 地址"))
					continue
				}
				results = append(results, Observation{Observer: evt.Peer, Addr: evt.ObservedAddr})
				details = append(details, result(evt.Peer, true, evt.ObservedAddr.String(), ""))
			case event.EvtPeerIdentificationFailed:
				if !waiting[evt.Peer] {
					continue
				}
				delete(waiting, evt.Peer)
				details = append(details, result(evt.Peer, false, "", "identify 失败："+ErrorText(evt.Reason)))
			}
		case item := <-dialed:
			// 拨号失败时 identify 不会完成；拨号成功则继续等 identify 事件
			if item.report.Err != nil && waiting[item.id] {
				delete(waiting, item.id)
				details = append(details, result(item.id, false, "", "连接观测节点失败："+item.report.ErrorText(512)))
			}
		case <-deadline.C:
			return finish("连接成功但等待 identify 超时")
		case <-ctx.Done():
			return finish(ctx.Err().Error())
		}
	}
	return sorted(), details
}

// ErrorText 返回错误文本，nil 为空字符串。
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// groupObservers 把观测地址按 /p2p/<peerId> 合并成 AddrInfo；无法解析的地址原样返回。
func groupObservers(observers []string) ([]peer.AddrInfo, []string) {
	byID := map[peer.ID]*peer.AddrInfo{}
	order := []peer.ID{}
	invalid := []string{}
	for _, raw := range observers {
		addr, err := ma.NewMultiaddr(strings.TrimSpace(raw))
		if err != nil {
			invalid = append(invalid, raw)
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			invalid = append(invalid, raw)
			continue
		}
		if existing := byID[info.ID]; existing != nil {
			existing.Addrs = append(existing.Addrs, info.Addrs...)
			continue
		}
		byID[info.ID] = info
		order = append(order, info.ID)
	}
	result := make([]peer.AddrInfo, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, invalid
}

// quicMapping 返回 QUIC（UDP）observed 地址的 `ip:port`；TCP 等其他地址返回空字符串。
func quicMapping(addr ma.Multiaddr) (mapping string, v4 bool) {
	if addr == nil {
		return "", false
	}
	port, err := addr.ValueForProtocol(ma.P_UDP)
	if err != nil || port == "" {
		return "", false
	}
	ip, err := manet.ToIP(addr)
	if err != nil {
		return "", false
	}
	return net.JoinHostPort(ip.String(), port), ip.To4() != nil
}

// NATMappings 返回每个观测节点看到的 QUIC 映射 `ip:port`。
// 职责：网络诊断区展示两个观测映射；使用：Collect；边界：每个节点只取第一条 QUIC 地址，顺序同 observations，没有 QUIC 地址的节点不出现。
func NATMappings(observations []Observation) []string {
	seen := map[peer.ID]bool{}
	result := []string{}
	for _, item := range observations {
		mapping, _ := quicMapping(item.Addr)
		if mapping == "" || seen[item.Observer] {
			continue
		}
		seen[item.Observer] = true
		result = append(result, mapping)
	}
	return result
}

// NATConsistent 比较不同观测节点（按 PeerID 区分）看到的 QUIC 映射 `ip:port`。
// 职责：计算 natConsistent；使用：peer.addrs net 与打洞前置判断；
// 边界：只看 QUIC（UDP）observed 地址，TCP 不参与（打洞走 QUIC）；同一 IP 族内比较；任一 IP 族都不足两个节点时返回 nil。
func NATConsistent(observations []Observation) *bool {
	type mapping struct {
		v4    bool
		value string
	}
	byObserver := map[peer.ID]mapping{}
	for _, item := range observations {
		value, v4 := quicMapping(item.Addr)
		if value == "" {
			continue
		}
		if _, ok := byObserver[item.Observer]; !ok {
			byObserver[item.Observer] = mapping{v4: v4, value: value}
		}
	}
	// 按 IP 族分组：IPv4 与 IPv6 映射本来就不同，不能互相比较
	families := map[bool][]string{}
	for _, item := range byObserver {
		families[item.v4] = append(families[item.v4], item.value)
	}
	compared, consistent := false, true
	for _, values := range families {
		if len(values) < 2 {
			continue
		}
		compared = true
		for _, value := range values[1:] {
			if value != values[0] {
				consistent = false
			}
		}
	}
	if !compared {
		return nil
	}
	return &consistent
}

// Report 是一次地址汇总结果。
// 职责：携带 peer.addrs 内容与诊断细节；使用：控制器上报与前端投影；边界：Addrs 已截断到 16 条。
type Report struct {
	// Addrs 职责：可达 multiaddr；使用：peer.addrs；边界：最多 16 条。
	Addrs []string
	// Net 职责：网络诊断；使用：peer.addrs net；边界：firewallRule 由调用方填写。
	Net mpAPI.NetInfo
	// Observed 职责：observed 地址；使用：前端 observedAddrs 与 CGNAT 判断；边界：可为空。
	Observed []string
	// UPnPAddrs 职责：UPnP 映射出的公网地址；使用：CGNAT 判断；边界：可为空。
	UPnPAddrs []string
	// NATMappings 职责：各观测节点看到的 QUIC 映射 `ip:port`；使用：前端 natMappings；边界：顺序同观测节点下发顺序，可为空。
	NATMappings []string
}

// Collect 汇总本机可达地址与诊断。
// 职责：接口地址（去回环与链路本地）、UPnP 映射地址、observed 地址合并去重；使用：控制器上报 peer.addrs；边界：顺序为 observed、UPnP、接口地址，keepLoopback 只用于进程内测试。
func Collect(h *Host, observations []Observation, firewallRule bool, keepLoopback bool) Report {
	listen := h.Network().ListenAddresses()
	ifaceAddrs, _ := manet.InterfaceMultiaddrs()
	resolved, _ := manet.ResolveUnspecifiedAddresses(listen, ifaceAddrs)

	interfaceSet := map[string]bool{}
	local := []string{}
	ipv6 := false
	for _, addr := range resolved {
		if IsCircuit(addr) {
			continue
		}
		if (manet.IsIPLoopback(addr) && !keepLoopback) || manet.IsIP6LinkLocal(addr) {
			continue
		}
		if ip, err := manet.ToIP(addr); err == nil && ip.To4() == nil && manet.IsPublicAddr(addr) {
			ipv6 = true
		}
		interfaceSet[addr.String()] = true
		local = append(local, addr.String())
	}

	// host.Addrs() 包含 NAT 管理器的映射地址；不在接口地址中的公网 IPv4 视为 UPnP/NAT-PMP 映射
	upnp := []string{}
	for _, addr := range h.Addrs() {
		if IsCircuit(addr) {
			continue
		}
		text := addr.String()
		if interfaceSet[text] || !manet.IsPublicAddr(addr) {
			continue
		}
		if ip, err := manet.ToIP(addr); err == nil && ip.To4() != nil && sameListenPort(addr, h.Port) {
			upnp = append(upnp, text)
		}
	}
	observed := []string{}
	for _, item := range observations {
		observed = append(observed, item.Addr.String())
	}

	seen := map[string]bool{}
	addrs := []string{}
	for _, group := range [][]string{h.CircuitAddrs(), observed, upnp, local} {
		for _, text := range group {
			if seen[text] || len(text) > 256 || len(addrs) >= maxReportAddrs {
				continue
			}
			seen[text] = true
			addrs = append(addrs, text)
		}
	}
	return Report{
		Addrs:       addrs,
		Net:         mpAPI.NetInfo{UPnP: len(upnp) > 0, IPv6: ipv6, NATConsistent: NATConsistent(observations), FirewallRule: firewallRule},
		Observed:    observed,
		UPnPAddrs:   upnp,
		NATMappings: NATMappings(observations),
	}
}

// sameListenPort 判断映射地址是否来自本 host 的监听端口。
// 职责：避免把观测地址误判为 UPnP；使用：Collect；边界：port 为 0（测试随机端口）时不限制。
func sameListenPort(addr ma.Multiaddr, port int) bool {
	if port == 0 {
		return true
	}
	for _, code := range []int{ma.P_UDP, ma.P_TCP} {
		if value, err := addr.ValueForProtocol(code); err == nil {
			return value != "" && value == strconv.Itoa(port)
		}
	}
	return false
}

// InterfaceSignature 返回本机网卡地址签名。
// 职责：检测换网与 DHCP 换 IP；使用：控制器 10 秒一次比较，变化时重算地址并清除目标地址缓存；边界：只看非回环地址。
func InterfaceSignature() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	items := []string{}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			items = append(items, ipNet.IP.String())
		}
	}
	sort.Strings(items)
	return strings.Join(items, ",")
}
