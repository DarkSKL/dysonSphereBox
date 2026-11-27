package forum

// 文件职责：执行社区论坛的鉴权 HTTP 请求、流式 multipart 上传与带 SHA-256 校验的托管资源下载。
// 使用：由 service/forum 各 Wails 方法调用，复用账号模块的已验证会话与鉴权副作用接纳。
// 边界：禁止跟随重定向与自动重试；令牌仅留存在 Go 侧，不向前端暴露。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	api "DysonSphereBox/apiDefinition/forum"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/system/account"
	"DysonSphereBox/utils"
)

// 论坛请求超时与响应读取上限，区分普通 JSON 与大文件流式传输。
const (
	// jsonRequestTimeout 限制普通读取与提交 JSON 请求的总时长。
	jsonRequestTimeout = 15 * time.Second
	// transferTimeout 限制大文件上传与下载的总时长（最长支持 512 MiB 存档）。
	transferTimeout = 15 * time.Minute
	// jsonRequestLimit 限制普通 JSON 请求体上限，防止异常大载荷发出本机。
	jsonRequestLimit = 1 << 20
	// jsonResponseLimit 限制普通 JSON 响应读取上限。
	jsonResponseLimit = 8 << 20
	// defaultPageSize 是列表查询未传分页大小时的默认值。
	defaultPageSize = 20
	// maxPageSize 是服务端契约允许的单页上限。
	maxPageSize = 100
)

// MultipartFilePart 描述一次 multipart 上传中的单个磁盘文件字段。
// 职责：把表单字段名和已通过本地校验的文件属性绑定给流式上传器。
// 使用：ForumService.PublishPost 与 PublishReply 组装文件字段后传给 DoMultipart。
// 边界：由流式上传器按序打开并关闭磁盘文件，不把文件内容整块读入内存。
type MultipartFilePart struct {
	// FieldName 职责：multipart 表单字段名（如 image1、video、blueprintFile、modFile、save）；使用：服务端按字段名识别资源；边界：须符合契约。
	FieldName string
	// File 职责：已校验的本地文件信息；使用：读取路径和上传展示名；边界：路径须指向存在的普通文件。
	File InspectedFile
}

// VerifiedDownload 描述通过长度和 SHA-256 校验的本地临时下载结果。
// 职责：把复核后的资源元数据、临时目录和临时文件路径交给导入流程。
// 使用：DownloadVerifiedFile 返回，调用方必须使用 defer Cleanup() 清理临时目录。
// 边界：不直接写入游戏目录或客户端资源库。
type VerifiedDownload struct {
	// Resource 职责：下载前重新向服务端确认的可见资源元数据；使用：展示与来源分支判断；边界：必然处于可见状态。
	Resource api.Resource
	// TempDir 职责：本次下载创建的独立临时目录；使用：导入结束后整体清理；边界：位于 appData/forum/tmp 下。
	TempDir string
	// FilePath 职责：校验通过后的本地临时文件完整路径；使用：交给环境管理服务导入或读取；边界：已保留安全文件名。
	FilePath string
}

// Cleanup 删除本次下载创建的临时目录。
// 职责：确保导入成功或失败后不残留临时文件。
// 使用：调用方在获取 VerifiedDownload 后通过 defer 调用。
// 边界：仅删除 TempDir 本身，不触碰外部目录。
func (download *VerifiedDownload) Cleanup() {
	if download == nil || strings.TrimSpace(download.TempDir) == "" {
		return
	}
	_ = os.RemoveAll(download.TempDir)
}

// requestSession 固定单次请求捕获的已验证会话快照，仅在 Go 侧流转。
// 职责：隔离账号切换、服务器切换与并发登录代次变化。
// 使用：captureSession 返回给 DoJSON、DoMultipart、DownloadVerifiedFile 和媒体代理。
// 边界：严禁把 token 返回前端或写入日志。
type requestSession struct {
	// generation 职责：区分同一账号的不同登录代次；使用：响应后核对未切号；边界：与 account.VerifiedSession 同步。
	generation uint64
	// token 职责：本次请求携带的 x-token；使用：设置鉴权请求头与续期接纳；边界：不离开 Go 进程。
	token string
	// ctx 职责：账号退出或切服时立即取消的会话上下文；使用：中断在途请求；边界：不可为空。
	ctx context.Context
	// client 职责：捕获时绑定的共享应用服务器客户端；使用：构造并发起 HTTP 请求；边界：不随全局切服漂移。
	client *core.HTTPClient
}

