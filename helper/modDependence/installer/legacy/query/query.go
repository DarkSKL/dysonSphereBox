package query

// 文件职责：查询旧官方整合包本地安装状态索引。
// 使用：service/modDependence 的 InstallerService 在刷新旧市场卡片安装状态时调用。
// 边界：只读取 legacy 整合包表，不访问远端接口、不读取文件系统、不处理 V2 installed 记录。

import (
	"DysonSphereBox/helper/modDependence/installer/shared"
	modDependenceModel "DysonSphereBox/model/modDependence/legacy"
)

// ListInstalledPackageIds 返回本地已安装整合包的远端 ID 列表。
// 职责：提供旧官方整合包本地安装状态索引。
// 使用：Mod 市场加载后同步卡片安装状态。
// 边界：仅读取本地 SQLite，不访问远端接口。
func ListInstalledPackageIds() ([]uint, error) {
	db, err := shared.RequireDatabase()
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0)
	if err := db.Model(&modDependenceModel.ModDependencyPackage{}).Order("id ASC").Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
