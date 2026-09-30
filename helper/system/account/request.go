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
func request[T any](ctx context.Context, method string, path string, payload interface{}, authenticated bool) (T, error) {
	var result T
	client := core.GLOBAL.ServerClient
	if client == nil {
		return result, errors.New("服务器连接尚未初始化，请稍后重试。")
	}

	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return result, fmt.Errorf("编码请求数据失败：%w", err)
		}
		body = bytes.NewReader(data)
	}

	httpRequest, err := client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return result, fmt.Errorf("构造服务器请求失败：%w", err)
	}
	httpRequest.Header.Set("Accept", "application/json")
	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	if authenticated {
		if token := Token(); token != "" {
			httpRequest.Header.Set(systemAPI.AccountTokenHeader, token)
		}
	}

	response, err := client.Do(httpRequest)
	if err != nil {
		return result, fmt.Errorf("无法连接服务器 %s，请确认服务已启动，或前往全局设置检查服务器地址。", client.BaseURL())
	}
	defer response.Body.Close()

	if refreshedToken := response.Header.Get(systemAPI.AccountNewTokenHeader); refreshedToken != "" {
		_ = SaveToken(refreshedToken)
	}
	if response.StatusCode == http.StatusUnauthorized {
		_ = ClearSession()
	}

	raw, err := io.ReadAll(io.LimitReader(response.Body, responseBodyLimit))
	if err != nil {
		return result, errors.New("读取服务器响应失败，请稍后重试。")
	}

	var envelope systemAPI.AccountResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return result, fmt.Errorf("服务器返回了无法识别的数据 (HTTP %d)。", response.StatusCode)
	}
	if response.StatusCode == http.StatusConflict && envelope.Code == systemAPI.AccountCodeNeedChangePassword {
		setNeedChangePassword(true)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || envelope.Code != systemAPI.AccountCodeSuccess {
		if envelope.Msg != "" {
			return result, errors.New(envelope.Msg)
		}
		return result, fmt.Errorf("请求失败 (HTTP %d)。", response.StatusCode)
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return result, nil
	}
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		return result, errors.New("服务器返回的数据格式不符合账号服务约定。")
	}
	return result, nil
}