// captureSession 捕获当前已验证会话并核对页面传入的会话作用域键。
// 职责：防止未登录、待改密或旧账号页面继续发出论坛请求。
// 使用：所有论坛请求入口在发送网络请求前调用。
// 边界：expectedKey 为空时不校验页面键，仍强制要求本机存在已验证会话。
func captureSession(expectedKey string) (requestSession, error) {
	generation, subject, serverKey, token, ctx, client, ok := account.VerifiedSession()
	if !ok {
		if account.SessionState().NeedChangePassword {
			return requestSession{}, failure("passwordRequired", "请先修改账号密码")
		}
		return requestSession{}, failure("unauthorized", "请先登录并完成账号验证")
	}
	key := strings.Join([]string{serverKey, subject, strconv.FormatUint(generation, 10)}, "\n")
	if expectedKey != "" && expectedKey != key {
		return requestSession{}, failure("stale", "账号会话已切换")
	}
	return requestSession{generation: generation, token: token, ctx: ctx, client: client}, nil
}

// PageQuery 归一化分页查询参数。
// 职责：将前端传入的页码与每页条数约束到服务端允许范围。
// 使用：ForumService.Posts 和 Replies 构造 query 参数时调用。
// 边界：page < 1 归一化为 1；pageSize 不在 [1, 100] 时回退为 20。
func PageQuery(page, pageSize int) url.Values {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("pageSize", strconv.Itoa(pageSize))
	return query
}

// ResolveIDPath 用正整数 ID 替换论坛路径模板中的 {{ID}}。
// 职责：统一校验正整数 ID 并复用 utils.ResolveAPIPath。
// 使用：帖子详情、回复、资源复核和下载路径构造时调用。
// 边界：id 为 0 时返回 invalidInput 错误。
func ResolveIDPath(pathTemplate string, id uint) (string, error) {
	if id == 0 {
		return "", InvalidInput("缺少有效的目标 ID")
	}
	resolved, err := utils.ResolveAPIPath(pathTemplate, api.IDPathParams{ID: id})
	if err != nil {
		return "", failure("requestFailed", "无法构造论坛请求路径")
	}
	return resolved, nil
}

