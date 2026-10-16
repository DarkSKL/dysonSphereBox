package forward

// 文件职责：房主目标地址候选、探测与缓存（方案第 7.2.2 节）。
// 使用：房主 TCP handler 拨号游戏端口 G、UDP 会话选择目标地址、前端 network.targets 投影。
// 边界：候选只包含本机地址，永远不包含局域网其他机器；网卡变化时由调用方 Clear。

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
)

// TargetState 是一个端口声明的目标地址探测结果。
// 职责：前端展示“已连通 127.0.0.1 / 未检测到监听”；使用：network.targets；边界：UDP 声明只在沿用 TCP 结果时有地址。
type TargetState struct {
	// Index 职责：端口声明下标；使用：前端对应端口表；边界：从 0 开始。
	Index int `json:"index"`
	// Proto 职责：声明协议；使用：展示；边界：tcp/udp/both。
	Proto string `json:"proto"`
	// Port 职责：游戏端口起点 G；使用：展示；边界：1–65535。
	Port int `json:"port"`
	// Address 职责：已连通的目标地址；使用：展示；边界：未探测或全部拒绝时为空。
	Address string `json:"address"`
	// Status 职责：unknown / ok / refused；使用：展示；边界：UDP 无法确认时为 unknown。
	Status string `json:"status"`
}

// Candidates 返回目标地址候选列表。
// 职责：127.0.0.1、::1、网卡 IPv4、网卡 IPv6 全局与链路本地（带 zone）；使用：探测与 UDP 换候选；边界：只取本机地址。
func Candidates() []string {
	result := []string{"127.0.0.1", "::1"}
	var ipv6 []string
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			switch {
			case ip.To4() != nil:
				result = append(result, ip.String())
			case ip.IsLinkLocalUnicast():
				ipv6 = append(ipv6, ip.String()+"%"+iface.Name)
			case ip.IsGlobalUnicast():
				ipv6 = append(ipv6, ip.String())
			}
		}
	}
	return append(result, ipv6...)
}

// targetCache 缓存每个端口声明探测到的目标地址。
// 职责：TCP 首次探测后复用、被拒绝时重探；使用：HostForwarder；边界：并发安全。
type targetCache struct {
	mu         sync.Mutex
	address    map[int]string
	refused    map[int]bool
	candidates func() []string
	probe      time.Duration
}

func newTargetCache() *targetCache {
	return &targetCache{address: map[int]string{}, refused: map[int]bool{}, candidates: Candidates,
		probe: mpConstant.TargetProbeTimeout}
}

// clear 清除全部缓存（网卡变化）。
func (cache *targetCache) clear() {
	cache.mu.Lock()
	cache.address = map[int]string{}
	cache.refused = map[int]bool{}
	cache.mu.Unlock()
}

// get 返回已缓存的目标地址。
func (cache *targetCache) get(index int) string {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.address[index]
}

// targetProbe 是一次 dialTCP 的探测结果。
// 职责：告诉调用方是否真正探测了候选地址及结果；使用：target_probe trace；边界：命中缓存时 probed 为 false。
type targetProbe struct {
	probed   bool
	address  string
	duration time.Duration
	err      error
}

// dialTCP 按缓存或候选顺序拨号目标端口。
// 职责：缓存地址被拒绝时清除并重新探测，全部拒绝时记 refused；使用：TCP handler；边界：每个候选最多 500 毫秒。
func (cache *targetCache) dialTCP(index int, port int) (net.Conn, targetProbe, error) {
	if cached := cache.get(index); cached != "" {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(cached, strconv.Itoa(port)), 2*time.Second)
		if err == nil {
			return conn, targetProbe{}, nil
		}
		cache.mu.Lock()
		delete(cache.address, index)
		cache.mu.Unlock()
	}
	started := time.Now()
	var lastErr error
	for _, candidate := range cache.candidates() {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(candidate, strconv.Itoa(port)), cache.probe)
		if err != nil {
			lastErr = err
			continue
		}
		cache.mu.Lock()
		cache.address[index] = candidate
		delete(cache.refused, index)
		cache.mu.Unlock()
		return conn, targetProbe{probed: true, address: candidate, duration: time.Since(started)}, nil
	}
	cache.mu.Lock()
	cache.refused[index] = true
	cache.mu.Unlock()
	if lastErr == nil {
		lastErr = errors.New("no target candidate")
	}
	return nil, targetProbe{probed: true, duration: time.Since(started), err: lastErr}, lastErr
}

// probeTCP 主动探测一个 TCP 端口声明的目标地址，连通后立即关闭连接。
// 职责：房主无成员连入时也能得知游戏是否在监听；使用：HostForwarder.prober 周期调用；边界：只拨本机候选，返回投影状态是否变化。
func (cache *targetCache) probeTCP(index int, port int) bool {
	before := cache.status(index)
	conn, _, err := cache.dialTCP(index, port)
	if err == nil {
		_ = conn.Close()
	}
	return cache.status(index) != before
}

// status 返回单个端口声明的投影状态。
func (cache *targetCache) status(index int) string {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if address := cache.address[index]; address != "" {
		return "ok:" + address
	}
	if cache.refused[index] {
		return "refused"
	}
	return "unknown"
}

// states 生成前端投影。
func (cache *targetCache) states(ports []mpAPI.PortDecl) []TargetState {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	result := make([]TargetState, 0, len(ports))
	for index, decl := range ports {
		state := TargetState{Index: index, Proto: decl.Proto, Port: decl.Port, Status: "unknown"}
		if address := cache.address[index]; address != "" {
			state.Address, state.Status = address, "ok"
		} else if cache.refused[index] {
			state.Status = "refused"
		}
		result = append(result, state)
	}
	return result
}
