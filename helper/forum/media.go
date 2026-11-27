package forum

// 文件职责：通过 Wails AssetServer 中间件提供论坛远端媒体鉴权代理与受控本地预览服务。
// 使用：在 main.go 中挂载 MediaProxyMiddleware，供 WebView 的 <img> 和 <video> 读取图片及支持 Range 的 MP4。
// 边界：不向前端暴露 x-token；本地预览仅允许访问经由本应用文件对话框登记的令牌路径。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	api "DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// 媒体中间件拦截路径前缀与本地预览登记表容量上限。
const (
	// remoteMediaPrefix 拦截已发布帖子或回复中的远端图片和 MP4 读取。
	remoteMediaPrefix = "/__dsbox/forum/media/"
	// localPreviewPrefix 拦截编辑器中本地选中图片和 MP4 的受控预览。
	localPreviewPrefix = "/__dsbox/forum/local-preview/"
	// maxLocalPreviewEntries 限制本地预览映射表上限，避免长会话内存增长。
	maxLocalPreviewEntries = 64
)

// localPreviewEntry 保存已登记本地媒体预览文件的元数据。
// 职责：将随机令牌映射到经过校验的本地路径与 MIME 类型。
// 使用：RegisterLocalPreview 写入，serveLocalPreview 读取。
// 边界：只保存来自系统文件对话框并经 InspectLocalFile 校验的路径。
type localPreviewEntry struct {
	// path 职责：本地文件绝对路径；使用：http.ServeContent 打开读取；边界：受控路径。
	path string
	// contentType 职责：图片或视频 MIME 类型；使用：设置响应头；边界：非空。
	contentType string
	// name 职责：文件展示名；使用：ServeContent 辅助；边界：基名。
	name string
}

// localPreviewRegistry 维护编辑器待上传媒体的受控本地预览令牌映射。
// 职责：安全隔离前端预览请求与本机文件系统路径。
// 使用：RegisterLocalPreview、RevokeLocalPreviews 与 serveLocalPreview 并发访问。
// 边界：超过上限时按最旧登记顺序淘汰。
var localPreviewRegistry = struct {
	mu      sync.RWMutex
	entries map[string]localPreviewEntry
	order   []string
}{
	entries: make(map[string]localPreviewEntry),
}

// RegisterLocalPreview 为编辑器选中的本地图片或 MP4 注册受控预览 URL。
// 职责：生成不可预测令牌并返回相对路径 /__dsbox/forum/local-preview/{token}。
// 使用：ForumService.SelectImages 与 SelectVideo 在文件校验通过后调用。
// 边界：仅登记已校验的图片或视频文件；超限时自动淘汰最早条目。
func RegisterLocalPreview(file InspectedFile) (token string, previewURL string) {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	token = hex.EncodeToString(buf[:])

	localPreviewRegistry.mu.Lock()
	defer localPreviewRegistry.mu.Unlock()
	if len(localPreviewRegistry.order) >= maxLocalPreviewEntries {
		evict := localPreviewRegistry.order[0]
		localPreviewRegistry.order = localPreviewRegistry.order[1:]
		delete(localPreviewRegistry.entries, evict)
	}
	localPreviewRegistry.entries[token] = localPreviewEntry{
		path:        file.Path,
		contentType: file.ContentType,
		name:        file.Name,
	}
	localPreviewRegistry.order = append(localPreviewRegistry.order, token)
	return token, localPreviewPrefix + token
}

// RevokeLocalPreviews 释放编辑器不再使用的本地预览令牌。
// 职责：草稿清空、移除附件或页面离开时清理映射项。
// 使用：ForumService.ReleaseLocalPreviews 调用。
// 边界：忽略未知或已失效令牌，不删除用户磁盘文件。
func RevokeLocalPreviews(tokens []string) {
	if len(tokens) == 0 {
		return
	}
	removeSet := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			removeSet[token] = struct{}{}
		}
	}
	if len(removeSet) == 0 {
		return
	}

	localPreviewRegistry.mu.Lock()
	defer localPreviewRegistry.mu.Unlock()
	for token := range removeSet {
		delete(localPreviewRegistry.entries, token)
	}
	nextOrder := localPreviewRegistry.order[:0]
	for _, token := range localPreviewRegistry.order {
		if _, removed := removeSet[token]; !removed {
			nextOrder = append(nextOrder, token)
		}
	}
	localPreviewRegistry.order = nextOrder
}

