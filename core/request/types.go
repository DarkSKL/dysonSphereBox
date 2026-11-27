package request

// 文件职责：定义客户端适配器与会话提供者交换的数据。
// 使用：请求执行、业务适配和认证状态机。
// 边界：底层会话和响应头只在客户端 Go 侧交换。

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	api "DysonSphereBox/apiDefinition"
	"DysonSphereBox/core/requestcache"
)

// Transport 职责：发送固定服务器的 HTTP 请求；使用：适配器复用全局 HTTPClient；边界：统一禁止自动重定向。
type Transport interface {
	NewRequest(context.Context, string, string, io.Reader) (*http.Request, error)
	DoNoRedirect(*http.Request) (*http.Response, error)
	DoStreamNoRedirect(*http.Request) (*http.Response, error)
}

// Input 职责：携带单次调用的路径、查询、载荷及运行控制；使用：适配器绑定到接口计划；边界：与 ApiDesc 分开保存。
type Input struct {
	PathParams  any
	Query       url.Values
	Payload     any
	Body        io.Reader
	ContentType string
	// Range、IfRange 仅在媒体 Range 接口允许透传。
	Range   string
	IfRange string
	// Target 表示读写失效范围，例如帖子 ID 或分类 ID。
	Target string
	// SessionKey 由前端传入以核对会话。Session 供多步 Go 工作流固定已经捕获的会话。
	SessionKey string
	Session    *Session
	Force      bool
}

// Parameters 职责：公开可归一化的调用参数；使用：BeforeMiddleware；边界：会话、强刷和传输详情由适配器掌控。
type Parameters struct {
	PathParams any
	Query      url.Values
	Payload    any
	Target     string
}

// Session 职责：固定本次使用的服务器与认证快照；使用：同一调用的网络、缓存和响应验收；边界：凭据字段不返回给 Wails。
type Session struct {
	Mode       api.AuthMode
	Action     api.SessionAction
	Scope      requestcache.Scope
	Subject    string
	Generation uint64
	Credential string `json:"-"`
	Transport  Transport
	// Current 核对原会话仍可交付结果。Accept 原子提交原凭据的认证响应。
	Current func() error
	Accept  func(AuthUpdate) error
}

// AuthUpdate 职责：传递经过协议识别的认证应答；使用：会话提供者原子处理；边界：Data 仅在业务成功时提供。
type AuthUpdate struct {
	Refreshed    string
	Unauthorized bool
	NeedPassword bool
	Success      bool
	Data         json.RawMessage
}

// CaptureFunc 职责：按接口策略固定全局服务器与会话；使用：适配器在读取缓存前调用；边界：由账号状态机提供。
type CaptureFunc func(*api.ApiDesc, string) (Session, error)

// Adapter 职责：执行已加载的请求计划；使用：盒子服务器 HTTP 入口；边界：连接池和当前服务器由会话提供者注入。
type Adapter struct {
	Loader  *Loader
	cache   *requestcache.Cache
	capture CaptureFunc
}

// NewAdapter 职责：组装计划、会话提供者和有界缓存；使用：客户端启动或回环测试；边界：不创建独立 HTTP 连接池。
func NewAdapter(loader *Loader, capture CaptureFunc) *Adapter {
	return &Adapter{Loader: loader, cache: requestcache.New(128, 8<<20), capture: capture}
}

// Capture 职责：提前固定多步业务的会话；使用：下载复核、系统通知等工作流；边界：后续调用仍会检查有效性。
func (adapter *Adapter) Capture(desc *api.ApiDesc, expectedKey string) (Session, error) {
	p, err := adapter.Loader.load(desc)
	if err != nil {
		return Session{}, err
	}
	return adapter.capture(&p.desc, expectedKey)
}