// DoJSON 执行社区论坛的 JSON GET/POST 请求并解码统一响应外壳。
// 职责：携带当前会话 x-token 发送固定目标请求、处理 401/409 续期副作用并解码业务数据。
// 使用：分类、帖子列表、帖子详情、回复列表、无图回复、举报和资源复核调用。
// 边界：禁止重定向和自动重试；支持对象或数组类型的 data 解码。
// 逻辑流：1. 捕获已验证会话与超时上下文 => 2. 构造并发送无重定向请求 => 3. 核对会话与鉴权状态 => 4. 解码业务响应或返回错误。
func DoJSON[T any](caller context.Context, sessionKey, method, path string, query url.Values, payload any) (result T, resultErr error) {
	ctx, cancel := context.WithTimeout(caller, jsonRequestTimeout)
	defer cancel()
	session, err := captureSession(sessionKey)
	if err != nil {
		return result, err
	}
	stop := context.AfterFunc(session.ctx, cancel)
	defer stop()
	if err := session.interrupted(ctx); err != nil {
		return result, err
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil || len(raw) > jsonRequestLimit {
			return result, InvalidInput("论坛请求数据无效或超过大小限制")
		}
		body = bytes.NewReader(raw)
	}
	req, err := session.client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return result, failure("requestFailed", "无法构造论坛请求")
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set(systemAPI.AccountTokenHeader, session.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := session.client.DoNoRedirect(req)
	if err != nil {
		if interrupted := session.interrupted(ctx); interrupted != nil {
			return result, interrupted
		}
		return result, failure("requestFailed", "无法连接论坛服务，请稍后重试")
	}
	defer resp.Body.Close()
	return decodeEnvelopeResponse[T](ctx, session, resp)
}

// DoMultipart 执行社区论坛的流式 multipart/form-data 上传请求。
// 职责：首段写入 JSON 字段，随后按序流式写入本地文件，避免大文件占满内存。
// 使用：ForumService.PublishPost 与带图回复调用。
// 边界：使用 DoStreamNoRedirect，不跟随重定向，不自动重试；失败保留本地原文件。
// 逻辑流：1. 捕获会话并序列化首段 JSON => 2. 建立 io.Pipe 后台流式写入 multipart => 3. 发送流式请求 => 4. 解码统一业务响应。
func DoMultipart[T any](caller context.Context, sessionKey, path, jsonField string, draft any, files []MultipartFilePart) (result T, resultErr error) {
	ctx, cancel := context.WithTimeout(caller, transferTimeout)
	defer cancel()
	session, err := captureSession(sessionKey)
	if err != nil {
		return result, err
	}
	stop := context.AfterFunc(session.ctx, cancel)
	defer stop()
	if err := session.interrupted(ctx); err != nil {
		return result, err
	}

	draftJSON, err := json.Marshal(draft)
	if err != nil || len(draftJSON) > jsonRequestLimit {
		return result, InvalidInput("提交的正文数据无效或超过 1 MiB 限制")
	}

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	contentType := writer.FormDataContentType()

	go func() {
		defer pipeWriter.Close()
		fieldWriter, err := writer.CreateFormField(jsonField)
		if err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		if _, err := fieldWriter.Write(draftJSON); err != nil {
			_ = pipeWriter.CloseWithError(err)
			return
		}
		for _, part := range files {
			if err := streamFilePart(ctx, writer, part); err != nil {
				_ = pipeWriter.CloseWithError(err)
				return
			}
		}
		if err := writer.Close(); err != nil {
			_ = pipeWriter.CloseWithError(err)
		}
	}()

	req, err := session.client.NewRequest(ctx, http.MethodPost, strings.TrimPrefix(path, "/"), pipeReader)
	if err != nil {
		_ = pipeReader.Close()
		return result, failure("requestFailed", "无法构造论坛上传请求")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(systemAPI.AccountTokenHeader, session.token)

	resp, err := session.client.DoStreamNoRedirect(req)
	if err != nil {
		_ = pipeReader.Close()
		if interrupted := session.interrupted(ctx); interrupted != nil {
			return result, interrupted
		}
		return result, failure("requestFailed", "上传失败，请检查网络后重试")
	}
	defer resp.Body.Close()
	return decodeEnvelopeResponse[T](ctx, session, resp)
}

// DownloadVerifiedFile 先复核资源可用性，再鉴权下载并严格校验内容长度与 SHA-256。
// 职责：保证下载前资源仍未下架，且落盘内容与服务端声明的长度和哈希完全一致。
// 使用：ForumService 获取托管蓝图编码、导入蓝图、导入存档和导入模组时调用。
// 边界：任一环节失败立即清理临时文件并返回结构化错误，不触碰游戏目录。
// 逻辑流：1. GET /resources/:id 复核可用性 => 2. 发起流式下载 => 3. 拦截非二进制/下架 JSON 错误 => 4. 流式落盘并计算 SHA-256 => 5. 校验长度与哈希一致。
func DownloadVerifiedFile(caller context.Context, sessionKey string, resourceID uint, expectedType, requiredExt string) (*VerifiedDownload, error) {
	resourcePath, err := ResolveIDPath(api.PathResource, resourceID)
	if err != nil {
		return nil, err
	}
	resource, err := DoJSON[api.Resource](caller, sessionKey, http.MethodGet, resourcePath, nil, nil)
	if err != nil {
		return nil, err
	}
	if expectedType != "" && resource.Type != expectedType {
		return nil, InvalidInput("目标资源类型与当前操作不匹配")
	}
	if resource.Source != "upload" {
		return nil, InvalidInput("该资源不是可下载的托管文件")
	}

	ctx, cancel := context.WithTimeout(caller, transferTimeout)
	defer cancel()
	session, err := captureSession(sessionKey)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(session.ctx, cancel)
	defer stop()
	if err := session.interrupted(ctx); err != nil {
		return nil, err
	}

	downloadPath, err := ResolveIDPath(api.PathResourceDownload, resourceID)
	if err != nil {
		return nil, err
	}
	req, err := session.client.NewRequest(ctx, http.MethodGet, strings.TrimPrefix(downloadPath, "/"), nil)
	if err != nil {
		return nil, failure("requestFailed", "无法构造资源下载请求")
	}
	req.Header.Set("Cache-Control", "no-store")
	req.Header.Set(systemAPI.AccountTokenHeader, session.token)

	resp, err := session.client.DoStreamNoRedirect(req)
	if err != nil {
		if interrupted := session.interrupted(ctx); interrupted != nil {
			return nil, interrupted
		}
		return nil, failure("requestFailed", "下载资源失败，请检查网络后重试")
	}
	defer resp.Body.Close()

	if err := session.interrupted(ctx); err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if !session.acceptAuth("", true, false) {
			return nil, failure("stale", "账号会话或凭据已更新")
		}
		return nil, failure("unauthorized", "登录已失效，请重新登录")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, failure("notFound", "资源不存在或已下架")
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if resp.StatusCode != http.StatusOK || mediaType == "application/json" {
		_, err := decodeEnvelopeResponse[struct{}](ctx, session, resp)
		if err != nil {
			return nil, err
		}
		return nil, failure("requestFailed", "资源下载响应无效")
	}

	expectedLength := resp.ContentLength
	if headerLen := strings.TrimSpace(resp.Header.Get("Content-Length")); headerLen != "" {
		parsed, parseErr := strconv.ParseInt(headerLen, 10, 64)
		if parseErr != nil || parsed <= 0 {
			return nil, failure("checksumMismatch", "资源响应长度无效")
		}
		expectedLength = parsed
	}
	if expectedLength <= 0 {
		return nil, failure("checksumMismatch", "服务端未提供有效的资源长度")
	}
	if resource.Size > 0 && expectedLength != resource.Size {
		return nil, failure("checksumMismatch", "下载文件长度与资源记录不一致")
	}
	expectedSHA := strings.ToLower(strings.TrimSpace(resp.Header.Get("X-Content-SHA256")))
	if expectedSHA == "" {
		return nil, failure("checksumMismatch", "服务端未提供文件校验摘要")
	}
	if resource.SHA256 != "" && !strings.EqualFold(expectedSHA, resource.SHA256) {
		return nil, failure("checksumMismatch", "下载校验摘要与资源记录不一致")
	}

	tempDir, err := CreateDownloadTempDir("dl")
	if err != nil {
		return nil, err
	}
	cleanupOnErr := true
	defer func() {
		if cleanupOnErr {
			_ = os.RemoveAll(tempDir)
		}
	}()

	displayName := resource.Name
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		if _, params, dispErr := mime.ParseMediaType(disp); dispErr == nil && strings.TrimSpace(params["filename"]) != "" {
			displayName = params["filename"]
		}
	}
	safeName := SanitizeDownloadFilename(displayName, "forum-resource-"+strconv.FormatUint(uint64(resourceID), 10), requiredExt)
	filePath := filepath.Join(tempDir, safeName)

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, failure("requestFailed", "无法创建临时下载文件")
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, expectedLength+1))
	closeErr := file.Close()
	if interrupted := session.interrupted(ctx); interrupted != nil {
		return nil, interrupted
	}
	if copyErr != nil || closeErr != nil {
		return nil, failure("requestFailed", "下载文件写入失败，请重试")
	}
	if written != expectedLength {
		return nil, failure("checksumMismatch", "下载文件大小不完整，已中止导入")
	}
	actualSHA := hex.EncodeToString(hasher.Sum(nil))
	if actualSHA != expectedSHA {
		return nil, failure("checksumMismatch", "下载文件 SHA-256 校验失败，已中止导入")
	}

	refresh := resp.Header.Get(systemAPI.AccountNewTokenHeader)
	if !session.acceptAuth(refresh, false, false) {
		return nil, failure("stale", "账号会话已切换")
	}
	if generation, _, _, _, _, client, ok := account.VerifiedSession(); !ok || generation != session.generation || client != session.client {
		return nil, failure("stale", "账号会话已失效")
	}

	cleanupOnErr = false
	return &VerifiedDownload{
		Resource: resource,
		TempDir:  tempDir,
		FilePath: filePath,
	}, nil
}

