package dense

// 文件职责：提供行星塔全场密铺的容量拆行准备。
// 使用：mode3 dense 组合；边界：保持同模式塔型能力，不建立配方独占庭院或闭合塔组。

import (
	"DysonSphereBox/helper/blueprint/build/adapters/mode3/common"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
	"context"
)

// configuration 职责：保留塔型能力及容量需要的最少行数；使用：全场布局与装配；边界：不改变塔槽数或单带容量。
func configuration() contract.StationConfig {
	config := common.Configuration()
	config.PreferredRows = 1
	return config
}

// Adapter 职责：提供密集组合的实体配置与独立布局阶段；使用：组合注册；边界：不调用 standard 组合。
func Adapter() contract.StationAdapter {
	return contract.StationAdapter{Config: configuration(), Layout: layout}
}

// layout 职责：保留逻辑分区与可信容量拆行供全场搜索；使用：统一塔流水线；边界：不把逻辑产线变成独占矩形。
func layout(ctx context.Context, input contract.StationLayoutInput) (contract.StationLayoutPlan, error) {
	return stationlayout.PlanField(ctx, input, configuration())
}
