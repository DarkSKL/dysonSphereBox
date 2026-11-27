package forum

// 文件职责：注册社区论坛的 Wails 绑定服务实例。
// 使用：由 service/enter.go 在应用启动时聚合注册。
// 边界：只返回服务指针，不在注册阶段访问数据库、账号或网络。

// ServiceGroup 聚合社区论坛领域暴露给前端的服务实例。
// 职责：保存 ForumService 单例，供根服务组统一聚合。
// 使用：service.ServiceGroup 内嵌并在 RegisterServices 中收集。
// 边界：不保存页面状态或会话缓存。
type ServiceGroup struct {
	// ForumService 职责：暴露社区论坛浏览、发帖、回复、举报与资源导入 Wails API；使用：前端论坛页调用；边界：无状态。
	ForumService
}

// RegisterServices 返回社区论坛领域需要绑定到 Wails 的全部服务实例。
// 职责：交付 ForumService 绑定实例给根服务组。
// 使用：service.RegisterServices 调用。
// 边界：不执行网络请求或数据库操作。
func (group *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{&group.ForumService}
}
