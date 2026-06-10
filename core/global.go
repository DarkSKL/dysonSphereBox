package core

import (
	"context"

	"gorm.io/gorm"
)

// Global 保存应用级基础设施引用。
// 用途：为服务层提供数据库、配置与 HTTP 客户端等单例资源。
// 边界：只保存全局依赖，不在这里执行初始化逻辑或承载业务状态。
type Global struct {
	// DB 是应用启动后打开的 SQLite 数据库连接。
	DB *gorm.DB
	// DBPath 是当前运行实例使用的 SQLite 数据库文件路径。
	DBPath string
	// Configure 保存数据库驱动的配置项访问入口。
	Configure *Configure
	// HTTPClient 是基于 Configure.BaseURL 创建的全局 HTTP 客户端。
	HTTPClient *HTTPClient
	// RuntimeContext 是 Wails 运行时上下文，用于服务层向前端发送事件。
	RuntimeContext context.Context
}

// GLOBAL 是应用启动后可复用的全局基础设施容器。
var GLOBAL = new(Global)
