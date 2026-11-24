package feedback

// 文件职责：执行星系桥接的有界 HTTP 请求和会话校验。
// 使用：由反馈 Wails 方法传入固定路由、查询与请求体。
// 边界：禁止重定向和自动重试，错误与日志只使用本地生成的提示。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/system/account"
)

// requestTimeout 限制一次桥接传输的总时间，包含响应体读取。
const requestTimeout = 10 * time.Second

// requestLimit 限制实际发送的 JSON 字节数，超限时请求不出本机。
const requestLimit = 16 << 10

// responseLimit 限制读取到的响应字节数，超限时丢弃整条响应。
const responseLimit = 4 << 20

// requestSession 固定一次调用的已验证账号和 HTTP 目标，只在 Go 侧传递。
type requestSession struct {
	// generation 区分同一账号的不同登录实例，切换时旧请求失效。
	generation uint64
	// token 用于请求鉴权和续期接纳，不参与 Wails 会话键。
	token string
	// ctx 在账号或服务器切换时取消。
	ctx context.Context
	// client 指向捕获时的共享 HTTP 客户端，不随全局替换改绑。
	client *core.HTTPClient
}

// captureSession 取得一次已验证会话并核对页面作用域，未登录或旧页面调用不会发请求。
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

// Do 职责：执行固定目标请求并解码类型化结果；使用：星系服务的 GET 与 POST；边界：只接纳当前会话的响应。
// 逻辑流：1. 捕获已验证会话与取消信号 => 2. 发送有界请求 => 3. 核对会话并接纳认证副作用 => 4. 投影业务结果。
func Do[T any](caller context.Context, sessionKey, method, path string, query url.Values, payload any) (result T, resultErr error) {
	ctx, cancel := context.WithTimeout(caller, requestTimeout)
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
		if err != nil || len(raw) > requestLimit {
			return result, failure("invalidInput", "反馈数据无效或超过 16 KiB")
		}
		body = bytes.NewReader(raw)
	}
	req, err := session.client.NewRequest(ctx, method, strings.TrimPrefix(path, "/"), body)
	if err != nil {
		return result, failure("requestFailed", "无法构造反馈请求")
	}
	req.URL.RawQuery = query.Encode()
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
		return result, failure("requestFailed", "无法连接反馈服务，请稍后重试")
	}
	defer resp.Body.Close()
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
		return result, failure("unsupported", "当前服务器暂未提供反馈星系")
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, responseLimit+1))
	if interrupted := session.interrupted(ctx); interrupted != nil {
		return result, interrupted
	}
	if err != nil || len(raw) > responseLimit {
		return result, failure("requestFailed", "反馈响应读取失败或超过 4 MiB")
	}
	var envelope struct {
		Code *int            `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  *string         `json:"msg"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Code == nil || len(envelope.Data) == 0 || envelope.Msg == nil {
		return result, failure("requestFailed", "反馈响应格式无效")
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
		return result, failure("requestFailed", "反馈服务请求失败，请稍后重试")
	}
	if code == systemAPI.AccountCodeSuccess {
		if envelope.Data[0] != '{' || json.Unmarshal(envelope.Data, &result) != nil {
			var zero T
			return zero, failure("requestFailed", "反馈响应数据无效")
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

// interrupted 将账号失效、调用取消和总超时转成不同的页面原因，切换账号优先。
func (session requestSession) interrupted(ctx context.Context) error {
	switch {
	case session.ctx.Err() != nil:
		return failure("stale", "账号会话已切换")
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return failure("timeout", "反馈请求超时，请稍后重试")
	case ctx.Err() != nil:
		return failure("canceled", "反馈请求已取消")
	default:
		return nil
	}
}

// acceptAuth 复用账号模块的原子鉴权更新，旧凭据的 401/409 不影响新凭据。
func (session requestSession) acceptAuth(refresh string, unauthorized, needPassword bool) bool {
	return account.HandleAuthenticatedResponse(session.generation, session.token, session.client, refresh, unauthorized, needPassword)
}
