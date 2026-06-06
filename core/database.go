package core

import (
	"fmt"
	"os"
	"path/filepath"

	"DysonSphereBox/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const databaseFileName = "dyson-sphere-box.db"

// InitDatabase 初始化本地 SQLite 数据库。
// databaseTemplate 来自 main.go 的 go:embed，首次启动时会复制到 exe 同级目录。
func InitDatabase(databaseTemplate []byte) error {
	dbPath, err := databasePath()
	if err != nil {
		return err
	}

	if err := ensureDatabaseFile(dbPath, databaseTemplate); err != nil {
		return err
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.Exec("PRAGMA foreign_keys = ON;").Error; err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	if err := db.Exec("PRAGMA journal_mode = DELETE;").Error; err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("disable sqlite wal mode: %w", err)
	}

	// AutoMigrate 用于基础表结构同步；复杂字段重命名或数据迁移后续单独做版本迁移。
	if err := db.AutoMigrate(model.RegisterModels()...); err != nil {
		_ = closeDatabase(db)
		return fmt.Errorf("auto migrate sqlite database: %w", err)
	}

	if err := pingDatabase(db); err != nil {
		_ = closeDatabase(db)
		return err
	}

	GLOBAL.DB = db
	GLOBAL.DBPath = dbPath
	GLOBAL.Configure = InitConfigure()
	return nil
}

func CloseDatabase() error {
	if GLOBAL.DB == nil {
		return nil
	}

	err := closeDatabase(GLOBAL.DB)
	GLOBAL.DB = nil
	GLOBAL.Configure = nil
	return err
}

func databasePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exePath), databaseFileName), nil
}

func ensureDatabaseFile(path string, databaseTemplate []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect sqlite database: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create sqlite database directory: %w", err)
	}

	if len(databaseTemplate) == 0 {
		return fmt.Errorf("embedded sqlite database is empty")
	}

	if err := os.WriteFile(path, databaseTemplate, 0644); err != nil {
		return fmt.Errorf("write sqlite database: %w", err)
	}

	return nil
}

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
