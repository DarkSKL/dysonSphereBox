package environment

// 文件职责：提供环境管理文件镜像同步、校验和安全删除能力。
// 使用：环境管理服务在首次纳管、启用、禁用 Mod/蓝图时调用这些低层文件操作。
// 边界：只处理文件系统细节，不访问数据库、不决定业务状态、不记录系统日志。

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/utils"
)

// SyncedFilePlan 描述一次镜像复制完成后的文件证据。
// 职责：保存源路径、目标路径、相对路径、hash 和文件大小。
// 使用：服务层把该结构转换为 EnvSyncedFile 持久化。
// 边界：该结构不包含资源 ID；归属关系由服务层补充。
type SyncedFilePlan struct {
	// SourcePath 职责：保存客户端镜像源文件路径；使用：同步记录和诊断；边界：不用于删除目标文件。
	SourcePath string
	// TargetPath 职责：保存写入游戏目录的目标路径；使用：同步记录和禁用前复核；边界：服务层必须保证位于允许根目录。
	TargetPath string
	// RelativePath 职责：保存相对同步根的路径；使用：展示和重新定位；边界：必须来自 filepath.Rel。
	RelativePath string
	// SHA256 职责：保存写入后目标文件 hash；使用：禁用前确认未被外部修改；边界：为空不能视为匹配。
	SHA256 string
	// FileSize 职责：保存写入后目标文件大小；使用：展示和诊断；边界：不替代 hash。
	FileSize int64
}

// ImportedFilePlan 描述导入到客户端资源库或存档目录后的文件证据。
// 职责：保存导入源文件、目标文件、相对路径和文件指纹。
// 使用：服务层导入蓝图、导入存档和导出存档后刷新列表或记录操作时读取。
// 边界：不包含资源 ID 或用户选择，冲突处理由服务层决定。
type ImportedFilePlan struct {
	// SourcePath 职责：保存导入来源文件路径；使用：诊断导入来源；边界：不用于后续删除。
	SourcePath string
	// TargetPath 职责：保存导入目标文件路径；使用：刷新扫描和诊断；边界：调用方必须保证位于允许根目录。
	TargetPath string
	// RelativePath 职责：保存相对目标根的路径；使用：列表刷新和同步证据；边界：单文件导入时通常为文件名。
	RelativePath string
	// SHA256 职责：保存导入完成后目标文件 hash；使用：校验与诊断；边界：不替代业务状态。
	SHA256 string
	// FileSize 职责：保存导入完成后的文件大小；使用：前端提示和诊断；边界：不替代 hash。
	FileSize int64
}

