package p2p

// 文件职责：将已验证的单 observer 公网 observed 地址注入官方地址工厂。
// 使用：AutoNAT 回拨候选与 Host.Addrs 地址宣告；边界：只接收 identify 匹配的观测结果，且对应 peer 原生连接仍存活。

import (
	"sync"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
)

type observedAddrFactory struct {
	mu           sync.RWMutex
	net          network.Network
	observer     peer.ID
	observations []Observation
}

func (f *observedAddrFactory) setNetwork(net network.Network) {
	f.mu.Lock()
	f.net = net
	f.mu.Unlock()
}

// setObservations 整轮替换 identify 证实的观测结果。
// 职责：排除其他 PeerID 与私网/回环地址；使用：addrLoop；边界：空结果会撤销上一轮注入。
func (f *observedAddrFactory) setObservations(observer peer.ID, observations []Observation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.observer = observer
	f.observations = nil
	for _, observation := range observations {
		if observation.Observer == observer && observation.Addr != nil && manet.IsPublicAddr(observation.Addr) && !IsCircuit(observation.Addr) {
			f.observations = append(f.observations, observation)
		}
	}
}

// addrs 在库原有地址后附加仍有活跃 observer 连接的公网观测地址。
// 职责：给官方 AutoNAT 与 host 地址提供单节点 identify 候选；使用：libp2p.AddrsFactory；边界：断连后立即不再注入。
func (f *observedAddrFactory) addrs(addrs []ma.Multiaddr) []ma.Multiaddr {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.net == nil || f.observer == "" || f.net.Connectedness(f.observer) != network.Connected {
		return addrs
	}
	result := append([]ma.Multiaddr{}, addrs...)
	for _, observation := range f.observations {
		if observationConnAlive(f.net, observation) {
			result = append(result, observation.Addr)
		}
	}
	return ma.Unique(result)
}

// observationConnAlive 检查观测结果所绑定的具体 libp2p 连接。
// 职责：防止同 PeerID 的另一传输仍在线时误保留旧映射；使用：工厂与观测轮次过滤；边界：连接 ID 必须精确匹配。
func observationConnAlive(net network.Network, observation Observation) bool {
	if net == nil || observation.ConnID == "" {
		return false
	}
	for _, conn := range net.ConnsToPeer(observation.Observer) {
		if conn.ID() == observation.ConnID && !conn.IsClosed() {
			return true
		}
	}
	return false
}

// LiveObservations 返回仍有对应真实连接的地址。
// 职责：避免上报被后续 TCP/QUIC 重建关闭的候选；使用：ObserveDetailed；边界：不会恢复已断连接。
func (h *Host) LiveObservations(observations []Observation) []Observation {
	result := make([]Observation, 0, len(observations))
	for _, observation := range observations {
		if observationConnAlive(h.Network(), observation) {
			result = append(result, observation)
		}
	}
	return result
}

// SetObservedAddrs 更新本轮由指定 observer 返回的地址候选。
// 职责：把连接级 identify 结果接入地址工厂；使用：控制器每轮观测后；边界：无合法 observer 时清除候选。
func (h *Host) SetObservedAddrs(observers []string, observations []Observation) {
	infos, _ := groupObservers(FirstObserverAddrs(observers))
	if len(infos) == 0 {
		h.observedFactory.setObservations("", nil)
		return
	}
	h.observedFactory.setObservations(infos[0].ID, observations)
}
