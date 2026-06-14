package environment

// 文件职责：聚合环境管理领域需要自动迁移的 GORM 模型。
// 使用：根 model.RegisterModels 调用本组 RegisterModels 后交给 core.AutoMigrate。
// 边界：只维护模型清单，不执行数据库迁移、不读写业务数据、不处理文件系统。

// ModelGroup 聚合环境管理领域的模型实例。
// 职责：组合资源项目、同步文件、操作记录和存档索引模型。
// 使用：根模型注册器在数据库初始化时读取。
// 边界：这里只保存模型结构体，不执行环境扫描或同步流程。
type ModelGroup struct {
	// EnvItem 职责：聚合环境资源项目表；使用：RegisterModels 返回给 AutoMigrate；边界：真实模型定义在 env_item.go。
	EnvItem EnvItem
	// EnvSyncedFile 职责：聚合同步文件清单表；使用：RegisterModels 返回给 AutoMigrate；边界：不执行文件校验。
	EnvSyncedFile EnvSyncedFile
	// EnvOperation 职责：聚合环境操作记录表；使用：RegisterModels 返回给 AutoMigrate；边界：不执行恢复流程。
	EnvOperation EnvOperation
	// EnvSaveIndex 职责：聚合存档最近扫描索引表；使用：RegisterModels 返回给 AutoMigrate；边界：不替代实时扫描。
	EnvSaveIndex EnvSaveIndex
}

// RegisterModels 返回环境管理领域需要 AutoMigrate 的模型集合。
// 职责：提供环境管理领域 GORM 模型指针列表。
// 使用：根模型注册器在数据库初始化时调用。
// 边界：只返回持久化模型，不包含前端 DTO 或文件扫描结构。
func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.EnvItem,
		&modelGroup.EnvSyncedFile,
		&modelGroup.EnvOperation,
		&modelGroup.EnvSaveIndex,
	}
}