// ExtractZipToDirectorySafely 将 zip 文件安全解压到目标目录。
// 职责：逐条清洗 zip 条目并防止路径穿越，供导入流程先落入临时目录。
// 使用：ImportMod 处理 zip 包时调用。
// 边界：只负责解压字节，不判断包内内容是否是有效 Mod。
func ExtractZipToDirectorySafely(zipPath string, destinationDir string) error {
	zipPath = utils.CleanPath(zipPath)
	destinationDir = utils.CleanPath(destinationDir)
	if !utils.FileExists(zipPath) {
		return fmt.Errorf("zip 文件不存在: %s", zipPath)
	}
	if destinationDir == "" {
		return fmt.Errorf("解压目标目录为空")
	}
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("打开 zip 文件失败: %w", err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		entryName := strings.ReplaceAll(file.Name, "\\", "/")
		entryName = strings.TrimLeft(entryName, "/")
		if strings.TrimSpace(entryName) == "" {
			continue
		}
		targetPath := filepath.Join(destinationDir, filepath.FromSlash(strings.TrimSuffix(entryName, "/")))
		if !fileops.IsSubPath(destinationDir, targetPath) {
			return fmt.Errorf("zip 条目路径越界: %s", file.Name)
		}
		if file.FileInfo().IsDir() || strings.HasSuffix(entryName, "/") {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return fmt.Errorf("创建 zip 目录失败: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("创建 zip 文件父目录失败: %w", err)
		}
		source, err := file.Open()
		if err != nil {
			return fmt.Errorf("打开 zip 条目失败: %w", err)
		}
		target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			source.Close()
			return fmt.Errorf("创建 zip 目标文件失败: %w", err)
		}
		_, copyErr := io.Copy(target, source)
		closeErr := target.Close()
		source.Close()
		if copyErr != nil {
			return fmt.Errorf("复制 zip 条目失败: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("关闭 zip 目标文件失败: %w", closeErr)
		}
	}
	return nil
}

// CopyDirectoryMirrorSafely 将源目录镜像复制到目标目录。
// 职责：递归创建目录并复制文件，遇到未知不同内容目标文件时返回冲突错误。
// 使用：EnableMod 和首次同步纳管目录型 Mod 时调用。
// 边界：不会删除目标目录中多出的文件；源目录不存在或目标越界时返回错误。
// 逻辑流：1. 校验源目录和目标根 => 2. 遍历目录创建子目录 => 3. 文件存在则校验 hash => 4. 原子复制文件并返回同步证据。
func CopyDirectoryMirrorSafely(sourceRoot string, targetRoot string) ([]SyncedFilePlan, error) {
	sourceRoot = utils.CleanPath(sourceRoot)
	targetRoot = utils.CleanPath(targetRoot)
	if !dirExists(sourceRoot) {
		return nil, fmt.Errorf("源目录不存在: %s", sourceRoot)
	}
	if targetRoot == "" {
		return nil, fmt.Errorf("目标目录为空")
	}

	plans := make([]SyncedFilePlan, 0)
	err := filepath.WalkDir(sourceRoot, func(sourcePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceRoot, sourcePath)
		if err != nil {
			return err
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if !fileops.IsSubPath(targetRoot, targetPath) {
			return fmt.Errorf("目标路径越界: %s", targetPath)
		}

		if entry.IsDir() {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return fmt.Errorf("创建目标目录失败: %w", err)
			}
			return nil
		}

		plan, err := CopyFileSafely(sourcePath, targetPath, relativePath)
		if err != nil {
			return err
		}
		plans = append(plans, plan)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return plans, nil
}

// CopyDirectoryIntoRootSafely 将一个目录导入目标根目录下。
// 职责：把来源目录作为一级资源目录复制到目标根，冲突时拒绝覆盖。
// 使用：ImportMod 导入目录型 Mod 到客户端资源库时调用。
// 边界：不支持单文件 DLL；不解压 zip；调用方负责业务提示。
func CopyDirectoryIntoRootSafely(sourceDir string, targetRoot string) ([]SyncedFilePlan, string, error) {
	sourceDir = utils.CleanPath(sourceDir)
	targetRoot = utils.CleanPath(targetRoot)
	if !dirExists(sourceDir) {
		return nil, "", fmt.Errorf("源目录不存在: %s", sourceDir)
	}
	if targetRoot == "" {
		return nil, "", fmt.Errorf("目标根目录为空")
	}
	targetDir := filepath.Join(targetRoot, filepath.Base(sourceDir))
	if !fileops.IsSubPath(targetRoot, targetDir) {
		return nil, "", fmt.Errorf("目标目录越界: %s", targetDir)
	}
	plans, err := CopyDirectoryMirrorSafely(sourceDir, targetDir)
	if err != nil {
		return nil, "", err
	}
	return plans, targetDir, nil
}

// CopyDirectoryAsNameSafely 将源目录按指定名称导入目标根目录。
// 职责：把临时解压目录或用户选择目录复制成稳定资源库目录名。
// 使用：ImportMod 处理 zip 根目录或需要重命名导入目录时调用。
// 边界：目录名必须由服务层校验；目标冲突时拒绝覆盖不同内容。
func CopyDirectoryAsNameSafely(sourceDir string, targetRoot string, targetName string) ([]SyncedFilePlan, string, error) {
	sourceDir = utils.CleanPath(sourceDir)
	targetRoot = utils.CleanPath(targetRoot)
	targetName = strings.TrimSpace(targetName)
	if !dirExists(sourceDir) {
		return nil, "", fmt.Errorf("源目录不存在: %s", sourceDir)
	}
	if targetRoot == "" || targetName == "" {
		return nil, "", fmt.Errorf("目标根目录或目录名为空")
	}
	targetDir := filepath.Join(targetRoot, targetName)
	if !fileops.IsSubPath(targetRoot, targetDir) {
		return nil, "", fmt.Errorf("目标目录越界: %s", targetDir)
	}
	plans, err := CopyDirectoryMirrorSafely(sourceDir, targetDir)
	if err != nil {
		return nil, "", err
	}
	return plans, targetDir, nil
}

// CopySingleFileSafely 将单个文件复制到目标路径。
// 职责：为蓝图单文件同步提供和目录镜像一致的冲突检测与原子写入。
// 使用：EnableBlueprint 和首次同步蓝图纳管时调用。
// 边界：不会覆盖不同 hash 的未知目标文件；目标目录由函数自动创建。
func CopySingleFileSafely(sourcePath string, targetPath string, relativePath string) (SyncedFilePlan, error) {
	sourcePath = utils.CleanPath(sourcePath)
	targetPath = utils.CleanPath(targetPath)
	if !utils.FileExists(sourcePath) {
		return SyncedFilePlan{}, fmt.Errorf("源文件不存在: %s", sourcePath)
	}
	if strings.TrimSpace(relativePath) == "" {
		relativePath = filepath.Base(targetPath)
	}
	return CopyFileSafely(sourcePath, targetPath, relativePath)
}

// OverwriteSingleFileWithEvidence 明确覆盖单个目标文件并返回同步证据。
// 职责：在用户选择冲突恢复或镜像更新时，用指定源文件替换目标文件并校验写入结果。
// 使用：环境管理冲突处理流程调用，普通启用流程仍使用 CopySingleFileSafely 拒绝未知覆盖。
// 边界：目标被占用、临时文件提交失败或 hash 校验失败时返回错误；不会删除源文件，不写数据库。
func OverwriteSingleFileWithEvidence(sourcePath string, targetPath string, relativePath string) (SyncedFilePlan, error) {
	sourcePath = utils.CleanPath(sourcePath)
	targetPath = utils.CleanPath(targetPath)
	if !utils.FileExists(sourcePath) {
		return SyncedFilePlan{}, fmt.Errorf("源文件不存在: %s", sourcePath)
	}
	if strings.TrimSpace(relativePath) == "" {
		relativePath = filepath.Base(targetPath)
	}
	sourceHash, sourceSize, err := HashFile(sourcePath)
	if err != nil {
		return SyncedFilePlan{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return SyncedFilePlan{}, fmt.Errorf("创建目标目录失败: %w", err)
	}
	tempPath := targetPath + ".dsbox.tmp"
	if err := copyFileContent(sourcePath, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, err
	}
	tempHash, tempSize, err := HashFile(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, err
	}
	if tempHash != sourceHash || tempSize != sourceSize {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, fmt.Errorf("覆盖校验失败: %s", targetPath)
	}
	if utils.FileExists(targetPath) {
		if err := os.Remove(targetPath); err != nil {
			_ = os.Remove(tempPath)
			return SyncedFilePlan{}, fmt.Errorf("删除旧目标文件失败: %w", err)
		}
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, fmt.Errorf("提交覆盖文件失败: %w", err)
	}
	return SyncedFilePlan{
		SourcePath:   sourcePath,
		TargetPath:   targetPath,
		RelativePath: filepath.ToSlash(filepath.Clean(relativePath)),
		SHA256:       tempHash,
		FileSize:     tempSize,
	}, nil
}

// ImportFilesIntoRootSafely 将多个文件复制到目标根目录下。
// 职责：按文件名导入普通文件，目标同名且内容不同则返回冲突错误。
// 使用：蓝图导入、存档导入和存档导出调用。
// 边界：不会递归目录；不会覆盖不同内容目标文件；返回已完成文件证据。
func ImportFilesIntoRootSafely(sourcePaths []string, targetRoot string) ([]ImportedFilePlan, error) {
	targetRoot = utils.CleanPath(targetRoot)
	if targetRoot == "" {
		return nil, fmt.Errorf("目标根目录为空")
	}
	plans := make([]ImportedFilePlan, 0, len(sourcePaths))
	for _, sourcePath := range sourcePaths {
		sourcePath = utils.CleanPath(sourcePath)
		if !utils.FileExists(sourcePath) {
			return nil, fmt.Errorf("源文件不存在: %s", sourcePath)
		}
		relativePath := filepath.Base(sourcePath)
		targetPath := filepath.Join(targetRoot, relativePath)
		if !fileops.IsSubPath(targetRoot, targetPath) {
			return nil, fmt.Errorf("目标文件越界: %s", targetPath)
		}
		syncedPlan, err := CopySingleFileSafely(sourcePath, targetPath, relativePath)
		if err != nil {
			return nil, err
		}
		plans = append(plans, ImportedFilePlan{
			SourcePath:   syncedPlan.SourcePath,
			TargetPath:   syncedPlan.TargetPath,
			RelativePath: syncedPlan.RelativePath,
			SHA256:       syncedPlan.SHA256,
			FileSize:     syncedPlan.FileSize,
		})
	}
	return plans, nil
}

// WriteTextFileIntoRootSafely 将文本内容写入目标根目录下的单个文件。
// 职责：为蓝图快速导入提供临时文件写入、冲突检测和校验能力。
// 使用：QuickImportBlueprint 调用，把蓝图编码保存为 .txt 文件。
// 边界：目标已存在且内容不同会返回冲突；调用方负责校验名称。
func WriteTextFileIntoRootSafely(targetRoot string, fileName string, content string) (ImportedFilePlan, error) {
	targetRoot = utils.CleanPath(targetRoot)
	if targetRoot == "" {
		return ImportedFilePlan{}, fmt.Errorf("目标根目录为空")
	}
	if strings.TrimSpace(fileName) == "" {
		return ImportedFilePlan{}, fmt.Errorf("目标文件名为空")
	}
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return ImportedFilePlan{}, fmt.Errorf("创建目标目录失败: %w", err)
	}
	tempDir, err := os.MkdirTemp(targetRoot, ".dsbox-write-*")
	if err != nil {
		return ImportedFilePlan{}, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)
	tempPath := filepath.Join(tempDir, fileName)
	if err := os.WriteFile(tempPath, []byte(content), 0644); err != nil {
		return ImportedFilePlan{}, fmt.Errorf("写入临时文件失败: %w", err)
	}
	targetPath := filepath.Join(targetRoot, fileName)
	if !fileops.IsSubPath(targetRoot, targetPath) {
		return ImportedFilePlan{}, fmt.Errorf("目标文件越界: %s", targetPath)
	}
	syncedPlan, err := CopySingleFileSafely(tempPath, targetPath, fileName)
	if err != nil {
		return ImportedFilePlan{}, err
	}
	return ImportedFilePlan{
		SourcePath:   tempPath,
		TargetPath:   syncedPlan.TargetPath,
		RelativePath: syncedPlan.RelativePath,
		SHA256:       syncedPlan.SHA256,
		FileSize:     syncedPlan.FileSize,
	}, nil
}

// CopyFileSafely 原子复制单个文件并返回写入证据。
// 职责：在目标文件不存在或内容相同时写入文件，目标内容不同则返回冲突。
// 使用：目录镜像和单文件同步内部复用。
// 边界：只处理单文件，不清理目标多余文件，不创建业务记录。
func CopyFileSafely(sourcePath string, targetPath string, relativePath string) (SyncedFilePlan, error) {
	sourceHash, sourceSize, err := HashFile(sourcePath)
	if err != nil {
		return SyncedFilePlan{}, err
	}

	if utils.FileExists(targetPath) {
		targetHash, _, err := HashFile(targetPath)
		if err != nil {
			return SyncedFilePlan{}, err
		}
		if targetHash != sourceHash {
			return SyncedFilePlan{}, fmt.Errorf("目标文件已存在且内容不同: %s", targetPath)
		}
		return SyncedFilePlan{
			SourcePath:   sourcePath,
			TargetPath:   targetPath,
			RelativePath: filepath.ToSlash(filepath.Clean(relativePath)),
			SHA256:       targetHash,
			FileSize:     sourceSize,
		}, nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return SyncedFilePlan{}, fmt.Errorf("创建目标目录失败: %w", err)
	}
	tempPath := targetPath + ".dsbox.tmp"
	if err := copyFileContent(sourcePath, tempPath); err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, err
	}
	tempHash, tempSize, err := HashFile(tempPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, err
	}
	if tempHash != sourceHash || tempSize != sourceSize {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, fmt.Errorf("复制校验失败: %s", targetPath)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return SyncedFilePlan{}, fmt.Errorf("提交目标文件失败: %w", err)
	}

	return SyncedFilePlan{
		SourcePath:   sourcePath,
		TargetPath:   targetPath,
		RelativePath: filepath.ToSlash(filepath.Clean(relativePath)),
		SHA256:       tempHash,
		FileSize:     tempSize,
	}, nil
}

// HashFileWhenVisible 在文件系统短暂延迟后读取目标文件指纹。
// 职责：为 Documents、云同步目录或安全软件扫描导致的瞬时不可见提供轻量重试。
// 使用：蓝图迁移在确认旧游戏侧文件仍存在且未被修改时调用。
// 边界：只等待文件重新可见并计算 hash；持续不存在、目录路径或读取失败仍返回原始错误。
func HashFileWhenVisible(targetPath string) (string, int64, error) {
	const attempts = 5
	const delay = 120 * time.Millisecond
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		info, statErr := os.Stat(targetPath)
		if statErr == nil && !info.IsDir() {
			hash, size, hashErr := HashFile(targetPath)
			if hashErr == nil {
				return hash, size, nil
			}
			lastErr = hashErr
		} else if statErr != nil {
			lastErr = statErr
		} else {
			lastErr = fmt.Errorf("目标路径是目录: %s", targetPath)
		}
		if attempt < attempts-1 {
			time.Sleep(delay)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("目标文件不可用: %s", targetPath)
	}
	return "", 0, lastErr
}

// RemoveFileIfHashMatches 按同步记录 hash 安全删除目标文件。
// 职责：只删除仍与记录 hash 一致的目标文件，避免误删用户手动修改文件。
// 使用：DisableMod 和 DisableBlueprint 删除已登记文件时调用。
// 边界：目标文件缺失视为已删除；hash 不一致返回错误并保留文件。
func RemoveFileIfHashMatches(targetPath string, expectedSHA256 string) error {
	if !utils.FileExists(targetPath) {
		return nil
	}
	if strings.TrimSpace(expectedSHA256) == "" {
		return fmt.Errorf("缺少同步 hash，拒绝删除: %s", targetPath)
	}
	currentSHA256, _, err := HashFile(targetPath)
	if err != nil {
		return err
	}
	if currentSHA256 != expectedSHA256 {
		return fmt.Errorf("目标文件已被修改，拒绝删除: %s", targetPath)
	}
	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("删除目标文件失败: %w", err)
	}
	return nil
}

