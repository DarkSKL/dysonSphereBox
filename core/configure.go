package core

// 文件职责：维护应用运行时配置单元集合。
// 使用：由启动流程初始化后挂载到 GLOBAL.Configure，service/helper 在运行期读取对应配置访问器。
// 边界：只聚合访问器，不读写数据库、不实现具体业务用例、不直接处理 Mod 安装或市场查询。

// Configure 聚合应用运行时使用的配置单元。
// 职责：集中保存数据库驱动的配置访问器，避免服务层硬编码配置分组和键名。
// 使用：通过 GLOBAL.Configure 向 service/helper 暴露稳定配置访问入口。
// 边界：只聚合访问器，不缓存配置值，不直接决定业务默认值。
type Configure struct {
	// AppTheme 访问应用日夜主题配置。
	// 职责：保存用户选择的界面主题模式。
	// 使用：前端启动和主题切换时通过配置服务读写该配置。
	// 边界：只保存 light/dark 文本，不在后端参与界面渲染或样式校验。
	AppTheme *ConfigUnit
	// ServerAddress 访问应用服务器连接地址配置。
	// 职责：为全局 HTTP 客户端提供可配置的服务器基础地址。
	// 使用：ServerClient 初始化时读取；全局设置页通过配置服务读写，保存时同步替换 GLOBAL.ServerClient。
	// 边界：空值表示使用 DefaultServerAddress；格式校验由配置服务和 HTTP 客户端完成。
	ServerAddress *ConfigUnit
	// AccountToken 访问当前账号会话 token 配置。
	// 职责：持久化应用服务器签发的登录 token，使重启后仍保持登录。
	// 使用：helper/system/account 在登录、续期、注销和 401 时读写。
	// 边界：token 只由后端持有并随请求发往服务器，不通过账号服务返回给前端。
	AccountToken *ConfigUnit
	// GameStartFilePath 访问游戏启动文件路径配置。
	// 职责：定位戴森球计划安装目录相关配置。
	// 使用：环境检测、BepInEx 和无头服务器流程读取该配置。
	// 边界：只保存访问器，不校验路径是否真实存在。
	GameStartFilePath *ConfigUnit
	// GameServerArchive 访问无头服务器启动存档配置。
	// 职责：保存用户选择的服务器存档名称。
	// 使用：无头服务器启动命令构造时读取该配置。
	// 边界：不验证存档文件是否存在。
	GameServerArchive *ConfigUnit
	// MultiplayerListenPort 访问联机 libp2p 监听端口配置。
	// 职责：持久化盒子 H 端口号，便于 UPnP 映射与防火墙规则稳定。
	// 使用：联机控制器启动 libp2p host 前读取；为空或绑定失败时随机新端口后写回。
	// 边界：只保存端口号文本，不保存任何密钥；密钥每次控制链路建立时临时生成。
	MultiplayerListenPort *ConfigUnit
}

// InitConfigure 初始化应用配置单元集合。
// 职责：构造全部已知配置访问器。
// 使用：数据库初始化完成后挂到 GLOBAL.Configure，供服务和 helper 读取稳定配置入口。
// 边界：不读取数据库，不创建默认配置记录。
func InitConfigure() *Configure {
	return &Configure{
		AppTheme:          InitConfigUnit("System", "AppTheme"),
		ServerAddress:     InitConfigUnit("System", "ServerAddress"),
		AccountToken:      InitConfigUnit("Account", "Token"),
		GameStartFilePath: InitConfigUnit("Game", "StartupFilePath"),
		GameServerArchive: InitConfigUnit("Game", "ServerArchive"),
		MultiplayerListenPort: InitConfigUnit("Multiplayer", "ListenPort"),
	}
}
