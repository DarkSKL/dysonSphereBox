package overlay

// 文件职责：注册 Overlay 原型领域需要暴露给 Wails 的服务。
// 使用：根 service.RegisterServices 聚合该领域服务，供主前端调用。
// 边界：只返回服务实例，不启动附进程、不执行窗口查找。

// ServiceGroup 聚合 Overlay 原型领域服务。
// 使用：根服务组持有该结构并调用 RegisterServices。
// 边界：当前仅包含 OverlayService，后续 IPC 服务可在这里扩展。
type ServiceGroup struct {
	// OverlayService 职责：暴露悬浮窗原型的生命周期和窗口检测方法；使用：主前端调用；边界：不保存页面状态。
	OverlayService OverlayService
	// OverlayRuntimeService 职责：生成并提供 Overlay 运行态查询绑定；使用：附进程页面读取真实贴附状态；边界：主进程内为空控制器兜底。
	OverlayRuntimeService OverlayRuntimeService
}

// RegisterServices 返回 Overlay 领域的 Wails 绑定服务。
// 使用：service.RegisterServices 启动时聚合。
// 边界：不执行服务方法、不生成绑定。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.OverlayService,
		&serviceGroup.OverlayRuntimeService,
	}
}
