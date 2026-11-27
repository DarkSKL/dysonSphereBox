package notification

// 职责：读取一次桌面通知 SSE；使用：应用通知会话循环；边界：本文件不重连、不解析消息正文。

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"time"

	systemAPI "DysonSphereBox/apiDefinition/system"
	systemConstant "DysonSphereBox/constant/system"
	"DysonSphereBox/core/request"
	"DysonSphereBox/helper/system/account"
)

// Stream 职责：从同一凭据建立并读取一条流；使用：会话循环串行调用；边界：无握手或两次心跳间隔无数据时取消。
// 逻辑流：1. 请求固定目标 => 2. 校验响应类型与续期 => 3. 按完整事件帧转发刷新信号。
func (s Session) Stream(parent context.Context, emit func(string)) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	watchdog := time.AfterFunc(15*time.Second, cancel)
	defer watchdog.Stop()
	_ = request.Stream(ctx, account.ServerRequests(), systemAPI.NotificationStreamAPI, request.Input{Session: &s.access},
		func(ctx context.Context, resp *http.Response) error {
			s.consumeEvents(ctx, resp, watchdog, emit)
			return nil
		})
}

// consumeEvents 处理已经过握手和认证校验的一条连接，重连由服务层负责。
func (s Session) consumeEvents(ctx context.Context, resp *http.Response, watchdog *time.Timer, emit func(string)) {
	const idleTimeout = 70 * time.Second
	watchdog.Reset(idleTimeout)
	scanner := bufio.NewScanner(resp.Body)
	var name string
	var hasData, connected bool
	for scanner.Scan() {
		if ctx.Err() != nil || s.Context.Err() != nil {
			return
		}
		watchdog.Reset(idleTimeout)
		line := scanner.Text()
		switch {
		case line == "":
			if hasData && (name == systemConstant.NotificationMessage || name == systemConstant.NotificationRecall) {
				emit(name)
			}
			name, hasData = "", false
		case !connected && line == ": connected":
			connected = true
			emit(systemConstant.NotificationConnected)
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case line == "data" || strings.HasPrefix(line, "data:"):
			hasData = true
		}
	}
}
