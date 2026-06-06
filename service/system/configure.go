package system

import (
	"errors"
	"reflect"

	"DysonSphereBox/core"
)

// ConfigurationService 提供前端可调用的配置读写能力。
// 用途：读取和保存数据库中的系统配置，并在关键配置变化时同步刷新运行时资源。
type ConfigurationService struct{}

// ConfigUnitInfo 描述一个可被前端识别和编辑的配置项。
type ConfigUnitInfo struct {
	// Name 是 Configure 结构体中的配置字段名称。
	Name string `json:"name"`
	// Group 是配置在数据库中的分组。
	Group string `json:"group"`
	// Key 是配置在数据库中的键名。
	Key string `json:"key"`
}

// GetConfig 按 group 和 key 读取唯一配置项。
func (model *ConfigurationService) GetConfig(group string, key string) (string, error) {
	return core.InitConfigUnit(group, key).Get()
}

// SetConfig 按 group 和 key 写入唯一配置项。
func (model *ConfigurationService) SetConfig(group string, key string, value string) error {
	if group == "System" && key == "BaseURL" {
		httpClient, err := core.NewHTTPClient(value)
		if err != nil {
			return err
		}

		if err := core.InitConfigUnit(group, key).Set(value); err != nil {
			return err
		}

		core.GLOBAL.HTTPClient = httpClient
		return nil
	}

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
