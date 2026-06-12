package system

// 文件职责：提供系统配置单元的反射枚举能力。
// 使用：由 ConfigurationService.GetConfigUnits 调用，把 core.Configure 中可编辑配置暴露给前端设置页。
// 边界：不读取或写入配置值，不创建配置项，不决定配置保存流程。

import (
	"errors"
	"reflect"

	"DysonSphereBox/core"
)

// ConfigUnitInfo 描述一个可被前端识别和编辑的配置项。
// 职责：承载配置字段名、数据库分组和键名的展示/编辑元数据。
// 使用：设置页生成配置表单或诊断配置清单时读取。
// 边界：不包含配置值本身，配置值仍通过 ConfigUnit 单独读取。
type ConfigUnitInfo struct {
	// Name 职责：保存 Configure 结构体中的字段名称；使用：前端展示配置项来源；边界：不作为数据库键。
	Name string `json:"name"`
	// Group 职责：保存配置在数据库中的分组；使用：GetConfig/SetConfig 定位配置项；边界：不能为空，否则配置读写会失去稳定范围。
	Group string `json:"group"`
	// Key 职责：保存配置在数据库中的键名；使用：GetConfig/SetConfig 定位配置项；边界：不能为空，否则配置读写会失去稳定身份。
	Key string `json:"key"`
}

// GetConfigUnits 返回前端可操作的配置项清单。
// 职责：从 core.GLOBAL.Configure 中枚举已初始化的 ConfigUnit 字段。
// 使用：ConfigurationService.GetConfigUnits 调用后返回给设置页。
// 边界：只返回已初始化且类型为 *core.ConfigUnit 的字段；Configure 未初始化时返回错误。
// 逻辑流：1. 读取全局 Configure 反射值 => 2. 遍历字段 => 3. 过滤非 ConfigUnit 或 nil 字段 => 4. 生成 ConfigUnitInfo 列表。
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

// configureValue 返回全局配置对象的可反射值。
// 职责：集中校验 core.GLOBAL.Configure 是否可用，并返回其 Elem 反射值。
// 使用：GetConfigUnits 遍历配置单元前调用。
// 边界：Configure 未初始化时返回错误，不尝试自动初始化。
func configureValue() (reflect.Value, error) {
	if core.GLOBAL.Configure == nil {
		return reflect.Value{}, errors.New("configure is not initialized")
	}

	return reflect.ValueOf(core.GLOBAL.Configure).Elem(), nil
}
