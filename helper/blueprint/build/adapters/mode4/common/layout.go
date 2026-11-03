package common

// 文件职责：维护星际物流塔模式共用的布局能力与阶段选择。
// 使用：本模式不同密度组合显式引用；边界：不依赖其他组合目录或旧 build 包，不保存任务状态。

import (
	"context"

	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
)

// Configuration 职责：提供星际塔的五槽及远程角色能力；使用：本模式阶段组合；边界：按值返回，不修改行星内塔配置。
func Configuration() contract.StationConfig {
	return contract.StationConfig{Mode: 4, StationID: 2104, ItemSlots: 5, DualRoleFields: true, LargeCapacity: true, EnableLeftovers: true}
}

// Layout 职责：按星际塔五物品槽执行布局规划；使用：本模式标准与预留组合；边界：共享纯规划原语，不接收私有 Builder 或运行时回调。
func Layout(ctx context.Context, input contract.StationLayoutInput) (contract.StationLayoutPlan, error) {
	return stationlayout.Plan(ctx, input, Configuration())
}
