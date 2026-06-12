package core

// 文件职责：定义单个可读写配置项的访问器结构。
// 使用：Configure 聚合这些访问器，service/helper 通过它们读取或写入 system_configs。
// 边界：结构本身不读写数据库；具体 Get/Set 行为由 config_unit_store.go 维护。

// ConfigUnit 表示一个可读写的配置项。
// 职责：封装 system_configs 表中 group/key/value 的访问规则。
// 使用：由 Configure 暴露给 service/helper 读取或写入配置。
// 边界：配置值存储在 SQLite system_configs 表，读写前会校验数据库已初始化。
type ConfigUnit struct {
	// Group 是配置分组。
	// 职责：区分 Game、System 等配置命名空间。
	// 使用：Get/Set 构造 GORM 查询条件时使用。
	// 边界：不能为空，具体分组语义由调用方约定。
	Group string
	// Key 是配置键名。
	// 职责：定位分组下的具体配置项。
	// 使用：Get/Set 构造 GORM 查询条件时使用。
	// 边界：不能为空，不在此处校验业务含义。
	Key string
}

// InitConfigUnit 创建指定分组和键名的配置单元。
// 职责：生成轻量配置访问器。
// 使用：InitConfigure 统一创建配置集合时调用。
// 边界：这里只构造访问器，不读取或写入数据库。
func InitConfigUnit(group string, key string) *ConfigUnit {
	return &ConfigUnit{
		Group: group,
		Key:   key,
	}
}
