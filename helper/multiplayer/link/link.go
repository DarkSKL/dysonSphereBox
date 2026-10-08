package link

// 文件职责：成员到房主的数据链路拨号（直连、按时刻同时拨号）、路径与 ipFamily 判定、失败分类（方案第 7.3–7.5 节）。
// 使用：service/multiplayer 成员链路状态机调用 DialDirect/DialPunch，连通后 PathOf，全部失败后 Classify；房主收到 link.punch after 时调用 DialPunch 回拨。
// 边界：只做单次拨号与判定，不维护状态机、不发控制链路请求；NAT 分类只是诊断提示。

import (
	"context"
	"net"
	"strings"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// AddrInfo 把 PeerID 与 multiaddr 字符串转换为 AddrInfo。
// 职责：解析控制链路下发的地址；使用：直连与打洞；边界：无法解析的地址跳过，PeerID 无效返回错误。
func AddrInfo(peerID string, addrs []string) (peer.AddrInfo, error) {
	id, err := peer.Decode(peerID)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	info := peer.AddrInfo{ID: id}
	for _, raw := range addrs {
		if addr, err := ma.NewMultiaddr(raw); err == nil {
			info.Addrs = append(info.Addrs, addr)
		}
	}
	return info, nil
}

// clearBackoff 清除 swarm 对该 peer 的拨号退避，保证每次重试都真正拨号。
func clearBackoff(h host.Host, id peer.ID) {
	if sw, ok := h.Network().(*swarm.Swarm); ok {
		sw.Backoff().Clear(id)
	}
}

// DialDirect 并发尝试对端全部地址。
// 职责：直连；使用：成员 connecting/reconnecting 第一步；边界：timeout 内未连上时 report.Err 非空，report 携带目标地址与逐地址错误。
func DialDirect(ctx context.Context, h *p2p.Host, info peer.AddrInfo, timeout time.Duration) p2p.DialReport {
	clearBackoff(h, info.ID)
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return h.ConnectReport(dialCtx, info)
}

// DialPunch 在本地时刻 at 用 WithSimultaneousConnect 同时拨号。
// 职责：打洞一轮；使用：成员 isClient=true，房主 isClient=false；边界：WithSimultaneousConnect 脱离 DCUtR 的行为属于 P0 待实测；等待 at 期间 ctx 结束时 report 只有错误。
func DialPunch(ctx context.Context, h *p2p.Host, info peer.AddrInfo, isClient bool, at time.Time, timeout time.Duration) p2p.DialReport {
	if wait := time.Until(at); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return p2p.DialReport{Targets: info.Addrs, Err: ctx.Err()}
		}
	}
	clearBackoff(h, info.ID)
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dialCtx = network.WithSimultaneousConnect(dialCtx, isClient, "punch")
	dialCtx = network.WithForceDirectDial(dialCtx, "punch")
	return h.ConnectReport(dialCtx, info)
}

// PathOf 判定已连通链路的路径与地址族。
// 职责：打洞成功为 punch；远端私网/链路本地/ULA 为 lan；公网 IPv4 且房主 upnp 为 upnp；其余 direct；使用：link.report；边界：远端地址无法解析时 ipFamily 为空。
func PathOf(conn network.Conn, punched bool, hostNet *mpAPI.NetInfo) (path string, family string) {
	ip, err := manet.ToIP(conn.RemoteMultiaddr())
	if err != nil {
		if punched {
			return mpConstant.PathPunch, ""
		}
		return mpConstant.PathDirect, ""
	}
	family = "v6"
	if ip.To4() != nil {
		family = "v4"
	}
	switch {
	case punched:
		return mpConstant.PathPunch, family
	case ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLoopback():
		return mpConstant.PathLAN, family
	case family == "v4" && hostNet != nil && hostNet.UPnP:
		return mpConstant.PathUPnP, family
	default:
		return mpConstant.PathDirect, family
	}
}

