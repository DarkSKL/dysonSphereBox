package request

// 文件职责：校验 ApiDesc 并装配接口执行计划。
// 使用：应用启动加载目录，适配器取得相同的只读计划。
// 边界：加载不联网、不读取凭据，接口 ID 绑定唯一的声明。

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	api "DysonSphereBox/apiDefinition"
	"DysonSphereBox/core/requestcache"
)

// BeforeFunc 职责：归一化本次参数；使用：生成网络请求与缓存键之前；边界：只能修改调用数据。
type BeforeFunc func(context.Context, *Parameters) error

// AfterFunc 职责：根据真实成功数据产生缓存失效；使用：业务写接口；边界：缓存命中与在途等待者不会重复执行。
type AfterFunc func(context.Context, Success) []requestcache.Source

// Success 职责：向成功中间件传递本次业务结果；使用：确定写入影响；边界：仅含已验证数据，不携带凭据和响应头。
type Success struct {
	API    string
	Target string
	Data   json.RawMessage
}

// Loader 职责：注册扩展阶段并复用接口计划；使用：共享适配器；边界：一个接口 ID 对应一个声明地址。
type Loader struct {
	mu     sync.Mutex
	before map[api.MiddlewareID]BeforeFunc
	after  map[api.MiddlewareID]AfterFunc
	plans  map[string]*plan
}

// plan 保存描述快照与已解析的执行策略，供并发调用共享。
type plan struct {
	source        *api.ApiDesc
	desc          api.ApiDesc
	before        []BeforeFunc
	after         []AfterFunc
	freshness     func(http.Header, time.Time, time.Duration) time.Duration
	authenticated bool
}

// NewLoader 职责：创建请求计划加载器；使用：客户端基础设施初始化与测试；边界：初始无扩展中间件。
func NewLoader() *Loader {
	return &Loader{
		before: make(map[api.MiddlewareID]BeforeFunc),
		after:  make(map[api.MiddlewareID]AfterFunc),
		plans:  make(map[string]*plan),
	}
}

// RegisterBefore 职责：注册命名的请求准备阶段；使用：业务模块初始化；边界：重复或空的注册立即报告程序错误。
func (loader *Loader) RegisterBefore(id api.MiddlewareID, run BeforeFunc) {
	loader.mu.Lock()
	defer loader.mu.Unlock()
	if id == "" || run == nil || loader.before[id] != nil {
		panic(fmt.Sprintf("invalid before middleware registration %q", id))
	}
	loader.before[id] = run
}

// RegisterAfter 职责：注册命名的真实成功阶段；使用：业务模块初始化；边界：同阶段的名称唯一。
func (loader *Loader) RegisterAfter(id api.MiddlewareID, run AfterFunc) {
	loader.mu.Lock()
	defer loader.mu.Unlock()
	if id == "" || run == nil || loader.after[id] != nil {
		panic(fmt.Sprintf("invalid after middleware registration %q", id))
	}
	loader.after[id] = run
}

// LoadAll 职责：提前解析并校验应用的接口目录；使用：启动与契约测试；边界：报告首个配置错误。
func (loader *Loader) LoadAll(descs ...*api.ApiDesc) error {
	plans := make(map[string]*plan, len(descs))
	for _, desc := range descs {
		p, err := loader.load(desc)
		if err != nil {
			return err
		}
		if plans[p.desc.ID] != nil {
			return fmt.Errorf("请求目录重复引用接口 %q", p.desc.ID)
		}
		plans[p.desc.ID] = p
	}
	for _, desc := range descs {
		p := plans[desc.ID]
		for _, source := range p.desc.Cache.HitSource {
			writer := plans[source.API]
			if writer == nil || len(writer.after) == 0 || writer.desc.Auth != p.desc.Auth ||
				writer.desc.Method == http.MethodGet || writer.desc.Method == http.MethodHead {
				return fmt.Errorf("接口 %q 的失效来源 %q 无法触发写后失效", p.desc.ID, source.API)
			}
		}
	}
	return nil
}

