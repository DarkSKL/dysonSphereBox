//go:build !windows

package forward

// 文件职责：非 Windows 平台的 UDP 端口不可达处理占位。
// 使用：保证 forward 包可在其他平台编译；边界：盒子只发布 Windows 版本。

import (
	"errors"
	"net"
	"syscall"
)

// enableConnReset 非 Windows 平台无需处理。
func enableConnReset(*net.UDPConn) {}

// isConnReset 判断读错误是否为端口不可达。
func isConnReset(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }
