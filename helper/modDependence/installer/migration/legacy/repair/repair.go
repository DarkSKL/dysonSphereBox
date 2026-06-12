package repair

// 文件职责：执行单个历史 BepInEx 裸露文件迁移入口。
// 使用：migration/legacy 入口遍历迁移计划时调用 RepairScopedFile。
// 边界：只迁移哈希匹配的受管理文件；哈希、路径、阻塞目录和数据库更新由同包独立文件维护。

import (
	"os"
	"path/filepath"
	"strings"

	legacyPlan "DysonSphereBox/helper/modDependence/installer/migration/legacy/plan"
)

// RepairScopedFile 执行单个历史裸露文件迁移。
// 职责：校验当前文件哈希后移动到新路径，并更新数据库路径字段。
// 使用：RepairScopedBepInExFiles 遍历计划时调用。
// 边界：文件不存在、哈希不匹配或目标已存在时跳过，避免覆盖用户文件。
func RepairScopedFile(item legacyPlan.ScopedFileRepairPlan) error {
	currentHash, _, err := fileSHA256(item.CurrentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !strings.EqualFold(currentHash, item.Record.Sha256) {
		return nil
	}

	if sameFilesystemPath(item.CurrentPath, item.NextPath) {
		return nil
	}
	if sameFilesystemPath(item.CurrentPath, filepath.Dir(item.NextPath)) {
		return repairScopedFileWithBlockingDirectory(item)
	}
	if _, err := os.Stat(item.NextPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(item.NextPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(item.CurrentPath, item.NextPath); err != nil {
		return err
	}

	return updateFileRecordPath(item)
}
