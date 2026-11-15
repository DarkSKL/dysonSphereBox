package build

// 文件职责：在正交球面迁移中保持分流器南北短接头的真实经线方向。
// 使用：原记录完成正交映射之后、重新计算包络之前；边界：只调整原先共线的三节接头，不改变槽位、朝向或检查阈值。

import (
	"DysonSphereBox/helper/blueprint"
	"math"
)

// alignRelocatedSplitterColumns 职责：以宿主所在行补偿南北短接头横向格距，消除逐点缩放造成的剪切；使用：前三档正交重定位；边界：外部路线保持原坐标，所有修改仍须完整终检。
func alignRelocatedSplitterColumns(move *denseSurfaceRelocation, source, records []blueprint.Building) error {
	if !move.orthogonal {
		return nil
	}
	byID := map[int32]int{}
	previous, attached := map[int32][]int{}, map[int32][]int{}
	for i, record := range source {
		byID[record.Index] = i
		if surfaceIsBelt(record.ItemID) {
			previous[record.OutputObjIdx] = append(previous[record.OutputObjIdx], i)
		}
	}
	for i, record := range source {
		if surfaceIsBelt(record.ItemID) {
			for _, id := range []int32{record.InputObjIdx, record.OutputObjIdx} {
				if index, exists := byID[id]; exists && source[index].ItemID == 2020 {
					attached[id] = append(attached[id], i)
				}
			}
		}
	}
	for _, gate := range source {
		if gate.ItemID != 2020 {
			continue
		}
		profile, err := loadSplitterPortProfile(gate.ModelIndex)
		if err != nil {
			return err
		}
		for _, i := range attached[gate.Index] {
			belt := source[i]
			fromGate := belt.InputObjIdx == gate.Index
			slot := belt.InputFromSlot
			if !fromGate {
				slot = belt.OutputToSlot
			}
			if slot < 0 || slot >= 4 {
				continue
			}
			port := splitterGateRotate(profile.ports[slot].point, gate.Yaw)
			direction := splitterGateRotate(profile.ports[slot].forward, gate.Yaw)
			if math.Abs(direction[2]) < .999 {
				continue
			}
			current := i
			for hop := 0; hop < 3; hop++ {
				original := source[current]
				dx := float64(original.LocalOffset.X-gate.LocalOffset.X) * planarGridLength
				dy := float64(original.LocalOffset.Y-gate.LocalOffset.Y) * planarGridLength
				if math.Abs(dx-port[0]) > splitterPortGeometryTolerance || (dy-port[2])*direction[2] < -splitterPortGeometryTolerance || math.Hypot(dx, dy) > 2.1*planarGridLength {
					break
				}
				anchor := original.LocalOffset
				anchor.Y = gate.LocalOffset.Y
				mapped, _ := move.mapPoint(anchor, original.Yaw)
				records[current].LocalOffset.X = mapped.X
				if original.LocalOffset2 == original.LocalOffset {
					records[current].LocalOffset2.X = mapped.X
				}
				next, exists := byID[original.OutputObjIdx]
				if !fromGate {
					incoming := previous[original.Index]
					exists = len(incoming) == 1
					if exists {
						next = incoming[0]
					}
				}
				if !exists || !surfaceIsBelt(source[next].ItemID) {
					break
				}
				current = next
			}
		}
	}
	return nil
}
