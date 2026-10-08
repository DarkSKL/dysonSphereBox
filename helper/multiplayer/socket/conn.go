package socket

// 文件职责：联机控制链路 WebSocket 连接：x-token 握手、读循环分流回复帧与主动帧、写锁、心跳与读超时、在途请求表、时钟偏移估算。
// 使用：service/multiplayer 控制器每次建立控制链路时调用 Dial，通过 Request 发请求、通过 EventHandler 回复服务器事件。
// 边界：不解释事件业务含义、不重连；回复帧在读循环中直接交付在途请求，不经事件队列，事件处理中发请求不会死锁。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"

	"github.com/gorilla/websocket"
)

// EventHandler 处理一个服务器主动帧并返回回复码与提示。
// 职责：由控制器实现事件分发；使用：Conn 的单个事件 goroutine 按到达顺序调用；边界：必须在 5 秒内返回，不得等待请求回复。
type EventHandler func(frame *mpAPI.ActiveFrame) (code int, msg string)

// Config 是建立控制链路所需参数。
// 职责：描述地址、token 与时长；使用：Dial；边界：时长为 0 时使用协议默认值，测试可缩短。
type Config struct {
	// URL 职责：ws/wss 地址；使用：拨号；边界：由 mpAPI.SocketURL 生成。
	URL string
	// Token 职责：账户 token；使用：x-token 请求头；边界：永远不放 URL、不进前端。
	Token string
	// PingInterval 职责：ping 间隔；使用：心跳；边界：默认 5 秒。
	PingInterval time.Duration
	// ReadTimeout 职责：读超时；使用：判定断线；边界：默认 15 秒。
	ReadTimeout time.Duration
	// RequestTimeout 职责：请求等待回复时限；使用：超时以 20901 失败；边界：默认 10 秒。
	RequestTimeout time.Duration
}

// LocalError 是只在盒子本地产生的请求失败。
// 职责：携带 20901/20902/20903；使用：控制器把 Code 透传给前端；边界：不出现在线上。
type LocalError struct {
	// Code 职责：本地码；使用：前端文案映射；边界：209xx。
	Code int
}

// Error 返回本地错误文本。
func (err *LocalError) Error() string { return fmt.Sprintf("multiplayer local error %d", err.Code) }

// HandshakeError 是 WebSocket 升级被服务器拒绝。
// 职责：携带 HTTP 状态与响应体 code；使用：控制器判断是否自动重连；边界：只在 Dial 返回。
type HandshakeError struct {
	// Status 职责：HTTP 状态码；使用：401/404/409 不重连；边界：0 表示没有响应。
	Status int
	// Code 职责：响应体 code；使用：7 未登录、10001 需要改密；边界：解析失败为 0。
	Code int
	// Msg 职责：响应体 msg；使用：日志与提示；边界：可为空。
	Msg string
}

// Error 返回握手错误文本。
func (err *HandshakeError) Error() string {
	return fmt.Sprintf("multiplayer handshake rejected: status=%d code=%d msg=%s", err.Status, err.Code, err.Msg)
}

// Permanent 判断握手拒绝是否不应自动重连。
// 职责：鉴权失败、需要改密、功能关闭时返回 true；使用：控制器重连策略；边界：其他状态按断线退避。
func (err *HandshakeError) Permanent() bool {
	return err.Status == http.StatusUnauthorized || err.Status == http.StatusConflict ||
		err.Status == http.StatusNotFound || err.Code == 7 || err.Code == 10001
}

// pendingRequest 是一个在途请求。
// 职责：保存回复通道与发送时刻；使用：读循环交付、时钟偏移估算；边界：通道容量 1。
type pendingRequest struct {
	reply    chan *mpAPI.ReplyFrame
	sentAtMs int64
}

