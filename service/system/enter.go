package system

// 文件职责：聚合系统领域需要暴露给 Wails 的服务结构体。
// 使用：根 service.RegisterServices 调用本文件的 RegisterServices 后统一绑定给前端。
// 边界：只维护系统服务列表，不实现配置、环境、更新或日志业务细节。

// ServiceGroup 聚合系统领域的 Wails 服务。
// 职责：聚合系统领域的环境、配置、更新和日志服务实例。
// 使用：根 service.RegisterServices 调用本组 RegisterServices 后绑定给 Wails。
// 边界：只保存服务结构体，不执行初始化、数据库访问或业务调用。
type ServiceGroup struct {
	// EnvironmentService 职责：暴露游戏目录探测和校验服务；使用：RegisterServices 绑定给设置页；边界：注册时不读取或修改配置。
	EnvironmentService
	// ConfigurationService 职责：暴露系统配置读取和写入服务；使用：RegisterServices 绑定给配置页面；边界：注册时不触发配置迁移。
	ConfigurationService
	// UpdateService 职责：暴露版本读取、检查更新和自更新服务；使用：RegisterServices 绑定给更新入口；边界：注册时不访问远端 Release。
	UpdateService
	// LogService 职责：暴露系统日志查询、补充记录和清理服务；使用：RegisterServices 绑定给日志页；边界：注册时不查询或写入日志。
	LogService
	// TutorialService 职责：暴露首次使用引导教程进度服务；使用：RegisterServices 绑定给前端教程模块；边界：注册时不查询或写入教程状态。
	TutorialService
	// ServerService 职责：暴露服务器地址连接测试服务；使用：RegisterServices 绑定给全局设置页；边界：注册时不发起网络请求。
	ServerService
	// AccountService 职责：暴露账号认证与资料服务；使用：RegisterServices 绑定给前端账号模块；边界：注册时不发起网络请求、不读取 token。
	AccountService
	AnnouncementService
}

// RegisterServices 返回系统领域需要绑定给前端的服务实例。
// 职责：提供系统领域 Wails 绑定对象列表。
// 使用：根服务注册器启动时调用。
// 边界：只注册服务结构体，不在这里执行初始化或业务调用。
func (serviceGroup *ServiceGroup) RegisterServices() []interface{} {
	return []interface{}{
		&serviceGroup.EnvironmentService,
		&serviceGroup.ConfigurationService,
		&serviceGroup.UpdateService,
		&serviceGroup.LogService,
		&serviceGroup.TutorialService,
		&serviceGroup.ServerService,
		&serviceGroup.AccountService,
		&serviceGroup.AnnouncementService,
	}
}
