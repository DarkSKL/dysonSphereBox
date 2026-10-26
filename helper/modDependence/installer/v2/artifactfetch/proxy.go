package artifactfetch

// 文件职责：构造并执行当前账号会话绑定的模组代理请求。
// 使用：远端 artifact 直连符合回退条件时，由下载流程针对同一坐标调用。
// 边界：token 只放 x-token 请求头；会话切换会取消请求，迟到响应不得用于安装。
// 逻辑流：1. 捕获已验证会话 => 2. 合并安装与会话取消上下文 => 3. 按三个坐标参数请求当前 server => 4. 校验会话仍有效并返回响应。

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	modDependenceAPI "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/core"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/system/account"
)

// proxyAttemptTimeout 是单次直连或代理请求的最大总预算。
// 职责：限制一次完整下载尝试的总耗时。
// 使用：下载流程从安装父 context 派生每次尝试的 deadline。
// 边界：父 context 或账号会话取消优先生效，不延长父级预算。
const proxyAttemptTimeout = 15 * time.Minute

// sessionBoundBody 将请求取消资源绑定到响应体生命周期。
// 职责：确保响应读取期间保留安装/会话取消联动，并在关闭时释放 context 回调。
// 使用：fetchProxyArtifact 返回成功响应前包装其 Body。
// 边界：只管理响应体关闭，不缓存或修改文件内容。
type sessionBoundBody struct {
	// ReadCloser 保存原始网络响应体；使用：Read 委托读取；边界：底层关闭由 Close 执行。
	io.ReadCloser
	// cleanup 释放 context 取消回调和尝试 deadline；使用：Close 调用一次；边界：不改变请求结果。
	cleanup func()
	// valid 检查响应读取完成时捕获会话仍为当前会话；使用：EOF 前验证迟到响应；边界：不访问 token 内容以外的数据。
	valid func() bool
}

type proxySession struct {
	generation uint64
	subject    string
	serverKey  string
	token      string
	ctx        context.Context
	client     *core.HTTPClient
	current    func() bool
}

func captureProxySession() proxySession {
	generation, subject, serverKey, token, sessionCtx, client, ok := account.VerifiedSession()
	if !ok {
		return proxySession{}
	}
	return proxySession{generation: generation, subject: subject, serverKey: serverKey, token: token, ctx: sessionCtx, client: client}
}

func (session proxySession) valid() bool {
	if session.current != nil {
		return session.current()
	}
	if session.ctx == nil {
		return false
	}
	generation, subject, serverKey, token, _, client, ok := account.VerifiedSession()
	return ok && generation == session.generation && subject == session.subject && serverKey == session.serverKey && token == session.token && client == session.client
}

// Read 读取代理文件并在响应结束时确认原会话仍有效。
// 职责：将会话切换期间已返回的迟到响应转成下载失败。
// 使用：下载器通过响应 Body 读取代理文件。
// 边界：不吞掉底层读取错误；失效时 partial 由下载器清理。
func (body *sessionBoundBody) Read(buffer []byte) (int, error) {
	count, err := body.ReadCloser.Read(buffer)
	if err == io.EOF && !body.valid() {
		return count, fmt.Errorf("盒子账号会话已变更，已丢弃代理响应")
	}
	return count, err
}

func (body *sessionBoundBody) Validate() error {
	if !body.valid() {
		return proxyFailure{message: "盒子账号会话已变更，已丢弃代理响应"}
	}
	return nil
}

// bindSessionContext 将账号会话取消传播到单次代理尝试。
// 职责：任一父安装任务或账号会话取消时结束请求。
// 使用：fetchProxyArtifact 构造请求 context。
// 边界：只传播取消，不改变父 context 的 deadline 或附加身份信息。
func bindSessionContext(parent, session context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	stopSessionCancel := context.AfterFunc(session, cancel)
	return ctx, func() {
		stopSessionCancel()
		cancel()
	}
}

// Close 关闭代理响应并释放会话 context 关联。
// 职责：关闭网络流并执行一次性 context 清理。
// 使用：下载流程完成、失败或取消时关闭响应体。
// 边界：底层关闭错误仍返回给调用方。
func (body *sessionBoundBody) Close() error {
	err := body.ReadCloser.Close()
	body.cleanup()
	return err
}

