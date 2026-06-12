package artifact

// 文件职责：下载或复用旧官方整合包依赖 zip 缓存。
// 使用：InstallDependency 在解压依赖 artifact 前调用 downloadDependencyZip。
// 边界：只处理下载缓存文件，不解压、不复制插件目录、不写入安装结果或数据库。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/fileops"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// downloadDependencyZip 下载旧官方依赖 zip 并返回缓存路径。
// 职责：优先复用有效 zip 缓存，否则通过 HTTP 下载到 partial 后原子替换。
// 使用：InstallDependency 在解压前调用。
// 边界：HTTP 非 2xx、无效 zip 或 partial 写入失败都会返回错误；不会解压或复制文件。
// 逻辑流：1. 创建下载缓存目录 => 2. 命中有效 zip 则复用 => 3. 解析下载地址并下载到 partial => 4. 原子替换正式 zip => 5. 校验 zip 可打开。
func downloadDependencyZip(bufferDir string, dependency modDependenceResponse.ModDependencyDTO) (string, bool, error) {
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", false, err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf(
		"%d-%s-%s.zip",
		dependency.ID,
		shared.SanitizePathSegment(legacyValue.String(dependency.ModName), "mod"),
		shared.SanitizePathSegment(legacyValue.String(dependency.ModVersion), "version"),
	))
	if fileops.IsValidZipFile(zipPath) {
		return zipPath, true, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", false, err
	}

	downloadURL, err := resolveDependencyDownloadURL(legacyValue.String(dependency.Url))
	if err != nil {
		return "", false, err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", false, err
	}

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", false, err
	}

	client := http.Client{Timeout: 180 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", false, err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", false, fmt.Errorf("download %s returned HTTP %d", legacyValue.String(dependency.ModName), response.StatusCode)
	}

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", false, err
	}
	_, copyErr := io.Copy(partialFile, response.Body)
	closeErr := partialFile.Close()
	if copyErr != nil {
		_ = os.Remove(partialPath)
		return "", false, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(partialPath)
		return "", false, closeErr
	}

	if err := os.Rename(partialPath, zipPath); err != nil {
		return "", false, err
	}
	if !fileops.IsValidZipFile(zipPath) {
		_ = os.Remove(zipPath)
		return "", false, fmt.Errorf("downloaded dependency is not a valid zip: %s", zipPath)
	}

	return zipPath, false, nil
}
