package stationlayout

// 文件职责：把共享物流塔设备事实与可选行数策略组合为规划输入。
// 使用：工作流工具和纯布局核心在准备塔计划时调用；边界：策略不得改写原型槽数、远程角色或容量类型。

import (
	"fmt"

	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/transport"
)

// Configuration 职责：返回指定真实物流塔的装配配置；使用：调用者仅传原型与行数策略；边界：未知设备和负行数直接失败，不借用其他塔的能力。
func Configuration(itemID, preferredRows int) (contract.StationConfig, error) {
	if preferredRows < 0 {
		return contract.StationConfig{}, fmt.Errorf("物流塔优先行数不能为负数")
	}
	config := contract.StationConfig{StationID: itemID, PreferredRows: preferredRows, EnableLeftovers: true}
	switch itemID {
	case 2103:
		config.Mode = 3
	case 2104:
		config.Mode = 4
		config.DualRoleFields, config.LargeCapacity = true, true
	default:
		return contract.StationConfig{}, fmt.Errorf("物流塔原型 %d 缺少已核对设备能力", itemID)
	}
	var err error
	config.ItemSlots, err = transport.StationItemKindLimit(itemID)
	if err != nil {
		return contract.StationConfig{}, err
	}
	return config, nil
}

// ValidateConfiguration 职责：阻止流程参数或核心调用者覆盖物流塔物理事实；使用：共享规划与验证入口；边界：只允许行数策略不同，不放宽副产物与实体端口约束。
func ValidateConfiguration(config contract.StationConfig) error {
	expected, err := Configuration(config.StationID, config.PreferredRows)
	if err != nil {
		return err
	}
	if expected != config {
		return fmt.Errorf("物流塔 %d 的配置与共享设备事实不一致", config.StationID)
	}
	return nil
}
