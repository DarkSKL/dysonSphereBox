package directory

// 文件职责：归一化并排序 V2 会话计划目标目录。
// 使用：CleanupPlannedDirs 删除目标目录前调用 sortedTargetDirs。
// 边界：只排序字符串，不访问文件系统、不判断目录安全性。

import (
	"path/filepath"
	"sort"
	"strings"

	"DysonSphereBox/helper/modDependence/installer/v2/sessionutil"
)

// sortedTargetDirs 归一化并按路径深度排序目标目录。
// 职责：去重计划目录，并让子目录优先于父目录清理。
// 使用：CleanupPlannedDirs 删除目录前调用。
// 边界：空目录和点目录会被丢弃；只排序字符串，不访问文件系统。
func sortedTargetDirs(dirs []string) []string {
	result := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(filepath.Clean(dir))
		if dir == "" || dir == "." {
			continue
		}
		result = sessionutil.AppendUniqueTargetDir(result, dir)
	}
	sort.SliceStable(result, func(leftIndex int, rightIndex int) bool {
		return len(result[leftIndex]) > len(result[rightIndex])
	})
	return result
}