// Conn 是一条控制链路 socket。
// 职责：串行写、分流读、维护在途请求与事件队列；使用：控制器持有直到 Done；边界：断开后不可复用。
type Conn struct {
	ws      *websocket.Conn
	cfg     Config
	handler EventHandler

	writeMu sync.Mutex
	sendMu  sync.Mutex

	mu       sync.Mutex
	nextSeq  int64
	pending  map[int64]*pendingRequest
	closed   bool
	closeErr error

	// 事件队列：读循环追加，事件 goroutine 消费，保证按到达顺序处理且读循环不被事件处理阻塞。
	queueMu sync.Mutex
	queue   []*mpAPI.ActiveFrame
	queueCh chan struct{}

	clockMu     sync.Mutex
	offsetMs    int64
	offsetRTTMs int64
	offsetOK    bool

	done chan struct{}
}

// Dial 建立控制链路并启动读循环、事件循环与心跳。
// 职责：x-token 握手、设置读限制与截止；使用：控制器每次连接；边界：握手被拒时返回 *HandshakeError，不发送任何帧。
func Dial(ctx context.Context, cfg Config, handler EventHandler) (*Conn, error) {
	if cfg.PingInterval <= 0 {
		cfg.PingInterval = mpConstant.PingInterval
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = mpConstant.ReadTimeout
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = mpConstant.RequestTimeout
	}
	header := http.Header{}
	header.Set("x-token", cfg.Token)
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second, Proxy: http.ProxyFromEnvironment}
	ws, resp, err := dialer.DialContext(ctx, cfg.URL, header)
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
			handshakeErr := &HandshakeError{Status: resp.StatusCode}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			var parsed struct {
				Code int    `json:"code"`
				Msg  string `json:"msg"`
			}
			if json.Unmarshal(body, &parsed) == nil {
				handshakeErr.Code, handshakeErr.Msg = parsed.Code, parsed.Msg
			}
			return nil, handshakeErr
		}
		return nil, err
	}
	conn := &Conn{
		ws:      ws,
		cfg:     cfg,
		handler: handler,
		pending: make(map[int64]*pendingRequest),
		queueCh: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
	ws.SetReadLimit(mpConstant.MaxFrameBytes)
	_ = ws.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
	})
	ws.SetPingHandler(func(data string) error {
		_ = ws.SetReadDeadline(time.Now().Add(cfg.ReadTimeout))
		err := ws.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
		if err != nil && !errors.Is(err, websocket.ErrCloseSent) {
			return err
		}
		return nil
	})
	go conn.readLoop()
	go conn.eventLoop()
	go conn.pingLoop()
	return conn, nil
}

// Done 返回连接结束信号。
// 职责：通知控制器 socket 已断开；使用：控制器等待；边界：只关闭一次。
func (conn *Conn) Done() <-chan struct{} { return conn.done }

// Err 返回连接结束原因。
// 职责：区分主动关闭、seqId 跳号、读错误；使用：控制器日志与重连判断；边界：Done 之前为 nil。
func (conn *Conn) Err() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.closeErr
}

// Close 以关闭码 1000 主动关闭连接。
// 职责：用户断开或退出登录；使用：控制器；边界：重复调用无副作用。
func (conn *Conn) Close() {
	_ = conn.ws.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	conn.shutdown(errors.New("closed by client"))
}

