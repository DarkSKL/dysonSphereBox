package store

// 文件职责：保存旧官方整合包及其依赖关联到 legacy 模型。
// 使用：legacy/install.InstallModDependencyPackage 在所有依赖文件安装完成后调用 SaveInstalledPackage。
// 边界：只处理旧 ModDependencyPackageDTO 与 ModDependencyDTO；不处理 V2 installed 模型、安装进度或文件清单。

import (
	modDependenceResponse "DysonSphereBox/apiDefinition/modDependence"
	legacyCleanup "DysonSphereBox/helper/modDependence/installer/legacy/cleanup"
	"DysonSphereBox/helper/modDependence/installer/shared"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SaveInstalledPackage 保存旧官方整合包及其依赖关联。
// 职责：使用 upsert 写入依赖、整合包记录和多对多关系。
// 使用：旧整合包安装文件写入成功后调用。
// 边界：数据库事务失败时返回错误；不负责文件回滚，调用方决定错误传播。
// 逻辑流：1. 转换 DTO 为 legacy 模型 => 2. upsert 依赖记录 => 3. upsert 整合包记录 => 4. 替换关联关系 => 5. 删除无人引用依赖。
func SaveInstalledPackage(pack modDependenceResponse.ModDependencyPackageDTO) error {
	db, err := shared.RequireDatabase()
	if err != nil {
		return err
	}

	packageRecord, dependencyRecords, err := packageDTOToModel(pack)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for index := range dependencyRecords {
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&dependencyRecords[index]).Error; err != nil {
				return err
			}
		}

		packageWithoutDependencies := packageRecord
		packageWithoutDependencies.Dependencies = nil
		if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&packageWithoutDependencies).Error; err != nil {
			return err
		}

		packageRecord.Dependencies = dependencyRecords
		if err := tx.Model(&packageRecord).Association("Dependencies").Replace(dependencyRecords); err != nil {
			return err
		}

		return legacyCleanup.DeleteUnreferencedDependencies(tx)
	})
}
