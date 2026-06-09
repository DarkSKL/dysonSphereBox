package core

import (
	"errors"
	"fmt"

	systemModel "DysonSphereBox/model/system"

	"gorm.io/gorm"
)

type Configure struct {
	GameStartFilePath *ConfigUnit
	BaseURL           *ConfigUnit
	GameServerArchive *ConfigUnit
}

func InitConfigure() *Configure {
	return &Configure{
		GameStartFilePath: InitConfigUnit("Game", "StartupFilePath"),
		BaseURL:           InitConfigUnit("System", "BaseURL"),
		GameServerArchive: InitConfigUnit("Game", "ServerArchive"),
	}
}

type ConfigUnit struct {
	Group string
	Key   string
}

func InitConfigUnit(group string, key string) *ConfigUnit {
	return &ConfigUnit{
		Group: group,
		Key:   key,
	}
}

// Get 直接从 SQLite 中获取值。
func (model *ConfigUnit) Get() (string, error) {
	if err := model.validate(); err != nil {
		return "", err
	}

	var config systemModel.Config
	err := GLOBAL.DB.
		Where(&systemModel.Config{Group: model.Group, Key: model.Key}).
		First(&config).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get config %s.%s: %w", model.Group, model.Key, err)
	}

	return config.Value, nil
}

// Set 直接向 SQLite 写入值。
func (model *ConfigUnit) Set(val string) error {
	if err := model.validate(); err != nil {
		return err
	}

	config := systemModel.Config{
		Group: model.Group,
		Key:   model.Key,
	}

	err := GLOBAL.DB.
		Where(&systemModel.Config{Group: model.Group, Key: model.Key}).
		Assign(systemModel.Config{Value: val}).
		FirstOrCreate(&config).
		Error
	if err != nil {
		return fmt.Errorf("set config %s.%s: %w", model.Group, model.Key, err)
	}

	return nil
}

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