// newProxyRequest 按捕获的 server 配置和 artifact 坐标构建代理 GET 请求。
// 职责：将协议规定的固定路径、三个 query 和 token 请求头绑定到同一会话。
// 使用：fetchProxyArtifact 在 VerifiedSession 成功后调用。
// 边界：不传 userId、来源覆盖、Range 或 token query；server URL 仅来自捕获客户端。
func newProxyRequest(ctx context.Context, client *core.HTTPClient, artifact v2Artifact.Artifact, token string) (*http.Request, error) {
	requestURL, err := client.ResolveURL(modDependenceAPI.APIURLDownloadPackage)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return nil, err
	}
	query := url.Values{}
	query.Set("namespace", artifact.Namespace)
	query.Set("name", artifact.Name)
	query.Set("version", artifact.Version)
	parsedURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("x-token", token)
	return request, nil
}

// fetchProxyArtifact 使用捕获的账号会话请求当前盒子服务器代理文件。
// 职责：在安装与会话任一取消时停止代理请求，并拒绝切服/换号后的迟到响应。
// 使用：直连下载器确认可回退且父 context 有效后调用。
// 边界：未登录、改密、配置缺失或旧会话均返回错误；不以新身份重放旧请求。
func fetchProxyArtifact(ctx context.Context, artifact v2Artifact.Artifact, session proxySession) (*http.Response, error) {
	if !session.valid() {
		return nil, proxyFailure{message: "模组代理需要有效的盒子账号会话"}
	}
	serverClient, err := core.NewHTTPVerifiedClient(session.serverKey)
	if err != nil {
		return nil, proxyFailure{message: "模组代理服务地址不可用"}
	}
	attemptCtx, cleanup := bindSessionContext(ctx, session.ctx)
	request, err := newProxyRequest(attemptCtx, serverClient, artifact, session.token)
	if err != nil {
		cleanup()
		serverClient.CloseIdleConnections()
		return nil, proxyFailure{message: "无法创建模组代理请求"}
	}
	response, err := serverClient.DoNoRedirect(request)
	if err != nil {
		cleanup()
		serverClient.CloseIdleConnections()
		if errors.Is(attemptCtx.Err(), context.Canceled) {
			return nil, proxyFailure{message: "盒子账号会话已变更或下载已取消"}
		}
		return nil, proxyFailure{message: "模组代理服务连接失败"}
	}
	if !session.valid() {
		_ = response.Body.Close()
		cleanup()
		serverClient.CloseIdleConnections()
		return nil, proxyFailure{message: "盒子账号会话已变更，已丢弃代理响应"}
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		cleanup()
		serverClient.CloseIdleConnections()
		return nil, proxyStatusError(response.StatusCode)
	}
	if response.ContentLength < 0 {
		_ = response.Body.Close()
		cleanup()
		serverClient.CloseIdleConnections()
		return nil, proxyFailure{message: "模组代理响应缺少有效长度"}
	}
	response.Body = &sessionBoundBody{
		ReadCloser: response.Body,
		cleanup: func() {
			cleanup()
			serverClient.CloseIdleConnections()
		},
		valid: session.valid,
	}
	return response, nil
}

func proxyStatusError(status int) error {
	message := "模组代理服务暂不可用"
	switch status {
	case http.StatusUnauthorized:
		message = "盒子账号登录已失效，请重新登录"
	case http.StatusConflict:
		message = "盒子账号需要先修改密码"
	case http.StatusNotFound:
		message = "代理未找到对应的模组版本"
	case http.StatusRequestEntityTooLarge:
		message = "模组超过代理服务允许的大小"
	case http.StatusTooManyRequests:
		message = "模组代理繁忙，请稍后重试"
	case http.StatusBadGateway:
		message = "模组来源暂不可用"
	case http.StatusServiceUnavailable:
		message = "模组代理当前未启用"
	case http.StatusGatewayTimeout:
		message = "模组代理请求超时"
	}
	return proxyFailure{message: message}
}

type proxyFailure struct{ message string }

func (failure proxyFailure) Error() string { return failure.message }