// ClassifyInput 是失败分类所需信息。
// 职责：汇总拨号结果与双方网络诊断；使用：Classify；边界：HostNet 未下发时为 nil。
type ClassifyInput struct {
	// DialErr 职责：最后一次拨号错误；使用：20406 判定；边界：可为 nil。
	DialErr error
	// TargetRefused 职责：房主是否回过 TARGET_REFUSED；使用：20407；边界：链路仍 connected。
	TargetRefused bool
	// SelfAddrs 职责：成员最近一次上报的地址；使用：地址族交集；边界：multiaddr 字符串。
	SelfAddrs []string
	// SelfNet 职责：成员网络诊断；使用：20402；边界：只读。
	SelfNet mpAPI.NetInfo
	// SelfObserved 职责：成员 observed 地址；使用：20403；边界：可为空。
	SelfObserved []string
	// SelfUPnP 职责：成员 UPnP 映射地址；使用：20403；边界：可为空。
	SelfUPnP []string
	// HostAddrs 职责：房主地址；使用：地址族交集与 CGNAT；边界：multiaddr 字符串。
	HostAddrs []string
	// HostNet 职责：房主网络诊断（HostEndpoint.net）；使用：20405/20402；边界：可为 nil。
	HostNet *mpAPI.NetInfo
}

// cgnatNet 是运营商级 NAT 共享地址段。
var cgnatNet = &net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

// Classify 返回链路失败码。
// 职责：按 20407 > 20406 > 20401 > 20405 > 20403 > 20402 > 20404 的优先级归类；使用：链路进入 failed 或 TARGET_REFUSED 时；边界：只是诊断提示。
func Classify(input ClassifyInput) int {
	if input.TargetRefused {
		return mpConstant.LinkErrTargetRefused
	}
	if input.DialErr != nil {
		text := strings.ToLower(input.DialErr.Error())
		if strings.Contains(text, "gater") || strings.Contains(text, "gated") {
			return mpConstant.LinkErrRejected
		}
	}
	selfV4, selfV6 := families(input.SelfAddrs)
	hostV4, hostV6 := families(input.HostAddrs)
	if (selfV4 || selfV6) && (hostV4 || hostV6) && !(selfV4 && hostV4) && !(selfV6 && hostV6) {
		return mpConstant.LinkErrIPFamily
	}
	if input.HostNet != nil && !input.HostNet.FirewallRule {
		return mpConstant.LinkErrHostFirewall
	}
	if anyCGNAT(input.SelfObserved) || anyCGNAT(input.HostAddrs) || mismatch(input.SelfObserved, input.SelfUPnP) {
		return mpConstant.LinkErrCGNAT
	}
	if isFalse(input.SelfNet.NATConsistent) || (input.HostNet != nil && isFalse(input.HostNet.NATConsistent)) {
		return mpConstant.LinkErrNATInconsistent
	}
	return mpConstant.LinkErrPunchTimeout
}

// families 返回地址列表中是否有可路由的 IPv4 / IPv6（忽略回环与链路本地）。
func families(addrs []string) (v4 bool, v6 bool) {
	for _, ip := range ips(addrs) {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.To4() != nil {
			v4 = true
		} else {
			v6 = true
		}
	}
	return v4, v6
}

// anyCGNAT 判断是否有地址落在 100.64.0.0/10。
func anyCGNAT(addrs []string) bool {
	for _, ip := range ips(addrs) {
		if cgnatNet.Contains(ip) {
			return true
		}
	}
	return false
}

// mismatch 判断 observed IPv4 与 UPnP 映射外网 IPv4 是否不一致。
func mismatch(observed, upnp []string) bool {
	if len(observed) == 0 || len(upnp) == 0 {
		return false
	}
	mapped := map[string]bool{}
	for _, ip := range ips(upnp) {
		if ip.To4() != nil {
			mapped[ip.String()] = true
		}
	}
	if len(mapped) == 0 {
		return false
	}
	for _, ip := range ips(observed) {
		if ip.To4() != nil && !mapped[ip.String()] {
			return true
		}
	}
	return false
}

// ips 解析 multiaddr 中的 IP。
func ips(addrs []string) []net.IP {
	result := []net.IP{}
	for _, raw := range addrs {
		addr, err := ma.NewMultiaddr(raw)
		if err != nil {
			continue
		}
		if ip, err := manet.ToIP(addr); err == nil {
			result = append(result, ip)
		}
	}
	return result
}

func isFalse(value *bool) bool { return value != nil && !*value }
