package system

// 文件职责：提供系统配置读取、写入和配置项元数据查询的 Wails 服务。
// 使用：设置页通过生成绑定读取配置、保存 BaseURL、游戏目录等系统配置。
// 边界：只编排配置写入和必要运行时刷新；具体配置单元持久化由 core.ConfigUnit 负责。

import (
	"fmt"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	"DysonSphereBox/utils/dysonsphere"
)

// ConfigurationService 提供前端可调用的配置读写能力。
// 职责：读取和保存数据库中的系统配置，并在关键配置变化时同步刷新运行时资源。
// 使用：设置页通过 Wails 绑定读取配置项、保存 BaseURL、游戏路径等配置。
// 边界：只校验已知关键配置；远端接口可用性和其它配置的领域规则仍由对应业务服务负责。
type ConfigurationService struct{}

// GetConfig 按 group 和 key 读取唯一配置项。
// 职责：读取指定分组与键名的配置值。
// 使用：前端初始化设置表单或业务服务需要读取动态配置时调用。
// 边界：配置不存在时返回空字符串；数据库未初始化时由 ConfigUnit 返回错误。
func (model *ConfigurationService) GetConfig(group string, key string) (string, error) {
	return core.InitConfigUnit(group, key).Get()
}

// SetConfig 按 group 和 key 写入唯一配置项。
// 职责：保存配置值，校验并归一化游戏路径，并在 BaseURL 变化时刷新全局 HTTP 客户端。
// 使用：设置页用户提交配置时调用。
// 边界：Game/StartupFilePath 必须指向 DSPGAME.exe 或包含该文件的目录；其它配置只写入数据库。
// 逻辑流：1. 归一化游戏路径或拒绝无效值 => 2. BaseURL 先创建 HTTPClient 校验格式 => 3. 写入 ConfigUnit => 4. 必要时刷新 GLOBAL.HTTPClient 并记录日志。
func (model *ConfigurationService) SetConfig(group string, key string, value string) error {
	if group == "Game" && key == "StartupFilePath" {
		normalizedPath := dysonsphere.NormalizeDir(value)
		if normalizedPath == "" {
			err := fmt.Errorf("未在指定路径下找到 %s", dysonsphere.ExeName)
			systemHelper.RecordSetConfigFailure(group, key, err)
			return err
		}
		value = normalizedPath
	}

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
// 职责：提供设置页可展示和编辑的配置元数据。
// 使用：前端渲染配置表单或配置项列表时调用。
// 边界：只返回 helper 定义的配置清单，不读取具体配置值。
func (model *ConfigurationService) GetConfigUnits() ([]systemHelper.ConfigUnitInfo, error) {
	return systemHelper.GetConfigUnits()
}
