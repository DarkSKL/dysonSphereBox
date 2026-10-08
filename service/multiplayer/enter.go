package multiplayer

// 文件职责：聚合联机领域需要暴露给 Wails 的服务结构体。
// 使用：根 service.RegisterServices 调用本文件的 RegisterServices 后统一绑定给前端。
// 边界：只维护联机服务列表，不建立控制链路、不访问数据库。

// ServiceGroup 聚合联机领域的 Wails 服务。
// 职责：保存联机服务实例；使用：根服务注册器；边界：注册时不执行业务。
type ServiceGroup struct {
	// MultiplayerService 职责：暴露联机意图方法；使用：RegisterServices 绑定给联机页面；边界：注册时不连接服务器。
	MultiplayerService
}

// RegisterServices 返回联机领域需要绑定给前端的服务实例。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{&serviceGroup.MultiplayerService}
}
