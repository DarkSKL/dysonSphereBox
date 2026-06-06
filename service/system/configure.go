package system

import (
	"errors"
	"reflect"

	"DysonSphereBox/core"
)

type ConfigurationService struct{}

type ConfigUnitInfo struct {
	Name  string `json:"name"`
	Group string `json:"group"`
	Key   string `json:"key"`
}

// GetConfig 按 group 和 key 读取唯一配置项。
func (model *ConfigurationService) GetConfig(group string, key string) (string, error) {
	return core.InitConfigUnit(group, key).Get()
}

// SetConfig 按 group 和 key 写入唯一配置项。
func (model *ConfigurationService) SetConfig(group string, key string, value string) error {
	return core.InitConfigUnit(group, key).Set(value)
}

// GetConfigUnits 返回前端可操作的配置项清单。
func (model *ConfigurationService) GetConfigUnits() ([]ConfigUnitInfo, error) {
	configureValue, err := model.configureValue()
	if err != nil {
		return nil, err
	}

	configureType := configureValue.Type()
	units := make([]ConfigUnitInfo, 0, configureValue.NumField())
	for index := 0; index < configureValue.NumField(); index++ {
		field := configureValue.Field(index)
		if field.Type() != reflect.TypeOf((*core.ConfigUnit)(nil)) || field.IsNil() {
			continue
		}

		configUnit := field.Interface().(*core.ConfigUnit)
		units = append(units, ConfigUnitInfo{
			Name:  configureType.Field(index).Name,
			Group: configUnit.Group,
			Key:   configUnit.Key,
		})
	}

	return units, nil
}

func (model *ConfigurationService) configureValue() (reflect.Value, error) {
	if core.GLOBAL.Configure == nil {
		return reflect.Value{}, errors.New("configure is not initialized")
	}

	return reflect.ValueOf(core.GLOBAL.Configure).Elem(), nil
}
