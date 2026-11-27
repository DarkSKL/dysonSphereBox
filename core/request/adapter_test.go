package request_test

// 文件职责：回环验证接口加载、请求装配、缓存与 HTTP 边界。
// 使用：go test ./core/request。
// 边界：仅请求本地临时测试服务器。

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	api "DysonSphereBox/apiDefinition"
	boxAPI "DysonSphereBox/apiDefinition/boxServer"
	"DysonSphereBox/core"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
	_ "DysonSphereBox/helper/forum" // 加载正式论坛中间件。
	"DysonSphereBox/helper/system/account"
)

// TestLoadServerCatalog 使用真实接口目录验证全部策略和失效来源可在启动时解析。
func TestLoadServerCatalog(t *testing.T) {
	if err := account.ServerRequests().Loader.LoadAll(boxAPI.Endpoints()...); err != nil {
		t.Fatal(err)
	}
}

// TestLoaderRejectsInvalidConfiguration 在联网前阻止接口冲突、危险路由与失效误配。
func TestLoaderRejectsInvalidConfiguration(t *testing.T) {
	for _, invalid := range []struct {
		name   string
		change func(*api.ApiDesc)
	}{
		{"external target", func(d *api.ApiDesc) { d.URL = "https://example.com/posts" }},
		{"absolute host reference", func(d *api.ApiDesc) { d.URL = "//example.com/posts" }},
		{"path escape", func(d *api.ApiDesc) { d.URL = "/%2e%2e/posts" }},
		{"auth omitted", func(d *api.ApiDesc) { d.Auth = 0 }},
		{"invalid auth combination", func(d *api.ApiDesc) { d.SessionAction = api.SessionLogout }},
		{"unknown middleware", func(d *api.ApiDesc) { d.BeforeMiddleware = []api.MiddlewareID{"missing"} }},
		{"stream with cache", func(d *api.ApiDesc) {
			d.Response = api.SSEResponse
			d.Cache = api.CachePolicy{Mode: api.HTTPCache, TTL: time.Minute}
		}},
		{"unknown invalidation source", func(d *api.ApiDesc) {
			d.Cache = api.CachePolicy{Mode: api.ViewSnapshot, TTL: time.Minute, HitSource: []api.CacheSource{{API: "missing"}}}
		}},
	} {
		t.Run(invalid.name, func(t *testing.T) {
			desc := api.ApiDesc{ID: "posts", URL: "/posts", Method: http.MethodGet, Auth: api.AuthVerified}
			invalid.change(&desc)
			if err := request.NewLoader().LoadAll(&desc); err == nil {
				t.Fatal("invalid plan was loaded")
			}
		})
	}
	loader := request.NewLoader()
	first := &api.ApiDesc{ID: "posts", URL: "/posts", Method: http.MethodGet, Auth: api.AuthNone}
	duplicate := *first
	if err := loader.LoadAll(first, &duplicate); err == nil {
		t.Fatal("duplicate endpoint ID was loaded")
	}
}

// TestAdapterCachesAfterAuthAndParameterBinding 验证请求前阶段、强刷和真实成功中间件的调用顺序。
func TestAdapterCachesAfterAuthAndParameterBinding(t *testing.T) {
	type value struct {
		Count int64 `json:"count"`
	}
	var network, prepared, accepted, captures, succeeded atomic.Int64
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		count := network.Add(1)
		if req.URL.Path != "/box/posts/23" || req.URL.Query().Get("page") != "1" ||
			req.Header.Get("x-token") != "fixture-token" || (count == 2 && req.Header.Get("Cache-Control") != "no-cache") {
			t.Errorf("unexpected request %s (count %d)", req.URL.RequestURI(), count)
		}
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintf(w, `{"code":0,"data":{"count":%d}}`, count)
	}))
	defer testServer.Close()
	client, err := core.NewServerClient(testServer.URL + "/box")
	if err != nil {
		t.Fatal(err)
	}
	scope, cancel := context.WithCancel(t.Context())
	defer cancel()
	loader := request.NewLoader()
	loader.RegisterBefore("page", func(_ context.Context, input *request.Parameters) error {
		prepared.Add(1)
		if input.Query.Get("page") == "" {
			input.Query.Set("page", "1")
		}
		return nil
	})
	loader.RegisterAfter("observe", func(_ context.Context, result request.Success) []requestcache.Source {
		if result.API != "posts" {
			t.Errorf("unexpected endpoint %q", result.API)
		}
		succeeded.Add(1)
		return nil
	})
	desc := &api.ApiDesc{
		ID: "posts", URL: "/posts/{{ID}}", Method: http.MethodGet, Auth: api.AuthVerified,
		Cache: api.CachePolicy{Mode: api.ViewSnapshot, TTL: time.Minute}, ShareRequest: true,
		BeforeMiddleware: []api.MiddlewareID{"page"}, AfterMiddleware: []api.MiddlewareID{"observe"},
	}
	if err := loader.LoadAll(desc); err != nil {
		t.Fatal(err)
	}
	adapter := request.NewAdapter(loader, func(desc *api.ApiDesc, key string) (request.Session, error) {
		captures.Add(1)
		return request.Session{
			Mode: desc.Auth, Credential: "fixture-token", Transport: client,
			Scope: requestcache.Scope{Context: scope, ID: "first-login"},
			Accept: func(update request.AuthUpdate) error {
				if !update.Success {
					t.Error("unexpected auth update")
				}
				accepted.Add(1)
				return nil
			},
		}, nil
	})
	originalQuery := make(url.Values)
	input := request.Input{PathParams: struct{ ID uint }{23}, Query: originalQuery, SessionKey: "first-login"}
	for i, want := range []int64{1, 1, 2, 2} {
		input.Force = i == 2
		got, err := request.Do[value](t.Context(), adapter, desc, input)
		if err != nil || got.Count != want {
			t.Fatalf("read %d = %+v, %v, want count %d", i, got, err, want)
		}
	}
	if len(originalQuery) != 0 || captures.Load() != 4 || prepared.Load() != 4 || accepted.Load() != 2 || succeeded.Load() != 2 {
		t.Fatal("unexpected cache, guard, or middleware execution")
	}
	cancel()
	_, err = request.Do[value](t.Context(), adapter, desc, input)
	if detail := request.Details(err); detail == nil || detail.Kind != request.Stale || network.Load() != 2 {
		t.Fatalf("old session received data or made a request: %v", err)
	}
}

// TestAdapterDoesNotFollowRedirects 验证认证请求只能将凭据发送给原始传输目标。
func TestAdapterDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		redirected.Add(1)
	}))
	defer other.Close()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, other.URL, http.StatusTemporaryRedirect)
	}))
	defer testServer.Close()
	client, err := core.NewServerClient(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	adapter := request.NewAdapter(request.NewLoader(), func(desc *api.ApiDesc, _ string) (request.Session, error) {
		return request.Session{Mode: desc.Auth, Credential: "fixture-token", Transport: client,
			Scope: requestcache.Scope{Context: t.Context()}}, nil
	})
	desc := &api.ApiDesc{ID: "redirect", URL: "/redirect", Method: http.MethodGet, Auth: api.AuthVerified}
	_, err = request.Do[struct{}](t.Context(), adapter, desc, request.Input{})
	if detail := request.Details(err); detail == nil || detail.Status != http.StatusTemporaryRedirect || redirected.Load() != 0 {
		t.Fatalf("redirect was followed or status lost: %v", err)
	}
}
