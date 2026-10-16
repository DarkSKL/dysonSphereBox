package system

// 文件职责：提供系统配置读取、写入和配置项元数据查询的 Wails 服务。
// 使用：设置页通过生成绑定读取配置、保存游戏目录等系统配置。
// 边界：只编排配置写入和必要运行时刷新；具体配置单元持久化由 core.ConfigUnit 负责。

import (
	"fmt"
	"strings"

	"DysonSphereBox/core"
	systemHelper "DysonSphereBox/helper/system"
	accountHelper "DysonSphereBox/helper/system/account"
	"DysonSphereBox/utils"
	"DysonSphereBox/utils/dysonsphere"
)

// ConfigurationService 提供前端可调用的配置读写能力。
// 职责：读取和保存数据库中的系统配置，并在关键配置变化时同步刷新运行时资源。
// 使用：设置页通过 Wails 绑定读取配置项、保存游戏路径等配置。
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
// 职责：保存配置值，校验并归一化游戏路径，并在服务器地址变化时刷新全局 HTTP 客户端。
// 使用：设置页用户提交配置时调用。
// 边界：Game/StartupFilePath 必须指向 DSPGAME.exe 或包含该文件的目录；System/ServerAddress 为空表示恢复默认，
// 非空时必须是 http/https 基础地址；其它配置只写入数据库。
// 逻辑流：1. 按配置键归一化值或拒绝无效值 => 2. 服务器地址交给 setServerAddress 刷新 GLOBAL.ServerClient => 3. 写入 ConfigUnit => 4. 记录配置变更结果。
func (model *ConfigurationService) SetConfig(group string, key string, value string) error {
	switch {
	case group == "Game" && key == "StartupFilePath":
		normalizedPath := dysonsphere.NormalizeDir(value)
		if normalizedPath == "" {
			err := fmt.Errorf("未在指定路径下找到 %s", dysonsphere.ExeName)
			systemHelper.RecordSetConfigFailure(group, key, err)
			return err
		}
		value = normalizedPath
	case group == "System" && key == "ServerAddress":
		return model.setServerAddress(group, key, value)
	}

	unit := core.InitConfigUnit(group, key)
	previous, readErr := unit.Get()
	if err := unit.Set(value); err != nil {
		systemHelper.RecordSetConfigFailure(group, key, err)
		return err
	}

	if readErr != nil || previous != value {
		systemHelper.RecordSetConfigSuccess(group, key)
	}
	return nil
}

// setServerAddress 保存服务器连接地址并刷新全局 HTTP 客户端。
// 职责：校验并归一化地址，先构造新客户端再持久化，成功后替换 GLOBAL.ServerClient；生效地址变化时清除账号会话。
// 使用：SetConfig 命中 System/ServerAddress 时调用。
// 边界：空值表示恢复默认地址；格式无效时不写入配置、不替换客户端。
// 逻辑流：1. 归一化地址 => 2. 构造新 ServerClient 校验格式 => 3. 写入 ConfigUnit => 4. 替换 GLOBAL.ServerClient => 5. 地址变化时清除账号会话 => 6. 记录日志。
func (model *ConfigurationService) setServerAddress(group string, key string, value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		normalizedAddress, err := utils.NormalizeHTTPBaseURL(value)
		if err != nil {
			err = fmt.Errorf("服务器地址无效：%w", err)
			systemHelper.RecordSetConfigFailure(group, key, err)
			return err
		}
		value = normalizedAddress
	}

	httpClient, err := core.NewServerClient(value)
	if err != nil {
		err = fmt.Errorf("服务器地址无效：%w", err)
		systemHelper.RecordSetConfigFailure(group, key, err)
		return err
	}

	if err := core.InitConfigUnit(group, key).Set(value); err != nil {
		systemHelper.RecordSetConfigFailure(group, key, err)
		return err
	}

	previousAddress := core.GLOBAL.ServerClient.BaseURL()
	core.GLOBAL.ServerClient = httpClient
	// 旧服务器签发的 token 不应发往新服务器
	if previousAddress != httpClient.BaseURL() {
		_ = accountHelper.ClearSession()
		systemHelper.RecordSetConfigSuccess(group, key)
	}

	return nil
}

// GetConfigUnits 返回前端可操作的配置项清单。
// 职责：提供设置页可展示和编辑的配置元数据。
// 使用：前端渲染配置表单或配置项列表时调用。
// 边界：只返回 helper 定义的配置清单，不读取具体配置值。
func (model *ConfigurationService) GetConfigUnits() ([]systemHelper.ConfigUnitInfo, error) {
	return systemHelper.GetConfigUnits()
}
