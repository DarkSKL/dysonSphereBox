package artifactfetch

// 文件职责：下载远端 V2 artifact zip 并维护下载缓存。
// 使用：ResolvePath 处理远端 artifact 时调用 downloadRemoteArtifact。
// 边界：不解析本地 artifact、不解压 zip、不复制文件、不写 installed 模型；失败时清理 partial 文件。

import (
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	progressReader "DysonSphereBox/helper/modDependence/installer/v2/progress/reader"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const directStageTimeout = 60 * time.Second

// downloadRemoteArtifact 下载远端 artifact zip 到缓存目录。
// 职责：复用有效 zip 缓存，或下载到 partial 文件后原子替换。
// 使用：ResolvePath 处理远端 artifact 时调用。
// 边界：下载取消、HTTP 非 2xx、无效 zip 或写入失败都会返回错误并清理 partial。
// 逻辑流：1. 检查取消 => 2. 命中有效缓存则返回 => 3. 解析 URL 并请求远端 => 4. 流式写入 partial => 5. 原子替换并校验 zip。
func downloadRemoteArtifact(ctx context.Context, bufferDir string, artifact v2Artifact.Artifact, progressStart int, progressEnd int) (string, error) {
	if err := cancel.Check(ctx); err != nil {
		return "", err
	}
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf("%s.zip", shared.SanitizePathSegment(artifact.ComponentKey, "artifact")))
	if fileops.IsValidZipFile(zipPath) {
		progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("%s 已使用下载缓存", artifact.DisplayName), progressEnd)
		return zipPath, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	session := captureProxySession()
	if err := fetchAndSave(ctx, artifact, partialPath, zipPath, progressStart, progressEnd, session, fetchOnce); err != nil {
		if cancelled := cancel.Check(ctx); cancelled != nil {
			return "", cancelled
		}
		return "", err
	}
	progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("%s 下载完成", artifact.DisplayName), progressEnd)
	return zipPath, nil
}

// fetchAndSave 执行直连及符合协议的一次代理回退，并校验最终缓存文件。
// 职责：协调每次 15 分钟尝试、partial 清理、回退分类和响应落盘。
// 使用：downloadRemoteArtifact 使用生产 fetchOnce；测试可注入本地 fake 响应。
// 边界：代理失败不反向回退；父 context 取消、内容错误和本地写入失败立即结束。
func fetchAndSave(ctx context.Context, artifact v2Artifact.Artifact, partialPath, zipPath string, progressStart, progressEnd int, session proxySession, fetch func(context.Context, v2Artifact.Artifact, bool, proxySession) (*http.Response, error)) error {
	return fetchAndSaveWithTimeout(ctx, artifact, partialPath, zipPath, progressStart, progressEnd, session, proxyAttemptTimeout, fetch)
}

func fetchAndSaveWithTimeout(ctx context.Context, artifact v2Artifact.Artifact, partialPath, zipPath string, progressStart, progressEnd int, session proxySession, timeout time.Duration, fetch func(context.Context, v2Artifact.Artifact, bool, proxySession) (*http.Response, error)) error {
	operationCtx := ctx
	cleanupOperation := func() {}
	if session.ctx != nil {
		operationCtx, cleanupOperation = bindSessionContext(ctx, session.ctx)
	}
	defer cleanupOperation()
	// 直连失败或 5xx 才允许对同一坐标代理一次；响应内容错误属于下载失败，不触发回退。
	var directErr error
	for attempt := 0; attempt < 2; attempt++ {
		useProxy := attempt == 1
		attemptCtx, cancelAttempt := context.WithTimeout(operationCtx, timeout)
		response, fetchErr := fetch(attemptCtx, artifact, useProxy, session)
		if fetchErr == nil {
			fetchErr = saveResponse(attemptCtx, response, partialPath, zipPath, artifact, progressStart, progressEnd, useProxy, !useProxy)
		}
		attemptContextErr := attemptCtx.Err()
		cancelAttempt()
		if response != nil {
			_ = response.Body.Close()
		}
		if fetchErr == nil {
			if session.ctx != nil && !session.valid() {
				_ = os.Remove(zipPath)
				return fmt.Errorf("盒子账号会话已变更，已丢弃下载结果")
			}
			return nil
		}
		_ = os.Remove(partialPath)
		if useProxy {
			return fallbackError{direct: directErr, proxy: fetchErr}
		}
		if !useProxy && canFallback(operationCtx, attemptContextErr, fetchErr) {
			directErr = fetchErr
			continue
		}
		if session.ctx != nil && session.ctx.Err() != nil {
			return fmt.Errorf("盒子账号会话已变更，旧下载已终止")
		}
		if isTLSIdentityError(fetchErr) || errors.Is(fetchErr, context.Canceled) {
			return safeFailure(fetchErr)
		}
		var transportFailure transportError
		var statusFailure responseStatusError
		if errors.As(fetchErr, &transportFailure) || errors.As(fetchErr, &statusFailure) {
			return safeFailure(fetchErr)
		}
		return fetchErr
	}
	return fmt.Errorf("download %s failed", artifact.DisplayName)
}

