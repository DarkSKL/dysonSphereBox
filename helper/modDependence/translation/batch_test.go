package translation

// 文件职责：验证翻译契约、认证桥接及会话失效保护。
// 使用：go test ./helper/modDependence/translation；所有 HTTP 和数据库均为临时测试资源。
// 边界：不调用真实服务器或计费供应商，不使用用户配置和凭据。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	translationAPI "DysonSphereBox/apiDefinition/modTranslation"
	"DysonSphereBox/core"
	accountHelper "DysonSphereBox/helper/system/account"
	systemModel "DysonSphereBox/model/system"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// translationFixture 职责：创建已验证测试账号；使用：HTTP 桥接场景；边界：只访问回环测试服务器，清理后恢复全局基础设施。
func translationFixture(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/prefix/user/getUserInfo" {
			_, _ = w.Write([]byte(`{"code":0,"data":{"userInfo":{"ID":7}},"msg":"ok"}`))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&systemModel.Config{}); err != nil {
		t.Fatal(err)
	}
	client, err := core.NewHTTPClient(server.URL + "/prefix/")
	if err != nil {
		t.Fatal(err)
	}
	previous := core.GLOBAL
	core.GLOBAL = &core.Global{DB: db, Configure: core.InitConfigure(), ServerClient: client}
	t.Cleanup(func() {
		_ = accountHelper.ClearSession()
		core.GLOBAL = previous
		sqlDB, _ := db.DB()
		_ = sqlDB.Close()
	})
	if err := accountHelper.SaveToken("test-credential"); err != nil {
		t.Fatal(err)
	}
	if _, err := accountHelper.CurrentUser(context.Background()); err != nil {
		t.Fatal(err)
	}
	generation, subject, key, _, _, _, ok := accountHelper.VerifiedSession()
	if !ok {
		t.Fatal("test session not verified")
	}
	return SessionKey(generation, subject, key)
}

// TestEncodeRequest 职责：验证批次硬上限和字节保留；使用：本地契约测试；边界：不请求网络。
func TestEncodeRequest(t *testing.T) {
	for _, texts := range [][]string{nil, make([]string, 21), {strings.Repeat("<", translationAPI.MaxRequestBytes)}, {string([]byte{0xff})}} {
		if _, err := EncodeRequest(texts); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
	texts := []string{" a", "a", "a\n", "😀"}
	raw, err := EncodeRequest(texts)
	var decoded translationAPI.BatchRequest
	if err != nil || json.Unmarshal(raw, &decoded) != nil || !reflect.DeepEqual(decoded.Texts, texts) {
		t.Fatal("source bytes or order changed")
	}
}

// TestDecodeBatch 职责：验证部分失败原文回退和等长约束；使用：异常服务器契约场景；边界：绝不展示错误项返回的替代正文。
func TestDecodeBatch(t *testing.T) {
	texts := []string{"first", "second", "third"}
	result, err := DecodeBatch(json.RawMessage(`{"items":[{"status":"ready","reason":"","data":"第一"},{"status":"error","reason":"translationDisabled","data":"wrong"},{"status":"ready","reason":"","data":"   "}]}`), texts)
	if err != nil || len(result.Items) != 3 || result.Items[0].Data != "第一" || result.Items[1].Data != "second" || result.Items[2].Data != "third" || result.Items[2].Status != "error" {
		t.Fatalf("fallback/order mismatch: %v", err)
	}
	for _, raw := range []string{`{"items":[]}`, `null`, `{`, `{"items":[null]}`} {
		if _, err := DecodeBatch(json.RawMessage(raw), texts); err == nil {
			t.Fatal("malformed batch accepted")
		}
	}
	if _, err := DecodeBatch(json.RawMessage(`{"items":[null,null,null]}`), texts); err == nil {
		t.Fatal("null item accepted as a failed translation")
	}
}

// TestTranslateBatchBridge 职责：验证路由前缀、认证、原文顺序和续期；使用：本地假服务器；边界：返回 DTO 不包含 token。
func TestTranslateBatchBridge(t *testing.T) {
	key := translationFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/prefix/translation/translateBatch" || r.Header.Get("x-token") != "test-credential" {
			t.Error("unexpected translation request contract")
		}
		var request translationAPI.BatchRequest
		if json.NewDecoder(r.Body).Decode(&request) != nil || !reflect.DeepEqual(request.Texts, []string{" b", "a", " b"}) {
			t.Error("source bytes or request order changed")
		}
		w.Header().Set("new-token", "renewed-test-credential")
		_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"status":"ready","reason":"","data":"乙"},{"status":"error","reason":"providerFailed","data":"wrong"},{"status":"ready","reason":"","data":"乙"}]},"msg":"ok"}`))
	})
	result, err := TranslateBatch(context.Background(), []string{" b", "a", " b"}, key)
	if err != nil || len(result.Items) != 3 || result.Items[0].Data != "乙" || result.Items[1].Data != "a" || result.Items[2].Data != "乙" {
		t.Fatalf("bridge result mismatch: %v", err)
	}
	if accountHelper.Token() != "renewed-test-credential" {
		t.Fatal("renewal not retained in Go")
	}
}

// TestTranslateBatchFailures 职责：验证鉴权、协议错误与禁用重定向；使用：桥接错误场景；边界：失败均无可展示译文且不自动重试。
func TestTranslateBatchFailures(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "password", "count", "missingCode", "redirect"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			key := translationFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				switch scenario {
				case "unauthorized":
					w.WriteHeader(http.StatusUnauthorized)
				case "password":
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"code":10001,"data":{"needChangePassword":true},"msg":"password"}`))
				case "count":
					_, _ = w.Write([]byte(`{"code":0,"data":{"items":[]},"msg":"ok"}`))
				case "missingCode":
					_, _ = w.Write([]byte(`{"data":{"items":[{"status":"ready","reason":"","data":"wrong"}]}}`))
				case "redirect":
					http.Redirect(w, r, "/redirect-target", http.StatusTemporaryRedirect)
				}
			})
			result, err := TranslateBatch(context.Background(), []string{"a"}, key)
			if err == nil || len(result.Items) != 0 || calls != 1 {
				t.Fatalf("expected single failed request, calls=%d err=%v", calls, err)
			}
			if scenario == "unauthorized" && accountHelper.Token() != "" {
				t.Fatal("401 did not clear session")
			}
			if scenario == "password" && !accountHelper.SessionState().NeedChangePassword {
				t.Fatal("409 did not update password requirement")
			}
		})
	}
}

