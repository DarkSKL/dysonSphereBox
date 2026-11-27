package forum_test

// 文件职责：通过论坛的公开 Go 服务入口验证查询封装与现有 HTTP 契约。
// 使用：运行 go test ./service/forum；边界：使用回环服务器和内存 SQLite。

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	api "DysonSphereBox/apiDefinition/forum"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/system/account"
	systemModel "DysonSphereBox/model/system"
	"DysonSphereBox/service/forum"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestForumQueriesRespectPolicyAndPublication(t *testing.T) {
	var categoryReads atomic.Int32
	var detailReads atomic.Int32
	var publishedPost atomic.Uint32
	var postReadsMu sync.Mutex
	postReads := make(map[string]int)
	postPath := strings.ReplaceAll(api.PathPostDetail, "{{ID}}", "42")
	repliesPath := strings.ReplaceAll(api.PathPostReplies, "{{ID}}", "42")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == systemAPI.APIURLLogin {
			io.WriteString(w, `{"code":0,"msg":"","data":{"user":{"ID":42},"token":"test-forum-session"}}`)
			return
		}
		if r.Header.Get(systemAPI.AccountTokenHeader) != "test-forum-session" || r.URL.Query().Has("force") {
			t.Error("请求丢失凭据或向服务器发送了客户端刷新参数")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == api.PathCategories:
			categoryReads.Add(1)
			w.Header().Set("Cache-Control", "no-store")
			io.WriteString(w, `{"code":0,"msg":"","data":[{"id":1,"name":"攻略"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == api.PathPosts:
			category := r.URL.Query().Get("categoryId")
			postReadsMu.Lock()
			postReads[category]++
			count := postReads[category]
			postReadsMu.Unlock()
			w.Header().Set("Cache-Control", "no-store")
			if category == "4" {
				w.Header().Add("Cache-Control", "no-cache")
			}
			if category == "3" && count == 1 {
				io.WriteString(w, `{"code":0,"msg":"","data":{"list":false}}`)
				return
			}
			fmt.Fprintf(w, `{"code":0,"msg":"","data":{"page":1,"pageSize":20,"total":1,"list":[{"id":42,"title":%q}]}}`,
				fmt.Sprintf("%s/%d/%s", category, count, r.Header.Get("Cache-Control")))
		case r.Method == http.MethodGet && r.URL.Path == postPath:
			count := detailReads.Add(1)
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprintf(w, `{"code":0,"msg":"","data":{"id":42,"title":%q}}`,
				fmt.Sprintf("%d/%s", count, r.Header.Get("Cache-Control")))
		case r.Method == http.MethodPost && r.URL.Path == api.PathPosts:
			io.Copy(io.Discard, r.Body)
			fmt.Fprintf(w, `{"code":0,"msg":"","data":{"id":%d}}`, publishedPost.Load())
		case r.Method == http.MethodPost && r.URL.Path == repliesPath:
			io.Copy(io.Discard, r.Body)
			io.WriteString(w, `{"code":0,"msg":"","data":{"id":64,"postId":42}}`)
		default:
			t.Errorf("unexpected route %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	login := forumTestLogin(t, server.URL)
	key := login()
	service := forum.ForumService{}
	for range 2 {
		if items, err := service.Categories(api.SessionRequest{SessionKey: key}); err != nil || len(items) != 1 {
			t.Fatalf("categories = %+v, %v", items, err)
		}
	}
	if got := categoryReads.Load(); got != 1 {
		t.Fatalf("no-store categories requests = %d; want 1", got)
	}
	if _, err := service.Categories(api.SessionRequest{SessionKey: key, Force: true}); err != nil || categoryReads.Load() != 2 {
		t.Fatalf("force categories requests = %d, %v; want 2", categoryReads.Load(), err)
	}
	readDetail := func(force bool, want string) {
		t.Helper()
		detail, err := service.PostDetail(api.PostDetailRequest{SessionKey: key, PostID: 42, Force: force})
		if err != nil || detail.ID != 42 || detail.Title != want {
			t.Fatalf("detail = %+v, %v; want title %q", detail, err, want)
		}
	}
	readDetail(false, "1/")
	readDetail(false, "1/")
	readDetail(true, "2/no-cache")
	readDetail(false, "2/no-cache")
	read := func(input api.PostListRequest, want string) {
		t.Helper()
		input.SessionKey = key
		data, err := service.Posts(input)
		if err != nil || len(data.List) != 1 || data.List[0].Title != want {
			t.Fatalf("posts(%+v) = %+v, %v; want %q", input, data, err, want)
		}
	}

	all, guide, other := api.PostListRequest{}, api.PostListRequest{CategoryID: 1}, api.PostListRequest{CategoryID: 2}
	read(all, "/1/")
	read(guide, "1/1/")
	read(other, "2/1/")
	read(api.PostListRequest{CategoryID: 1, Page: 1, PageSize: 20}, "1/1/")
	read(api.PostListRequest{CategoryID: 1, Force: true}, "1/2/no-cache")
	read(guide, "1/2/no-cache")
	read(api.PostListRequest{CategoryID: 4}, "4/1/")
	read(api.PostListRequest{CategoryID: 4}, "4/2/")

	post := api.PublishPostRequest{SessionKey: key, RequestID: "4b3acd99-e866-4176-8e85-ea9e11f99ff6", CategoryID: 1, Title: "新帖", Body: "正文"}
	if result, err := service.PublishPost(post); err != nil || result.ID != 0 {
		t.Fatalf("pending publication = %+v, %v", result, err)
	}
	read(guide, "1/2/no-cache")
	publishedPost.Store(42)
	post.RequestID = "fbd24592-a984-4f22-a6dd-061a8b870e58"
	if result, err := service.PublishPost(post); err != nil || result.ID != 42 {
		t.Fatalf("visible publication = %+v, %v", result, err)
	}
	read(all, "/2/")
	read(guide, "1/3/")
	read(other, "2/1/")
	if reply, err := service.PublishReply(api.PublishReplyRequest{SessionKey: key, PostID: 42, Body: "回复"}); err != nil || reply.ID != 64 {
		t.Fatalf("visible reply = %+v, %v", reply, err)
	}
	readDetail(false, "3/")
	readDetail(false, "3/")
	read(guide, "1/4/")
	read(other, "2/2/")

	bad := api.PostListRequest{SessionKey: key, CategoryID: 3}
	if _, err := service.Posts(bad); err == nil {
		t.Fatal("invalid business data was accepted")
	}
	read(bad, "3/2/")

	oldKey := key
	if err := account.ClearSession(); err != nil {
		t.Fatal(err)
	}
	key = login()
	if _, err := service.Posts(api.PostListRequest{SessionKey: oldKey, CategoryID: 1}); err == nil {
		t.Fatal("old session accessed forum posts")
	}
	read(guide, "1/5/")
	readDetail(false, "4/")
}

// forumTestLogin 初始化临时账号存储并返回登录函数，复用客户端真实会话守卫。
func forumTestLogin(t *testing.T, address string) func() string {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&systemModel.Config{}, &systemModel.Log{}); err != nil {
		t.Fatal(err)
	}
	client, err := core.NewHTTPClient(address)
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
	return func() string {
		t.Helper()
		login, err := account.Login(t.Context(), systemAPI.AccountLoginRequest{})
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join([]string{login.Session.ServerKey, login.Session.SubjectID, login.Session.Generation}, "\n")
	}
}
