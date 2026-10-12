package p2p

// 文件职责：接收原生 DCUtR 的实际协议事件。使用：控制器注册诊断回调。边界：不自行协调打洞、不推断定时拨号成功。

import (
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"sync"
)

// punchTracer 适配库事件。职责：并发安全回调；使用：EnableHolePunching；边界：空回调不产生业务行为。
type punchTracer struct {
	mu       sync.RWMutex
	callback func(*holepunch.Event)
}

// Trace 转交库原始事件。职责：记录实际 DCUtR；使用：libp2p；边界：回调必须立即返回。
func (tracer *punchTracer) Trace(evt *holepunch.Event) {
	tracer.mu.RLock()
	callback := tracer.callback
	tracer.mu.RUnlock()
	if callback != nil {
		callback(evt)
	}
}

// SetDCUtRTracer 注册原生事件监听。职责：诊断库结果；使用：控制器建 session 后；边界：不改变库协议或重试参数。
func (h *Host) SetDCUtRTracer(callback func(*holepunch.Event)) {
	h.punches.mu.Lock()
	h.punches.callback = callback
	h.punches.mu.Unlock()
}
