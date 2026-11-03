package common

// 文件职责：维护行星内物流塔模式共用的布局能力与阶段选择。
// 使用：本模式不同密度组合显式引用；边界：不依赖其他组合目录或旧 build 包，不保存任务状态。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
)

// Configuration 职责：提供行星内塔的独立装配能力；使用：本模式阶段组合；边界：按值返回，不向星际塔借用可变配置。
func Configuration() contract.StationConfig {
	return contract.StationConfig{Mode: 3, StationID: 2103, ItemSlots: 4, EnableLeftovers: true}
}

// Layout 职责：按行星内塔四物品槽执行布局规划；使用：本模式标准与预留组合；边界：共享纯规划原语，不接收私有 Builder 或运行时回调。
func Layout(ctx context.Context, input contract.StationLayoutInput) (contract.StationLayoutPlan, error) {
	return stationlayout.Plan(ctx, input, Configuration())
}
