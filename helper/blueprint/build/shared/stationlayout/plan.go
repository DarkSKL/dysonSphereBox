package stationlayout

// 文件职责：从显式准备结果生成物流塔布局计划，不访问旧 Builder。
// 使用：行星内及星际物流塔组合的布局实现；边界：只执行分区、依赖排序、均衡分组与端口预算，不装配实体。

import (
	"context"
	"sort"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/zoning"
)

// Plan 职责：生成可独立验证的分区与塔组计划；使用：组合布局阶段；边界：只返回完整计划，输入切片与规范物料图保持只读。
// 逻辑流：1. 校验阶段输入 => 2. 创建并稳定排序分区 => 3. 均衡机器行 => 4. 为规范剩余边预留出口。
func Plan(ctx context.Context, input contract.StationLayoutInput, config contract.StationConfig) (contract.StationLayoutPlan, error) {
	if input.FlowGraph == nil {
		return contract.StationLayoutPlan{}, &blueprint.Error{Code: "GEN_E_FLOW", Message: "物流塔生成缺少规范物料图"}
	}
	if len(input.Productions) != len(input.Plan.Productions) || config.ItemSlots <= 0 || config.ItemSlots > 12 {
		return contract.StationLayoutPlan{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局准备结果或槽位配置无效"}
	}
	if ctx.Err() != nil {
		return contract.StationLayoutPlan{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔布局已取消或超时"}
	}
	zones := zoning.Plan(input.Plan)
	order := make(map[int]int, len(input.FlowGraph.ProductionOrder))
	for rank, index := range input.FlowGraph.ProductionOrder {
		order[index] = rank
	}
	plan := contract.StationLayoutPlan{Zones: make([]contract.StationZonePlan, 0, len(zones)), ProductionCount: len(input.Plan.Productions)}
	for _, zone := range zones {
		sort.SliceStable(zone.ProductionIndices, func(left, right int) bool {
			return order[zone.ProductionIndices[left]] < order[zone.ProductionIndices[right]]
		})
		zonePlan := contract.StationZonePlan{Zone: zone, Productions: make([]contract.StationProductionPlan, 0, len(zone.ProductionIndices))}
		for _, index := range zone.ProductionIndices {
			if ctx.Err() != nil {
				return contract.StationLayoutPlan{}, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔布局已取消或超时"}
			}
			prepared := input.Productions[index]
			production := input.Plan.Productions[index]
			routeCount := routeKinds(prepared)
			counts, perGroup, err := GroupRows(prepared.Counts, routeCount, config.ItemSlots)
			if err != nil {
				return contract.StationLayoutPlan{}, err
			}
			total := 0
			for _, count := range counts {
				total += count
			}
			if len(counts) == 0 || total != production.MachineCount {
				return contract.StationLayoutPlan{}, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "物流塔布局拆列与规范机器总数不一致"}
			}
			leftovers := len(LeftoverKinds(input.FlowGraph, index))
			perGroup = min(perGroup, max(1, 12/max(1, min(routeCount, config.ItemSlots)+leftovers)))
			zonePlan.Productions = append(zonePlan.Productions, contract.StationProductionPlan{Index: index, Counts: counts, PerGroup: perGroup})
		}
		plan.Zones = append(plan.Zones, zonePlan)
	}
	return plan, nil
}