// Request 发送一个请求并等待回复。
// 职责：分配 seqId、写帧、等待回复或超时；使用：控制器意图方法与链路 goroutine；边界：超时返回 LocalError 20901，断开返回 20902，回复非 0 码由调用方判断。
func (conn *Conn) Request(ctx context.Context, opcode int, data any) (*mpAPI.ReplyFrame, error) {
	request := &pendingRequest{reply: make(chan *mpAPI.ReplyFrame, 1)}
	// sendMu 覆盖“取 seq → 写帧”，保证 seqId 按发送顺序严格递增；mu 只保护在途表，读循环交付回复不被慢写阻塞
	conn.sendMu.Lock()
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		conn.sendMu.Unlock()
		return nil, &LocalError{Code: mpConstant.CodeDisconnected}
	}
	seq := conn.nextSeq + 1
	request.sentAtMs = time.Now().UnixMilli()
	raw, err := EncodeActive(opcode, data, mpAPI.ActiveExData{SeqID: seq, Timestamp: request.sentAtMs})
	if err != nil {
		conn.mu.Unlock()
		conn.sendMu.Unlock()
		return nil, err
	}
	conn.nextSeq = seq
	conn.pending[seq] = request
	conn.mu.Unlock()
	err = conn.write(raw)
	conn.sendMu.Unlock()
	if err != nil {
		conn.dropPending(seq)
		return nil, &LocalError{Code: mpConstant.CodeDisconnected}
	}

	timer := time.NewTimer(conn.cfg.RequestTimeout)
	defer timer.Stop()
	select {
	case reply := <-request.reply:
		if reply == nil {
			return nil, &LocalError{Code: mpConstant.CodeDisconnected}
		}
		return reply, nil
	case <-timer.C:
		conn.dropPending(seq)
		return nil, &LocalError{Code: mpConstant.CodeRequestTimeout}
	case <-ctx.Done():
		conn.dropPending(seq)
		return nil, ctx.Err()
	}
}

// ServerToLocal 把服务器毫秒时刻换算为本地时刻。
// 职责：打洞对时；使用：link.punch after 的 atMs；边界：没有偏移样本时按 0 偏移。
func (conn *Conn) ServerToLocal(serverMs int64) time.Time {
	conn.clockMu.Lock()
	offset := conn.offsetMs
	conn.clockMu.Unlock()
	return time.UnixMilli(serverMs - offset)
}

// ClockOffset 返回当前估算的时钟偏移（服务器 − 本地，毫秒）与是否有样本。
func (conn *Conn) ClockOffset() (int64, bool) {
	conn.clockMu.Lock()
	defer conn.clockMu.Unlock()
	return conn.offsetMs, conn.offsetOK
}

// write 串行写一帧文本。
// 职责：写锁与写截止；使用：请求与事件回复；边界：写失败即关闭连接。
func (conn *Conn) write(raw []byte) error {
	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()
	_ = conn.ws.SetWriteDeadline(time.Now().Add(conn.cfg.RequestTimeout))
	if err := conn.ws.WriteMessage(websocket.TextMessage, raw); err != nil {
		go conn.shutdown(fmt.Errorf("write: %w", err))
		return err
	}
	return nil
}

// reply 写一帧回复。
// 职责：回复服务器事件或非法帧；使用：事件循环与读循环；边界：失败只关闭连接。
func (conn *Conn) reply(code int, msg string, seq int64, opcode int) {
	raw, err := EncodeReply(code, msg, nil, mpAPI.ReplyExData{SeqID: seq, Opcode: opcode, Timestamp: time.Now().UnixMilli()})
	if err != nil {
		return
	}
	_ = conn.write(raw)
}

// dropPending 移除一个在途请求。
func (conn *Conn) dropPending(seq int64) {
	conn.mu.Lock()
	delete(conn.pending, seq)
	conn.mu.Unlock()
}

