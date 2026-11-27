package forum

// 文件职责：绑定论坛 multipart 上传和带完整性校验的下载。
// 使用：发布帖子、带图回复和托管资源导入。
// 边界：传输经全局请求适配器，磁盘、内容校验与临时目录由论坛业务负责。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	api "DysonSphereBox/apiDefinition"
	forumAPI "DysonSphereBox/apiDefinition/forum"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// MultipartFilePart 职责：绑定一个 multipart 字段与已验文件；使用：发帖与带图回复；边界：按字段顺序流式读取。
type MultipartFilePart struct {
	FieldName string
	File      InspectedFile
}

// VerifiedDownload 职责：交付经长度与 SHA-256 校验的文件；使用：导入或读取；边界：调用方负责 Cleanup。
type VerifiedDownload struct {
	Resource forumAPI.Resource
	TempDir  string
	FilePath string
}

// Cleanup 职责：删除本次临时目录；使用：导入完成或传输失败；边界：空结果可以安全调用。
func (download *VerifiedDownload) Cleanup() {
	if download == nil || strings.TrimSpace(download.TempDir) == "" {
		return
	}
	_ = os.RemoveAll(download.TempDir)
}

// DoMultipart 职责：流式写入正文和本地文件；使用：论坛发布接口；边界：文件按块读取，结束时关闭写入管道。
func DoMultipart[T any](ctx context.Context, desc *api.ApiDesc, input request.Input, field string, draft any, files []MultipartFilePart) (result T, err error) {
	data, err := json.Marshal(draft)
	if err != nil || len(data) > forumAPI.JSONRequestLimit {
		return result, InvalidInput("提交的正文数据无效或超过 1 MiB 限制")
	}
	reader, pipe := io.Pipe()
	defer reader.Close()
	writer := multipart.NewWriter(pipe)
	input.Body, input.ContentType = reader, writer.FormDataContentType()
	go func() {
		_ = pipe.CloseWithError(writeMultipart(ctx, writer, field, data, files))
	}()
	result, err = request.Do[T](ctx, account.ServerRequests(), desc, input)
	if err != nil {
		if detail := request.Details(err); detail != nil && detail.Kind == request.TransportFailed {
			return result, failure("requestFailed", "上传失败，请检查网络后重试")
		}
		return result, forumRequestFailure(desc, err)
	}
	return result, nil
}

// writeMultipart 先写正文，再按表单顺序打开并关闭各磁盘文件。
func writeMultipart(ctx context.Context, writer *multipart.Writer, field string, data []byte, files []MultipartFilePart) error {
	body, err := writer.CreateFormField(field)
	if err != nil {
		return err
	}
	if _, err := body.Write(data); err != nil {
		return err
	}
	for _, part := range files {
		if err := streamFilePart(ctx, writer, part); err != nil {
			return err
		}
	}
	return writer.Close()
}

// streamFilePart 一次只打开一个本地文件，关闭管道时复制随即返回。
func streamFilePart(ctx context.Context, writer *multipart.Writer, part MultipartFilePart) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.Open(part.File.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	field, err := writer.CreateFormFile(part.FieldName, part.File.Name)
	if err != nil {
		return err
	}
	_, err = io.Copy(field, file)
	return err
}

// DownloadVerifiedFile 职责：实时复核并下载托管文件；使用：蓝图、存档或 Mod 导入；边界：两步固定同一会话，允许该会话续期。
func DownloadVerifiedFile(ctx context.Context, sessionKey string, resourceID uint, expectedType, requiredExt string) (*VerifiedDownload, error) {
	input, err := IDInput(sessionKey, resourceID)
	if err != nil {
		return nil, err
	}
	adapter := account.ServerRequests()
	initial, err := adapter.Capture(forumAPI.ResourceAPI, sessionKey)
	if err != nil {
		return nil, forumRequestFailure(forumAPI.ResourceAPI, err)
	}
	input.Session = &initial
	resource, err := DoJSON[forumAPI.Resource](ctx, forumAPI.ResourceAPI, input)
	if err != nil {
		return nil, err
	}
	if expectedType != "" && resource.Type != expectedType {
		return nil, InvalidInput("目标资源类型与当前操作不匹配")
	}
	if resource.Source != "upload" {
		return nil, InvalidInput("该资源不是可下载的托管文件")
	}
	access, err := adapter.Capture(forumAPI.DownloadAPI, initial.Scope.ID)
	if err != nil || access.Scope != initial.Scope {
		return nil, failure("stale", "账号会话已切换")
	}
	input.Session = &access
	var download *VerifiedDownload
	err = request.Stream(ctx, adapter, forumAPI.DownloadAPI, input, func(_ context.Context, resp *http.Response) error {
		var writeErr error
		download, writeErr = saveVerifiedDownload(resp, resource, resourceID, requiredExt)
		return writeErr
	})
	if err != nil {
		download.Cleanup()
		return nil, forumRequestFailure(forumAPI.DownloadAPI, err)
	}
	return download, nil
}

// saveVerifiedDownload 核对元数据、流式写入临时文件，并验证最终长度和摘要。
func saveVerifiedDownload(resp *http.Response, resource forumAPI.Resource, resourceID uint, requiredExt string) (result *VerifiedDownload, resultErr error) {
	expectedLength := resp.ContentLength
	if header := strings.TrimSpace(resp.Header.Get("Content-Length")); header != "" {
		parsed, err := strconv.ParseInt(header, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, failure("checksumMismatch", "资源响应长度无效")
		}
		expectedLength = parsed
	}
	if expectedLength <= 0 || expectedLength == math.MaxInt64 {
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
	defer func() {
		if resultErr != nil {
			_ = os.RemoveAll(tempDir)
		}
	}()

	displayName := resource.Name
	if disp := resp.Header.Get("Content-Disposition"); disp != "" {
		if _, params, err := mime.ParseMediaType(disp); err == nil && strings.TrimSpace(params["filename"]) != "" {
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
	if copyErr != nil || closeErr != nil {
		return nil, failure("requestFailed", "下载文件写入失败，请重试")
	}
	if written != expectedLength {
		return nil, failure("checksumMismatch", "下载文件大小不完整，已中止导入")
	}
	if hex.EncodeToString(hasher.Sum(nil)) != expectedSHA {
		return nil, failure("checksumMismatch", "下载文件 SHA-256 校验失败，已中止导入")
	}
	return &VerifiedDownload{Resource: resource, TempDir: tempDir, FilePath: filePath}, nil
}
