package core

// 文件职责：维护应用运行时配置单元集合。
// 使用：由启动流程初始化后挂载到 GLOBAL.Configure，service/helper 在运行期读取对应配置访问器。
// 边界：只聚合访问器，不读写数据库、不实现具体业务用例、不直接处理 Mod 安装或市场查询。

// Configure 聚合应用运行时使用的配置单元。
// 职责：集中保存数据库驱动的配置访问器，避免服务层硬编码配置分组和键名。
// 使用：通过 GLOBAL.Configure 向 service/helper 暴露稳定配置访问入口。
// 边界：只聚合访问器，不缓存配置值，不直接决定业务默认值。
type Configure struct {
	// GameStartFilePath 访问游戏启动文件路径配置。
	// 职责：定位戴森球计划安装目录相关配置。
	// 使用：环境检测、BepInEx 和无头服务器流程读取该配置。
	// 边界：只保存访问器，不校验路径是否真实存在。
	GameStartFilePath *ConfigUnit
	// BaseURL 访问后端基础请求地址配置。
	// 职责：为全局 HTTP 客户端提供可配置的基础地址。
	// 使用：HTTPClient 初始化时读取该配置。
	// 边界：不保证地址可访问，格式校验由 HTTP 客户端完成。
	BaseURL *ConfigUnit
	// GameServerArchive 访问无头服务器启动存档配置。
	// 职责：保存用户选择的服务器存档名称。
	// 使用：无头服务器启动命令构造时读取该配置。
	// 边界：不验证存档文件是否存在。
	GameServerArchive *ConfigUnit
}

// InitConfigure 初始化应用配置单元集合。
// 职责：构造全部已知配置访问器。
// 使用：数据库初始化完成后挂到 GLOBAL.Configure，供服务和 helper 读取稳定配置入口。
// 边界：不读取数据库，不创建默认配置记录。
func InitConfigure() *Configure {
	return &Configure{
		GameStartFilePath: InitConfigUnit("Game", "StartupFilePath"),
		BaseURL:           InitConfigUnit("System", "BaseURL"),
		GameServerArchive: InitConfigUnit("Game", "ServerArchive"),
	}
}