// readLoop 读取帧并分流。
// 职责：刷新读截止、回复帧直接交付、主动帧校验 seqId 后入事件队列；使用：Dial 启动；边界：读错误或 seqId 跳号即关闭连接。
func (conn *Conn) readLoop() {
	var tracker SeqTracker
	for {
		messageType, raw, err := conn.ws.ReadMessage()
		if err != nil {
			conn.shutdown(fmt.Errorf("read: %w", err))
			return
		}
		recvMs := time.Now().UnixMilli()
		_ = conn.ws.SetReadDeadline(time.Now().Add(conn.cfg.ReadTimeout))
		if messageType != websocket.TextMessage {
			conn.reply(mpConstant.CodeBadFrame, "binary frame", 0, 0)
			continue
		}
		frame, err := DecodeFrame(raw)
		if err != nil {
			conn.reply(mpConstant.CodeBadFrame, err.Error(), 0, 0)
			continue
		}
		if frame.Reply != nil {
			conn.deliverReply(frame.Reply, recvMs)
			continue
		}
		if err := tracker.Next(frame.Active.ExData.SeqID); err != nil {
			conn.shutdown(err)
			return
		}
		conn.clockMu.Lock()
		if !conn.offsetOK {
			// 没有请求样本时用事件粗估（忽略单程延迟）
			conn.offsetMs = frame.Active.ExData.Timestamp - recvMs
		}
		conn.clockMu.Unlock()
		conn.queueMu.Lock()
		conn.queue = append(conn.queue, frame.Active)
		conn.queueMu.Unlock()
		select {
		case conn.queueCh <- struct{}{}:
		default:
		}
	}
}

// deliverReply 把回复交给在途请求并更新时钟偏移。
// 职责：按 exData.seqId 匹配；使用：读循环；边界：找不到请求（已超时）时丢弃。
func (conn *Conn) deliverReply(reply *mpAPI.ReplyFrame, recvMs int64) {
	conn.mu.Lock()
	request := conn.pending[reply.ExData.SeqID]
	delete(conn.pending, reply.ExData.SeqID)
	conn.mu.Unlock()
	if request == nil {
		return
	}
	if reply.ExData.Timestamp > 0 {
		rtt := recvMs - request.sentAtMs
		conn.clockMu.Lock()
		if !conn.offsetOK || rtt <= conn.offsetRTTMs {
			conn.offsetMs = reply.ExData.Timestamp - (request.sentAtMs+recvMs)/2
			conn.offsetRTTMs = rtt
			conn.offsetOK = true
		}
		conn.clockMu.Unlock()
	}
	request.reply <- reply
}

// eventLoop 按到达顺序处理服务器事件并回复。
// 职责：调用 EventHandler 后写回复帧；使用：Dial 启动；边界：连接结束即退出，剩余事件不再回复。
func (conn *Conn) eventLoop() {
	for {
		conn.queueMu.Lock()
		if len(conn.queue) == 0 {
			conn.queueMu.Unlock()
			select {
			case <-conn.queueCh:
				continue
			case <-conn.done:
				return
			}
		}
		frame := conn.queue[0]
		conn.queue = conn.queue[1:]
		conn.queueMu.Unlock()

		code, msg := mpConstant.CodeUnknownOpcode, "unknown opcode"
		if conn.handler != nil {
			code, msg = conn.handler(frame)
		}
		select {
		case <-conn.done:
			return
		default:
		}
		conn.reply(code, msg, frame.ExData.SeqID, frame.Opcode)
	}
}

// pingLoop 定时发送 WS ping。
// 职责：协议心跳；使用：Dial 启动；边界：写失败即关闭连接。
func (conn *Conn) pingLoop() {
	ticker := time.NewTicker(conn.cfg.PingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := conn.ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(conn.cfg.PingInterval)); err != nil {
				conn.shutdown(fmt.Errorf("ping: %w", err))
				return
			}
		case <-conn.done:
			return
		}
	}
}

// shutdown 关闭连接并失败全部在途请求。
// 职责：只执行一次的清理；使用：任意错误或主动关闭；边界：在途请求以 20902 失败，不重发。
func (conn *Conn) shutdown(cause error) {
	conn.mu.Lock()
	if conn.closed {
		conn.mu.Unlock()
		return
	}
	conn.closed = true
	conn.closeErr = cause
	pending := conn.pending
	conn.pending = map[int64]*pendingRequest{}
	conn.mu.Unlock()

	_ = conn.ws.Close()
	close(conn.done)
	for _, request := range pending {
		request.reply <- nil
	}
}