// MediaProxyMiddleware 返回挂载在 Wails AssetServer 上的论坛媒体代理中间件。
// 职责：拦截远端论坛媒体代理与编辑器受控本地预览请求，其余路径原样交给默认资源服务器。
// 使用：main.go 配置 assetserver.Options.Middleware 时传入。
// 边界：仅接受 GET 和 HEAD 方法；严禁透传 x-token 或开放任意本地路径读取。
func MediaProxyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasPrefix(req.URL.Path, remoteMediaPrefix):
			serveRemoteMedia(writer, req)
		case strings.HasPrefix(req.URL.Path, localPreviewPrefix):
			serveLocalPreview(writer, req)
		default:
			next.ServeHTTP(writer, req)
		}
	})
}

// serveRemoteMedia 通过当前已验证账号会话向应用服务器转发图片/MP4 读取与 Range 请求。
// 逻辑流：1. 解析资源 ID 与会话代次 => 2. 捕获已验证会话 => 3. 转发 Range 头并流式读取 => 4. 接纳鉴权副作用并回写响应头与内容。
func serveRemoteMedia(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rawID := strings.TrimPrefix(req.URL.Path, remoteMediaPrefix)
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 {
		http.NotFound(writer, req)
		return
	}

	desc := api.MediaAPI
	if req.Method == http.MethodHead {
		desc = api.MediaHeadAPI
	}
	adapter := account.ServerRequests()
	session, err := adapter.Capture(desc, "")
	if err != nil {
		writer.Header().Set("Cache-Control", "no-store")
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	if rawGen := strings.TrimSpace(req.URL.Query().Get("g")); rawGen != "" {
		if expectedGen, parseErr := strconv.ParseUint(rawGen, 10, 64); parseErr != nil || expectedGen != session.Generation {
			writer.Header().Set("Cache-Control", "no-store")
			http.Error(writer, "stale session", http.StatusUnauthorized)
			return
		}
	}

	input, err := IDInput("", uint(id))
	if err != nil {
		http.NotFound(writer, req)
		return
	}
	input.Session, input.Range, input.IfRange = &session, req.Header.Get("Range"), req.Header.Get("If-Range")
	started := false
	err = request.Stream(req.Context(), adapter, desc, input, func(_ context.Context, resp *http.Response) error {
		started = true
		return writeMedia(writer, req.Method, resp)
	})
	if err != nil && !started {
		status := mediaErrorStatus(err)
		writer.Header().Set("Cache-Control", "no-store")
		http.Error(writer, http.StatusText(status), status)
	}
}

// writeMedia 仅向 WebView 透传媒体允许的响应头和正文，HEAD 只写响应头。
func writeMedia(writer http.ResponseWriter, method string, resp *http.Response) error {
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Content-Disposition"} {
		if value := resp.Header.Get(header); value != "" {
			writer.Header().Set(header, value)
		}
	}
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(resp.StatusCode)
	if method != http.MethodHead {
		_, err := io.Copy(writer, resp.Body)
		return err
	}
	return nil
}

// mediaErrorStatus 将请求失败映射为 WebView 可处理的本地媒体响应状态。
func mediaErrorStatus(err error) int {
	if detail := request.Details(err); detail != nil {
		switch detail.Kind {
		case request.AuthRequired, request.Unauthorized, request.Stale:
			return http.StatusUnauthorized
		case request.PasswordRequired:
			return http.StatusConflict
		case request.BuildFailed, request.TransportFailed:
			return http.StatusBadGateway
		}
	}
	return http.StatusNotFound
}

// serveLocalPreview 使用 http.ServeContent 提供编辑器本地选中图片或 MP4 的预览（原生支持 Range）。
func serveLocalPreview(writer http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := strings.TrimPrefix(req.URL.Path, localPreviewPrefix)
	localPreviewRegistry.mu.RLock()
	entry, ok := localPreviewRegistry.entries[token]
	localPreviewRegistry.mu.RUnlock()
	if !ok {
		http.NotFound(writer, req)
		return
	}
	file, err := os.Open(entry.path)
	if err != nil {
		http.NotFound(writer, req)
		return
	}
	defer file.Close()

	writer.Header().Set("Content-Type", entry.contentType)
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(writer, req, entry.name, time.Time{}, file)
}
