package account_test

// 文件职责：通过回环接口验证真实客户端会话和通知传输。
// 使用：go test ./helper/system/account。
// 边界：临时服务器与内存 SQLite 仅保存测试凭据。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	api "DysonSphereBox/apiDefinition"
	translationAPI "DysonSphereBox/apiDefinition/modTranslation"
	systemAPI "DysonSphereBox/apiDefinition/system"
	systemConstant "DysonSphereBox/constant/system"
	"DysonSphereBox/core"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/modDependence/translation"
	"DysonSphereBox/helper/system/account"
	"DysonSphereBox/helper/system/announcement"
	"DysonSphereBox/helper/system/notification"
	serverHelper "DysonSphereBox/helper/system/server"
	systemModel "DysonSphereBox/model/system"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// 夹具接口身份在重复运行中保持稳定，使用与生产一致的全局加载器。
var (
	slowAPI     = &api.ApiDesc{ID: "test.slow", URL: "/slow", Method: http.MethodGet, Auth: api.AuthVerified}
	rotateAPI   = &api.ApiDesc{ID: "test.rotate", URL: "/rotate", Method: http.MethodGet, Auth: api.AuthVerified}
	passwordAPI = &api.ApiDesc{ID: "test.password", URL: "/password", Method: http.MethodGet, Auth: api.AuthVerified}
)

