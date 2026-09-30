package system

// 文件职责：定义应用服务器连接相关的稳定常量。
// 使用：core 全局 HTTP 客户端初始化与服务器地址保存流程引用。
// 边界：只保存跨层共享的固定值，不读取配置、不发起网络请求；用户自定义地址保存在 System/ServerAddress 配置。

// DefaultServerAddress 表示未自定义服务器地址时使用的账号服务基础地址。
// 职责：作为 System/ServerAddress 为空或历史值无效时的兜底地址。
// 使用：core.NewServerClient / core.InitServerClient 创建全局 HTTP 客户端时读取。
// 边界：只描述本机默认部署位置，不保证该地址上的服务一定在运行；修改时需同步前端 services/server.ts 中的 DEFAULT_SERVER_ADDRESS。
const DefaultServerAddress = "http://127.0.0.1:8888"
