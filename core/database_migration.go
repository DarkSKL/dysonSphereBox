package core

// 文件职责：维护 SQLite 数据库初始化前需要执行的兼容性迁移。
// 使用：InitDatabase 在 AutoMigrate 前调用 migrateModPackageComponentJoinTable，避免旧结构阻塞新模型迁移。
// 边界：只处理历史结构兼容，不定义模型、不执行业务数据修复、不替代正式版本迁移系统。

import (
	"database/sql"
	"fmt"

	"gorm.io/gorm"
)

// migrateModPackageComponentJoinTable 将早期 GORM 多对多中间表迁移为 key 关联表。
// 职责：兼容旧版本 mod_installed_package_components 表结构。
// 使用：InitDatabase 在 AutoMigrate 前调用，避免旧外键结构阻塞新模型迁移。
// 边界：只处理 mod_installed_package_components，避免旧外键阻塞 SQLite AutoMigrate。
// 逻辑流：1. 检查表和字段 => 2. 创建新结构临时表 => 3. 从旧关联表转换 key 数据 => 4. 替换旧表。
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
// 职责：读取指定表的字段名集合。
// 使用：迁移前判断旧表结构，避免对已是新结构的表重复改写。
// 边界：只读取 PRAGMA table_info，不修改表结构。
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
