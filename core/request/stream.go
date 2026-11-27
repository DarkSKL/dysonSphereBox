package request

// 文件职责：安全打开文件、Range 和 SSE 响应流并管理生命周期。
// 使用：适配器持有 HTTP 响应，业务回调消费合法的流。
// 边界：流不进入结果缓存，回调返回后关闭响应体。

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
)

// Stream 职责：发送并消费经认证的流；使用：资源下载、媒体和 SSE；边界：文件通过消费端完整校验后才接纳续期。
func Stream(ctx context.Context, adapter *Adapter, desc *api.ApiDesc, input Input, consume func(context.Context, *http.Response) error) error {
	c, err := adapter.bind(ctx, desc, input, true)
	if err != nil {
		return err
	}
	ctx, done := c.networkContext(ctx)
	defer done()
	resp, err := c.send(ctx)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !c.validStream(resp) {
		if resp.StatusCode == http.StatusOK && c.plan.desc.Response != api.FileResponse {
			return &Error{Kind: InvalidData, Status: resp.StatusCode}
		}
		if _, _, err := decodeJSON[json.RawMessage](ctx, c, resp); err != nil {
			return err
		}
		return &Error{Kind: InvalidData, Status: resp.StatusCode}
	}
	update := AuthUpdate{Refreshed: resp.Header.Get(systemAPI.AccountNewTokenHeader), Success: true}
	file := c.plan.desc.Response == api.FileResponse
	if !file {
		if err := c.accept(ctx, update); err != nil {
			return err
		}
	}
	if err := c.check(ctx); err != nil {
		return err
	}
	err = consume(ctx, resp)
	if interrupted := c.check(ctx); interrupted != nil {
		return interrupted
	}
	if err == nil && file {
		return c.accept(ctx, update)
	}
	return err
}

// validStream 只接受各传输模式承诺的状态及媒体类型。
func (c call) validStream(resp *http.Response) bool {
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	switch c.plan.desc.Response {
	case api.FileResponse:
		return resp.StatusCode == http.StatusOK && mediaType != "application/json"
	case api.RangeResponse:
		return mediaType != "application/json" && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable)
	case api.SSEResponse:
		return resp.StatusCode == http.StatusOK && mediaType == "text/event-stream"
	default:
		return false
	}
}
