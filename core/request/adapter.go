package request

// 文件职责：将本次参数绑定到执行计划，运行认证、缓存及 HTTP 适配。
// 使用：盒子服务器 JSON 请求和流式读取共用。
// 边界：通过捕获的全局客户端发送，固定目标，不执行自动写重试。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core/requestcache"
	"DysonSphereBox/utils"
)

// call 保存本次解析后的路由、参数和会话，供缓存与传输阶段共享。
type call struct {
	plan    *plan
	input   Input
	path    string
	query   string
	session Session
}

// Do 职责：执行带类型化结果的 JSON 请求；使用：各业务 helper；边界：缓存和扩展中间件由 ApiDesc 策略决定。
func Do[T any](ctx context.Context, adapter *Adapter, desc *api.ApiDesc, input Input) (result T, err error) {
	c, err := adapter.bind(ctx, desc, input, false)
	if err != nil {
		return result, err
	}
	if c.plan.freshness == nil && !c.plan.desc.ShareRequest {
		result, _, err = fetchJSON[T](ctx, adapter, c)
		return result, err
	}
	method := c.cacheMethod()
	load := func(work context.Context) (requestcache.Result, error) {
		value, metadata, err := fetchJSON[T](work, adapter, c)
		if err != nil {
			return requestcache.Result{}, err
		}
		data := requestcache.Result{Data: metadata.data}
		if c.plan.freshness != nil {
			ttl := c.plan.freshness(metadata.header, time.Now(), method.CacheFor)
			if ttl > 0 {
				if snapshot, err := json.Marshal(value); err == nil {
					data.Data, data.FreshFor = snapshot, ttl
				}
			}
		}
		return data, nil
	}
	// 已失效的并发读取只允许在原会话重试一次。
	for range 2 {
		raw, err := adapter.cache.Read(ctx, c.session.Scope, method, c.input.Force, load)
		if errors.Is(err, requestcache.ErrInvalidated) {
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if interrupted := c.check(ctx); interrupted != nil {
					return result, interrupted
				}
			}
			return result, err
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &result); err != nil {
				var zero T
				return zero, &Error{Kind: InvalidData}
			}
		}
		if err := c.check(ctx); err != nil {
			var zero T
			return zero, err
		}
		return result, nil
	}
	return result, requestcache.ErrInvalidated
}

// bind 解析接口计划和本次已归一化的请求参数，再固定将用于交付结果的会话。
func (adapter *Adapter) bind(ctx context.Context, desc *api.ApiDesc, input Input, streaming bool) (call, error) {
	p, err := adapter.Loader.load(desc)
	if err != nil {
		return call{}, err
	}
	if (p.desc.Response != api.JSONResponse) != streaming {
		return call{}, &Error{Kind: InvalidInput}
	}
	if err := ctx.Err(); err != nil {
		return call{}, err
	}
	if len(p.before) > 0 {
		query := make(url.Values, len(input.Query))
		for key, values := range input.Query {
			query[key] = slices.Clone(values)
		}
		params := Parameters{PathParams: input.PathParams, Query: query, Payload: input.Payload, Target: input.Target}
		for _, before := range p.before {
			if err := before(ctx, &params); err != nil {
				return call{}, err
			}
		}
		input.PathParams, input.Query, input.Payload, input.Target = params.PathParams, params.Query, params.Payload, params.Target
	}
	if (input.Body != nil && (input.Payload != nil || input.ContentType == "")) ||
		((p.desc.Method == http.MethodGet || p.desc.Method == http.MethodHead) && (input.Body != nil || input.Payload != nil)) ||
		(p.desc.Response != api.RangeResponse && (input.Range != "" || input.IfRange != "")) {
		return call{}, &Error{Kind: InvalidInput}
	}
	path, err := utils.ResolveAPIPath(p.desc.URL, input.PathParams)
	if err != nil || !localPath(path) {
		return call{}, &Error{Kind: BuildFailed}
	}
	var session Session
	if input.Session == nil {
		session, err = adapter.capture(&p.desc, input.SessionKey)
		if err != nil {
			return call{}, err
		}
	} else {
		session = *input.Session
	}
	if session.Mode != p.desc.Auth || session.Action != p.desc.SessionAction ||
		(input.SessionKey != "" && input.SessionKey != session.Scope.ID) {
		return call{}, &Error{Kind: Stale}
	}
	if session.Transport == nil || session.Scope.Context == nil {
		return call{}, &Error{Kind: Unavailable}
	}
	if p.authenticated && session.Credential == "" {
		return call{}, &Error{Kind: AuthRequired}
	}
	c := call{plan: p, input: input, path: path, query: input.Query.Encode(), session: session}
	return c, c.check(ctx)
}

