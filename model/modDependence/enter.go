package modDependence

// 文件职责：定义数据库模型或模型注册模块 enter。
// 使用：由 core 自动迁移并由 service/helper 通过 GORM 读写。
// 边界：不包含远端接口 DTO、服务编排或文件系统操作。

import (
	installedModel "DysonSphereBox/model/modDependence/installed"
	legacyModel "DysonSphereBox/model/modDependence/legacy"
)

// ModelGroup 聚合 Mod 依赖领域的 GORM 模型。
// 职责：聚合 Mod 依赖领域所有需要迁移的模型。
// 使用：根 model.RegisterModels 调用本组 RegisterModels 后交给 core.AutoMigrate。
// 边界：只组合模型结构，不执行迁移、不读写数据、不包含 API DTO。
type ModelGroup struct {
	// ModDependency 是旧官方源单个 Mod 依赖模型。
	// 职责：参与 AutoMigrate 注册并保留旧整合包依赖表结构。
	// 使用：RegisterModels 返回其指针给 core 数据库初始化流程。
	// 边界：真实模型定义在 legacy 子包，此处只聚合迁移对象。
	ModDependency legacyModel.ModDependency
	// ModDependencyPackage 是旧官方源整合包模型。
	// 职责：参与 AutoMigrate 注册并保留旧整合包表结构。
	// 使用：RegisterModels 返回其指针给 core 数据库初始化流程。
	// 边界：真实模型定义在 legacy 子包，此处不执行安装逻辑。
	ModDependencyPackage legacyModel.ModDependencyPackage
	// ModInstalledPackage 是 V2 已安装包模型。
	// 职责：聚合 installed 子包中的安装包表结构。
	// 使用：RegisterModels 返回其指针给 GORM 自动迁移。
	// 边界：真实字段定义在 installed 子包，此处不维护表结构。
	ModInstalledPackage installedModel.ModInstalledPackage
	// ModInstalledComponent 是 V2 已安装组件模型。
	// 职责：聚合 installed 子包中的组件表结构。
	// 使用：RegisterModels 返回其指针给 GORM 自动迁移。
	// 边界：真实字段定义在 installed 子包，此处不维护组件关系逻辑。
	ModInstalledComponent installedModel.ModInstalledComponent
	// ModInstalledPackageComponent 是 V2 安装包组件关联模型。
	// 职责：聚合 installed 子包中的关联表结构。
	// 使用：RegisterModels 返回其指针给 GORM 自动迁移。
	// 边界：真实字段定义在 installed 子包，此处不判断共享依赖。
	ModInstalledPackageComponent installedModel.ModInstalledPackageComponent
	// ModInstalledFile 是 V2 安装文件清单模型。
	// 职责：聚合 installed 子包中的文件归属表结构。
	// 使用：RegisterModels 返回其指针给 GORM 自动迁移。
	// 边界：真实字段定义在 installed 子包，此处不执行文件删除或回滚。
	ModInstalledFile installedModel.ModInstalledFile
	// ModRemotePackageCache 是远端包缓存模型。
	// 职责：聚合 installed 子包中的市场缓存表结构。
	// 使用：RegisterModels 返回其指针给 GORM 自动迁移。
	// 边界：真实字段定义在 installed 子包，此处不决定缓存过期策略。
	ModRemotePackageCache installedModel.ModRemotePackageCache
}

// RegisterModels 返回 Mod 依赖领域需要 AutoMigrate 的模型集合。
// 职责：提供 Mod 依赖领域 GORM 模型指针列表。
// 使用：根模型注册器在数据库初始化时调用。
// 边界：只返回持久化模型，不包含外部 API DTO 或前端响应结构。
func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.ModDependency,
		&modelGroup.ModDependencyPackage,
		&modelGroup.ModInstalledPackage,
		&modelGroup.ModInstalledComponent,
		&modelGroup.ModInstalledPackageComponent,
		&modelGroup.ModInstalledFile,
		&modelGroup.ModRemotePackageCache,
	}
}
