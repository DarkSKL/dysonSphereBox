package core

// 文件职责：维护应用基础设施模块 global。
// 使用：由启动流程初始化，service/helper 在运行期读取已初始化资源。
// 边界：不实现具体业务用例，不直接处理 Mod 安装、市场查询或页面交互。

import (
	"context"

	"gorm.io/gorm"
)

// Global 保存应用级基础设施引用。
// 职责：集中保存应用启动后可复用的基础设施对象。
// 使用：service/helper/core 初始化流程通过 GLOBAL 共享数据库、配置、HTTP 客户端和 Wails 上下文。
// 边界：只保存全局依赖，不在这里执行初始化逻辑或承载业务状态。
type Global struct {
	// DB 是应用启动后打开的 SQLite 数据库连接。
	// 职责：提供全局 GORM 访问入口。
	// 使用：service/helper 在数据库初始化完成后执行 CRUD。
	// 边界：不得在未初始化时使用，不在这里创建或关闭连接。
	DB *gorm.DB
	// DBPath 是当前运行实例使用的 SQLite 数据库文件路径。
	// 职责：记录运行期数据库文件位置。
	// 使用：诊断、日志或需要展示数据库位置时读取。
	// 边界：只读记录路径，不代表文件一定仍存在。
	DBPath string
	// Configure 保存数据库驱动的配置项访问入口。
	// 职责：提供配置单元集合。
	// 使用：service/helper 读取或写入系统配置时访问。
	// 边界：依赖 DB 初始化，不缓存配置值。
	Configure *Configure
	// ServerClient 是基于 Configure.ServerAddress 创建的私有应用服务器客户端。
	// 职责：复用访问应用服务器的连接与基础地址，是客户端后端与应用服务器通信的唯一出口。
	// 使用：账号等需要访问应用服务器的 service/helper 调用；Thunderstore、Gitee 等第三方地址不得经过该客户端。
	// 边界：地址未配置时使用默认服务器地址；保存新地址时由 ConfigurationService.SetConfig 整体替换，不在这里处理业务重试。
	ServerClient *HTTPClient
	// RuntimeContext 是 Wails 运行时上下文，用于服务层向前端发送事件。
	// 职责：保存 Wails runtime 事件发送上下文。
	// 使用：进度事件、状态事件等 helper 发送前端通知时读取。
	// 边界：未启动 Wails 时可能为空，调用方需做空值保护。
	RuntimeContext context.Context
}

// GLOBAL 是应用启动后可复用的全局基础设施容器。
// 职责：作为进程内唯一基础设施持有者。
// 使用：启动流程写入，service/helper 读取。
// 边界：不应存放请求级状态、页面状态或业务缓存。
var GLOBAL = new(Global)