// localPath 约束路由留在配置的服务器及其路径前缀内。
func localPath(path string) bool {
	tail, found := strings.CutPrefix(path, "/")
	if !found || strings.HasPrefix(tail, "/") || strings.ContainsAny(path, "\\?#\x00\r\n") {
		return false
	}
	parsed, err := url.Parse(tail)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.Opaque != "" {
		return false
	}
	for encoded := range strings.SplitSeq(parsed.EscapedPath(), "/") {
		segment, err := url.PathUnescape(encoded)
		if err != nil || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\\x00\r\n") {
			return false
		}
	}
	return true
}

// check 优先拒绝旧会话，在当前会话下保留调用取消和超时的标准错误。
func (c call) check(ctx context.Context) error {
	if c.session.Scope.Context.Err() != nil {
		return &Error{Kind: Stale}
	}
	if c.session.Current != nil {
		if err := c.session.Current(); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// cacheMethod 把接口级失效规则和本次目标映射为通用缓存键。
func (c call) cacheMethod() requestcache.Method {
	policy := c.plan.desc.Cache
	method := requestcache.Method{
		Name: c.plan.desc.ID, Parameters: c.path + "?" + c.query, CacheFor: policy.TTL,
		ShareRequest: c.plan.desc.ShareRequest,
	}
	for _, source := range policy.HitSource {
		target := ""
		if source.MatchTarget {
			target = c.input.Target
		}
		method.HitSource = append(method.HitSource, requestcache.Source{Method: source.API, Target: target})
	}
	return method
}

// networkContext 将调用与会话生命周期合并；文件流和流式上传使用描述自己的时限。
func (c call) networkContext(ctx context.Context) (context.Context, func()) {
	timeout := c.plan.desc.Timeout
	if c.input.Body != nil && c.plan.desc.UploadTimeout > 0 {
		timeout = c.plan.desc.UploadTimeout
	}
	var work context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		work, cancel = context.WithTimeout(ctx, timeout)
	} else {
		work, cancel = context.WithCancel(ctx)
	}
	stop := context.AfterFunc(c.session.Scope.Context, cancel)
	return work, func() {
		stop()
		cancel()
	}
}

// send 构造并发送一次固定目标请求，调用者负责读取并关闭响应体。
func (c call) send(ctx context.Context) (*http.Response, error) {
	if err := c.check(ctx); err != nil {
		return nil, err
	}
	body, contentType := c.input.Body, c.input.ContentType
	if c.input.Payload != nil {
		data, err := json.Marshal(c.input.Payload)
		if err != nil || (c.plan.desc.RequestLimit > 0 && len(data) > c.plan.desc.RequestLimit) {
			return nil, &Error{Kind: InvalidInput}
		}
		body, contentType = bytes.NewReader(data), "application/json"
	}
	req, err := c.session.Transport.NewRequest(ctx, c.plan.desc.Method, strings.TrimPrefix(c.path, "/"), body)
	if err != nil {
		return nil, &Error{Kind: BuildFailed}
	}
	req.URL.RawQuery = c.query
	switch c.plan.desc.Response {
	case api.JSONResponse:
		req.Header.Set("Accept", "application/json")
	case api.SSEResponse:
		req.Header.Set("Accept", "text/event-stream")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.input.Force {
		req.Header.Set("Cache-Control", "no-cache")
	} else if c.plan.freshness == nil {
		req.Header.Set("Cache-Control", "no-store")
	}
	if c.plan.authenticated {
		req.Header.Set(systemAPI.AccountTokenHeader, c.session.Credential)
	}
	if c.input.Range != "" {
		req.Header.Set("Range", c.input.Range)
	}
	if c.input.IfRange != "" {
		req.Header.Set("If-Range", c.input.IfRange)
	}
	if c.plan.desc.Response != api.JSONResponse || (c.input.Body != nil && c.plan.desc.UploadTimeout > 0) {
		resp, err := c.session.Transport.DoStreamNoRedirect(req)
		return c.checkTransport(ctx, resp, err)
	}
	resp, err := c.session.Transport.DoNoRedirect(req)
	return c.checkTransport(ctx, resp, err)
}

// checkTransport 在返回底层响应前区分通用传输错误与已经变化的会话。
func (c call) checkTransport(ctx context.Context, resp *http.Response, err error) (*http.Response, error) {
	if err == nil && resp != nil && resp.Body != nil {
		if stale := c.check(ctx); stale == nil {
			return resp, nil
		} else {
			_ = resp.Body.Close()
			return nil, stale
		}
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if interrupted := c.check(ctx); interrupted != nil {
		return nil, interrupted
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, context.DeadlineExceeded
	}
	return nil, &Error{Kind: TransportFailed}
}
