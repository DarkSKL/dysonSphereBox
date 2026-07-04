package system

// 文件职责：定义数据库模型或模型注册模块 enter。
// 使用：由 core 自动迁移并由 service/helper 通过 GORM 读写。
// 边界：不包含远端接口 DTO、服务编排或文件系统操作。

// ModelGroup 聚合系统领域的 GORM 模型。
// 职责：聚合系统领域需要迁移的配置与日志模型。
// 使用：根 model.RegisterModels 调用本组 RegisterModels 后交给 core.AutoMigrate。
// 边界：只组合模型结构，不执行迁移、不读写数据、不包含服务逻辑。
type ModelGroup struct {
	// Config 职责：嵌入系统配置模型注册对象；使用：RegisterModels 返回配置表迁移指针；边界：不读取或写入具体配置值。
	Config
	// Log 职责：嵌入系统日志模型注册对象；使用：RegisterModels 返回日志表迁移指针；边界：不查询或记录日志内容。
	Log
	// TutorialProgress 职责：嵌入首次使用教程进度模型注册对象；使用：RegisterModels 返回教程进度表迁移指针；边界：不查询或写入教程阅读状态。
	TutorialProgress
}

// RegisterModels 返回系统领域需要 AutoMigrate 的模型集合。
// 职责：提供系统领域 GORM 模型指针列表。
// 使用：根模型注册器在数据库初始化时调用。
// 边界：只返回数据库模型，不承载配置读写或日志查询逻辑。
func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{
		&modelGroup.Config,
		&modelGroup.Log,
		&modelGroup.TutorialProgress,
	}
}
