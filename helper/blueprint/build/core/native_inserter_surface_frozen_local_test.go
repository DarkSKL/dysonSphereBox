package build

// 文件职责：冻结优化前的分拣器端口、跨度与动态盒实现；使用：独立局部差分与基准；边界：只用于本轮验证，不随生产实现同步优化。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
)

// surfaceInserterBoxesMeasured 职责：在同一端口修正流程中记录游戏实际跨格参数；使用：球面重定位后的能力一致性复核；边界：任一规则失败时调用方不得使用部分结果。
func (encoding *surfaceEncoding) frozenSurfaceInserterBoxesMeasured(ctx context.Context, records []blueprint.Building, spans map[int32]int32) ([]placement.OwnedBox, error) {
	first, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return nil, err
	}
	second, err := encoding.surfaceSecondPositions(records)
	if err != nil {
		return nil, err
	}
	return encoding.frozenSurfaceInserterBoxesAtWorld(ctx, records, first, second, spans)
}

// surfaceInserterBoxesAtWorld 职责：用明确世界端点执行正式分拣器槽位、方向、跨度及动态盒计算；使用：最终编码记录与原生单元落位共用；边界：端点须与记录一一对应，不修改实体姿态、碰撞原型或规则阈值。
func (encoding *surfaceEncoding) frozenSurfaceInserterBoxesAtWorld(ctx context.Context, records []blueprint.Building, first, second []vector, spans map[int32]int32) ([]placement.OwnedBox, error) {
	if len(first) != len(records) || len(second) != len(records) {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "分拣器世界端点与建筑记录数量不一致"}
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	boxes := []placement.OwnedBox{}
	for i, record := range records {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "分拣器动态检查已取消或超时"}
		}
		if record.ItemID < 2011 || record.ItemID > 2014 {
			continue
		}
		poses := [2]surfacePose{}
		poses[0], err = encoding.recordPose(first[i], record.Yaw, record.Pitch, record.Tilt)
		if err != nil {
			return nil, err
		}
		poses[1], err = encoding.recordPose(second[i], record.Yaw2, record.Pitch2, record.Tilt2)
		if err != nil {
			return nil, err
		}
		connections := [2]int32{record.InputObjIdx, record.OutputObjIdx}
		slots := [2]int8{record.InputFromSlot, record.OutputToSlot}
		belt := [2]bool{}
		for end, index := range connections {
			if index < 0 {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 缺少已知连接端点", record.Index)}
			}
			j, exists := byIndex[index]
			if !exists {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "分拣器连接指向蓝图外部"}
			}
			belt[end] = surfaceIsBelt(records[j].ItemID)
		}
		for end, index := range connections {
			if belt[end] {
				continue
			}
			j := byIndex[index]
			geometry, err := catalog.Geometry(records[j].ItemID, records[j].ModelIndex)
			if err != nil || int(slots[end]) < 0 || int(slots[end]) >= len(geometry.SlotPoses) {
				return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: fmt.Sprintf("分拣器 %d 连接建筑 %d 的端口 %d 无已核对几何", record.Index, index, slots[end])}
			}
			base, err := encoding.recordPose(first[j], records[j].Yaw, 0, 0)
			if err != nil {
				return nil, err
			}
			slot := geometry.SlotPoses[slots[end]]
			port := surfaceSlotPose(base, slot, end == 1)
			delta := port.position.Sub(poses[end].position)
			poses[end] = port
			other := 1 - end
			if belt[other] {
				// 带端沿原分拣器 forward 投影，保持带方向垂直的端点校正分量。
				beltRecord := records[byIndex[connections[other]]]
				beltPose, err := encoding.recordPose(first[byIndex[connections[other]]], beltRecord.Yaw, 0, 0)
				if err != nil {
					return nil, err
				}
				if math.Abs(surfaceUnit(poses[1].position.Sub(poses[0].position)).Dot(beltPose.axes[2])) < .5 {
					forward := poses[other].axes[2]
					poses[other].position = poses[other].position.Add(delta.Sub(forward.Scale(delta.Dot(forward))))
				}
			}
			grid, err := encoding.localGridSize(base.position, port.axes[2])
			if err != nil {
				return nil, err
			}
			correction := math.Sqrt(delta.Dot(delta)) / grid
			lateral := math.Abs(port.axes[0].Dot(delta))
			invalid := correction > .8001 && (lateral > .5001 || lateral < .1 && correction > 1.6001 || lateral >= .1 && correction > .8001)
			if records[j].ItemID == 2312 {
				limit := 2.5 - .1*float64(end)
				invalid = correction > limit+1e-4 && lateral > limit+1e-4
			}
			if invalid {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 的端口修正量超出游戏限制", record.Index)}
			}
			// 游戏在修正每一端后立即保留反向数据错误；不能只检查两端都修正后的最终姿态。
			if surfaceUnit(poses[1].position.Sub(poses[0].position)).Dot(poses[end].axes[2]) < 0 {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 在第 %d 端口修正时方向反向", record.Index, end+1)}
			}
		}
		direction := poses[1].position.Sub(poses[0].position)
		length := math.Sqrt(direction.Dot(direction))
		minimum, maximum := .6, 5.5
		if belt[0] && belt[1] {
			minimum, maximum = .4, 5
		} else if !belt[0] && !belt[1] {
			minimum, maximum = .9, 7.5
		}
		if length < minimum-1e-4 || length > maximum+1e-4 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后长度 %.3f 超出游戏范围", record.Index, length)}
		}
		centers := [2]placement.Vector{}
		for end, index := range connections {
			p := first[byIndex[index]]
			centers[end] = placement.Vector{p.x, p.y, p.z}
		}
		reference := centers[0].Add(centers[1]).Scale(.5)
		acrossLimit, minimumGrid := 3.499, .88
		if belt[0] && !belt[1] {
			reference = centers[1]
		} else if !belt[0] && belt[1] {
			reference = centers[0]
		} else if belt[0] && belt[1] {
			acrossLimit, minimumGrid = 3.2, .8
		} else {
			acrossLimit, minimumGrid = 3.799, 1.451
		}
		across, err := encoding.segmentsAcross(reference, poses[0].position, poses[1].position)
		if err != nil {
			return nil, err
		}
		height := encoding.endpointHeightGap(poses[0].position, poses[1].position)
		if across > acrossLimit+1e-4 || math.Hypot(across, height) < minimumGrid-1e-4 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的跨格距离 %.3f 超出游戏范围", record.Index, across)}
		}
		forward := surfaceUnit(direction)
		trace := 0.
		for axis := range poses[0].axes {
			trace += poses[0].axes[axis].Dot(poses[1].axes[axis])
		}
		backwards := !belt[0] && forward.Dot(poses[0].axes[2]) < 0 || !belt[1] && forward.Dot(poses[1].axes[2]) < 0
		if backwards || surfaceInserterAngleLimitExceeded(forward.Dot(poses[0].axes[2]), forward.Dot(poses[1].axes[2]), (trace-1)*.5) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的端点偏斜超过游戏限制（端点夹角%.2f°/%.2f°，姿态差%.2f°）", record.Index, math.Acos(math.Min(1, math.Abs(forward.Dot(poses[0].axes[2]))))*180/math.Pi, math.Acos(math.Min(1, math.Abs(forward.Dot(poses[1].axes[2]))))*180/math.Pi, math.Acos(math.Max(-1, math.Min(1, (trace-1)*.5)))*180/math.Pi)}
		}
		if encoding.inserterBuried(poses) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的两端埋入行星基准球面", record.Index)}
		}
		if spans != nil {
			// 游戏在最终姿态上取跨格值，双建筑端先减0.3，再用Mathf的偶数舍入写参数。
			reach := float32(across)
			if !belt[0] && !belt[1] {
				reach -= .3
			}
			spans[record.Index] = int32(math.RoundToEven(float64(max(float32(1), min(float32(3), reach)))))
		}
		right := surfaceUnit(poses[0].axes[1].Add(poses[1].axes[1]).Cross(forward))
		frame := surfacePose{position: poses[0].position.Add(poses[1].position).Scale(.5), axes: [3]placement.Vector{right, forward.Cross(right), forward}}
		shapes, err := catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return nil, err
		}
		for _, shape := range shapes {
			shape.Extents[2] += float32(length*.5) - .5
			for end := range belt {
				if belt[end] {
					shape.Position[2] += float32(end*2-1) * .35
					shape.Extents[2] += .35
				}
			}
			shape.Extents[2] = max(.1, shape.Extents[2])
			box := placement.Box{Center: frame.position.Add(frame.transform(placement.Vector{float64(shape.Position[0]), float64(shape.Position[1]), float64(shape.Position[2])})), HalfSize: placement.Vector{float64(shape.Extents[0]), float64(shape.Extents[1]), float64(shape.Extents[2])}}
			for axis, unit := range [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
				box.Axes[axis] = frame.transform(rotateSurfaceQuaternion(unit, shape.Rotation))
			}
			boxes = append(boxes, placement.OwnedBox{Owner: int(record.Index), Geometry: box})
		}
	}
	return boxes, nil
}
