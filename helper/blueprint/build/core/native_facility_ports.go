package build

// 文件职责：在原生球面中按宿主真实槽位校正储液罐短接头。
// 使用：候选端口适配事务内、正式碰撞检查前；边界：保留物品、槽号和连接方向，只处理原先共线的短带，不跳过最终距离和选择体积规则。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// alignNativeTankPorts 职责：校正已标明真实罐槽的短接头；使用：原生端口事务；边界：不改分流器已经冻结的构造证据，不借改变槽号恢复连通。
func (b *builder) alignNativeTankPorts(ctx context.Context, encoding *surfaceEncoding, records []blueprint.Building) error {
	if !encoding.rootCoordinates {
		return nil
	}
	hasTank := false
	for _, record := range records {
		if record.ItemID == 2106 {
			hasTank = true
			break
		}
	}
	if !hasTank {
		return nil
	}
	world, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	byID := make(map[int32]int, len(records))
	previous := make(map[int32][]int)
	for i, r := range records {
		byID[r.Index] = i
		if surfaceIsBelt(r.ItemID) {
			previous[r.OutputObjIdx] = append(previous[r.OutputObjIdx], i)
		}
	}
	proposals := map[int]vector{}
	for i, belt := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !surfaceIsBelt(belt.ItemID) {
			continue
		}
		for end, hostID := range []int32{belt.InputObjIdx, belt.OutputObjIdx} {
			host, ok := byID[hostID]
			if !ok || records[host].ItemID != 2106 {
				continue
			}
			slot := int(belt.InputFromSlot)
			if end == 1 {
				slot = int(belt.OutputToSlot)
			}
			geometry, err := catalog.Geometry(records[host].ItemID, records[host].ModelIndex)
			if err != nil {
				return err
			}
			if slot < 0 || slot >= len(geometry.PortPoses) {
				return fmt.Errorf("设施%d缺少短带槽%d", hostID, slot)
			}
			pose, err := encoding.recordPose(world[host], records[host].Yaw, 0, 0)
			if err != nil {
				return err
			}
			port := surfaceSlotPose(pose, geometry.PortPoses[slot], false)
			for node, step := i, 0; node >= 0 && step < 3; step++ {
				point := placement.Vector{world[node].x, world[node].y, world[node].z}
				delta := point.Sub(port.position)
				along := delta.Dot(port.axes[2])
				perp := delta.Sub(port.axes[2].Scale(along))
				if step > 0 && (along < 0 || along > 4 || perp.Dot(perp) > .75*.75) {
					break
				}
				if step == 0 {
					along = math.Max(0, math.Min(1, along))
				}
				target := port.position.Add(port.axes[2].Scale(along))
				local := nativeSingleAreaPoint(encoding, target, float64(records[node].LocalOffset.X))
				local.z = float64(records[node].LocalOffset.Z)
				if other, exists := proposals[node]; exists && (math.Abs(other.x-local.x) > .02 || math.Abs(other.y-local.y) > .02) {
					return fmt.Errorf("设施短带%d同时要求不一致的端口校正", records[node].Index)
				}
				proposals[node] = local
				next := -1
				if end == 0 {
					if n, ok := byID[records[node].OutputObjIdx]; ok && surfaceIsBelt(records[n].ItemID) {
						next = n
					}
				} else if parents := previous[records[node].Index]; len(parents) == 1 {
					next = parents[0]
				}
				if next < 0 || len(previous[records[next].Index]) > 1 {
					break
				}
				node = next
			}
		}
	}
	moved := make(map[*placed]int, len(proposals))
	for i, point := range proposals {
		records[i].LocalOffset = blueprint.Point{X: float32(point.x), Y: float32(point.y), Z: float32(point.z)}
		records[i].LocalOffset2 = records[i].LocalOffset
		b.all[i].a, b.all[i].b = point, point
		moved[b.all[i]] = i
	}
	for i, entity := range b.all {
		if host, ok := moved[entity.flowMonitorHost]; ok {
			entity.a, entity.b = b.all[host].a, b.all[host].b
			records[i].LocalOffset = records[host].LocalOffset
			records[i].LocalOffset2 = records[host].LocalOffset
		}
	}
	return nil
}