// MovePathWithinRootSafely 在同一根目录内移动文件或目录。
// 职责：校验源和目标都位于根目录内，并拒绝覆盖已存在目标。
// 使用：MoveBlueprintItem 调整客户端蓝图目录树时调用。
// 边界：只做本地资源库移动，不同步游戏侧已启用文件迁移。
func MovePathWithinRootSafely(root string, sourcePath string, targetPath string) error {
	root = utils.CleanPath(root)
	sourcePath = utils.CleanPath(sourcePath)
	targetPath = utils.CleanPath(targetPath)
	if root == "" || sourcePath == "" || targetPath == "" {
		return fmt.Errorf("移动路径不能为空")
	}
	if !fileops.IsSubPath(root, sourcePath) || !fileops.IsSubPath(root, targetPath) {
		return fmt.Errorf("移动路径越界")
	}
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("源路径不可用: %w", err)
	}
	if pathExists(targetPath) {
		return fmt.Errorf("目标路径已存在: %s", targetPath)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("创建目标父目录失败: %w", err)
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		return fmt.Errorf("移动路径失败: %w", err)
	}
	return nil
}

// RemovePathWithinRootSafely 删除根目录内的文件或目录。
// 职责：在确认路径位于根目录内后删除客户端资源库节点。
// 使用：DeleteMod 和 DeleteBlueprintItem 在已禁用后清理客户端镜像。
// 边界：不会删除根目录本身；路径不存在视为成功。
func RemovePathWithinRootSafely(root string, targetPath string) error {
	root = utils.CleanPath(root)
	targetPath = utils.CleanPath(targetPath)
	if root == "" || targetPath == "" {
		return fmt.Errorf("删除路径不能为空")
	}
	if targetPath == root || !fileops.IsSubPath(root, targetPath) {
		return fmt.Errorf("删除路径越界: %s", targetPath)
	}
	if err := os.RemoveAll(targetPath); err != nil {
		return fmt.Errorf("删除路径失败: %w", err)
	}
	return nil
}

