package stationlayout

// 文件职责：计算物流塔环绕行分组和真实副产物出口预算。
// 使用：多个物流塔组合共享的纯规划阶段；边界：不创建建筑、不修改物料图或容量拆列结果。

import (
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// GroupRows 职责：在十二口预算内把已验证机器行均衡分配到中央塔四侧；使用：物流塔布局；边界：不增加机器、不超过输入最大行长，无效拆列明确失败。
func GroupRows(counts []int, itemKinds, slots, preferredRows int) ([]int, int, error) {
	if slots <= 0 || itemKinds < 0 {
		return nil, 0, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔物品槽或路线种类无效"}
	}
	if len(counts) == 0 {
		return []int{}, 1, nil
	}
	perTower := min(itemKinds, slots)
	perGroup := min(4, 12/max(1, perTower))
	if perGroup < 1 {
		return nil, 0, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔物品端口超过十二口预算"}
	}
	total, longest := 0, 0
	for _, count := range counts {
		if count <= 0 || count > 10000 || total > 10000-count {
			return nil, 0, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "物流塔机器行数量无效或超限"}
		}
		total += count
		longest = max(longest, count)
	}
	minimumRows := perGroup
	if preferredRows > 0 {
		minimumRows = min(minimumRows, preferredRows)
	}
	rows := max(len(counts), min(minimumRows, total))
	if rows > perGroup {
		rows = min(total, ((rows+perGroup-1)/perGroup)*perGroup)
	}
	rows = max(rows, int(math.Ceil(float64(total)/float64(longest))))
	result := make([]int, rows)
	for i := range result {
		result[i] = total / rows
		if i < total%rows {
			result[i]++
		}
	}
	return result, perGroup, nil
}

// LeftoverKinds 职责：从规范剩余边取得一条生产的真实副产物集合；使用：分组及出口预留；边界：不从配方毛产物猜测，不改写规范图。
func LeftoverKinds(graph *quant.FlowGraph, owner int) map[int]bool {
	kinds := map[int]bool{}
	if graph == nil {
		return kinds
	}
	for _, node := range graph.Nodes {
		if node.Kind != "production" || node.ProductionIndex != owner {
			continue
		}
		for _, edge := range graph.Edges {
			if edge.SourceNodeID == node.ID && edge.FlowKind == "leftover" {
				kinds[edge.ItemID] = true
			}
		}
	}
	return kinds
}
