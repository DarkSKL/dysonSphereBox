package environment

// 文件职责：聚合环境管理领域需要暴露给 Wails 的服务结构体。
// 使用：根 service.RegisterServices 调用本文件的 RegisterServices 后统一绑定给前端。
// 边界：只维护环境管理服务列表，不执行扫描、同步或文件操作。

// ServiceGroup 聚合环境管理领域的 Wails 服务。
// 职责：组合环境管理页查询与后续同步服务实例。
// 使用：根服务注册器调用 RegisterServices 后绑定给 Wails。
// 边界：只保存服务结构体，不执行目录解析或状态扫描。
type ServiceGroup struct {
	// EnvironmentManagerService 职责：暴露环境页只读扫描和目录入口；使用：RegisterServices 绑定给环境页；边界：注册时不访问文件系统。
	EnvironmentManagerService
}

// RegisterServices 返回环境管理领域需要绑定给前端的服务实例。
// 职责：提供环境管理 Wails 绑定对象列表。
// 使用：根 service.RegisterServices 启动时调用。
// 边界：只注册服务结构体，具体业务流程仍由服务方法负责。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentManagerService,
	}
}
