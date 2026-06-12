package core

// 文件职责：维护应用基础设施模块 database。
// 使用：由启动流程初始化，service/helper 在运行期读取已初始化资源。
// 边界：不实现具体业务用例，不直接处理 Mod 安装、市场查询或页面交互。

import (
	"fmt"

	"DysonSphereBox/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// databaseFileName 是运行期 SQLite 数据库文件名。
// 职责：统一数据库文件在 exe 同级目录下的命名。
// 使用：databasePath 组装真实数据库路径时读取。
// 边界：不包含目录信息，目录由 os.Executable 解析。
const databaseFileName = "dyson-sphere-box.db"

// InitDatabase 初始化本地 SQLite 数据库。
// 职责：创建数据库文件、打开 GORM、执行迁移并初始化依赖数据库的全局资源。
// 使用：应用启动时传入 main.go go:embed 的 databaseTemplate 调用一次。
// 边界：失败时关闭已打开连接并保持 GLOBAL 不指向半初始化资源。
// 逻辑流：1. 解析运行期数据库路径 => 2. 确保模板库文件存在 => 3. 打开 SQLite/GORM => 4. 执行兼容迁移和 AutoMigrate => 5. 初始化 Configure 与 HTTPClient。
func InitDatabase(databaseTemplate []byte) error {
	dbPath, err := databasePath()
	if err != nil {
		return err
	}

	if err := ensureDatabaseFile(dbPath, databaseTemplate); err != nil {
		return err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sqlite database handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.Exec("PRAGMA foreign_keys = OFF;").Error; err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("disable sqlite foreign keys for migration: %w", err)
	}

	if err := db.Exec("PRAGMA journal_mode = DELETE;").Error; err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("disable sqlite wal mode: %w", err)
	}

	if err := migrateModPackageComponentJoinTable(db); err != nil {
		_ = closeDatabase(db)
		return err
	}

	// AutoMigrate 职责：同步基础表结构；使用：模型注册更新后自动补齐表结构；边界：复杂字段重命名或数据迁移后续单独做版本迁移。
	if err := db.AutoMigrate(model.RegisterModels()...); err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("auto migrate sqlite database: %w", err)
	}

	if err := db.Exec("PRAGMA foreign_keys = ON;").Error; err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	if err := pingDatabase(db); err != nil {
		_ = closeDatabase(db)
		return err
	}

	GLOBAL.DB = db
	GLOBAL.DBPath = dbPath
	GLOBAL.Configure = InitConfigure()

	httpClient, err := InitHTTPClient()
	if err != nil {
		_ = closeDatabase(db)
		GLOBAL.DB = nil
		GLOBAL.DBPath = ""
		GLOBAL.Configure = nil
		return fmt.Errorf("init http client: %w", err)
	}

	GLOBAL.HTTPClient = httpClient
	return nil
}

// CloseDatabase 关闭全局 SQLite 连接并清理依赖数据库的全局资源。
// 职责：释放数据库连接并清空 Configure/HTTPClient 等数据库派生资源。
// 使用：应用退出或测试清理时调用。
// 边界：数据库未初始化时直接返回 nil；不删除数据库文件。
func CloseDatabase() error {
	if GLOBAL.DB == nil {
		return nil
	}

	err := closeDatabase(GLOBAL.DB)
	GLOBAL.DB = nil
	GLOBAL.DBPath = ""
	GLOBAL.Configure = nil
	GLOBAL.HTTPClient = nil
	return err
}
