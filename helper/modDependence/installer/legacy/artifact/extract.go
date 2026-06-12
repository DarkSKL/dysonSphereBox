package artifact

// 文件职责：把旧官方整合包依赖 zip 解压到本次安装缓冲目录。
// 使用：InstallDependency 下载或复用 zip 后调用 extractDependencyZip。
// 边界：只展开 zip 条目到 bufferDir 内，不复制到插件目录、不记录数据库、不发送进度事件。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	"DysonSphereBox/helper/modDependence/fileops"
	legacyValue "DysonSphereBox/helper/modDependence/installer/legacy/value"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

// extractDependencyZip 解压旧官方依赖 zip 到本次安装缓冲目录。
// 职责：重建依赖解压目录并通过 fileops 安全展开 zip 条目。
// 使用：InstallDependency 在复制插件目录前调用。
// 边界：解压目录必须位于 bufferDir 内；zip 条目越界由 fileops.ExtractZipFile 拦截。
// 逻辑流：1. 构造解压目录 => 2. 校验目录未逃逸 bufferDir => 3. 清空并重建目录 => 4. 遍历 zip 条目安全解压。
func extractDependencyZip(bufferDir string, zipPath string, dependency modDependenceResponse.ModDependencyDTO) (string, error) {
	extractPath := filepath.Join(bufferDir, fmt.Sprintf(
		"extract-%d-%s-%s",
		dependency.ID,
		shared.SanitizePathSegment(legacyValue.String(dependency.ModName), "mod"),
		shared.SanitizePathSegment(legacyValue.String(dependency.ModVersion), "version"),
	))
	if !fileops.IsSubPath(bufferDir, extractPath) {
		return "", fmt.Errorf("extract path escapes buffer directory: %s", extractPath)
	}
	if err := os.RemoveAll(extractPath); err != nil {
		return "", err
	}
	if err := os.MkdirAll(extractPath, 0o755); err != nil {
		return "", err
	}

	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if err := fileops.ExtractZipFile(file, extractPath); err != nil {
			return "", err
		}
	}

	return extractPath, nil
}
