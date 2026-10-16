package p2p

// 文件职责：解析 session.open 指定观测节点的公网来源 IP，供官方 AutoNAT 临时回拨准入。
// 使用：控制器绑定会话后配置 Gater；边界：DNS 只在配置时解析，缺少公网解析结果时不开放未知入站。

import (
	"context"
	"net"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

// AutoNATSources 职责：从指定 observer 入口及服务端显式出口提取公网 IP；使用：配置回拨准入；边界：出口仅接受 IP 字面量，拒绝内网、主机名及网段。
func AutoNATSources(ctx context.Context, observers []string, egressIPs []string) []net.IP {
	observers = FirstObserverAddrs(observers)
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	seen := map[string]struct{}{}
	result := []net.IP{}
	add := func(ip net.IP) {
		addr := ipMultiaddr(ip)
		if addr == nil || !manet.IsPublicAddr(addr) {
			return
		}
		if _, ok := seen[ip.String()]; !ok {
			seen[ip.String()] = struct{}{}
			result = append(result, ip)
		}
	}
	for _, raw := range observers {
		addr, err := ma.NewMultiaddr(raw)
		if err != nil || IsCircuit(addr) {
			continue
		}
		if _, err := peer.AddrInfoFromP2pAddr(addr); err != nil {
			continue
		}
		if ip, err := manet.ToIP(addr); err == nil {
			add(ip)
			continue
		}
		for _, code := range []int{ma.P_DNS, ma.P_DNS4, ma.P_DNS6} {
			name, err := addr.ValueForProtocol(code)
			if err != nil {
				continue
			}
			ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, name)
			if err != nil {
				break
			}
			for _, item := range ips {
				add(item.IP)
			}
			break
		}
	}
	for _, raw := range egressIPs {
		add(net.ParseIP(raw))
	}
	return result
}

// ipMultiaddr 职责：构造供公网判断使用的 IP 地址；使用：AutoNATSources；边界：非法 IP 返回 nil。
func ipMultiaddr(ip net.IP) ma.Multiaddr {
	if ip == nil {
		return nil
	}
	proto := "/ip6/"
	if ip.To4() != nil {
		proto = "/ip4/"
		ip = ip.To4()
	}
	addr, _ := ma.NewMultiaddr(proto + ip.String())
	return addr
}
