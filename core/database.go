package core

import (
	"database/sql"
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

	// AutoMigrate 用于基础表结构同步；复杂字段重命名或数据迁移后续单独做版本迁移。
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

// migrateModPackageComponentJoinTable 将早期 GORM 多对多中间表迁移为 key 关联表。
// 边界：只处理 mod_installed_package_components，避免旧外键阻塞 SQLite AutoMigrate。
func migrateModPackageComponentJoinTable(db *gorm.DB) error {
	if !db.Migrator().HasTable("mod_installed_package_components") {
		return nil
	}

	columns, err := readSQLiteTableColumns(db, "mod_installed_package_components")
	if err != nil {
		return fmt.Errorf("inspect mod package component join table: %w", err)
	}
	if columns["package_key"] && columns["component_key"] {
		return nil
	}

	statements := []string{
		"DROP TABLE IF EXISTS mod_installed_package_components_v2",
		`CREATE TABLE mod_installed_package_components_v2 (
			id integer PRIMARY KEY AUTOINCREMENT,
			package_key text NOT NULL,
			component_key text NOT NULL,
			created_at datetime
		)`,
	}
	if columns["mod_installed_package_id"] && columns["mod_installed_component_id"] &&
		db.Migrator().HasTable("mod_installed_packages") &&
		db.Migrator().HasTable("mod_installed_components") {
		statements = append(statements, `INSERT INTO mod_installed_package_components_v2 (package_key, component_key, created_at)
			SELECT DISTINCT packages.package_key, components.component_key, CURRENT_TIMESTAMP
			FROM mod_installed_package_components relations
			INNER JOIN mod_installed_packages packages ON packages.id = relations.mod_installed_package_id
			INNER JOIN mod_installed_components components ON components.id = relations.mod_installed_component_id
			WHERE packages.package_key <> '' AND components.component_key <> ''`)
	}
	statements = append(statements,
		"DROP TABLE mod_installed_package_components",
		"ALTER TABLE mod_installed_package_components_v2 RENAME TO mod_installed_package_components",
	)

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("migrate mod package component join table: %w", err)
		}
	}
	return nil
}

// readSQLiteTableColumns 读取 SQLite 表字段集合。
// 用途：迁移前判断旧表结构，避免对已是新结构的表重复改写。
func readSQLiteTableColumns(db *gorm.DB, tableName string) (map[string]bool, error) {
	rows, err := db.Raw("PRAGMA table_info(" + tableName + ")").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
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
