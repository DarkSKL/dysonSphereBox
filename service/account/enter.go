package account

// 文件职责：聚合账号领域需要暴露给 Wails 的服务结构体。
// 使用：根 service.RegisterServices 调用本文件，将 AccountService 绑定给个人页。
// 边界：只负责服务注册，不执行登录、资料更新、上传或注销请求。

// ServiceGroup 聚合账号领域的 Wails 服务。
// 职责：持有 AccountService 的绑定实例。
// 使用：应用启动时由根服务注册器收集。
// 边界：注册阶段不读取本地登录态、不访问远端服务器。
type ServiceGroup struct {
	// AccountService 职责：提供个人页账号全流程调用；使用：Wails 生成前端绑定；边界：注册时不执行任何账号请求。
	AccountService
}

// RegisterServices 返回账号领域需要绑定给 Wails 的服务实例。
// 职责：提供 AccountService 给根服务注册器。
// 使用：service.RegisterServices 在 Wails 启动前调用。
// 边界：只返回服务实例，不保存或校验登录状态。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{&serviceGroup.AccountService}
}