// MoveFileToSnapshotSafely 将文件移动到快照目录。
// 职责：用应用内快照目录模拟可恢复删除，避免直接永久删除存档。
// 使用：DeleteSave 调用，删除真实存档前生成可恢复副本。
// 边界：只移动单个文件；目标快照已存在时返回冲突。
func MoveFileToSnapshotSafely(sourcePath string, snapshotRoot string, relativePath string) (string, error) {
	sourcePath = utils.CleanPath(sourcePath)
	snapshotRoot = utils.CleanPath(snapshotRoot)
	if !utils.FileExists(sourcePath) {
		return "", fmt.Errorf("源文件不存在: %s", sourcePath)
	}
	if snapshotRoot == "" {
		return "", fmt.Errorf("快照目录为空")
	}
	if strings.TrimSpace(relativePath) == "" {
		relativePath = filepath.Base(sourcePath)
	}
	snapshotPath := filepath.Join(snapshotRoot, filepath.FromSlash(filepath.ToSlash(relativePath)))
	if !fileops.IsSubPath(snapshotRoot, snapshotPath) {
		return "", fmt.Errorf("快照路径越界: %s", snapshotPath)
	}
	if pathExists(snapshotPath) {
		return "", fmt.Errorf("快照文件已存在: %s", snapshotPath)
	}
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0755); err != nil {
		return "", fmt.Errorf("创建快照目录失败: %w", err)
	}
	if err := os.Rename(sourcePath, snapshotPath); err != nil {
		if _, copyErr := CopyFileSafely(sourcePath, snapshotPath, relativePath); copyErr != nil {
			return "", fmt.Errorf("移动到快照失败: %w", err)
		}
		if removeErr := os.Remove(sourcePath); removeErr != nil {
			return "", fmt.Errorf("删除原文件失败: %w", removeErr)
		}
	}
	return snapshotPath, nil
}

