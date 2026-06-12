package plan

// 文件职责：排序历史 BepInEx 裸露文件迁移计划。
// 使用：legacy 入口执行迁移前调用，避免文件占用目录路径导致后续创建目录失败。
// 边界：只调整同一批计划顺序，不改变计划内容、不扫描数据库、不移动文件。

import (
	"path/filepath"
	"sort"
)

// SortScopedFileRepairPlans 对迁移计划排序。
// 职责：优先处理会阻塞其它目标目录创建的文件。
// 使用：legacy 入口执行迁移前调用。
// 边界：只调整同一批计划顺序，不改变计划内容。
func SortScopedFileRepairPlans(plans []ScopedFileRepairPlan) {
	sort.SliceStable(plans, func(leftIndex int, rightIndex int) bool {
		return scopedPlanBlocksTargetDirectory(plans[leftIndex], plans) && !scopedPlanBlocksTargetDirectory(plans[rightIndex], plans)
	})
}

// scopedPlanBlocksTargetDirectory 判断当前文件是否占用了其它计划的目标目录。
// 职责：发现需要先搬到临时路径的阻塞文件。
// 使用：排序迁移计划时调用。
// 边界：只比较本批计划，不扫描整个文件系统。
func scopedPlanBlocksTargetDirectory(item ScopedFileRepairPlan, plans []ScopedFileRepairPlan) bool {
	for _, candidate := range plans {
		if sameFilesystemPath(item.CurrentPath, filepath.Dir(candidate.NextPath)) {
			return true
		}
	}
	return false
}
