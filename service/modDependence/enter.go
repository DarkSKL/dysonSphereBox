package modDependence

// 文件职责：聚合 Mod 依赖领域需要暴露给 Wails 的服务结构体。
// 使用：根 service.RegisterServices 调用本文件的 RegisterServices 后统一绑定给前端。
// 边界：只维护 Mod 依赖服务列表，不执行安装、卸载、市场查询或开服准备流程。

// ServiceGroup 聚合 Mod 依赖领域的 Wails 服务。
// 职责：组合 BepInEx、开服、安装器和市场服务实例。
// 使用：根服务注册器调用 RegisterServices 后绑定给 Wails。
// 边界：只聚合服务结构体，不执行安装、市场查询或状态检测流程。
type ServiceGroup struct {
	// BepInExService 职责：暴露 BepInEx 前置环境维护服务；使用：RegisterServices 绑定给设置页和 Mod 页；边界：注册时不执行安装或检测。
	BepInExService
	// HeadlessService 职责：暴露无头服务器环境、设置和进程服务；使用：RegisterServices 绑定给开服页；边界：注册时不启动或停止进程。
	HeadlessService
	// InstallerService 职责：暴露 Mod 安装、卸载、冲突恢复和进度服务；使用：RegisterServices 绑定给 Mod 市场页；边界：注册时不创建安装会话。
	InstallerService
	// MarketplaceService 职责：暴露统一 Mod 市场查询与刷新服务；使用：RegisterServices 绑定给 Mod 列表页；边界：注册时不拉取远端市场数据。
	MarketplaceService
}

// RegisterServices 返回 Mod 依赖领域需要绑定给前端的服务实例。
// 职责：提供 Mod 依赖领域 Wails 绑定对象列表。
// 使用：根 service.RegisterServices 启动时调用。
// 边界：只注册服务结构体，具体业务流程仍由各服务方法负责。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.BepInExService,
		&serviceGroup.HeadlessService,
		&serviceGroup.InstallerService,
		&serviceGroup.MarketplaceService,
	}
}
