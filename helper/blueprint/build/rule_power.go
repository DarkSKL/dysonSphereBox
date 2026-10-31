package build

// 文件职责：检查生成设施的感应塔覆盖并统计需要外部供电的网络分量。
// 使用：最终记录数值规则出口；边界：验证覆盖与网络连通，不假设蓝图自带发电，也不估算玩家电网容量。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// finalPowerConsumer 职责：识别本生成器支持且需要供电的实体；使用：覆盖检查；边界：带、箱、分流器和喷涂机不伪造耗电需求。
func finalPowerConsumer(record blueprint.Building) bool {
	return record.RecipeID > 0 || record.ItemID >= 2011 && record.ItemID <= 2014 || surfaceIsStation(record.ItemID) || record.ItemID == 2107 || record.ItemID == 2313
}

// validateFinalPowerCoverage 职责：检查每个地面消费者在真实感应塔半径内，并统计塔网络；使用：NeedPower 请求；边界：研究站上层复用已连接底层的电力，不把未知供电半径当无限。
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
	secondary := append([]blueprint.Building(nil), records...)
	byIndex := make(map[int32]int, len(records))
	for i := range records {
		secondary[i].LocalOffset = records[i].LocalOffset2
		byIndex[records[i].Index] = i
	}
	second, err := encoding.surfaceWorldPositions(secondary)
	if err != nil {
		return 0, err
	}
	poles := []int{}
	occupied := map[[3]int][]int{}
	cellAt := func(point vector) [3]int {
		return [3]int{int(math.Floor(point.x / connectDistance)), int(math.Floor(point.y / connectDistance)), int(math.Floor(point.z / connectDistance))}
	}
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
		cell := cellAt(positions[i])
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, j := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if float64(surfacePowerDistanceSquared(positions[i], positions[j])) <= connectDistance*connectDistance {
							parents[root(i)] = root(j)
						}
					}
				}
			}
		}
		poles = append(poles, i)
		occupied[cell] = append(occupied[cell], i)
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
		} else if record.ItemID >= 2011 && record.ItemID <= 2014 {
			point = vector{(point.x + second[i].x) * .5, (point.y + second[i].y) * .5, (point.z + second[i].z) * .5}
		}
		covered, cell := false, cellAt(point)
		for x := -1; x <= 1 && !covered; x++ {
			for y := -1; y <= 1 && !covered; y++ {
				for z := -1; z <= 1 && !covered; z++ {
					for _, j := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if float64(surfacePowerDistanceSquared(point, positions[j])) <= coverRadius*coverRadius {
							covered = true
							break
						}
					}
				}
			}
		}
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
