package p2p

// 文件职责：读取与持久化盒子 libp2p 监听端口 H（Multiplayer/ListenPort）。
// 使用：控制器启动 host 前调用 ListenPort；绑定失败时 NewRandomListenPort 换号并写回。
// 边界：只保存端口号，不保存任何密钥；配置未初始化时只返回随机端口不写回。

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"strings"

	mpConstant "DysonSphereBox/constant/multiplayer"
	"DysonSphereBox/core"
)

// ListenPort 返回已持久化的监听端口，没有时随机一个并写回。
// 职责：保证端口首次随机后稳定；使用：控制器每次建立控制链路；边界：配置值非法时视为没有。
func ListenPort() int {
	if core.GLOBAL.Configure != nil && core.GLOBAL.Configure.MultiplayerListenPort != nil {
		if raw, err := core.GLOBAL.Configure.MultiplayerListenPort.Get(); err == nil {
			if port, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && port > 0 && port < 65536 {
				return port
			}
		}
	}
	return NewRandomListenPort()
}

// NewRandomListenPort 随机一个新端口并写回配置。
// 职责：首次使用或绑定失败时换号；使用：ListenPort 与控制器重试；边界：写回失败只影响下次启动。
func NewRandomListenPort() int {
	port := mpConstant.LocalPortMin
	if value, err := rand.Int(rand.Reader, big.NewInt(mpConstant.LocalPortMax-mpConstant.LocalPortMin)); err == nil {
		port += int(value.Int64())
	}
	if core.GLOBAL.Configure != nil && core.GLOBAL.Configure.MultiplayerListenPort != nil {
		_ = core.GLOBAL.Configure.MultiplayerListenPort.Set(strconv.Itoa(port))
	}
	return port
}

// HasIPv4QUIC 判断 host 是否成功监听了 IPv4 QUIC。
// 职责：部分监听失败时决定是否换端口；使用：控制器启动 host 后；边界：IPv6 不可用的机器不因此失败。
func HasIPv4QUIC(h *Host) bool {
	for _, addr := range h.Network().ListenAddresses() {
		text := addr.String()
		if strings.HasPrefix(text, "/ip4/") && strings.HasSuffix(text, "/quic-v1") {
			return true
		}
	}
	return false
}
