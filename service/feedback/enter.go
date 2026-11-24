package feedback

// 文件职责：注册反馈星系的 Wails 服务实例。
// 使用：根服务组在应用启动时聚合。
// 边界：只返回绑定对象，不访问账号、数据库或网络。

// ServiceGroup 保存星系桥接的单例，供根服务组聚合。
type ServiceGroup struct {
	// FeedbackService 接收星系页面调用，注册时不执行任何请求。
	FeedbackService
}

// RegisterServices 交付星系绑定实例，供应用一次性注册。
func (group *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{&group.FeedbackService}
}
