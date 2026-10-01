package artifactplan

// 文件职责：扫描 artifact 解压目录或本地目录，生成 V2 安装文件计划。
// 使用：V2 artifact 安装流程在下载/解压完成后调用，获取后续复制和持久化所需的 installplan.File。
// 边界：不下载、不解压、不复制文件、不写 manifest；框架包拦截由 layout.go 维护。
// 逻辑流：1. 拦截框架包 => 2. 遍历目录文件 => 3. 按 R2 安装规则计算目标相对路径 => 4. 校验目标不越出游戏目录 => 5. 计算哈希并返回文件计划。

import (
	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/helper/modDependence/installer/artifactlayout"
	v2Artifact "DysonSphereBox/helper/modDependence/installer/v2/artifact"
	"DysonSphereBox/helper/modDependence/installer/v2/cancel"
	"DysonSphereBox/helper/modDependence/installer/v2/filemeta"
	"DysonSphereBox/helper/modDependence/installer/v2/installplan"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// PlanFiles 扫描解压目录并生成安装文件计划。
// 职责：计算目标相对路径、校验目标位于游戏目录内并生成文件哈希。
// 使用：artifact 安装流程在 zip 解压后或本地目录安装时调用。
// 边界：目录中没有可安装文件会返回错误；不执行实际复制。
func PlanFiles(ctx context.Context, extractPath string, gameDir string, artifact v2Artifact.Artifact) ([]installplan.File, error) {
	if err := validateInstallLayout(extractPath, artifact); err != nil {
		return nil, err
	}

	files := make([]installplan.File, 0)
	err := filepath.WalkDir(extractPath, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if err := cancel.Check(ctx); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(extractPath, sourcePath)
		if err != nil {
			return err
		}
		normalizedRelativePath := filepath.ToSlash(relativePath)

		target := artifactlayout.ResolveTarget(normalizedRelativePath, artifact.Descriptor())
		targetRelativePath := target.RelativePath
		targetPath := filepath.Join(gameDir, targetRelativePath)
		if !fileops.IsSubPath(gameDir, targetPath) {
			return fmt.Errorf("artifact target escapes game directory: %s", targetPath)
		}

		hash, size, err := filemeta.SHA256(sourcePath)
		if err != nil {
			return err
		}
		files = append(files, installplan.File{
			PackageKey:   artifact.InstallRecordPackageKey(),
			ComponentKey: artifact.ComponentKey,
			SourcePath:   sourcePath,
			TargetPath:   targetPath,
			RelativePath: targetRelativePath,
			Sha256:       hash,
			FileSize:     size,
			Tracked:      target.Tracked,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("artifact %s has no installable files", artifact.DisplayName)
	}
	return files, nil
}
