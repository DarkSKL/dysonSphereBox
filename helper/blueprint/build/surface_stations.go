package build

// 文件职责：复核物流塔独立于静态建造盒的间距、物品槽及带端口规则。
// 使用：最终记录规则沙箱与球面候选检查；边界：只覆盖原版行星/星际物流塔，不将未知站点当作无限槽位。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/transport"
)

// surfaceStationDistanceLimit 职责：返回已核对 TowerTooClose 的距离平方；使用：物流塔窄相位；边界：普通塔为225，任一星际塔为841，严格小于才冲突。
func surfaceStationDistanceLimit(left, right int16) float32 {
	if left == 2104 || right == 2104 {
		return 841
	}
	return 225
}

// surfaceIsStation 职责：识别生成器支持的两种原版物流塔；使用：站点规则分派；边界：不覆盖采矿站或轨道采集器。
func surfaceIsStation(item int16) bool { return item == 2103 || item == 2104 }

// surfaceStationConflict 职责：用空间索引检查所有塔对的最小间距；使用：平面/球面最终位置；边界：单精度计算，阈值相等允许，取消立即返回。
func surfaceStationConflict(ctx context.Context, records []blueprint.Building, positions []vector) (int, int, error) {
	if len(records) != len(positions) {
		return -1, -1, fmt.Errorf("物流塔坐标数量不一致")
	}
	occupied := map[[3]int][]int{}
	for i, record := range records {
		if ctx.Err() != nil {
			return -1, -1, ctx.Err()
		}
		if !surfaceIsStation(record.ItemID) {
			continue
		}
		point := positions[i]
		cell := [3]int{int(math.Floor(point.x / 29)), int(math.Floor(point.y / 29)), int(math.Floor(point.z / 29))}
		for x := -1; x <= 1; x++ {
			for y := -1; y <= 1; y++ {
				for z := -1; z <= 1; z++ {
					for _, j := range occupied[[3]int{cell[0] + x, cell[1] + y, cell[2] + z}] {
						if surfacePowerDistanceSquared(point, positions[j]) < surfaceStationDistanceLimit(record.ItemID, records[j].ItemID) {
							return j, i, nil
						}
					}
				}
			}
		}
		occupied[cell] = append(occupied[cell], i)
	}
	return -1, -1, nil
}

// validateSurfaceStationSpacing 职责：验证最终记录中的 TowerTooClose；使用：动态设施共同出口；边界：不以建造盒不相交代替塔最小间距。
func (encoding *surfaceEncoding) validateSurfaceStationSpacing(ctx context.Context, records []blueprint.Building) error {
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	left, right, err := surfaceStationConflict(ctx, records, positions)
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔间距检查已取消或超时"}
	}
	if left >= 0 {
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("物流塔相隔太近：建筑 %d 与 %d，距离 %.3f 米，小于 %g 米", records[left].Index, records[right].Index, math.Sqrt(float64(surfacePowerDistanceSquared(positions[left], positions[right]))), math.Sqrt(float64(surfaceStationDistanceLimit(records[left].ItemID, records[right].ItemID))))}
	}
	return nil
}

// validateStationPorts 职责：复核连接到塔的真实带端口、方向及筛选槽；使用：最终编号后；边界：未用端口不冒充供货，网络运输仍需外界机队与电力。
func validateStationPorts(ctx context.Context, records []blueprint.Building) error {
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
		if !surfaceIsStation(record.ItemID) {
			continue
		}
		if len(record.Parameters) != 2048 {
			return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: fmt.Sprintf("物流塔 %d 缺少完整物品槽和端口参数", record.Index)}
		}
		kinds, err := transport.StationItemKindLimit(int(record.ItemID))
		if err != nil {
			return err
		}
		capacity, err := transport.StationItemCapacity(int(record.ItemID))
		if err != nil {
			return err
		}
		seen := map[int32]bool{}
		for slot := 0; slot < 5; slot++ {
			base := slot * 6
			item := record.Parameters[base]
			if item == 0 {
				continue
			}
			role, remote, count := record.Parameters[base+1], record.Parameters[base+2], record.Parameters[base+3]
			if slot >= kinds || item < 0 || seen[item] || role < 0 || role > 2 || remote < 0 || remote > 2 || count < 0 || int(count) > capacity {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: fmt.Sprintf("物流塔 %d 物品槽 %d 的物品、角色或零科技容量不合法", record.Index, slot)}
			}
			seen[item] = true
		}
	}
	used := map[[2]int32]bool{}
	for _, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔端口检查已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		for end, connection := range []int32{record.InputObjIdx, record.OutputObjIdx} {
			j, exists := byIndex[connection]
			if !exists || !surfaceIsStation(records[j].ItemID) {
				continue
			}
			tower := records[j]
			ports, err := transport.StationPortCount(int(tower.ItemID))
			if err != nil {
				return err
			}
			slot, direction := record.InputFromSlot, int32(1)
			if end == 1 {
				slot, direction = record.OutputToSlot, 2
			}
			key := [2]int32{tower.Index, int32(slot)}
			if slot < 0 || int(slot) >= ports || used[key] {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: fmt.Sprintf("传送带 %d 使用了物流塔 %d 的重复或无效端口 %d", record.Index, tower.Index, slot)}
			}
			used[key] = true
			base := 192 + int(slot)*4
			filter := tower.Parameters[base+1]
			kinds, err := transport.StationItemKindLimit(int(tower.ItemID))
			if err != nil {
				return err
			}
			itemMatches := filter == 0
			if end == 0 {
				// 塔向外送货的端口必须选择对应的需求物品槽，避免带路接到另一个物料。
				itemMatches = len(record.Parameters) > 0 && filter > 0 && int(filter) <= kinds && tower.Parameters[(filter-1)*6] == record.Parameters[0] && tower.Parameters[(filter-1)*6+1] == 2
			} else if itemMatches {
				// 产线向塔供货时端口不筛选，但塔内必须声明同物品的供给槽。
				itemMatches = false
				if len(record.Parameters) > 0 {
					for itemIndex := 0; itemIndex < kinds; itemIndex++ {
						if tower.Parameters[itemIndex*6] == record.Parameters[0] && tower.Parameters[itemIndex*6+1] == 1 {
							itemMatches = true
							break
						}
					}
				}
			}
			if tower.Parameters[base] != direction || !itemMatches {
				return &blueprint.Error{Code: "GEN_E_STATION_PORT", Message: fmt.Sprintf("传送带 %d 与物流塔 %d 端口 %d 的方向或物品筛选不一致", record.Index, tower.Index, slot)}
			}
		}
	}
	return nil
}
