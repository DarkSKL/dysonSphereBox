package account

// 文件职责：共享账号HTTP传输、响应解码和原子会话副作用。
// 使用：账号helper通过request或requestBody调用；登录和注销使用显式捕获快照。
// 边界：响应限4MiB且禁止重定向，不把凭据或正文写日志；旧会话结果不能更新当前身份。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// responseBodyLimit 职责：限制完整编码响应；使用：limit+1检测；边界：超限拒绝而非截断。
const responseBodyLimit = 4 << 20

// request 职责：编码普通账号JSON请求；使用：资料和认证helper；边界：捕获一次凭据后不再读取当前token构造请求。
func request[T any](ctx context.Context, method, path string, payload interface{}, authenticated bool) (T, error) {
	return requestCaptured[T](ctx, captureRequest(), method, path, payload, authenticated)
}

// requestCaptured 职责：编码绑定快照的JSON；使用：登录/注销及普通request；边界：不重新捕获账号或目标。
func requestCaptured[T any](ctx context.Context, snapshot requestSnapshot, method, path string, payload interface{}, authenticated bool) (result T, err error) {
	var body io.Reader
	contentType := ""
	if payload != nil {
		raw, encodeErr := json.Marshal(payload)
		if encodeErr != nil {
			return result, encodeErr
		}
		body = bytes.NewReader(raw)
		contentType = "application/json"
	}
	return requestBodyCaptured[T](ctx, snapshot, method, path, body, contentType, authenticated)
}

// requestBody 职责：捕获multipart账号请求上下文；使用：头像上传；边界：与JSON请求共享鉴权实现。
func requestBody[T any](ctx context.Context, method, path string, body io.Reader, contentType string, authenticated bool) (T, error) {
	return requestBodyCaptured[T](ctx, captureRequest(), method, path, body, contentType, authenticated)
}

// requestBodyCaptured 职责：发送固定凭据请求、完整解析再原子接纳；使用：JSON/multipart请求；边界：旧响应无token/401/409/用户身份副作用。
// 逻辑流：1. 合并调用与会话取消 => 2. 发固定目标请求 => 3. 有界解码 => 4. 提取可信身份/登录凭据 => 5. 同锁检查代次并提交副作用。
func requestBodyCaptured[T any](ctx context.Context, snapshot requestSnapshot, method, path string, body io.Reader, contentType string, authenticated bool) (result T, resultErr error) {
	status, code := 0, 0
	defer func() {
		if resultErr != nil {
			recordRequestFailure(method, path, "accountResponse", status, code, resultErr)
		}
	}()
	if snapshot.client == nil {
		return result, errors.New("服务器连接尚未初始化")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(snapshot.ctx, cancel)
	defer stop()
	req, err := snapshot.client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return result, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if authenticated {
		req.Header.Set(systemAPI.AccountTokenHeader, snapshot.token)
	}
	resp, err := snapshot.client.DoNoRedirect(req)
	if err != nil {
		return result, errors.New("无法连接账号服务器，请稍后重试。")
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	if authenticated && status == http.StatusUnauthorized {
		if err := acceptAccountResponse(snapshot, "", true, false, 0, "", nil, false); err != nil {
			return result, err
		}
		return result, errors.New("登录已失效，请重新登录。")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, responseBodyLimit+1))
	if err != nil || len(raw) > responseBodyLimit {
		return result, errors.New("账号响应读取失败或超过4MiB")
	}
	var envelope systemAPI.AccountResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return result, fmt.Errorf("账号响应格式无效 (HTTP %d)", status)
	}
	code = envelope.Code
	success := status == http.StatusOK && code == systemAPI.AccountCodeSuccess
	var verifiedID uint
	var loginToken string
	var loginPassword *bool
	if success {
		if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
			if err := json.Unmarshal(envelope.Data, &result); err != nil {
				return result, errors.New("账号响应字段格式无效")
			}
		}
		if path == systemAPI.APIURLGetUserInfo {
			var data systemAPI.AccountCurrentUserData
			if json.Unmarshal(envelope.Data, &data) != nil || data.UserInfo.ID == 0 {
				return result, errors.New("账号验证未返回有效用户ID")
			}
			verifiedID = data.UserInfo.ID
		}
		if path == systemAPI.APIURLLogin {
			var data systemAPI.AccountLoginData
			if json.Unmarshal(envelope.Data, &data) != nil {
				return result, errors.New("登录响应无效")
			}
			loginToken = data.Token
			loginPassword = &data.NeedChangePassword
			// 登录响应已经包含当前用户身份，直接建立可信主体，避免登录后还要额外等待一次用户信息请求。
			verifiedID = data.User.ID
		}
	}
	if authenticated || path == systemAPI.APIURLLogin {
		if err := acceptAccountResponse(snapshot, resp.Header.Get(systemAPI.AccountNewTokenHeader), false, status == http.StatusConflict && code == systemAPI.AccountCodeNeedChangePassword, verifiedID, loginToken, loginPassword, success && path == systemAPI.APIURLChangePassword); err != nil {
			return result, err
		}
	}
	if err := ensureRequestCurrent(snapshot); err != nil {
		return result, err
	}
	if !success {
		if envelope.Msg != "" {
			return result, errors.New(envelope.Msg)
		}
		return result, fmt.Errorf("账号请求失败 (HTTP %d)", status)
	}
	return result, nil
}