type fallbackError struct {
	direct error
	proxy  error
}

func (failure fallbackError) Error() string {
	return fmt.Sprintf("%s；%s", safeDirectError(failure.direct), safeProxyError(failure.proxy))
}

func (failure fallbackError) Unwrap() []error {
	return []error{failure.direct, failure.proxy}
}

// canFallback 在父安装仍有效时接受直连传输/5xx或单次预算超时。
// 职责：区分允许一次代理回退的 attempt deadline 与不可回退的父任务取消。
// 使用：fetchAndSave 完成直连失败分类时调用。
// 边界：证书身份、长度/ZIP、4xx 和本地写入错误仍不可回退。
func canFallback(parent context.Context, attemptErr, fetchErr error) bool {
	if parent.Err() != nil || isTLSIdentityError(fetchErr) {
		return false
	}
	if shouldFallback(fetchErr) {
		return true
	}
	return errors.Is(attemptErr, context.DeadlineExceeded) && errors.Is(fetchErr, fileops.ErrInstallCancelled)
}

// fetchOnce 执行单次直连或代理 HTTP 请求。
// 职责：统一创建带父 context 的请求并返回原始响应，供上层决定是否回退。
// 使用：downloadRemoteArtifact 每个 artifact 最多调用直连一次和代理一次。
// 边界：只处理传输与响应状态，不写 partial、不校验 ZIP；代理响应已绑定会话取消。
func fetchOnce(ctx context.Context, artifact v2Artifact.Artifact, useProxy bool, session proxySession) (*http.Response, error) {
	if useProxy {
		return fetchProxyArtifact(ctx, artifact, session)
	}
	downloadURL, err := resolveDownloadURL(artifact.DownloadURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, err
	}
	client := core.NewHTTPStandardClientWithStageTimeout(0, directStageTimeout)
	response, err := client.Do(request)
	if err != nil {
		client.CloseIdleConnections()
		return nil, transportError{err: err}
	}
	response.Body = &closeIdleResponseBody{ReadCloser: response.Body, client: client}
	return response, nil
}

type closeIdleResponseBody struct {
	io.ReadCloser
	client *http.Client
}

func (body *closeIdleResponseBody) Close() error {
	err := body.ReadCloser.Close()
	body.client.CloseIdleConnections()
	return err
}

// transportError 标记 HTTP 请求阶段的传输失败。
// 职责：将可回退的网络传输失败与本地写入/内容校验错误区分。
// 使用：fetchOnce 包装直连 Do 错误，shouldFallback 识别。
// 边界：TLS 身份错误与 context 取消仍由 shouldFallback 排除。
type transportError struct {
	// err 保存标准库返回的底层网络错误；使用：Error/Unwrap 暴露分类信息；边界：不记录或展示敏感 URL。
	err error
}

// Error 返回传输错误文本。
// 职责：满足 error 接口并保留底层原因。
// 使用：调用方记录或展示受控错误时读取。
// 边界：不添加请求 URL、token 或响应正文。
func (failure transportError) Error() string { return failure.err.Error() }

// Unwrap 暴露底层传输原因供错误分类使用。
// 职责：支持 errors.Is/errors.As 检查 context 与 TLS 类型。
// 使用：shouldFallback 对传输错误执行分类。
// 边界：不改变底层错误。
func (failure transportError) Unwrap() error { return failure.err }

// responseStatusError 保留非成功 HTTP 状态码供回退策略判断。
// 职责：区分 5xx 与不可回退状态。
// 使用：saveResponse 检测非 2xx 响应后返回。
// 边界：不携带服务端响应体或其他远端细节。
type responseStatusError struct {
	// statusCode 保存 HTTP 状态码；使用：shouldFallback 判定 5xx；边界：不包含响应正文。
	statusCode int
}

// Error 返回不含响应正文的状态错误。
// 职责：满足 error 接口。
// 使用：下载流程返回受控状态原因。
// 边界：只公开 HTTP 状态码。
func (failure responseStatusError) Error() string {
	return fmt.Sprintf("download returned HTTP %d", failure.statusCode)
}

