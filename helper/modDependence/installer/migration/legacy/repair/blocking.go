package repair

// 文件职责：处理历史裸露文件迁移中“源文件占用目标目录”的特殊场景。
// 使用：RepairScopedFile 发现 CurrentPath 等于目标父目录时调用，先让出目录位置再迁移。
// 边界：只处理单个阻塞文件；中途失败会尽量恢复到原路径，不处理批量排序。

import (
	"fmt"
	"os"
	"path/filepath"

	legacyPlan "DysonSphereBox/helper/modDependence/installer/migration/legacy/plan"
)

// repairScopedFileWithBlockingDirectory 处理当前文件占用目标目录的迁移。
// 职责：先把阻塞文件移动到临时路径，再创建目标目录并更新数据库。
// 使用：RepairScopedFile 发现 CurrentPath 等于目标父目录时调用。
// 边界：中途失败会尽量把临时文件恢复到原路径。
func repairScopedFileWithBlockingDirectory(item legacyPlan.ScopedFileRepairPlan) error {
	tempPath := scopedRepairTempPath(item.CurrentPath)
	if _, err := os.Stat(tempPath); err == nil {
		return fmt.Errorf("legacy migration temp file already exists: %s", tempPath)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(item.CurrentPath, tempPath); err != nil {
		return err
	}

	completed := false
	defer func() {
		if !completed {
			_ = os.Rename(tempPath, item.CurrentPath)
		}
	}()

	if err := os.MkdirAll(filepath.Dir(item.NextPath), 0o755); err != nil {
		return err
	}
	if err := os.Rename(tempPath, item.NextPath); err != nil {
		return err
	}
	completed = true

	return updateFileRecordPath(item)
}