// streamFilePart 从磁盘打开单个本地文件并流式复制到 multipart 字段中。
func streamFilePart(ctx context.Context, writer *multipart.Writer, part MultipartFilePart) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(part.File.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	fileWriter, err := writer.CreateFormFile(part.FieldName, part.File.Name)
	if err != nil {
		return err
	}
	_, err = io.Copy(fileWriter, file)
	return err
}

// decodeEnvelopeResponse 解析服务端统一的 {code, data, msg} 响应外壳并接纳认证副作用。
func decodeEnvelopeResponse[T any](ctx context.Context, session requestSession, resp *http.Response) (result T, resultErr error) {
	if err := session.interrupted(ctx); err != nil {
		return result, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if !session.acceptAuth("", true, false) {
			return result, failure("stale", "账号会话或凭据已更新")
		}
		return result, failure("unauthorized", "登录已失效，请重新登录")
	}
	if resp.StatusCode == http.StatusNotFound {
		return result, failure("unsupported", "当前服务器暂未提供社区论坛功能")
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, jsonResponseLimit+1))
	if interrupted := session.interrupted(ctx); interrupted != nil {
		return result, interrupted
	}
	if err != nil || len(raw) > jsonResponseLimit {
		return result, failure("requestFailed", "论坛响应读取失败或超过大小限制")
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  *string         `json:"msg"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Code == nil || envelope.Msg == nil {
		return result, failure("requestFailed", "论坛响应格式无效")
	}
	code := *envelope.Code
	refresh := resp.Header.Get(systemAPI.AccountNewTokenHeader)
	if resp.StatusCode == http.StatusConflict && code == systemAPI.AccountCodeNeedChangePassword {
		if !session.acceptAuth(refresh, false, true) {
			return result, failure("stale", "账号会话或凭据已更新")
		}
		return result, failure("passwordRequired", "请先修改账号密码")
	}
	if resp.StatusCode != http.StatusOK {
		return result, failure("requestFailed", "论坛服务请求失败，请稍后重试")
	}
	if code == systemAPI.AccountCodeSuccess {
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			if err := json.Unmarshal(envelope.Data, &result); err != nil {
				var zero T
				return zero, failure("requestFailed", "论坛响应数据解析失败")
			}
		}
	} else {
		resultErr = businessFailure(code, envelope.Data)
	}
	if err := session.interrupted(ctx); err != nil {
		var zero T
		return zero, err
	}
	if !session.acceptAuth(refresh, false, false) {
		var zero T
		return zero, failure("stale", "账号会话已切换")
	}
	if generation, _, _, _, _, client, ok := account.VerifiedSession(); !ok || generation != session.generation || client != session.client {
		var zero T
		return zero, failure("stale", "账号会话已失效")
	}
	if err := session.interrupted(ctx); err != nil {
		var zero T
		return zero, err
	}
	return result, resultErr
}

// interrupted 将账号切换、请求取消和总超时映射为明确的前端错误原因。
func (session requestSession) interrupted(ctx context.Context) error {
	switch {
	case session.ctx.Err() != nil:
		return failure("stale", "账号会话已切换")
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return failure("timeout", "论坛请求超时，请稍后重试")
	case ctx.Err() != nil:
		return failure("canceled", "论坛请求已取消")
	default:
		return nil
	}
}

// acceptAuth 复用账号模块的原子鉴权副作用处理，防止旧凭据覆盖新会话。
func (session requestSession) acceptAuth(refresh string, unauthorized, needPassword bool) bool {
	return account.HandleAuthenticatedResponse(session.generation, session.token, session.client, refresh, unauthorized, needPassword)
}
