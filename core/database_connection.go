package core

// 文件职责：提供 SQLite/GORM 连接可用性检查和底层连接关闭能力。
// 使用：InitDatabase 完成迁移后调用 pingDatabase，初始化失败或退出时调用 closeDatabase。
// 边界：只处理连接生命周期，不清空 GLOBAL、不删除数据库文件、不检查业务表数据。

import (
	"fmt"

	"gorm.io/gorm"
)

// pingDatabase 校验 SQLite 连接可用。
// 职责：通过底层 sql.DB 执行 Ping，确认连接没有处于不可用状态。
// 使用：InitDatabase 完成迁移后、写入 GLOBAL 前调用。
// 边界：只检查连接，不执行表结构或业务数据校验。
func pingDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sqlite database handle: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}

	return nil
}

// closeDatabase 关闭指定 GORM 底层连接。
// 职责：从 gorm.DB 取出 sql.DB 并释放连接资源。
// 使用：CloseDatabase 或初始化失败回滚时调用。
// 边界：只关闭连接，不清空 GLOBAL，也不删除数据库文件。
func closeDatabase(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sqlite database handle: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close sqlite database: %w", err)
	}

	return nil
}
