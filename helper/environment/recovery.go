package environment

// 文件职责：清理环境管理中断后遗留的临时文件和临时目录。
// 使用：环境管理服务在应用启动或进入环境页恢复 pending/running 操作时调用。
// 边界：只删除明确属于环境管理的 .dsbox 临时痕迹和 import-tmp，不删除用户资源、不访问数据库、不决定 operation 状态。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"DysonSphereBox/helper/modDependence/fileops"
	"DysonSphereBox/utils"
)

// TemporaryCleanupTarget 描述一次环境临时文件清理范围。
// 职责：保存 AppData、Mod、蓝图和存档根目录，让清理逻辑只在明确根目录内工作。
// 使用：EnvironmentManagerService 解析当前目录后传入。
// 边界：空路径会被跳过，不猜测用户目录。
type TemporaryCleanupTarget struct {
	// AppDataRoot 职责：保存客户端 appData 根目录；使用：清理 environment/import-tmp；边界：不会删除 plugins 或 blueprints 资源库。
	AppDataRoot string
	// ModRoot 职责：保存当前游戏侧 Mod 根目录；使用：清理中断复制留下的 .dsbox 临时文件；边界：路径为空时跳过。
	ModRoot string
	// BlueprintRoot 职责：保存当前真实蓝图根目录；使用：清理中断复制留下的 .dsbox 临时文件；边界：路径为空时跳过。
	BlueprintRoot string
	// SaveRoot 职责：保存当前真实存档根目录；使用：清理 .dsbox-tmp 目录和临时存档文件；边界：路径为空时跳过。
	SaveRoot string
}

// TemporaryCleanupSummary 描述临时文件清理结果。
// 职责：记录已清理路径、失败路径和参与扫描的根目录，供 operation 详情和测试断言使用。
// 使用：恢复流程写入 environment_operations.detail_json。
// 边界：只包含路径摘要，不包含文件内容。
type TemporaryCleanupSummary struct {
	// RemovedPaths 职责：保存成功删除的临时路径；使用：恢复日志和测试断言；边界：可能为空。
	RemovedPaths []string `json:"removedPaths"`
	// FailedPaths 职责：保存删除失败的临时路径和原因；使用：恢复失败诊断；边界：失败不代表所有恢复失败。
	FailedPaths []string `json:"failedPaths"`
	// ScannedRoots 职责：保存参与扫描的根目录；使用：诊断恢复覆盖范围；边界：只记录传入且存在的目录。
	ScannedRoots []string `json:"scannedRoots"`
}

// CleanupInterruptedTemporaryFiles 清理环境管理中断后遗留的临时路径。
// 职责：删除 AppData/environment/import-tmp、SaveRoot/.dsbox-tmp 和各根目录内 .dsbox.tmp/.dsbox-part 文件。
// 使用：启动恢复和环境页进入恢复流程调用。
// 边界：只删除文件名或目录名能明确归属环境管理的路径；删除失败会记录并继续其它路径。
// 逻辑流：1. 清理 import-tmp 和 save 临时目录 => 2. 遍历当前根目录找临时文件 => 3. 删除并记录结果 => 4. 返回摘要给服务层。
func CleanupInterruptedTemporaryFiles(target TemporaryCleanupTarget) TemporaryCleanupSummary {
	summary := TemporaryCleanupSummary{
		RemovedPaths: []string{},
		FailedPaths:  []string{},
		ScannedRoots: []string{},
	}
	if strings.TrimSpace(target.AppDataRoot) != "" {
		removeKnownTemporaryDirectory(filepath.Join(target.AppDataRoot, "environment", "import-tmp"), &summary)
	}
	if strings.TrimSpace(target.SaveRoot) != "" {
		removeKnownTemporaryDirectory(filepath.Join(target.SaveRoot, ".dsbox-tmp"), &summary)
	}
	cleanupTemporaryFilesUnderRoot(target.ModRoot, &summary)
	cleanupTemporaryFilesUnderRoot(target.BlueprintRoot, &summary)
	cleanupTemporaryFilesUnderRoot(target.SaveRoot, &summary)
	return summary
}

// removeKnownTemporaryDirectory 删除固定命名的环境管理临时目录。
// 职责：删除 import-tmp 或 .dsbox-tmp 这种完整归属环境管理的目录树。
// 使用：CleanupInterruptedTemporaryFiles 调用。
// 边界：路径不存在或为空直接跳过；不接受任意目录名作为清理目标。
func removeKnownTemporaryDirectory(path string, summary *TemporaryCleanupSummary) {
	path = utils.CleanPath(path)
	if path == "" {
		return
	}
	base := strings.ToLower(filepath.Base(path))
	if base != "import-tmp" && base != ".dsbox-tmp" {
		summary.FailedPaths = append(summary.FailedPaths, fmt.Sprintf("%s: 非环境临时目录", path))
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	if err := os.RemoveAll(path); err != nil {
		summary.FailedPaths = append(summary.FailedPaths, fmt.Sprintf("%s: %v", path, err))
		return
	}
	summary.RemovedPaths = append(summary.RemovedPaths, path)
}

// cleanupTemporaryFilesUnderRoot 遍历指定根目录并删除环境管理临时文件。
// 职责：清理 .dsbox.tmp 和 .dsbox-part 后缀文件，防止半写入文件出现在用户视图或后续复制中。
// 使用：CleanupInterruptedTemporaryFiles 对 Mod、蓝图和存档根目录调用。
// 边界：只删除文件，不删除业务目录；遇到访问错误时继续扫描。
func cleanupTemporaryFilesUnderRoot(root string, summary *TemporaryCleanupSummary) {
	root = utils.CleanPath(root)
	if root == "" {
		return
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}
	summary.ScannedRoots = append(summary.ScannedRoots, root)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || path == root {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".dsbox-tmp" {
				removeKnownTemporaryDirectory(path, summary)
				return filepath.SkipDir
			}
			return nil
		}
		if !isRecoveryTemporaryFile(entry.Name()) {
			return nil
		}
		if !fileops.IsSubPath(root, path) {
			summary.FailedPaths = append(summary.FailedPaths, fmt.Sprintf("%s: 临时文件越界", path))
			return nil
		}
		if err := os.Remove(path); err != nil {
			summary.FailedPaths = append(summary.FailedPaths, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		summary.RemovedPaths = append(summary.RemovedPaths, path)
		return nil
	})
}

// isRecoveryTemporaryFile 判断文件是否属于环境管理中断写入临时文件。
// 职责：统一 .dsbox.tmp 和 .dsbox-part 后缀判断。
// 使用：cleanupTemporaryFilesUnderRoot 调用。
// 边界：只按文件名判断，不读取文件内容。
func isRecoveryTemporaryFile(name string) bool {
	lowerName := strings.ToLower(name)
	return strings.HasSuffix(lowerName, ".dsbox.tmp") || strings.HasSuffix(lowerName, ".dsbox-part")
}
