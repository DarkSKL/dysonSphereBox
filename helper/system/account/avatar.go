package account

// 文件职责：校验 Wails 传入的头像内容，构造 multipart 并读取上传后用户。
// 使用：AccountService 上传头像时调用；沿用账号请求的 token 与错误处理。
// 边界：不读取任意本地路径，不自行修改资料；仅认可服务器已持久化的成功响应。

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// avatarByteLimit 职责：限制头像为 2MiB；使用：解码前后检查；边界：服务器仍校验图片尺寸和实际内容。
const avatarByteLimit = 2 << 20

// UploadAvatar 职责：将选中图片上传并返回服务器提交后的用户。
// 使用：服务层传入文件名及纯 Base64 内容，HTTP 使用 multipart file。
// 边界：仅支持 JPEG/PNG/WebP；失败不发送资料更新，不返回本地预览作为头像。
// 逻辑流：1. 限长解码并嗅探类型 => 2. 构造 multipart => 3. 认证上传 => 4. 校验成功响应用户。
func UploadAvatar(ctx context.Context, fileName, contentBase64 string) (*systemAPI.AccountUser, error) {
	if len(contentBase64) > base64.StdEncoding.EncodedLen(avatarByteLimit) {
		return nil, errors.New("头像不能超过 2 MiB，请选择较小图片重试。")
	}
	data, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil || len(data) == 0 || len(data) > avatarByteLimit {
		return nil, errors.New("头像内容无效或超过 2 MiB，请重新选择图片。")
	}
	contentType := http.DetectContentType(data)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return nil, errors.New("请选择 JPEG、PNG 或 WebP 图片。")
	}
	fileName = strings.ReplaceAll(fileName, "\\", "/")
	fileName = fileName[strings.LastIndex(fileName, "/")+1:]
	if fileName == "" || len(fileName) > 255 {
		fileName = "avatar"
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": fileName}))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(data); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	result, err := requestBody[systemAPI.AccountAvatarData](ctx, http.MethodPost, systemAPI.APIURLUploadAvatar, &body, writer.FormDataContentType(), true)
	if err != nil {
		return nil, err
	}
	if result.User.ID == 0 || strings.TrimSpace(result.User.HeaderImg) == "" {
		return nil, errors.New("服务器未返回已保存的头像，请重新选择图片重试。")
	}
	return &result.User, nil
}
