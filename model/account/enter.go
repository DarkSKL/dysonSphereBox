package account

// 文件职责：聚合账号领域的 SQLite 模型注册。
// 使用：根模型注册器在应用启动迁移数据库时调用。
// 边界：只返回模型指针，不执行会话读写或远端鉴权。

// ModelGroup 聚合账号领域需要迁移的数据库模型。
// 职责：为账户会话提供统一的模型注册入口。
// 使用：model.RegisterModels 读取本组的模型集合。
// 边界：不承载账号服务流程或前端状态。
type ModelGroup struct {
	// Session 职责：嵌入当前账号会话模型；使用：RegisterModels 返回迁移指针；边界：不读取具体会话内容。
	Session
}

// RegisterModels 返回账号领域需要 AutoMigrate 的模型集合。
// 职责：声明账号会话表的迁移模型。
// 使用：根模型注册器在 SQLite 初始化阶段调用。
// 边界：只提供模型，不直接执行迁移或数据操作。
func (modelGroup *ModelGroup) RegisterModels() []interface{} {
	return []interface{}{&modelGroup.Session}
}
