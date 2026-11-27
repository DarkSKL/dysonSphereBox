package request

// 文件职责：有界解码 JSON 外壳并处理经过识别的认证应答。
// 使用：JSON 请求和流式接口的错误响应。
// 边界：业务失败与校验失败不会进入结果缓存或成功中间件。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// envelope 保留公共协议字段，使用指针区分成功码零与字段缺失。
type envelope struct {
	Code *int            `json:"code"`
	Data json.RawMessage `json:"data"`
	Msg  *string         `json:"msg"`
}

// responseData 保存有效结果及其新鲜度依据，供缓存适配使用。
type responseData struct {
	data   json.RawMessage
	header http.Header
}

// fetchJSON 在真实网络读取完成之后执行认证响应和成功中间件。
func fetchJSON[T any](ctx context.Context, adapter *Adapter, c call) (result T, metadata responseData, err error) {
	ctx, done := c.networkContext(ctx)
	defer done()
	resp, err := c.send(ctx)
	if err != nil {
		return result, metadata, err
	}
	defer resp.Body.Close()
	result, data, err := decodeJSON[T](ctx, c, resp)
	if err != nil {
		return result, metadata, err
	}
	for _, after := range c.plan.after {
		for _, source := range after(ctx, Success{API: c.plan.desc.ID, Target: c.input.Target, Data: bytes.Clone(data)}) {
			adapter.cache.Invalidate(c.session.Scope, source)
		}
	}
	if err := c.check(ctx); err != nil {
		var zero T
		return zero, metadata, err
	}
	return result, responseData{data: data, header: resp.Header}, nil
}

// readEnvelope 检查完整大小、编码和描述所要求的协议字段。
func readEnvelope(c call, resp *http.Response) (envelope, error) {
	var result envelope
	status, policy := resp.StatusCode, c.plan.desc.Envelope
	limit := c.plan.desc.ResponseLimit
	raw, err := io.ReadAll(io.LimitReader(resp.Body, int64(limit)+1))
	if err != nil || len(raw) > limit {
		return result, &Error{Kind: ReadFailed, Status: status}
	}
	if !utf8.Valid(raw) || json.Unmarshal(raw, &result) != nil || result.Code == nil ||
		(policy.RequireMessage && result.Msg == nil) || (policy.RequireData && len(result.Data) == 0) {
		return result, &Error{Kind: InvalidEnvelope, Status: status}
	}
	return result, nil
}

// decodeJSON 校验认证状态与业务结果，业务错误携带原协议码以供领域映射。
func decodeJSON[T any](ctx context.Context, c call, resp *http.Response) (result T, raw json.RawMessage, err error) {
	status := resp.StatusCode
	if status == http.StatusUnauthorized && c.plan.authenticated {
		return result, nil, c.rejectUnauthorized(ctx, status)
	}
	if status == http.StatusNotFound && !c.plan.desc.Envelope.RenewOnHTTPError {
		return result, nil, &Error{Kind: HTTPFailed, Status: status}
	}
	data, err := readEnvelope(c, resp)
	if interrupted := c.check(ctx); interrupted != nil {
		return result, nil, interrupted
	}
	if err != nil {
		if status != http.StatusOK && !c.plan.desc.Envelope.RenewOnHTTPError {
			return result, nil, &Error{Kind: HTTPFailed, Status: status}
		}
		return result, nil, err
	}
	code := *data.Code
	update := AuthUpdate{Refreshed: resp.Header.Get(systemAPI.AccountNewTokenHeader)}
	if c.plan.authenticated && status == http.StatusConflict && code == systemAPI.AccountCodeNeedChangePassword && c.isPasswordRequired(data.Data) {
		update.NeedPassword = true
		if err := c.accept(ctx, update); err != nil {
			return result, nil, err
		}
		return result, nil, &Error{Kind: PasswordRequired, Status: status, Code: code, Message: message(data)}
	}
	if status == http.StatusOK && code == systemAPI.AccountCodeSuccess {
		if c.plan.desc.Envelope.ObjectData && (len(data.Data) == 0 || data.Data[0] != '{') {
			return result, nil, &Error{Kind: InvalidData, Status: status}
		}
		if len(data.Data) > 0 && string(data.Data) != "null" {
			if err := json.Unmarshal(data.Data, &result); err != nil {
				var zero T
				return zero, nil, &Error{Kind: InvalidData, Status: status}
			}
		}
		update.Success, update.Data = true, data.Data
	}
	if status == http.StatusOK || c.plan.desc.Envelope.RenewOnHTTPError {
		if err := c.accept(ctx, update); err != nil {
			var zero T
			return zero, nil, err
		}
	}
	if err := c.check(ctx); err != nil {
		var zero T
		return zero, nil, err
	}
	if status != http.StatusOK {
		return result, nil, &Error{Kind: HTTPFailed, Status: status, Code: code, Message: message(data), Data: data.Data, Protocol: true}
	}
	if !update.Success {
		return result, nil, &Error{Kind: BusinessFailed, Status: status, Code: code, Message: message(data), Data: data.Data, Protocol: true}
	}
	return result, data.Data, nil
}

// accept 只在原会话仍然有效时交给会话提供者提交认证响应。
func (c call) accept(ctx context.Context, update AuthUpdate) error {
	if err := c.check(ctx); err != nil {
		return err
	}
	if c.session.Accept == nil {
		return nil
	}
	return c.session.Accept(update)
}

// rejectUnauthorized 处理无需 JSON 外壳的明确鉴权拒绝。
func (c call) rejectUnauthorized(ctx context.Context, status int) error {
	if err := c.accept(ctx, AuthUpdate{Unauthorized: true}); err != nil {
		return err
	}
	return &Error{Kind: Unauthorized, Status: status}
}

// isPasswordRequired 按接口约定从明确业务码和可选标志识别强制改密。
func (c call) isPasswordRequired(raw json.RawMessage) bool {
	if !c.plan.desc.Envelope.PasswordFlag {
		return true
	}
	var data struct {
		NeedChangePassword bool `json:"needChangePassword"`
	}
	return json.Unmarshal(raw, &data) == nil && data.NeedChangePassword
}

// message 读取可选协议提示，调用方决定是否对用户展示。
func message(data envelope) string {
	if data.Msg != nil {
		return *data.Msg
	}
	return ""
}

// Details 职责：取得请求阶段详情；使用：领域层错误映射；边界：非请求错误返回 nil。
func Details(err error) *Error {
	var failure *Error
	if errors.As(err, &failure) {
		return failure
	}
	return nil
}
