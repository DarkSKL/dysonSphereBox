package stationlayout

// 文件职责：在阶段交接前校验物流塔布局输出的守恒和实体容量边界。
// 使用：统一编排验证任意组合布局结果后才允许装配；边界：不限定候选形状，不读取或创建建筑，不替代最终几何校验。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
)

// routeKinds 职责：统计独立输入输出物品及实际供剂口；使用：分组与交接校验共用同一预算；边界：同物品同时出入仍占两个方向，不按净流抵消物理接口。
func routeKinds(prepared contract.PreparedStationProduction) int {
	inputs, outputs := map[int]bool{}, map[int]bool{}
	for _, item := range prepared.Recipe.Items {
		inputs[item] = true
	}
	for _, item := range prepared.Recipe.Results {
		outputs[item] = true
	}
	if prepared.SprayItemID > 0 {
		inputs[prepared.SprayItemID] = true
	}
	return len(inputs) + len(outputs)
}

// Validate 职责：拒绝漏产线、重复装配、零分组及超容量布局；使用：布局到装配交接；边界：只接受完整计划，失败时尚未创建实体。
func Validate(ctx context.Context, input contract.StationLayoutInput, plan contract.StationLayoutPlan, config contract.StationConfig) error {
	if input.FlowGraph == nil || len(input.Productions) != len(input.Plan.Productions) || plan.ProductionCount != len(input.Plan.Productions) || len(plan.Zones) == 0 || config.ItemSlots <= 0 || config.ItemSlots > 12 {
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局阶段输出不完整"}
	}
	seen := make([]bool, len(input.Plan.Productions))
	zoneKeys := map[string]bool{}
	for _, zone := range plan.Zones {
		if zone.Zone.Key == "" || zoneKeys[zone.Zone.Key] || len(zone.Productions) != len(zone.Zone.ProductionIndices) {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局分区身份重复或产线归属不完整"}
		}
		zoneKeys[zone.Zone.Key] = true
		for position, work := range zone.Productions {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔布局交接已取消或超时"}
			}
			if work.Index < 0 || work.Index >= len(seen) || seen[work.Index] || zone.Zone.ProductionIndices[position] != work.Index || work.PerGroup <= 0 || len(work.Counts) == 0 {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局存在无效下标、重复产线或空分组"}
			}
			seen[work.Index] = true
			prepared := input.Productions[work.Index]
			longest := 0
			for _, count := range prepared.Counts {
				longest = max(longest, count)
			}
			total := 0
			for _, count := range work.Counts {
				if count <= 0 || count > longest || count > 10000 || total > 10000-count {
					return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局行长超过已验证容量"}
				}
				total += count
			}
			portsPerRow := max(1, min(routeKinds(prepared), config.ItemSlots)+len(LeftoverKinds(input.FlowGraph, work.Index)))
			if total != input.Plan.Productions[work.Index].MachineCount || work.PerGroup > 12/portsPerRow {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局机器数不守恒或超过十二口预算"}
			}
		}
	}
	for _, planned := range seen {
		if !planned {
			return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔布局遗漏规范产线"}
		}
	}
	if ctx.Err() != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔布局交接已取消或超时"}
	}
	return nil
}
