package factory

// 文件职责：注册本地工厂服务；使用：根服务聚合；边界：构造阶段不访问数据库或文件。

// ServiceGroup 职责：聚合本地量化服务；使用：根注册器；边界：不执行计算。
type ServiceGroup struct {
	// FactoryService 职责：保存绑定实例；使用：注册；边界：状态仅在请求后初始化。
	FactoryService Service
}

// RegisterServices 职责：返回 Wails 绑定对象；使用：应用启动；边界：不初始化业务依赖。
func (g *ServiceGroup) RegisterServices() []interface{} { return []interface{}{&g.FactoryService} }
