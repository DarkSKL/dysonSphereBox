package shared

// 文件职责：提供安装器子包共用的数据库访问入口。
// 使用：V2 安装器、旧官方整合包安装器在进入持久化读写前调用 RequireDatabase。
// 边界：只校验 core.GLOBAL.DB 是否可用，不定义模型、不发起事务、不承载任何安装业务。

import (
	"DysonSphereBox/core"
	"fmt"

	"gorm.io/gorm"
)

// RequireDatabase 返回已初始化的全局数据库连接。
// 职责：把安装器内部分散的全局数据库空值检查收敛到单一入口。
// 使用：任何安装器子包在读写 SQLite 前调用；调用方继续决定查询、事务和错误包装。
// 边界：数据库未初始化时返回错误，不尝试初始化数据库，也不吞掉上层业务错误。
func RequireDatabase() (*gorm.DB, error) {
	if core.GLOBAL.DB == nil {
		return nil, fmt.Errorf("database is not initialized")
	}

	return core.GLOBAL.DB, nil
}
