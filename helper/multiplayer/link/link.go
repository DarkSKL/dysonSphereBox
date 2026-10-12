package link

// 文件职责：成员到房主的数据链路拨号（直连、指定原生中继）、路径与 ipFamily 判定、失败分类（方案第 7.3–7.5 节）。
// 使用：service/multiplayer 成员链路状态机调用 DialDirect/DialRelay，连通后 PathOf，全部失败后 Classify。
// 边界：只做单次拨号与判定，不维护状态机、不发控制链路请求；NAT 分类只是诊断提示。

import (
	"context"
	"fmt"
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
			if !p2p.IsCircuit(addr) {
				if _, err := addr.ValueForProtocol(ma.P_P2P); err == nil {
					parsed, err := peer.AddrInfoFromP2pAddr(addr)
					if err != nil || parsed.ID != id || len(parsed.Addrs) != 1 {
						continue
					}
					addr = parsed.Addrs[0]
				}
			}
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
	direct := peer.AddrInfo{ID: info.ID}
	for _, addr := range info.Addrs {
		if !p2p.IsCircuit(addr) {
			direct.Addrs = append(direct.Addrs, addr)
		}
	}
	clearBackoff(h, info.ID)
	dialCtx, cancel := context.WithTimeout(network.WithForceDirectDial(ctx, "direct"), timeout)
	defer cancel()
	return h.ConnectReport(dialCtx, direct)
}

// DialRelay 仅拨 WS 指定的房主中继入口。
// 职责：建立原生 DCUtR 初始连接；使用：直连失败后；边界：默认能力关闭及未知入口全部拒绝。
func DialRelay(ctx context.Context, h *p2p.Host, info peer.AddrInfo, timeout time.Duration) p2p.DialReport {
	trusted := peer.AddrInfo{ID: info.ID}
	for _, addr := range info.Addrs {
		if h.TrustedCircuit(addr, info.ID) {
			trusted.Addrs = append(trusted.Addrs, addr)
		}
	}
	if !h.RelayEnabled() || len(trusted.Addrs) == 0 {
		return p2p.DialReport{Err: fmt.Errorf("服务器未授权可用中继入口")}
	}
	clearBackoff(h, info.ID)
	dialCtx, cancel := context.WithTimeout(network.WithAllowLimitedConn(ctx, "dcutr-bootstrap"), timeout)
	defer cancel()
	return h.ConnectReport(dialCtx, trusted)
}

// PathOf 判定已连通链路的路径与地址族。
// 职责：受限连接为 relay，私网为 lan，UPnP 公网 IPv4 为 upnp；使用：link.report；边界：relay 不用中继服务器的 IP 推断游戏对端地址族。
func PathOf(conn network.Conn, hostNet *mpAPI.NetInfo) (path string, family string) {
	if conn.Stat().Limited || p2p.IsCircuit(conn.RemoteMultiaddr()) {
		return mpConstant.PathRelay, ""
	}
	ip, err := manet.ToIP(conn.RemoteMultiaddr())
	if err != nil {
		return mpConstant.PathDirect, ""
	}
	family = "v6"
	if ip.To4() != nil {
		family = "v4"
	}
	switch {
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
		if err != nil || p2p.IsCircuit(addr) {
			continue
		}
		if ip, err := manet.ToIP(addr); err == nil {
			result = append(result, ip)
		}
	}
	return result
}

func isFalse(value *bool) bool { return value != nil && !*value }
