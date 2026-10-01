package core

// 文件职责：实现 ConfigUnit 对 system_configs 表的读写。
// 使用：service/helper 通过 GLOBAL.Configure 中的配置单元读取或保存运行期配置。
// 边界：只处理基础设施层读写和前置校验，不解释配置值业务含义、不刷新 HTTP 客户端。

import (
	"errors"
	"fmt"

	systemModel "DysonSphereBox/model/system"
)

// Get 直接从 SQLite 中获取值。
// 职责：按 ConfigUnit 的分组和键名读取单个配置值。
// 使用：service/helper 需要最新配置时调用。
// 边界：配置不存在时返回空字符串；数据库未初始化或查询失败时返回错误。
func (model *ConfigUnit) Get() (string, error) {
	if err := model.validate(); err != nil {
		return "", err
	}

	var config systemModel.Config
	result := GLOBAL.DB.
		Where(&systemModel.Config{Group: model.Group, Key: model.Key}).
		Order("id").
		Limit(1).
		Find(&config)
	if result.Error != nil {
		return "", fmt.Errorf("get config %s.%s: %w", model.Group, model.Key, result.Error)
	}
	if result.RowsAffected == 0 {
		return "", nil
	}

	return config.Value, nil
}

// Set 直接向 SQLite 写入值。
// 职责：按 ConfigUnit 的分组和键名创建或更新配置值。
// 使用：配置服务保存用户设置时调用。
// 边界：数据库未初始化或写入失败时返回错误，不做业务合法性校验；空字符串会真实写入，用于清除配置。
func (model *ConfigUnit) Set(val string) error {
	if err := model.validate(); err != nil {
		return err
	}

	config := systemModel.Config{
		Group: model.Group,
		Key:   model.Key,
	}

	// Assign 必须使用 map：GORM 对结构体会忽略零值字段，Set("") 将无法清空已有值。
	err := GLOBAL.DB.
		Where(&systemModel.Config{Group: model.Group, Key: model.Key}).
		Assign(map[string]interface{}{"value": val}).
		FirstOrCreate(&config).
		Error
	if err != nil {
		return fmt.Errorf("set config %s.%s: %w", model.Group, model.Key, err)
	}

	return nil
}

// validate 校验配置访问器是否具备数据库读写前置条件。
// 职责：统一阻断空分组、空键名和未初始化数据库。
// 使用：Get/Set 执行数据库操作前调用。
// 边界：只做基础设施校验，不检查配置值内容是否符合业务规则。
func (model *ConfigUnit) validate() error {
	if model == nil {
		return errors.New("config unit is nil")
	}
	if model.Group == "" {
		return errors.New("config group cannot be empty")
	}
	if model.Key == "" {
		return errors.New("config key cannot be empty")
	}
	if GLOBAL.DB == nil {
		return errors.New("database is not initialized")
	}

	return nil
}
