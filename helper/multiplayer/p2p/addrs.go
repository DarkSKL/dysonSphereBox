package p2p

// 文件职责：连接地址观测节点拿 observed address，汇总本机可达地址与网络诊断（UPnP/IPv6/AutoNAT），提供网卡变化签名。
// 使用：控制器在 session.open 后与网卡变化时调用 Observe + Collect，结果用于 peer.addrs 与前端 network 投影。
// 边界：观测结果只是地址候选，公网入站状态由官方 AutoNAT 判断；不访问控制链路。

import (
	"context"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// maxReportAddrs 职责：peer.addrs 地址条数上限；使用：Collect 截断；边界：协议规定 16 条。
const maxReportAddrs = 16

// Observation 是一个观测节点看到的本机地址。
// 职责：记录观测节点 PeerID 与 observed address；使用：地址上报与诊断展示；边界：失败的观测节点不出现。
type Observation struct {
	// Observer 职责：观测节点 PeerID；使用：按节点区分映射（两个节点可以同 IP 不同端口）；边界：无。
	Observer peer.ID
	// ConnID 职责：标识产生该 observed 地址的真实连接；使用：过滤被后续传输探测关闭的候选；边界：空值不发布到地址工厂。
	ConnID string
	// Addr 职责：observed address；使用：peer.addrs 与 CGNAT 判断；边界：不含 /p2p。
	Addr ma.Multiaddr
}

// ObserverResult 是一个观测节点的观测结果（成功或失败）。
// 职责：observe trace 与前端“拿不到 observed 地址的原因”；使用：控制器上报 link.trace；边界：每个观测节点按 TCP、QUIC 各一条；无可解析节点时返回配置错误。
type ObserverResult struct {
	// Recovery 职责：QUIC 失败后 TCP 原生连接恢复的独立结果；使用：额外 observe trace；边界：不覆盖两条主探测结果，不递归恢复。
	Recovery *ObserverResult
	// Transport 职责：本次独立探测的传输；使用：tcp/quic；边界：未配置入口时仍记录。
	Transport string
	// Diagnostic 职责：原始地址和逐地址错误；使用：服务端导出；边界：不含凭证。
	Diagnostic *mpAPI.TraceDiagnostic
	// Observer 职责：观测节点 PeerID；使用：trace detail；边界：字符串形式，无法确定节点时为空。
	Observer string
	// Targets 职责：本次传输的观测节点拨号候选；使用：trace detail；边界：可为空，不证明全部实际尝试。
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

// FirstObserverAddrs 仅保留首个观测 PeerID 的全部传输入口。
// 职责：保留同一 observer 的 TCP 与 QUIC 地址；使用：地址观测和 AutoNAT 来源解析；边界：不同 PeerID 的入口忽略。
func FirstObserverAddrs(observers []string) []string {
	var first peer.ID
	result := make([]string, 0, len(observers))
	for _, raw := range observers {
		addr, err := ma.NewMultiaddr(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		info, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			continue
		}
		if first == "" {
			first = info.ID
		}
		if info.ID == first {
			result = append(result, raw)
		}
	}
	return result
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
}

// Collect 汇总本机可达地址与诊断。
// 职责：接口地址（去回环与链路本地）、UPnP 映射地址、observed 地址合并去重；使用：控制器上报 peer.addrs；边界：顺序为 observed、UPnP、接口地址，keepLoopback 只用于进程内测试。
func Collect(h *Host, observations []Observation, firewallRule bool, keepLoopback bool, reachability string) Report {
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
		Addrs:     addrs,
		Net:       mpAPI.NetInfo{UPnP: len(upnp) > 0, IPv6: ipv6, AutoNATReachability: reachability, FirewallRule: firewallRule},
		Observed:  observed,
		UPnPAddrs: upnp,
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
