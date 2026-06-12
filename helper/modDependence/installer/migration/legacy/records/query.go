package records

// 文件职责：读取需要参与历史 BepInEx 裸露文件迁移的数据库记录。
// 使用：migration/legacy 入口启动迁移时调用 LoadScopedBepInExFileRecords。
// 边界：只查询 installed 文件表，不判断文件是否存在、不计算目标路径、不执行文件移动。

import (
	"DysonSphereBox/core"
	modDependenceModel "DysonSphereBox/model/modDependence/installed"
)

// LoadScopedBepInExFileRecords 读取需要检查的历史裸露文件记录。
// 职责：从 installed 文件表中筛选 BepInEx/plugins 与 BepInEx/patchers 根下的记录。
// 使用：RepairScopedBepInExFiles 启动迁移时调用。
// 边界：只读取数据库，不判断文件是否存在。
func LoadScopedBepInExFileRecords() ([]modDependenceModel.ModInstalledFile, error) {
	records := make([]modDependenceModel.ModInstalledFile, 0)
	err := core.GLOBAL.DB.
		Where("relative_path LIKE ?", "BepInEx\\plugins\\%").
		Or("relative_path LIKE ?", "BepInEx/plugins/%").
		Or("relative_path LIKE ?", "BepInEx\\patchers\\%").
		Or("relative_path LIKE ?", "BepInEx/patchers/%").
		Order("target_path ASC").
		Find(&records).Error
	return records, err
}
