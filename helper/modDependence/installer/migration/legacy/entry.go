package legacy

// 文件职责：提供历史 BepInEx 裸露文件路径迁移的启动入口。
// 使用：应用启动完成数据库初始化后调用 RepairScopedBepInExFiles，迁移历史 installed 文件记录。
// 边界：只编排历史迁移步骤，不处理普通安装、卸载、冲突恢复或前端进度。

import (
	"fmt"

	"DysonSphereBox/core"
	legacyPlan "DysonSphereBox/helper/modDependence/installer/migration/legacy/plan"
	legacyRecords "DysonSphereBox/helper/modDependence/installer/migration/legacy/records"
	legacyRepair "DysonSphereBox/helper/modDependence/installer/migration/legacy/repair"
)

// RepairScopedBepInExFiles 修复早期安装器把 plugins/patchers 文件裸露到根目录的问题。
// 职责：扫描数据库中仍指向 BepInEx/plugins/*.dll 或 BepInEx/patchers/*.dll 的受管理文件并迁移。
// 使用：App.startup 在数据库初始化后调用一次。
// 边界：只迁移哈希完全匹配的已管理文件；目标存在且不是本次迁移源文件时跳过，交给安装冲突流程处理。
// 逻辑流：1. 查询历史裸露文件记录 => 2. 构造目标路径计划 => 3. 排序避免目录阻塞 => 4. 校验哈希并移动文件 => 5. 更新数据库路径。
func RepairScopedBepInExFiles() error {
	if core.GLOBAL.DB == nil {
		return fmt.Errorf("database is not initialized")
	}

	records, err := legacyRecords.LoadScopedBepInExFileRecords()
	if err != nil {
		return err
	}
	plans := legacyPlan.BuildScopedFileRepairPlans(records)
	legacyPlan.SortScopedFileRepairPlans(plans)
	for _, item := range plans {
		if err := legacyRepair.RepairScopedFile(item); err != nil {
			return err
		}
	}
	return nil
}
