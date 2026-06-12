package artifactextract

// 文件职责：负责 V2 artifact zip 的安全解压。
// 使用：artifactinstall.InstallFiles 在远端或本地 zip artifact 下载完成后调用。
// 边界：只解压到缓存目录，不规划目标路径、不复制到游戏目录、不写数据库。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/progresscalc"
	"DysonSphereBox/helper/modDependence/installer/shared"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	progressEvent "DysonSphereBox/helper/modDependence/installer/v2/progress/event"
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ExtractZip 解压 artifact zip 到当前组件的缓存目录。
// 职责：重建解压目录、逐项安全展开 zip 条目并上报解压进度。
// 使用：artifactinstall.InstallFiles 处理 zip artifact 时调用。
// 边界：解压目录必须位于 bufferDir 内；zip 条目越界由 fileops.ExtractZipFile 拦截。
// 逻辑流：1. 生成组件解压目录 => 2. 清理旧目录并重建 => 3. 打开 zip => 4. 逐项检查取消并解压 => 5. 返回解压目录。
func ExtractZip(ctx context.Context, bufferDir string, zipPath string, artifact v2Artifact.Artifact, progressStart int, progressEnd int) (string, error) {
	extractPath := filepath.Join(bufferDir, fmt.Sprintf("extract-%s", shared.SanitizePathSegment(artifact.ComponentKey, "artifact")))
	if !fileops.IsSubPath(bufferDir, extractPath) {
		return "", fmt.Errorf("extract path escapes buffer directory: %s", extractPath)
	}
	if err := cancel.Check(ctx); err != nil {
		return "", err
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

	totalFiles := len(reader.File)
	for index, file := range reader.File {
		if err := cancel.Check(ctx); err != nil {
			return "", err
		}
		progressEvent.Package(artifact.PackageKey, "extract", fmt.Sprintf("正在解压文件 %d/%d", index+1, totalFiles), progresscalc.Map(progressStart, progressEnd, progresscalc.Percent(int64(index+1), int64(max(totalFiles, 1)))))
		if err := fileops.ExtractZipFile(file, extractPath); err != nil {
			return "", err
		}
	}
	return extractPath, nil
}