// load 在锁内复用已存在计划或原子装配新的计划，装配失败时不写入目录。
func (loader *Loader) load(desc *api.ApiDesc) (*plan, error) {
	if desc == nil || strings.TrimSpace(desc.ID) == "" {
		return nil, fmt.Errorf("请求接口缺少 ID")
	}
	loader.mu.Lock()
	defer loader.mu.Unlock()
	if existing := loader.plans[desc.ID]; existing != nil {
		if existing.source != desc {
			return nil, fmt.Errorf("请求接口 ID 重复 %q", desc.ID)
		}
		return existing, nil
	}
	if err := validateDesc(desc); err != nil {
		return nil, fmt.Errorf("加载接口 %q 失败 %w", desc.ID, err)
	}
	p := &plan{source: desc, desc: *desc, authenticated: desc.Auth != api.AuthNone}
	p.desc.Cache.HitSource = slices.Clone(desc.Cache.HitSource)
	switch desc.Cache.Mode {
	case api.HTTPCache:
		p.freshness = requestcache.FreshFor
	case api.ViewSnapshot:
		p.freshness = requestcache.ViewFreshFor
	}
	var err error
	p.before, err = resolveMiddleware(desc.BeforeMiddleware, loader.before)
	if err == nil {
		p.after, err = resolveMiddleware(desc.AfterMiddleware, loader.after)
	}
	if err != nil {
		return nil, fmt.Errorf("加载接口 %q 失败 %w", desc.ID, err)
	}
	if p.desc.ResponseLimit == 0 {
		p.desc.ResponseLimit = 4 << 20
	}
	loader.plans[desc.ID] = p
	return p, nil
}

// resolveMiddleware 依据声明顺序解析注册项，每个阶段只执行同一名称一次。
func resolveMiddleware[M any](ids []api.MiddlewareID, registry map[api.MiddlewareID]M) ([]M, error) {
	var result []M
	seen := make(map[api.MiddlewareID]bool, len(ids))
	for _, id := range ids {
		fn, ok := registry[id]
		if !ok || seen[id] {
			return nil, fmt.Errorf("中间件未注册或重复 %q", id)
		}
		seen[id] = true
		result = append(result, fn)
	}
	return result, nil
}

// validateDesc 检查路由、认证、缓存和传输约束，禁止危险组合进入运行期。
func validateDesc(desc *api.ApiDesc) error {
	if !localPath(desc.URL) {
		return fmt.Errorf("API 路径必须相对于盒子服务器")
	}
	switch desc.Method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return fmt.Errorf("不支持的 HTTP 方法")
	}
	if desc.Auth < api.AuthNone || desc.Auth > api.AuthVerified || desc.Response > api.SSEResponse {
		return fmt.Errorf("认证或响应模式无效")
	}
	switch desc.SessionAction {
	case api.SessionUnchanged:
	case api.SessionLogin:
		if desc.Auth != api.AuthNone || desc.Cache.Mode != api.CacheOff || desc.ShareRequest || desc.Method != http.MethodPost || desc.Response != api.JSONResponse {
			return fmt.Errorf("登录需要公开 JSON POST")
		}
	case api.SessionVerify, api.SessionChangePassword, api.SessionLogout:
		if desc.Auth != api.AuthToken || desc.Cache.Mode != api.CacheOff || desc.ShareRequest || desc.Response != api.JSONResponse {
			return fmt.Errorf("账号维护需要实时凭据和 JSON 响应")
		}
	default:
		return fmt.Errorf("会话动作无效")
	}
	if desc.Timeout < 0 || desc.UploadTimeout < 0 || desc.RequestLimit < 0 || desc.ResponseLimit < 0 {
		return fmt.Errorf("超时或请求大小限制无效")
	}
	if desc.Cache.Mode > api.ViewSnapshot || desc.Cache.TTL < 0 || (desc.Cache.Mode == api.CacheOff) != (desc.Cache.TTL == 0) {
		return fmt.Errorf("缓存模式和有效期不匹配")
	}
	if desc.Cache.Mode != api.CacheOff || desc.ShareRequest || len(desc.Cache.HitSource) > 0 {
		if desc.Response != api.JSONResponse || desc.Method != http.MethodGet || desc.UploadTimeout > 0 {
			return fmt.Errorf("结果缓存和在途共享需要普通 JSON GET")
		}
	}
	if len(desc.Cache.HitSource) > 0 && desc.Cache.Mode == api.CacheOff && !desc.ShareRequest {
		return fmt.Errorf("失效来源需要结果缓存或在途共享")
	}
	if (desc.Cache.Mode != api.CacheOff || desc.ShareRequest) && desc.Auth == api.AuthToken {
		return fmt.Errorf("持有 Token 的账号维护请求需要实时结果")
	}
	if desc.Response != api.JSONResponse && (desc.Method != http.MethodGet && !(desc.Response == api.RangeResponse && desc.Method == http.MethodHead) ||
		len(desc.AfterMiddleware) != 0 || desc.UploadTimeout != 0) {
		return fmt.Errorf("流式读取需要 GET 和对应的响应处理阶段")
	}
	for _, source := range desc.Cache.HitSource {
		if source.API == "" || source.API == desc.ID {
			return fmt.Errorf("缓存失效来源无效")
		}
	}
	return nil
}
