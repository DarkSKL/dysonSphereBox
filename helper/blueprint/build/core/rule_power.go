package build

// 文件职责：检查生成设施的感应塔覆盖并统计需要外部供电的网络分量。
// 使用：最终记录数值规则出口；边界：验证覆盖与网络连通，不假设蓝图自带发电，也不估算玩家电网容量。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// finalPowerConsumer 职责：识别本生成器支持且需要供电的实体；使用：覆盖检查；边界：带、箱、分流器和喷涂机不伪造耗电需求。
func finalPowerConsumer(record blueprint.Building) bool {
	return record.RecipeID > 0 || record.ItemID >= 2011 && record.ItemID <= 2014 || surfaceIsStation(record.ItemID) || record.ItemID == 2030 || record.ItemID == 2107 || record.ItemID == 2313
}

// validateFinalPowerCoverage 职责：检查实际消费者接电位置的感应塔覆盖，并统计塔网络；使用：NeedPower 请求；边界：分拣器使用首端实体位置，研究站上层复用有效连接的底层，不以碰撞盒中点代替接电点。
func validateFinalPowerCoverage(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) (int, error) {
	coverRadius, err := transport.TeslaPowerCoverRadius()
	if err != nil {
		return 0, err
	}
	connectDistance, err := transport.TeslaPowerConnectDistance()
	if err != nil {
		return 0, err
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return 0, err
	}
	byIndex := make(map[int32]int, len(records))
	for i := range records {
		byIndex[records[i].Index] = i
	}
	poles := []int{}
	occupied := worldSpatialIndex{}

	parents := make([]int, len(records))
	for i := range parents {
		parents[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parents[i] != i {
			parents[i] = parents[parents[i]]
			i = parents[i]
		}
		return i
	}
	for i, record := range records {
		if ctx.Err() != nil {
			return 0, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "供电网络检查已取消或超时"}
		}
		if record.ItemID != 2201 {
			continue
		}
		occupied.any(worldSpatialNeighbors(positions[i], connectDistance), func(j int) bool {
			if float64(surfacePowerDistanceSquared(positions[i], positions[j])) <= connectDistance*connectDistance {
				parents[root(i)] = root(j)
			}
			return false
		})
		poles = append(poles, i)
		occupied.addPoint(positions[i], connectDistance, i)
	}
	for i, record := range records {
		if ctx.Err() != nil {
			return 0, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "供电覆盖检查已取消或超时"}
		}
		if !finalPowerConsumer(record) {
			continue
		}
		point := positions[i]
		if record.RecipeID > 0 && record.ItemID == 2901 && record.InputObjIdx >= 0 {
			// 堆叠研究站由底层供电，连接链必须是同配方研究站，不能任意跳到远处电网。
			current, visited := i, map[int]bool{}
			for records[current].InputObjIdx >= 0 {
				j, exists := byIndex[records[current].InputObjIdx]
				if !exists || visited[j] || records[j].ItemID != record.ItemID || records[j].RecipeID != record.RecipeID {
					return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("研究站 %d 的堆叠供电链无效", record.Index)}
				}
				visited[current] = true
				current = j
			}
			point = positions[current]
		}
		covered := occupied.any(worldSpatialNeighbors(point, connectDistance), func(j int) bool {
			return float64(surfacePowerDistanceSquared(point, positions[j])) <= coverRadius*coverRadius
		})
		if !covered {
			return 0, &blueprint.Error{Code: "GEN_E_POWER_COVERAGE", Message: fmt.Sprintf("建筑 %d（物品 %d）不在已生成感应塔的 %.1f 米覆盖范围内", record.Index, record.ItemID, coverRadius)}
		}
	}
	networks := map[int]bool{}
	for _, i := range poles {
		networks[root(i)] = true
	}
	return len(networks), nil
}
