package stationlayout

// 文件职责：为物流塔全场密铺保留可信容量行与逻辑分区，取消为局部庭院凑满行数。
// 使用：mode3/4 dense 规划阶段；边界：不创建实体，后续仍必须通过 Validate 与真实塔口预算。

import (
	"DysonSphereBox/helper/blueprint/build/contract"
	"context"
)

// PlanField 职责：在已有分区及身份校验基础上恢复原始容量拆行；使用：全场共享塔库存的实体装配；边界：PerGroup 只保留兼容契约的单行上限，不代表一行独占一塔。
func PlanField(ctx context.Context, input contract.StationLayoutInput, config contract.StationConfig) (contract.StationLayoutPlan, error) {
	plan, err := Plan(ctx, input, config)
	if err != nil {
		return contract.StationLayoutPlan{}, err
	}
	for zone := range plan.Zones {
		for production := range plan.Zones[zone].Productions {
			work := &plan.Zones[zone].Productions[production]
			work.Counts = append([]int(nil), input.Productions[work.Index].Counts...)
			work.PerGroup = 1
		}
	}
	return plan, nil
}
