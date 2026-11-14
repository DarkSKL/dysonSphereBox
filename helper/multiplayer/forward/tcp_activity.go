package forward

// 文件职责：记录成员侧中继 TCP 成功写入游戏的数据；使用：链路重连判定；边界：不观察包内容、握手、上行或直连流量。

import (
	"net"
	"sync/atomic"
	"time"

	"DysonSphereBox/helper/multiplayer/p2p"

	"github.com/libp2p/go-libp2p/core/network"
)

// tcpReceiveActivity 记录一条中继游戏流的接收活动；职责：观察本地写入结果；使用：作为下行转发 writer；边界：零值时间表示还没有写入。
type tcpReceiveActivity struct {
	conn   *net.TCPConn
	linkID string
	start  time.Time
	// lastWrite 职责：记录最近成功写入时间；使用：探测失败时检查；边界：保存单调时间偏移加一，零表示没有数据。
	lastWrite atomic.Int64
}

// Write 将数据写入游戏；职责：仅在有字节写入时更新时间；使用：TCP 下行转发；边界：返回原始写入结果和错误。
func (activity *tcpReceiveActivity) Write(data []byte) (int, error) {
	n, err := activity.conn.Write(data)
	if n > 0 {
		activity.lastWrite.Store(int64(time.Since(activity.start)) + 1)
	}
	return n, err
}

// pipeGameTCP 转发已完成握手的游戏连接；职责：按实际承载的中继连接跟踪收包；使用：成员侧 TCP；边界：仅保护仍在转发的连接。
func (forwarder *LocalForwarder) pipeGameTCP(stream network.Stream, conn *net.TCPConn) {
	link := stream.Conn()
	if !link.Stat().Limited && !p2p.IsCircuit(link.RemoteMultiaddr()) {
		Pipe(stream, conn)
		return
	}
	activity := &tcpReceiveActivity{conn: conn, linkID: link.ID(), start: time.Now()}
	forwarder.mu.Lock()
	if _, tracked := forwarder.tcp[conn]; !tracked {
		forwarder.mu.Unlock()
		_ = stream.Reset()
		return
	}
	forwarder.tcp[conn] = activity
	forwarder.mu.Unlock()
	pipeTCP(stream, conn, activity)
}

// HasRecentRelayTCPReceive 判断指定中继连接是否仍在向游戏交付数据；职责：提供链路活性证据；使用：关闭连接前检查；边界：只统计当前转发中的游戏连接。
func (forwarder *LocalForwarder) HasRecentRelayTCPReceive(linkID string, since time.Time) bool {
	forwarder.mu.Lock()
	defer forwarder.mu.Unlock()
	for _, activity := range forwarder.tcp {
		if activity == nil || activity.linkID != linkID {
			continue
		}
		if at := activity.lastWrite.Load(); at != 0 && !activity.start.Add(time.Duration(at-1)).Before(since) {
			return true
		}
	}
	return false
}
