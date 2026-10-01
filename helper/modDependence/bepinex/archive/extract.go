package archive

// 文件职责：安全解压 BepInEx 官方 zip 包。
// 使用：bepinex 安装流程下载 zip 后调用，用于获取 BepInExPack 解压目录。
// 边界：只处理 zip 解压，不校验 BepInExPack 业务结构、不复制文件到游戏目录、不修改配置。

import (
	"DysonSphereBox/helper/modDependence/bepinex/contract"
	"DysonSphereBox/helper/modDependence/fileops"
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// ExtractPackage 将 BepInEx 下载包解压到专用缓存目录。
// 职责：创建干净解压目录并逐项安全写出 zip 内容。
// 使用：InstallBepInEx 下载 zip 后调用，返回 BepInExPack 所在的上级解压目录。
// 边界：解压目标必须位于缓存目录内；不校验包业务结构，BepInExPack 检查由安装函数完成。
// 逻辑流：1. 计算解压目录 => 2. 校验目录不越界 => 3. 清空旧解压目录并重建 => 4. 遍历 zip 条目安全解压 => 5. 返回解压路径。
func ExtractPackage(bufferDir string, zipPath string, pkg *contract.BepInExPackage) (string, error) {
	return ExtractPackageWithContext(context.Background(), bufferDir, zipPath, pkg)
}

// ExtractPackageWithContext 安全解压前置框架并传递取消信号。
// 职责：阻止取消后继续解压缓存；使用：可取消 BepInEx 安装；边界：只处理专用缓存目录，不写游戏文件。
func ExtractPackageWithContext(ctx context.Context, bufferDir string, zipPath string, pkg *contract.BepInExPackage) (string, error) {
	if ctx.Err() != nil {
		return "", fileops.ErrInstallCancelled
	}
	extractPath := filepath.Join(bufferDir, fmt.Sprintf("extract-%s-%s-%s", pkg.Owner, pkg.Name, pkg.Version))
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
		if err := fileops.ExtractZipFileWithContext(ctx, file, extractPath); err != nil {
			return "", err
		}
	}

	return extractPath, nil
}
