package artifactfetch

// 文件职责：下载远端 V2 artifact zip 并维护下载缓存。
// 使用：ResolvePath 处理远端 artifact 时调用 downloadRemoteArtifact。
// 边界：不解析本地 artifact、不解压 zip、不复制文件、不写 installed 模型；失败时清理 partial 文件。

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	progressReader "DysonSphereBox/helper/modDependence/installer/v2/progress/reader"
)

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

	downloadURL, err := resolveDownloadURL(artifact.DownloadURL)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", err
	}
	client := http.Client{Timeout: 180 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download %s returned HTTP %d", artifact.DisplayName, response.StatusCode)
	}
	progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("开始下载 %s", artifact.DisplayName), progressStart)

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", err
	}
	progressBody := progressReader.New(ctx, artifact.PackageKey, "download", fmt.Sprintf("正在下载 %s", artifact.DisplayName), response.Body, response.ContentLength, progressStart, progressEnd)
	_, copyErr := io.Copy(partialFile, progressBody)
	closeErr := partialFile.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return "", copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return "", closeErr
	}

	if err := os.Rename(partialPath, zipPath); err != nil {
		return "", err
	}
	if !fileops.IsValidZipFile(zipPath) {
		_ = os.Remove(zipPath)
		return "", fmt.Errorf("downloaded artifact is not a valid zip: %s", zipPath)
	}
	progressEvent.Package(artifact.PackageKey, "download", fmt.Sprintf("%s 下载完成", artifact.DisplayName), progressEnd)
	return zipPath, nil
}
