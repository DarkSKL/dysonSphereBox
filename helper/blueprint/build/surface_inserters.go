package build

// 文件职责：根据原型 slotPoses 修正球面分拣器端点并还原动态建造盒。
// 使用：最终 float32 记录的动态预检；边界：数值复现不等同于 Unity Physics、地形或已有建筑查询。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfacePose 职责：保存世界位置及单位旋转基；使用：端口与动态盒变换；边界：不表示原始蓝图格坐标。
type surfacePose struct {
	position placement.Vector
	axes     [3]placement.Vector
}

// transform 职责：把局部向量旋转到世界；使用：端口位置和轴；边界：不加位置平移。
func (pose surfacePose) transform(v placement.Vector) placement.Vector {
	return pose.axes[0].Scale(v[0]).Add(pose.axes[1].Scale(v[1])).Add(pose.axes[2].Scale(v[2]))
}

// rotateSurfaceQuaternion 职责：按已提取的 Unity 四元数旋转局部向量；使用：slotPoses 与碰撞体；边界：四元数须为单位值。
func rotateSurfaceQuaternion(v placement.Vector, q [4]float32) placement.Vector {
	u := placement.Vector{float64(q[0]), float64(q[1]), float64(q[2])}
	cross := u.Cross(v)
	return v.Add(cross.Scale(2 * float64(q[3]))).Add(u.Cross(cross).Scale(2))
}

// surfaceSlotPose 职责：把原型端口变换为世界姿态，并按输出端反转水平朝向；使用：分拣器修正及独立 DLL 契约测试；边界：端口与建筑姿态须来自已核对目录，不判断修正量是否合法。
func surfaceSlotPose(base surfacePose, slot placement.Pose, output bool) surfacePose {
	port := surfacePose{position: base.position.Add(base.transform(placement.Vector{float64(slot.Position[0]), float64(slot.Position[1]), float64(slot.Position[2])}))}
	for axis, unit := range [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		port.axes[axis] = base.transform(rotateSurfaceQuaternion(unit, slot.Rotation))
	}
	if output {
		port.axes[0] = port.axes[0].Scale(-1)
		port.axes[2] = port.axes[2].Scale(-1)
	}
	return port
}

// surfaceRecordPose 职责：按 Unity Euler 的 Y-X-Z 顺序建立球面姿态；使用：原始分拣器端点；边界：极点仍由既有切线变换拒绝。
func surfaceRecordPose(position vector, yaw, pitch, tilt float32) (surfacePose, error) {
	box, err := surfaceColliderBox(position, yaw, placement.Collider{Extents: [3]float32{1, 1, 1}, Rotation: [4]float32{0, 0, 0, 1}})
	if err != nil {
		return surfacePose{}, err
	}
	pose := surfacePose{position: box.Center, axes: box.Axes}
	x, z := float64(pitch)*math.Pi/180, float64(tilt)*math.Pi/180
	sx, cx, sz, cz := math.Sin(x), math.Cos(x), math.Sin(z), math.Cos(z)
	pose.axes = [3]placement.Vector{
		pose.transform(placement.Vector{cz, sz * cx, sz * sx}),
		pose.transform(placement.Vector{-sz, cz * cx, cz * sx}),
		pose.transform(placement.Vector{0, -sx, cx}),
	}
	return pose, nil
}

// surfaceUnit 职责：安全归一化世界向量；使用：动态方向计算；边界：零向量返回零，不产生 NaN。
func surfaceUnit(v placement.Vector) placement.Vector {
	if length := math.Sqrt(v.Dot(v)); length > 1e-9 {
		return v.Scale(1 / length)
	}
	return placement.Vector{}
}

// surfaceInserterBuried 职责：复现游戏两端均埋入基准球面的 MayBeBuried 判定；使用：端口修正后；边界：仅支持已核对的半径 200，不代替实际地形高度查询。
func surfaceInserterBuried(poses [2]surfacePose) bool {
	limit := float32(200) - float32(.1)
	limit *= limit
	for _, pose := range poses {
		// Unity 对位置加端点 up 后按 float32 顺序求平方和，边界相等时不判埋入。
		x := float32(pose.position[0]) + float32(pose.axes[1][0])
		y := float32(pose.position[1]) + float32(pose.axes[1][1])
		z := float32(pose.position[2]) + float32(pose.axes[1][2])
		if x*x+y*y+z*z >= limit {
			return false
		}
	}
	return true
}

// surfaceInserterBoxes 职责：修正连接非带端口的两端，检查数据方向与距离并生成拉伸盒；使用：动态碰撞入口；边界：不改写蓝图原始端点，不冒充完整 CalcSegmentsAcross 与科技验证。
func (encoding *surfaceEncoding) surfaceInserterBoxes(ctx context.Context, records []blueprint.Building) ([]placement.OwnedBox, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	first, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return nil, err
	}
	secondary := append([]blueprint.Building(nil), records...)
	byIndex := make(map[int32]int, len(records))
	for i := range secondary {
		secondary[i].LocalOffset = records[i].LocalOffset2
		byIndex[records[i].Index] = i
	}
	second, err := encoding.surfaceWorldPositions(secondary)
	if err != nil {
		return nil, err
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
		poses[0], err = surfaceRecordPose(first[i], record.Yaw, record.Pitch, record.Tilt)
		if err != nil {
			return nil, err
		}
		poses[1], err = surfaceRecordPose(second[i], record.Yaw2, record.Pitch2, record.Tilt2)
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
			base, err := surfaceRecordPose(first[j], records[j].Yaw, 0, 0)
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
				beltPose, err := surfaceRecordPose(first[byIndex[connections[other]]], beltRecord.Yaw, 0, 0)
				if err != nil {
					return nil, err
				}
				if math.Abs(surfaceUnit(poses[1].position.Sub(poses[0].position)).Dot(beltPose.axes[2])) < .5 {
					forward := poses[other].axes[2]
					poses[other].position = poses[other].position.Add(delta.Sub(forward.Scale(delta.Dot(forward))))
				}
			}
			grid := surfaceLocalGridSize(base.position, port.axes[2])
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
		across := surfaceSegmentsAcross(reference, poses[0].position, poses[1].position)
		height := math.Abs(math.Sqrt(poses[0].position.Dot(poses[0].position))-math.Sqrt(poses[1].position.Dot(poses[1].position))) / .2
		if across > acrossLimit+1e-4 || math.Hypot(across, height) < minimumGrid-1e-4 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的跨格距离 %.3f 超出游戏范围", record.Index, across)}
		}
		forward := surfaceUnit(direction)
		trace := 0.
		for axis := range poses[0].axes {
			trace += poses[0].axes[axis].Dot(poses[1].axes[axis])
		}
		backwards := !belt[0] && forward.Dot(poses[0].axes[2]) < 0 || !belt[1] && forward.Dot(poses[1].axes[2]) < 0
		if backwards || (trace-1)*.5 < math.Cos(30*math.Pi/180)-1e-4 || math.Abs(forward.Dot(poses[0].axes[2])) < math.Cos(24*math.Pi/180)-1e-4 || math.Abs(forward.Dot(poses[1].axes[2])) < math.Cos(24*math.Pi/180)-1e-4 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的端点偏斜超过游戏限制", record.Index)}
		}
		if surfaceInserterBuried(poses) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("分拣器 %d 修正后的两端埋入行星基准球面", record.Index)}
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
