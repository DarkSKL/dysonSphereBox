package server

// 文件职责：实现服务器地址的只读连接测试。
// 使用：系统服务 ServerService.ProbeServerAddress 在用户点击“测试连接”时调用。
// 边界：只请求无需鉴权的验证码接口并校验响应结构，不保存地址、不写入 token、不记录日志。
// 逻辑流：1. 拼接验证码接口地址 => 2. 带超时发起 POST 请求 => 3. 读取并解析响应体 => 4. 校验 {code} 结构 => 5. 返回测试结论。

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	systemAPI "DysonSphereBox/apiDefinition/system"
)

// probeTimeout 表示连接测试的总超时时间。
// 职责：限制测试请求最长等待时间，避免设置页长时间停留在测试中。
// 使用：Probe 创建带超时的请求上下文时读取。
// 边界：超时后直接判定失败，不重试。
const probeTimeout = 6 * time.Second

// probeBodyLimit 表示连接测试读取响应体的最大字节数。
// 职责：防止目标地址返回超大响应体占用内存。
// 使用：Probe 读取响应体时通过 io.LimitReader 限制。
// 边界：验证码接口响应远小于该值，超出部分直接丢弃。
const probeBodyLimit = 256 << 10

// Probe 测试目标地址是否为可用的账号服务。
// 职责：请求目标地址的验证码接口，并根据网络结果和响应结构生成测试结论。
// 使用：调用方传入已通过 utils.NormalizeHTTPBaseURL 归一化的基础地址。
// 边界：任何失败都以 ProbeResult.OK=false 返回，不向调用方返回错误；保留标准 TLS 证书校验。
func Probe(ctx context.Context, address string) *ProbeResult {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	startedAt := time.Now()
	elapsed := func() int64 { return time.Since(startedAt).Milliseconds() }

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address+systemAPI.APIURLCaptcha, nil)
	if err != nil {
		return &ProbeResult{Message: "无法构造测试请求，请检查地址格式。"}
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return &ProbeResult{LatencyMs: elapsed(), Message: fmt.Sprintf("连接超时（%d 秒无响应）。", int(probeTimeout.Seconds()))}
		}
		return &ProbeResult{LatencyMs: elapsed(), Message: "无法连接到该地址，请检查地址和服务状态。"}
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, probeBodyLimit))
	latency := elapsed()
	if err != nil {
		return &ProbeResult{LatencyMs: latency, Message: "读取服务响应失败，请稍后重试。"}
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return &ProbeResult{LatencyMs: latency, Message: fmt.Sprintf("服务有响应 (%d)，但不是可识别的账号服务。", response.StatusCode)}
	}
	if _, ok := envelope["code"]; !ok {
		return &ProbeResult{LatencyMs: latency, Message: "服务有响应，但返回结构不符合账号服务约定。"}
	}

	return &ProbeResult{OK: true, LatencyMs: latency, Message: fmt.Sprintf("连接正常，响应耗时 %d ms。", latency)}
}