// TestServerAuthKeepsRenewedSession 验证恢复会话、公开探测、静默续期和旧凭据的迟到拒绝。
func TestServerAuthKeepsRenewedSession(t *testing.T) {
	oldStarted, releaseOld := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get(systemAPI.AccountTokenHeader)
		switch r.URL.Path {
		case systemAPI.APIURLLogin:
			if token != "" {
				t.Error("login attached an old credential")
			}
			w.Header().Set(systemAPI.AccountNewTokenHeader, "discard-login")
			fmt.Fprint(w, `{"code":7,"msg":"login rejected"}`)
		case systemAPI.APIURLCaptcha:
			if token != "" {
				t.Error("public request attached a credential")
			}
			w.Header().Set(systemAPI.AccountNewTokenHeader, "discard-public")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"code":7,"msg":"service available"}`)
		case systemAPI.APIURLGetUserInfo:
			if token != "stored-token" {
				t.Error("saved credential was not used")
			}
			fmt.Fprint(w, `{"code":0,"data":{"userInfo":{"ID":42}}}`)
		case "/slow":
			close(oldStarted)
			select {
			case <-releaseOld:
				w.WriteHeader(http.StatusUnauthorized)
			case <-r.Context().Done():
			}
		case "/rotate":
			if token != "stored-token" {
				t.Error("original credential missing before renewal")
			}
			w.Header().Set(systemAPI.AccountNewTokenHeader, "renewed-token")
			fmt.Fprint(w, `{"code":0,"data":{}}`)
		case "/password":
			if token != "renewed-token" {
				t.Error("renewed credential was not used")
			}
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"code":10001,"msg":"password required","data":{"needChangePassword":true}}`)
		case systemAPI.APIURLLogout:
			if token != "renewed-token" || account.Token() != "" {
				t.Error("logout did not send only the original credential after local clear")
			}
			fmt.Fprint(w, `{"code":0,"data":{}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	installAccountGlobals(t, server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if _, err := account.Login(ctx, systemAPI.AccountLoginRequest{}); err == nil || account.Token() != "" {
		t.Fatalf("rejected login accepted a renewal credential: %v", err)
	}
	if err := account.SaveToken("stored-token"); err != nil {
		t.Fatal(err)
	}
	if result := serverHelper.Probe(ctx, server.URL); !result.OK || account.Token() != "stored-token" || account.SessionState().Verified {
		t.Fatalf("public candidate check altered the saved session: %+v", result)
	}
	adapter := account.ServerRequests()
	fetch := func(desc *api.ApiDesc) error {
		_, err := request.Do[struct{}](ctx, adapter, desc, request.Input{})
		return err
	}
	if err := fetch(slowAPI); !requestKind(err, request.AuthRequired) {
		t.Fatalf("unverified stored token was accepted as a verified session: %v", err)
	}
	snapshot, err := account.SyncSession(ctx, false)
	if err != nil || !snapshot.Session.Verified || snapshot.Session.SubjectID != "42" {
		t.Fatalf("saved session was not verified: %+v, %v", snapshot, err)
	}
	original, err := adapter.Capture(slowAPI, "")
	if err != nil {
		t.Fatal(err)
	}
	awaitOld := make(chan error, 1)
	go func() { awaitOld <- fetch(slowAPI) }()
	waitSignal(t, oldStarted)
	if err := fetch(rotateAPI); err != nil || account.Token() != "renewed-token" {
		t.Fatalf("renewal failed: %v", err)
	}
	close(releaseOld)
	select {
	case err := <-awaitOld:
		if !requestKind(err, request.Stale) || account.Token() != "renewed-token" || !account.SessionState().Verified {
			t.Fatalf("old 401 changed the renewed session: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("old request did not finish")
	}
	if err := fetch(passwordAPI); !requestKind(err, request.PasswordRequired) || !account.SessionState().NeedChangePassword {
		t.Fatalf("password-required response was not accepted: %v", err)
	}
	waitSignal(t, original.Scope.Context.Done())
	if err := account.Logout(ctx); err != nil || account.SessionState().Authenticated {
		t.Fatalf("logout after forced password change failed: %v", err)
	}
	if err := account.SaveToken("another-token"); err != nil {
		t.Fatal(err)
	}
	canceled, cancelLogout := context.WithCancel(ctx)
	cancelLogout()
	if err := account.Logout(canceled); !errors.Is(err, context.Canceled) || account.SessionState().Authenticated {
		t.Fatalf("canceled logout retained the local session: %v", err)
	}
}

// TestNotificationStreamUsesAndCancelsVerifiedScope 验证 SSE 握手续期、事件帧和切服时的取消。
func TestNotificationStreamUsesAndCancelsVerifiedScope(t *testing.T) {
	streamClosed := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case systemAPI.APIURLLogin:
			if r.Header.Get(systemAPI.AccountTokenHeader) != "" {
				t.Error("login attached an old credential")
			}
			fmt.Fprint(w, `{"code":0,"data":{"user":{"ID":8},"token":"login-token"}}`)
		case systemAPI.APIURLNotificationStream:
			if r.Header.Get("Accept") != "text/event-stream" || r.Header.Get(systemAPI.AccountTokenHeader) != "login-token" {
				t.Error("stream request did not use the verified transport")
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set(systemAPI.AccountNewTokenHeader, "stream-token")
			fmt.Fprint(w, ": connected\n\nevent: "+systemConstant.NotificationMessage+"\ndata: {}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(streamClosed)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	installAccountGlobals(t, server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if _, err := account.Login(ctx, systemAPI.AccountLoginRequest{}); err != nil {
		t.Fatal(err)
	}
	session, err := notification.Capture("")
	if err != nil {
		t.Fatal(err)
	}
	events, done := make(chan string, 4), make(chan struct{})
	go func() {
		defer close(done)
		session.Stream(ctx, func(event string) { events <- event })
	}()
	for _, expected := range []string{systemConstant.NotificationConnected, systemConstant.NotificationMessage} {
		select {
		case actual := <-events:
			if actual != expected {
				t.Fatalf("SSE event = %q, want %q", actual, expected)
			}
		case <-ctx.Done():
			t.Fatal("SSE event did not arrive")
		}
	}
	if account.Token() != "stream-token" {
		t.Fatal("SSE handshake did not commit renewal")
	}
	// 只改变地址前缀即可切换全局目标，无需另起真实服务。
	next, err := core.NewServerClient(server.URL + "/next")
	if err != nil {
		t.Fatal(err)
	}
	if err := account.SwapServer(next); err != nil {
		t.Fatal(err)
	}
	waitSignal(t, session.Context.Done())
	waitSignal(t, streamClosed)
	waitSignal(t, done)
	if account.Token() != "" || core.GLOBAL.ServerClient != next {
		t.Fatal("switch retained old account or transport")
	}
}

// TestLiveServerEndpointsUseTheirDescriptors 验证公告、收件箱和翻译在没有缓存头时仍实时请求。
func TestLiveServerEndpointsUseTheirDescriptors(t *testing.T) {
	var notices, inbox, batches atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == systemAPI.APIURLLogin {
			fmt.Fprint(w, `{"code":0,"data":{"user":{"ID":8},"token":"live-token"}}`)
			return
		}
		if r.Header.Get(systemAPI.AccountTokenHeader) != "live-token" || r.Header.Get("Cache-Control") != "no-store" {
			t.Error("live endpoint missing auth or cache policy")
		}
		switch r.URL.Path {
		case systemAPI.APIURLClientNoticeGetLatestUnread:
			notices.Add(1)
			fmt.Fprint(w, `{"code":0,"msg":"","data":{"subjectId":"8","serverTime":"2026-11-27T00:00:00Z","item":null}}`)
		case systemAPI.APIURLNotificationUnread:
			fmt.Fprintf(w, `{"code":0,"data":{"total":%d}}`, inbox.Add(1))
		case translationAPI.APIURLTranslateBatch:
			batches.Add(1)
			var input translationAPI.BatchRequest
			if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&input) != nil || len(input.Texts) != 1 || input.Texts[0] != "Hello" {
				t.Error("wrong translation payload")
			}
			fmt.Fprint(w, `{"code":0,"data":{"items":[{"status":"ready","data":"你好"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	installAccountGlobals(t, server.URL)
	if _, err := account.Login(t.Context(), systemAPI.AccountLoginRequest{}); err != nil {
		t.Fatal(err)
	}
	access, err := notification.Capture("")
	if err != nil {
		t.Fatal(err)
	}
	id, err := announcement.CaptureSessionID()
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(1); i <= 2; i++ {
		latest, err := announcement.GetLatest(t.Context(), id)
		if err != nil || latest.SubjectID != "8" || latest.Item != nil {
			t.Fatalf("notice response = %+v, %v", latest, err)
		}
		unread, err := notification.Request[systemAPI.NotificationUnread](t.Context(), access.Key, systemAPI.NotificationUnreadAPI, nil, nil)
		if err != nil || unread.Total != i {
			t.Fatalf("unread response = %+v, %v", unread, err)
		}
		batch, err := translation.TranslateBatch(t.Context(), []string{"Hello"}, access.Key)
		if err != nil || len(batch.Items) != 1 || batch.Items[0].Data != "你好" {
			t.Fatalf("translation response = %+v, %v", batch, err)
		}
	}
	if notices.Load() != 2 || inbox.Load() != 2 || batches.Load() != 2 {
		t.Fatal("a live response was reused")
	}
}

// installAccountGlobals 创建隔离配置库，并在用例结束后恢复全局传输。
func installAccountGlobals(t *testing.T, address string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	conn.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&systemModel.Config{}, &systemModel.Log{}); err != nil {
		t.Fatal(err)
	}
	client, err := core.NewServerClient(address)
	if err != nil {
		t.Fatal(err)
	}
	previous := core.GLOBAL
	core.GLOBAL = &core.Global{DB: db, Configure: core.InitConfigure(), ServerClient: client}
	t.Cleanup(func() {
		if err := account.ClearSession(); err != nil {
			t.Error(err)
		}
		core.GLOBAL = previous
	})
}

// requestKind 用标准请求错误判定具体阶段。
func requestKind(err error, kind request.ErrorKind) bool {
	detail := request.Details(err)
	return detail != nil && detail.Kind == kind
}

// waitSignal 给回环并发断言设置有限等待。
func waitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("expected signal did not arrive")
	}
}
