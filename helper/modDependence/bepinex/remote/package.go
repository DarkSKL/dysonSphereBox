package remote

// 文件职责：解析并下载 Thunderstore 上的 BepInEx 官方安装包。
// 使用：BepInEx 安装流程需要最新可用包与 zip 缓存路径时调用。
// 边界：只处理远端索引、版本选择和 zip 下载，不解压、不复制到游戏目录、不修改 Doorstop 配置。

import (
	thunderstoreAPI "DysonSphereBox/apiDefinition/modDependence/thunderstore"
	bepinexConstant "DysonSphereBox/constant/modDependence/bepinex"
	"DysonSphereBox/core"
	"DysonSphereBox/helper/modDependence/bepinex/contract"
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/marketplace/thunderstoremeta"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ResolveLatestBepInExPackage 从 Thunderstore 包索引中解析最新可用的 BepInEx 包。
// 职责：从 Thunderstore 索引中定位官方 BepInEx 包并选择最新 active 版本。
// 使用：InstallBepInEx 在需要下载安装包时调用。
// 边界：只选择 active 版本；远端缺失或没有可用版本时返回错误，不使用本地缓存兜底。
// 逻辑流：1. 获取 Thunderstore 索引 => 2. 匹配 BepInEx 作者和包名 => 3. 选择最新 active 版本 => 4. 转换成安装器需要的包信息。
func ResolveLatestBepInExPackage() (*contract.BepInExPackage, error) {
	return ResolveLatestBepInExPackageWithContext(context.Background())
}

// ResolveLatestBepInExPackageWithContext 解析框架版本并响应取消。
// 职责：使首次框架安装的索引请求可取消；使用：Mod 安装准备阶段；边界：仍只选择官方 active 版本。
func ResolveLatestBepInExPackageWithContext(ctx context.Context) (*contract.BepInExPackage, error) {
	packages, err := fetchThunderstorePackagesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	for _, pkg := range packages {
		if pkg.Owner != bepinexConstant.Owner || pkg.Name != bepinexConstant.Name {
			continue
		}

		version, ok := thunderstoremeta.LatestActiveVersion(pkg.Versions)
		if !ok {
			return nil, fmt.Errorf("BepInEx has no active Thunderstore version")
		}

		return &contract.BepInExPackage{
			Owner:        pkg.Owner,
			Name:         pkg.Name,
			FullName:     version.FullName,
			Version:      version.VersionNumber,
			DownloadURL:  version.DownloadURL,
			Dependencies: append([]string(nil), version.Dependencies...),
		}, nil
	}

	return nil, fmt.Errorf("Thunderstore package not found: %s/%s", bepinexConstant.Owner, bepinexConstant.Name)
}

// fetchThunderstorePackages 请求 Thunderstore 包索引。
// 职责：加载远端完整包列表并解析为 API 契约类型。
// 使用：ResolveLatestBepInExPackage 选择 BepInEx 版本前调用。
// 边界：HTTP 非 2xx 或 JSON 解析失败时返回错误，不写入缓存。
func fetchThunderstorePackages() ([]thunderstoreAPI.Package, error) {
	return fetchThunderstorePackagesWithContext(context.Background())
}

// fetchThunderstorePackagesWithContext 请求带取消信号的官方索引。
// 职责：停止已取消的前置下载准备；使用：可取消版本解析；边界：不写市场缓存。
func fetchThunderstorePackagesWithContext(ctx context.Context) ([]thunderstoreAPI.Package, error) {
	if ctx.Err() != nil {
		return nil, fileops.ErrInstallCancelled
	}
	client := core.NewHTTPStandardClient(60 * time.Second)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, thunderstoreAPI.IndexURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Thunderstore returned HTTP %d", response.StatusCode)
	}

	var packages []thunderstoreAPI.Package
	if err := json.NewDecoder(response.Body).Decode(&packages); err != nil {
		return nil, err
	}

	return packages, nil
}

// DownloadPackage 下载 BepInEx zip 包到缓存目录。
// 职责：复用有效缓存、清理损坏缓存、以 partial 文件完成原子下载。
// 使用：InstallBepInEx 获取远端包后调用，返回可解压的 zip 路径。
// 边界：HTTP 非 2xx、写入失败或 zip 校验失败时返回错误并尽量删除损坏文件。
// 逻辑流：1. 创建下载目录 => 2. 复用有效 zip 缓存 => 3. 清理损坏/残留 partial => 4. 下载到 partial => 5. 重命名并校验 zip。
func DownloadPackage(bufferDir string, pkg *contract.BepInExPackage) (string, error) {
	return DownloadPackageWithContext(context.Background(), bufferDir, pkg)
}

// DownloadPackageWithContext 下载框架 ZIP 并传递取消上下文。
// 职责：中止网络请求和流式写入；使用：前置安装准备；边界：取消删除 partial，完整缓存仍可复用。
func DownloadPackageWithContext(ctx context.Context, bufferDir string, pkg *contract.BepInExPackage) (string, error) {
	if ctx.Err() != nil {
		return "", fileops.ErrInstallCancelled
	}
	downloadDir := filepath.Join(bufferDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		return "", err
	}

	zipPath := filepath.Join(downloadDir, fmt.Sprintf("%s-%s-%s.zip", pkg.Owner, pkg.Name, pkg.Version))
	if fileops.IsValidZipFile(zipPath) {
		return zipPath, nil
	}
	if err := os.Remove(zipPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	partialPath := zipPath + ".partial"
	if err := os.Remove(partialPath); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	client := core.NewHTTPStandardClient(180 * time.Second)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download %s/%s returned HTTP %d", pkg.Owner, pkg.Name, response.StatusCode)
	}

	partialFile, err := os.Create(partialPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(partialFile, response.Body)
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
		return "", fmt.Errorf("downloaded file is not a valid zip: %s", zipPath)
	}

	return zipPath, nil
}