// TestTranslateBatchStaleContext 职责：验证切号、注销和切服丢弃旧结果；使用：阻塞假服务器响应后改变上下文；边界：旧凭据不能覆盖新会话。
func TestTranslateBatchStaleContext(t *testing.T) {
	for _, scenario := range []string{"account", "logout", "server"} {
		t.Run(scenario, func(t *testing.T) {
			started, release := make(chan struct{}), make(chan struct{})
			key := translationFixture(t, func(w http.ResponseWriter, r *http.Request) {
				close(started)
				<-release
				w.Header().Set("new-token", "obsolete-test-credential")
				_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"status":"ready","reason":"","data":"obsolete"}]},"msg":"ok"}`))
			})
			defer close(release)
			done := make(chan error, 1)
			go func() {
				_, err := TranslateBatch(context.Background(), []string{"a"}, key)
				done <- err
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("request did not start")
			}
			switch scenario {
			case "account":
				if err := accountHelper.SaveToken("next-test-credential"); err != nil {
					t.Fatal(err)
				}
			case "logout":
				if err := accountHelper.ClearSession(); err != nil {
					t.Fatal(err)
				}
			case "server":
				client, _ := core.NewHTTPClient("http://127.0.0.1:1/unused/")
				if err := accountHelper.SwapServer(client); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("stale translation accepted")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("session change did not cancel request")
			}
			if scenario == "account" && accountHelper.Token() != "next-test-credential" {
				t.Fatal("old response changed new credentials")
			}
		})
	}
}

// TestTranslateBatchRejectsWrongScope 职责：验证排队请求不能改绑新账号；使用：发送前作用域检查；边界：无网络调用。
func TestTranslateBatchRejectsWrongScope(t *testing.T) {
	calls := 0
	translationFixture(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
	if _, err := TranslateBatch(context.Background(), []string{"a"}, "obsolete-scope"); err == nil || calls != 0 {
		t.Fatal("wrong-scope request reached server")
	}
}