// RemoveEmptyParents 向上清理空目录。
// 职责：从起始目录向同步根方向删除空目录，保持根目录本身不被删除。
// 使用：蓝图和 Mod 禁用后清理本次同步产生的空目录。
// 边界：遇到非空目录或越界目录立即停止；清理失败返回错误。
func RemoveEmptyParents(root string, startDir string) error {
	root = utils.CleanPath(root)
	current := utils.CleanPath(startDir)
	for current != "" && current != root && fileops.IsSubPath(root, current) {
		err := os.Remove(current)
		if err != nil {
			if os.IsNotExist(err) {
				current = filepath.Dir(current)
				continue
			}
			if isDirectoryNotEmptyError(err) {
				return nil
			}
			return fmt.Errorf("清理空目录失败: %w", err)
		}
		current = filepath.Dir(current)
	}
	return nil
}

// HashFile 计算文件 SHA256 与大小。
// 职责：为同步、冲突检测和安全删除提供统一文件指纹。
// 使用：CopyFileSafely、RemoveFileIfHashMatches 和测试调用。
// 边界：只读取普通文件；路径不存在或目录路径会返回错误。
func HashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("读取文件失败: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

// copyFileContent 复制文件内容到指定路径。
// 职责：以普通文件权限复制源文件内容。
// 使用：CopyFileSafely 写入临时文件时调用。
// 边界：不做 hash 校验，调用方负责校验和提交。
func copyFileContent(sourcePath string, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer source.Close()

	sourceInfo, err := source.Stat()
	if err != nil {
		return fmt.Errorf("读取源文件信息失败: %w", err)
	}
	target, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sourceInfo.Mode())
	if err != nil {
		return fmt.Errorf("创建目标临时文件失败: %w", err)
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}
	return nil
}

// isDirectoryNotEmptyError 判断目录非空错误。
// 职责：兼容 Windows 与类 Unix 的非空目录错误文案。
// 使用：RemoveEmptyParents 遇到非空目录时停止清理。
// 边界：只用于目录清理分支，不作为通用错误分类工具。
func isDirectoryNotEmptyError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "directory not empty") ||
		strings.Contains(message, "not empty") ||
		strings.Contains(message, "目录不是空的")
}

// pathExists 判断文件或目录是否存在。
// 职责：供文件 helper 在写入、移动和删除前做保守冲突检测。
// 使用：MovePathWithinRootSafely 与 MoveFileToSnapshotSafely 调用。
// 边界：访问失败且不是不存在时保守视为存在。
func pathExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	return true
}
