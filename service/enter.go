package service

// 文件职责：聚合系统领域与 Mod 依赖领域的 Wails 绑定服务。
// 使用：main.go 启动时调用 RegisterServices，把返回对象交给 Wails Bind。
// 边界：只负责服务实例注册，不执行业务流程、不访问数据库、不生成前端绑定文件。

import (
	"DysonSphereBox/service/modDependence"
	"DysonSphereBox/service/system"
)

// ServiceGroupApp 是 Wails 绑定服务的全局聚合实例。
// 职责：保存所有领域服务组的单例聚合实例。
// 使用：main.go 通过 RegisterServices 一次性取得全部服务绑定对象。
// 边界：只用于服务注册，不保存请求状态或页面状态。
var ServiceGroupApp = new(ServiceGroup)

// ServiceGroup 聚合各业务域暴露给前端调用的服务。
// 职责：组合系统领域和 Mod 依赖领域的服务组。
// 使用：RegisterServices 遍历各领域服务组时读取。
// 边界：这里只组合服务组，不放业务流程或 helper 函数。
type ServiceGroup struct {
	// SystemServiceGroup 职责：聚合系统领域 Wails 服务；使用：RegisterServices 收集环境、配置、更新和日志服务；边界：不执行系统服务方法。
	SystemServiceGroup system.ServiceGroup
	// ModDependenceServiceGroup 职责：聚合 Mod 依赖领域 Wails 服务；使用：RegisterServices 收集 BepInEx、开服、安装器和市场服务；边界：不执行 Mod 业务流程。
	ModDependenceServiceGroup modDependence.ServiceGroup
}

// RegisterServices 返回需要绑定到 Wails 的全部服务实例。
// 职责：聚合所有领域服务实例并提供给 Wails 绑定。
// 使用：新增服务领域或服务结构体后，必须在对应 enter.go 中注册并经由这里聚合。
// 边界：不初始化数据库，不执行服务方法，不记录业务日志。
func RegisterServices() []interface{} {
	services := make([]interface{}, 0)
	services = append(services,
		ServiceGroupApp.SystemServiceGroup.RegisterServices()...)
	services = append(services, ServiceGroupApp.ModDependenceServiceGroup.RegisterServices()...)

	return services
}
