package system

import (
	"errors"
	"reflect"

	"DysonSphereBox/core"
)

// ConfigUnitInfo 描述一个可被前端识别和编辑的配置项。
type ConfigUnitInfo struct {
	// Name 是 Configure 结构体中的配置字段名称。
	Name string `json:"name"`
	// Group 是配置在数据库中的分组。
	Group string `json:"group"`
	// Key 是配置在数据库中的键名。
	Key string `json:"key"`
}

// GetConfigUnits 返回前端可操作的配置项清单。
func GetConfigUnits() ([]ConfigUnitInfo, error) {
	configureValue, err := configureValue()
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

func configureValue() (reflect.Value, error) {
	if core.GLOBAL.Configure == nil {
		return reflect.Value{}, errors.New("configure is not initialized")
	}

	return reflect.ValueOf(core.GLOBAL.Configure).Elem(), nil
}
