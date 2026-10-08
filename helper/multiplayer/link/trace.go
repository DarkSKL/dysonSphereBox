package link

// 文件职责：link.trace（1303）上报器：把数据链路建立过程中的每一步异步发给服务器（排障用）。
// 使用：service/multiplayer 每个控制链路会话 NewTracer，各上报点调用 Trace，会话结束时 Close。
// 边界：Trace 不阻塞（只加锁与非阻塞入队），可在事件 goroutine 与转发路径中直接调用；单个后台 goroutine 顺序发送、不重试；
// 字段按协议截断；本地每分钟最多 120 条，超出与队列满时直接丢弃；收到 20002 后本会话停止发送。

import (
	"context"
	"sync"
	"time"
	"unicode/utf8"

	mpAPI "DysonSphereBox/apiDefinition/multiplayer"
	mpConstant "DysonSphereBox/constant/multiplayer"
)

// traceQueueSize 职责：待发送 trace 队列长度；使用：Tracer；边界：满时丢弃新条目。
const traceQueueSize = 64

// Sender 是发送控制链路请求的能力。
// 职责：抽象 socket.Conn.Request；使用：Tracer；边界：测试可用替身。
type Sender interface {
	Request(ctx context.Context, opcode int, data any) (*mpAPI.ReplyFrame, error)
}

// Tracer 是一个会话的 link.trace 上报器。
// 职责：截断、限频、异步顺序发送；使用：控制器与成员链路；边界：nil 接收者的方法什么都不做；Close 后不再发送。
type Tracer struct {
	sender Sender
	now    func() time.Time
	queue  chan mpAPI.LinkTraceRequest
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	stopped bool
	sent    []time.Time
}

// NewTracer 创建上报器并启动发送 goroutine。
func NewTracer(sender Sender) *Tracer {
	tracer := newTracer(sender, time.Now)
	go tracer.loop()
	return tracer
}

func newTracer(sender Sender, now func() time.Time) *Tracer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Tracer{sender: sender, now: now, queue: make(chan mpAPI.LinkTraceRequest, traceQueueSize), ctx: ctx, cancel: cancel}
}

// Trace 提交一条 trace。
// 职责：截断字段、按每分钟 120 条限频后非阻塞入队；使用：各 stage 结束时；边界：已停止、已关闭、超限或队列满时丢弃。
func (tracer *Tracer) Trace(request mpAPI.LinkTraceRequest) {
	if tracer == nil || tracer.ctx.Err() != nil {
		return
	}
	request.Addr = truncateUTF8(request.Addr, mpConstant.TraceAddrMax)
	request.Error = truncateUTF8(request.Error, mpConstant.TraceErrorMax)
	request.Detail = truncateUTF8(request.Detail, mpConstant.TraceDetailMax)
	tracer.mu.Lock()
	if tracer.stopped || !tracer.allowLocked() {
		tracer.mu.Unlock()
		return
	}
	tracer.mu.Unlock()
	select {
	case tracer.queue <- request:
	default:
	}
}

// Stopped 返回是否因服务器不认识 opcode 而停止。
func (tracer *Tracer) Stopped() bool {
	if tracer == nil {
		return true
	}
	tracer.mu.Lock()
	defer tracer.mu.Unlock()
	return tracer.stopped
}

// Close 停止发送并结束后台 goroutine（会话结束时调用，重复调用无副作用）。
func (tracer *Tracer) Close() {
	if tracer != nil {
		tracer.cancel()
	}
}

// allowLocked 按滑动一分钟窗口判断是否还能发送（持 mu 调用）。
func (tracer *Tracer) allowLocked() bool {
	now := tracer.now()
	cutoff := now.Add(-time.Minute)
	expired := 0
	for expired < len(tracer.sent) && !tracer.sent[expired].After(cutoff) {
		expired++
	}
	tracer.sent = tracer.sent[expired:]
	if len(tracer.sent) >= mpConstant.TracePerMinute {
		return false
	}
	tracer.sent = append(tracer.sent, now)
	return true
}

// loop 顺序发送队列中的 trace；回复 20002 时本会话停止，其他失败直接丢弃。
func (tracer *Tracer) loop() {
	for {
		select {
		case <-tracer.ctx.Done():
			return
		case request := <-tracer.queue:
			reply, err := tracer.sender.Request(tracer.ctx, mpConstant.OpLinkTrace, request)
			if err == nil && reply != nil && reply.Code == mpConstant.CodeUnknownOpcode {
				tracer.mu.Lock()
				tracer.stopped = true
				tracer.mu.Unlock()
				return
			}
		}
	}
}

// truncateUTF8 把字符串截断到 max 字节以内，不切开多字节字符。
func truncateUTF8(text string, max int) string {
	if len(text) <= max {
		return text
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut]
}

// ErrText 返回错误文本，nil 为空字符串。
func ErrText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
