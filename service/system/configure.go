package system

import (
	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
)

// ConfigurationService 提供前端可调用的配置读写能力。
// 用途：读取和保存数据库中的系统配置，并在关键配置变化时同步刷新运行时资源。
type ConfigurationService struct{}

// GetConfig 按 group 和 key 读取唯一配置项。
func (model *ConfigurationService) GetConfig(group string, key string) (string, error) {
	return core.InitConfigUnit(group, key).Get()
}

// SetConfig 按 group 和 key 写入唯一配置项。
func (model *ConfigurationService) SetConfig(group string, key string, value string) error {
	if group == "System" && key == "BaseURL" {
		httpClient, err := core.NewHTTPClient(value)
		if err != nil {
			systemHelper.RecordSetConfigFailure(group, key, err)
			return err
		}

		if err := core.InitConfigUnit(group, key).Set(value); err != nil {
			systemHelper.RecordSetConfigFailure(group, key, err)
			return err
		}

		core.GLOBAL.HTTPClient = httpClient
		systemHelper.RecordSetConfigSuccess(group, key)
		return nil
	}

	if err := core.InitConfigUnit(group, key).Set(value); err != nil {
		systemHelper.RecordSetConfigFailure(group, key, err)
		return err
	}

	systemHelper.RecordSetConfigSuccess(group, key)
	return nil
}

// GetConfigUnits 返回前端可操作的配置项清单。
func (model *ConfigurationService) GetConfigUnits() ([]systemHelper.ConfigUnitInfo, error) {
	return systemHelper.GetConfigUnits()
}
