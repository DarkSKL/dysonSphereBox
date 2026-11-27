package server

// 文件职责：对设置页尚未保存的盒子服务器地址进行连接测试。
// 使用：系统服务 ProbeServerAddress。
// 边界：候选地址经共享适配器执行公开请求，不接触已登录会话。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	api "DysonSphereBox/apiDefinition"
	systemAPI "DysonSphereBox/apiDefinition/system"
	"DysonSphereBox/core"
	"DysonSphereBox/core/request"
	"DysonSphereBox/core/requestcache"
	"DysonSphereBox/helper/system/account"
)

// Probe 职责：校验候选地址的账号协议响应；使用：设置页连接测试；边界：只报告连通性，不替换全局客户端。
func Probe(ctx context.Context, address string) *ProbeResult {
	startedAt := time.Now()
	client, err := core.NewServerClient(address)
	if err != nil {
		return &ProbeResult{Message: "无法构造测试请求，请检查地址格式。"}
	}
	session := request.Session{Mode: api.AuthNone, Transport: client, Scope: requestcache.Scope{Context: ctx, ID: client.BaseURL()}}
	_, err = request.Do[json.RawMessage](ctx, account.ServerRequests(), systemAPI.ProbeAPI, request.Input{Session: &session})
	latency := time.Since(startedAt).Milliseconds()
	detail := request.Details(err)
	if err == nil || (detail != nil && detail.Protocol) {
		return &ProbeResult{OK: true, LatencyMs: latency, Message: fmt.Sprintf("连接正常，响应耗时 %d ms。", latency)}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &ProbeResult{LatencyMs: latency, Message: fmt.Sprintf("连接超时（%d 秒无响应）。", int(systemAPI.ProbeAPI.Timeout.Seconds()))}
	}
	if detail != nil {
		switch detail.Kind {
		case request.ReadFailed:
			return &ProbeResult{LatencyMs: latency, Message: "读取服务响应失败，请稍后重试。"}
		case request.InvalidEnvelope:
			return &ProbeResult{LatencyMs: latency, Message: fmt.Sprintf("服务有响应 (%d)，但不是可识别的账号服务。", detail.Status)}
		}
	}
	return &ProbeResult{LatencyMs: latency, Message: "无法连接到该地址，请检查地址和服务状态。"}
}
