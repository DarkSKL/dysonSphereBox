package system

// 文件职责：定义数据库模型或模型注册模块 config。
// 使用：由 core 自动迁移并由 service/helper 通过 GORM 读写。
// 边界：不包含远端接口 DTO、服务编排或文件系统操作。

// Config 保存系统配置项。
// 职责：描述 system_configs 表中的 group/key/value 配置记录。
// 使用：core.ConfigUnit 通过 GORM 读写该模型，服务层通过配置服务间接访问。
// 边界：只保存配置值，不校验配置内容是否符合业务规则。
type Config struct {
	// ID 是配置记录主键。
	// 职责：提供数据库自增主键。
	// 使用：GORM 创建和更新记录时维护。
	// 边界：业务逻辑不应依赖该值定位配置。
	ID uint64 `gorm:"primary_key;auto_increment"`
	// Group 是配置分组。
	// 职责：区分系统、游戏等配置命名空间。
	// 使用：ConfigUnit 查询唯一配置时作为条件之一。
	// 边界：不在模型层限制枚举值。
	Group string `gorm:"type:varchar(255);" json:"group"`
	// Key 是配置键名。
	// 职责：定位分组下的具体配置项。
	// 使用：ConfigUnit 查询唯一配置时作为条件之一。
	// 边界：不在模型层解释键名业务含义。
	Key string `gorm:"type:varchar(255);" json:"key"`
	// Value 是配置值文本。
	// 职责：保存配置的原始字符串值。
	// 使用：ConfigUnit.Get/Set 读取和写入。
	// 边界：不在模型层解析类型或校验路径、URL 等业务格式。
	Value string `gorm:"type:text" json:"value"`
}
