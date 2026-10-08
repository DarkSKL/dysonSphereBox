//go:build windows

package forward

// 文件职责：在 Windows 上为房主 UDP 会话 socket 重新开启 ICMP 端口不可达报告。
// 使用：房主创建 UDP 会话 socket 后调用，让读操作在目标端口无人监听时返回 WSAECONNRESET，以便换下一个候选地址。
// 边界：Go 运行时默认关闭 SIO_UDP_CONNRESET；这里只对房主到游戏的会话 socket 开启，不影响 libp2p 的 QUIC socket。

import (
	"errors"
	"net"
	"syscall"
	"unsafe"
)

// enableConnReset 开启 SIO_UDP_CONNRESET。
func enableConnReset(conn *net.UDPConn) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		flag := uint32(1)
		ret := uint32(0)
		_ = syscall.WSAIoctl(syscall.Handle(fd), syscall.SIO_UDP_CONNRESET, (*byte)(unsafe.Pointer(&flag)),
			uint32(unsafe.Sizeof(flag)), nil, 0, &ret, nil, 0)
	})
}

// isConnReset 判断读错误是否为 WSAECONNRESET（目标端口不可达）。
func isConnReset(err error) bool {
	return errors.Is(err, syscall.WSAECONNRESET) || errors.Is(err, syscall.Errno(10054))
}
