package feedback_test

// 文件职责：验证桌面预取经真实会话守卫发出的请求及可交给页面的回执。
// 使用：修改反馈服务、绑定 DTO 或兼容逻辑后运行。
// 边界：账号配置仅存在于内存 SQLite，网络请求仅发给回环测试服务器。

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	api "DysonSphereBox/apiDefinition/feedback"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/system/account"
	systemModel "DysonSphereBox/model/system"
	"DysonSphereBox/service/feedback"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// TestPreviewsTransport 职责：验证会话保护、批量出站白名单及旧端降级。使用：从公开服务方法调用。边界：只模拟服务端回执。
func TestPreviewsTransport(t *testing.T) {
	const exactID = "18446744073709551615"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == systemAPI.APIURLLogin {
			io.WriteString(w, `{"code":0,"msg":"","data":{"user":{"ID":42},"token":"test-session"}}`)
			return
		}
		if r.Header.Get(systemAPI.AccountTokenHeader) != "test-session" || r.Header.Get("Cache-Control") != "no-store" {
			t.Error("反馈请求缺少测试会话凭据或缓存约束")
		}
		switch r.URL.Path {
		case api.PathPreviews:
			if r.Method != http.MethodPost || r.URL.RawQuery != "" || r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("unexpected batch request: %s %s %s", r.Method, r.URL, r.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			switch string(body) {
			case `{"starIds":["18446744073709551615","2","3"]}`:
				io.WriteString(w, `{"code":0,"msg":"","data":{"items":[{"starId":"18446744073709551615","preview":"首行","displayName":"匿名"},{"starId":"3","preview":"另一行"}]}}`)
			case `{"starIds":["4"]}`:
				io.WriteString(w, `{"code":0,"msg":"","data":{"items":[]}}`)
			case `{"starIds":["5"]}`:
				http.NotFound(w, r)
			case `{"starIds":["6"]}`:
				if err := account.ClearSession(); err != nil {
					t.Error(err)
				}
				io.WriteString(w, `{"code":0,"msg":"","data":{"items":[{"starId":"6","preview":"过期回执"}]}}`)
			default:
				t.Errorf("unexpected batch body: %s", body)
				w.WriteHeader(http.StatusBadRequest)
			}
		case api.PathPreview:
			if r.Method != http.MethodGet || r.URL.RawQuery != "starId=5" {
				t.Errorf("unexpected single request: %s %s", r.Method, r.URL)
			}
			io.WriteString(w, `{"code":0,"msg":"","data":{"starId":"5","preview":"旧版首行"}}`)
		default:
			t.Errorf("unexpected route %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&systemModel.Config{}); err != nil {
		t.Fatal(err)
	}
	client, err := core.NewHTTPClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	previous := core.GLOBAL
	core.GLOBAL = &core.Global{DB: db, Configure: core.InitConfigure(), ServerClient: client}
	defer func() {
		account.ClearSession()
		core.GLOBAL = previous
	}()
	service := feedback.FeedbackService{}
	input := api.PreviewsRequest{StarIDs: []string{exactID, "2", exactID, "3"}}
	_, err = service.Previews(input)
	requireReason(t, err, "unauthorized")
	login, err := account.Login(t.Context(), systemAPI.AccountLoginRequest{})
	if err != nil {
		t.Fatal(err)
	}
	input.SessionKey = strings.Join([]string{login.Session.ServerKey, login.Session.SubjectID, login.Session.Generation}, "\n")
	batch, err := service.Previews(input)
	if err != nil || len(batch.Items) != 2 || batch.Items[0] != (api.PreviewResult{StarID: exactID, Preview: "首行", DisplayName: "匿名"}) ||
		batch.Items[1] != (api.PreviewResult{StarID: "3", Preview: "另一行"}) {
		t.Fatalf("unexpected batch: %+v, %v", batch, err)
	}
	empty, err := service.Previews(api.PreviewsRequest{StarIDs: []string{"4"}})
	if err != nil || empty.Items == nil || len(empty.Items) != 0 {
		t.Fatalf("unexpected empty result: %+v, %v", empty, err)
	}
	_, err = service.Previews(api.PreviewsRequest{StarIDs: []string{"5"}, SessionKey: input.SessionKey})
	requireReason(t, err, "unsupported")
	single, err := service.Preview(api.PreviewRequest{StarID: "5", SessionKey: input.SessionKey})
	if err != nil || single != (api.PreviewResult{StarID: "5", Preview: "旧版首行"}) {
		t.Fatalf("unexpected legacy preview: %+v, %v", single, err)
	}
	_, err = service.Previews(api.PreviewsRequest{StarIDs: input.StarIDs, SessionKey: "previous-session"})
	requireReason(t, err, "stale")
	expired, err := service.Previews(api.PreviewsRequest{StarIDs: []string{"6"}, SessionKey: input.SessionKey})
	requireReason(t, err, "stale")
	if len(expired.Items) != 0 {
		t.Fatal("账号切换后仍返回预览")
	}
}

// requireReason 职责：核对 Wails 拒绝原因。使用：桥接兼容测试。边界：仅检查稳定的机读字段。
func requireReason(t *testing.T, err error, want string) {
	t.Helper()
	var failure struct{ Reason string }
	if err == nil || json.Unmarshal([]byte(err.Error()), &failure) != nil || failure.Reason != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
}
