package account

// 文件职责：通过全局 HTTP 客户端向应用服务器发送账号请求并解析统一响应。
// 使用：本包账号流程函数调用 request，前端经 Wails 绑定进入 AccountService 后到达这里。
// 边界：只处理请求构造、token 附加与续期、统一响应解析；不决定页面交互，不记录密码等敏感字段。
// 逻辑流：1. 取 GLOBAL.ServerClient => 2. 编码请求体并附加 x-token => 3. 发送请求 => 4. 处理 new-token 与 401 => 5. 解析 {code,data,msg} => 6. 成功时解码 data。

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
	"DysonSphereBox/core"
)

// responseBodyLimit 表示账号响应体的最大读取字节数。
// 职责：防止异常服务返回超大响应体占用内存。
// 使用：request 读取响应体时通过 io.LimitReader 限制。
// 边界：账号接口响应远小于该值。
const responseBodyLimit = 4 << 20

// request 发送账号请求并把成功响应的 data 解码到 T。
// 职责：统一账号接口的鉴权、续期、401 清理、强制改密识别与错误文案。
// 使用：本包的 Captcha、Login、GetCurrentUser 等流程调用。
// 边界：路径按服务器基础地址的相对路径解析，保留基础地址中的路径前缀；payload 为 nil 时不发送请求体。
func request[T any](ctx context.Context, method string, path string, payload interface{}, authenticated bool) (result T, resultErr error) {
	var body io.Reader
	contentType := ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			recordRequestFailure(method, path, "encodeRequest", 0, 0, err)
			return result, fmt.Errorf("编码请求数据失败：%w", err)
		}
		body = bytes.NewReader(data)
		contentType = "application/json"
	}
	return requestBody[T](ctx, method, path, body, contentType, authenticated)
}

// requestBody 职责：统一发送 JSON 或 multipart 账号请求并处理鉴权与响应。
// 使用：request 和头像上传传入已编码正文及匹配的 Content-Type。
// 边界：不记录正文，复用 token 续期、401 清理及业务错误语义。
func requestBody[T any](ctx context.Context, method, path string, body io.Reader, contentType string, authenticated bool) (result T, resultErr error) {
	// 传输错误已有 URL 与错误链记录；只补齐构造、业务拒绝和响应解析失败，不记录成功读取与请求正文。
	stage, status, code := "initialize", 0, 0
	transportFailed := false
	var evidenceErr error
	defer func() {
		if resultErr != nil && !transportFailed {
			if evidenceErr == nil {
				evidenceErr = resultErr
			}
			recordRequestFailure(method, path, stage, status, code, evidenceErr)
		}
	}()
	client := core.GLOBAL.ServerClient
	if client == nil {
		return result, errors.New("服务器连接尚未初始化，请稍后重试。")
	}

	stage = "constructRequest"
	httpRequest, err := client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return result, fmt.Errorf("构造服务器请求失败：%w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	if contentType != "" {
		httpRequest.Header.Set("Content-Type", contentType)
	}
	requestToken := ""
	var requestGeneration uint64
	if authenticated {
		requestGeneration, requestToken = requestSessionSnapshot()
		// 快照前若已切换客户端，不得把新会话 token 发给先前构造请求的服务器。
		if client != core.GLOBAL.ServerClient {
			return result, errors.New("服务器已切换，请重试。")
		}
		if requestToken != "" {
			httpRequest.Header.Set(systemAPI.AccountTokenHeader, requestToken)
		}
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		transportFailed = true
		return result, fmt.Errorf("无法连接服务器 %s，请确认服务已启动，或前往全局设置检查服务器地址。", client.BaseURL())
	}
	defer response.Body.Close()
	status = response.StatusCode
	// 同一会话中的并发续期仍接受，旧会话的续期和401由同一锁下的代次检查阻断。
	if authenticated {
		valid, refreshErr, clearErr := applySessionResponse(requestGeneration, client, response.Header.Get(systemAPI.AccountNewTokenHeader), status == http.StatusUnauthorized)
		if !valid {
			return result, errors.New("账号会话或服务器已切换，请重试。")
		}
		if refreshErr != nil {
			recordRequestFailure(method, path, "persistSession", status, code, refreshErr)
		}
		if clearErr != nil {
			recordRequestFailure(method, path, "clearSession", status, code, clearErr)
		}
	}

	stage = "readResponse"
	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBodyLimit))
	if err != nil {
		evidenceErr = err
		return result, errors.New("读取服务器响应失败，请稍后重试。")
	}

	var envelope systemAPI.AccountResponse
	stage = "decodeResponse"
	if err := json.Unmarshal(raw, &envelope); err != nil {
		evidenceErr = err
		return result, fmt.Errorf("服务器返回了无法识别的数据 (HTTP %d)。", response.StatusCode)
	}
	code = envelope.Code
	if response.StatusCode == http.StatusConflict && envelope.Code == systemAPI.AccountCodeNeedChangePassword {
		setNeedChangePassword(true)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || envelope.Code != systemAPI.AccountCodeSuccess {
		stage = "businessResponse"
		if envelope.Msg != "" {
			return result, errors.New(envelope.Msg)
		}
		return result, fmt.Errorf("请求失败 (HTTP %d)。", response.StatusCode)
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return result, nil
	}
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		stage, evidenceErr = "decodeData", err
		return result, errors.New("服务器返回的数据格式不符合账号服务约定。")
	}
	return result, nil
}
