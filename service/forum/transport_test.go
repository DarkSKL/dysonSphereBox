package forum_test

// 文件职责：通过论坛公开入口验证资源和上传流的实际 HTTP 契约。
// 使用：go test ./service/forum。
// 边界：回环服务、测试图片和下载文件均为本地临时数据。

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	api "DysonSphereBox/apiDefinition/forum"
	systemAPI "DysonSphereBox/apiDefinition/system"
	forumHelper "DysonSphereBox/helper/forum"
	"DysonSphereBox/helper/system/account"
	"DysonSphereBox/service/forum"
)

// TestForumStreamContracts 验证 multipart、托管文件校验和媒体 Range 转发。
func TestForumStreamContracts(t *testing.T) {
	content := "fixture-blueprint"
	sum := sha256.Sum256([]byte(content))
	digest := hex.EncodeToString(sum[:])
	var badDigest atomic.Bool
	var uploads, requests atomic.Int64
	id := strings.NewReplacer("{{ID}}", "42")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == systemAPI.APIURLLogin {
			fmt.Fprint(w, `{"code":0,"msg":"","data":{"user":{"ID":42},"token":"fixture-media-token"}}`)
			return
		}
		if r.Header.Get(systemAPI.AccountTokenHeader) != "fixture-media-token" {
			t.Error("missing forum credential")
		}
		switch r.URL.Path {
		case id.Replace(api.PathPostReplies):
			uploads.Add(1)
			if r.Method != http.MethodPost || !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
				t.Error("invalid multipart request")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			multipart, err := r.MultipartReader()
			if err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			for _, expected := range []string{"reply", "image1"} {
				part, err := multipart.NextPart()
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				data, err := io.ReadAll(part)
				if part.FormName() != expected || len(data) == 0 || err != nil {
					t.Errorf("unexpected multipart field %q: %v", part.FormName(), err)
				}
			}
			fmt.Fprint(w, `{"code":0,"msg":"","data":{"id":64}}`)
		case id.Replace(api.PathResource):
			fmt.Fprintf(w, `{"code":0,"msg":"","data":{"id":42,"type":"blueprint","source":"upload","name":"fixture.txt","size":%d,"sha256":"%s"}}`, len(content), digest)
		case id.Replace(api.PathResourceDownload):
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("X-Content-SHA256", digest)
			if badDigest.Load() {
				fmt.Fprint(w, strings.Repeat("!", len(content)))
			} else {
				fmt.Fprint(w, content)
			}
		case id.Replace(api.PathResourceContent):
			requests.Add(1)
			w.Header().Set("Content-Type", "video/mp4")
			w.Header().Set("Accept-Ranges", "bytes")
			if r.Method == http.MethodHead {
				return
			}
			if r.Header.Get("Range") != "bytes=1-3" || r.Header.Get("If-Range") != `"test-etag"` {
				t.Error("media Range headers were changed")
			}
			w.Header().Set("Content-Range", "bytes 1-3/4")
			w.WriteHeader(http.StatusPartialContent)
			fmt.Fprint(w, "bcd")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	login := forumTestLogin(t, server.URL)
	key := login()
	service := forum.ForumService{}
	imagePath := writePNG(t)
	reply, err := service.PublishReply(api.PublishReplyRequest{
		SessionKey: key, PostID: 42, Body: "带图回复", RequestID: "4b3acd99-e866-4176-8e85-ea9e11f99ff6",
		Images: []api.PublishMediaSlot{{Slot: "image1", Path: imagePath}},
	})
	if err != nil || reply.ID != 64 || uploads.Load() != 1 {
		t.Fatalf("multipart reply = %+v, %v", reply, err)
	}
	input := api.ResourceActionRequest{SessionKey: key, ResourceID: 42}
	file, err := service.FetchBlueprintCode(input)
	if err != nil || file.BlueprintCode != content {
		t.Fatalf("verified download = %+v, %v", file, err)
	}
	badDigest.Store(true)
	if _, err := service.FetchBlueprintCode(input); err == nil || !strings.Contains(err.Error(), `"reason":"checksumMismatch"`) {
		t.Fatalf("damaged download was accepted: %v", err)
	}
	handler := forumHelper.MediaProxyMiddleware(http.NotFoundHandler())
	path := "/__dsbox/forum/media/42?g=" + account.SessionState().Generation
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Range", "bytes=1-3")
	r.Header.Set("If-Range", `"test-etag"`)
	part := httptest.NewRecorder()
	handler.ServeHTTP(part, r)
	if part.Code != http.StatusPartialContent || part.Header().Get("Content-Range") != "bytes 1-3/4" || part.Body.String() != "bcd" {
		t.Fatalf("unexpected Range response: %d, %v, %q", part.Code, part.Header(), part.Body.String())
	}
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, path, nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || requests.Load() != 2 {
		t.Fatalf("unexpected HEAD response: %d, %q", head.Code, head.Body.String())
	}
	stale := httptest.NewRecorder()
	handler.ServeHTTP(stale, httptest.NewRequest(http.MethodGet, "/__dsbox/forum/media/42?g=0", nil))
	if stale.Code != http.StatusUnauthorized || requests.Load() != 2 {
		t.Fatalf("old media URL made an upstream request: %d", stale.Code)
	}
}

// writePNG 创建由论坛正常图片校验接受的最小测试文件。
func writePNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "preview.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 1, 1)))
	if closeErr := file.Close(); err != nil || closeErr != nil {
		t.Fatalf("create test image: %v %v", err, closeErr)
	}
	return path
}