// saveResponse 将单次响应流式写入 partial 并完成长度、ZIP 与原子替换校验。
// 职责：保证不完整或不可安装内容不会进入下载缓存。
// 使用：fetchOnce 返回成功状态后调用。
// 边界：任何读取、取消、长度、关闭、重命名或 ZIP 错误都会清理 partial；不负责回退决策。
func saveResponse(ctx context.Context, response *http.Response, partialPath, zipPath string, artifact v2Artifact.Artifact, progressStart, progressEnd int, requireLength, allowTransportFallback bool) error {
	keepFile := false
	defer func() {
		if !keepFile {
			_ = os.Remove(partialPath)
		}
	}()
	if response == nil {
		return fmt.Errorf("download response is nil")
	}
	if (requireLength && response.StatusCode != http.StatusOK) || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return httpStatusError(response.StatusCode)
	}
	if requireLength && response.ContentLength < 0 {
		return fmt.Errorf("download %s has no content length", artifact.DisplayName)
	}
	progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("开始下载 %s", artifact.DisplayName), progressStart)
	partialFile, err := os.Create(partialPath)
	if err != nil {
		return err
	}
	progressBody := progressReader.New(ctx, artifact.PackageKey, "download", fmt.Sprintf("正在下载 %s", artifact.DisplayName), response.Body, response.ContentLength, progressStart, progressEnd)
	count, copyErr := io.Copy(partialFile, progressBody)
	closeErr := partialFile.Close()
	if copyErr != nil {
		if allowTransportFallback && ctx.Err() == nil && !errors.Is(copyErr, fileops.ErrInstallCancelled) && !errors.Is(copyErr, io.ErrUnexpectedEOF) {
			var pathError *os.PathError
			if !errors.As(copyErr, &pathError) {
				return transportError{err: copyErr}
			}
		}
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := cancel.Check(ctx); err != nil {
		return err
	}
	if response.ContentLength >= 0 && count != response.ContentLength {
		return fmt.Errorf("download %s length mismatch: got %d, want %d", artifact.DisplayName, count, response.ContentLength)
	}
	if validatedBody, ok := response.Body.(interface{ Validate() error }); ok {
		if err := validatedBody.Validate(); err != nil {
			return err
		}
	}
	if err := os.Rename(partialPath, zipPath); err != nil {
		return err
	}
	if !fileops.IsValidZipFile(zipPath) {
		_ = os.Remove(zipPath)
		return fmt.Errorf("downloaded artifact is not a valid zip: %s", zipPath)
	}
	keepFile = true
	return nil
}

// httpStatusError 返回保留状态码的下载错误，供回退分类识别。
// 职责：让直连 5xx 与 4xx 具有可判定的错误语义。
// 使用：saveResponse 处理非 2xx 响应。
// 边界：不读取或透传服务端响应正文。
func httpStatusError(statusCode int) error {
	return responseStatusError{statusCode: statusCode}
}

// shouldFallback 判断直连错误是否允许对同一坐标请求代理一次。
// 职责：集中执行协议规定的传输错误/5xx回退条件。
// 使用：downloadRemoteArtifact 直连失败后调用。
// 边界：4xx、TLS 身份错误、取消、长度/ZIP/本地写入失败均不可回退。
func shouldFallback(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, fileops.ErrInstallCancelled) {
		return false
	}
	if isTLSIdentityError(err) {
		return false
	}
	var statusErr responseStatusError
	if errors.As(err, &statusErr) {
		return statusErr.statusCode >= 500 && statusErr.statusCode <= 599
	}
	var networkErr transportError
	return errors.As(err, &networkErr)
}

// isTLSIdentityError 识别证书链或主机名校验失败。
// 职责：阻止把服务器身份错误降级为代理请求。
// 使用：shouldFallback 分类直连传输错误。
// 边界：普通连接失败仍允许一次代理回退。
func isTLSIdentityError(err error) bool {
	var verificationError *tls.CertificateVerificationError
	if errors.As(err, &verificationError) {
		return true
	}
	var unknownAuthority x509.UnknownAuthorityError
	if errors.As(err, &unknownAuthority) {
		return true
	}
	var hostnameError x509.HostnameError
	if errors.As(err, &hostnameError) {
		return true
	}
	var invalidCertificate x509.CertificateInvalidError
	if errors.As(err, &invalidCertificate) {
		return true
	}
	var invalidCertificatePointer *x509.CertificateInvalidError
	return errors.As(err, &invalidCertificatePointer)
}

func safeProxyError(err error) string {
	var controlled proxyFailure
	if errors.As(err, &controlled) {
		return controlled.message
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, fileops.ErrInstallCancelled) {
		return "模组代理下载已取消"
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return "模组代理响应长度不符或传输中断"
	}
	return "模组代理下载失败"
}

func safeDirectError(err error) string {
	if isTLSIdentityError(err) {
		return "直连 TLS 证书验证失败"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, fileops.ErrInstallCancelled) {
		return "直连下载已取消"
	}
	var statusErr responseStatusError
	if errors.As(err, &statusErr) {
		return fmt.Sprintf("直连下载返回 HTTP %d", statusErr.statusCode)
	}
	var networkErr transportError
	if errors.As(err, &networkErr) {
		var timeout net.Error
		if errors.As(networkErr, &timeout) && timeout.Timeout() {
			return "直连下载超时"
		}
		return "直连网络传输失败"
	}
	return "直连下载失败"
}

func safeFailure(err error) error {
	return sanitizedFailure{message: safeDirectError(err), cause: err}
}

type sanitizedFailure struct {
	message string
	cause   error
}

func (failure sanitizedFailure) Error() string { return failure.message }

func (failure sanitizedFailure) Unwrap() error { return failure.cause }
