package build

// 文件职责：校验球面带节点与静态建筑的动态占用和附加通道。
// 使用：最终记录预检；边界：只检查蓝图内部，连接豁免不代表已有世界实体或地形允许建造。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// surfaceBeltBuildingLink 职责：识别游戏预览支持的三跳塔口连接及两跳堆叠分流器连接；使用：带与建筑盒相交后的豁免；边界：只沿 input 或 output 同向追溯，不做无界连通分量豁免。
func surfaceBeltBuildingLink(belt, building int, records []blueprint.Building, byIndex map[int32]int) bool {
	for _, output := range []bool{false, true} {
		current := belt
		for hop := 1; hop <= 3; hop++ {
			index := records[current].InputObjIdx
			if output {
				index = records[current].OutputObjIdx
			}
			next, exists := byIndex[index]
			if !exists {
				break
			}
			if next == building {
				return true
			}
			if hop <= 2 && records[building].ItemID == 2020 && (records[building].InputObjIdx == index || records[building].OutputObjIdx == index) {
				return true
			}
			if !surfaceIsBelt(records[next].ItemID) {
				break
			}
			current = next
		}
	}
	return false
}

// surfaceAddonBeltPass 职责：按原型附加通道中心线和末端朝向核对喷涂带；使用：带与喷涂机盒相交后的豁免；边界：当前只核对已提取的原版喷涂机，不把任意端口当通道。
func (encoding *surfaceEncoding) surfaceAddonBeltPass(belt, addon int, records []blueprint.Building, positions []vector, byIndex map[int32]int, geometry placement.PrefabGeometry) bool {
	if records[addon].ItemID != 2313 || len(geometry.AddonAreaColPoses) == 0 || len(geometry.AddonAreaColPoses) != len(geometry.AddonAreaSize) || len(geometry.AddonAreaPoses) != len(geometry.AddonAreaSize) {
		return false
	}
	pose, err := encoding.recordPose(positions[addon], records[addon].Yaw, 0, 0)
	if err != nil {
		return false
	}
	point := placement.Vector{positions[belt].x, positions[belt].y, positions[belt].z}
	direction, terminal := placement.Vector{}, false
	input, hasInput := byIndex[records[belt].InputObjIdx]
	output, hasOutput := byIndex[records[belt].OutputObjIdx]
	if hasInput != hasOutput {
		terminal = true
		other := input
		if hasOutput {
			other = output
		}
		direction = surfaceUnit(placement.Vector{positions[other].x, positions[other].y, positions[other].z}.Sub(point))
	}
	for i, channel := range geometry.AddonAreaColPoses {
		center := pose.position.Add(pose.transform(placement.Vector{float64(channel.Position[0]), float64(channel.Position[1]), float64(channel.Position[2])}))
		forward := pose.transform(rotateSurfaceQuaternion(placement.Vector{0, 0, 1}, channel.Rotation))
		length := float64(geometry.AddonAreaSize[i][2]) * 3
		span := surfaceBeltSpan{ends: [2]placement.Vector{center.Sub(forward.Scale(length)), center.Add(forward.Scale(length))}}
		logical := geometry.AddonAreaPoses[i].Position
		logicalCenter := pose.position.Add(pose.transform(placement.Vector{float64(logical[0]), float64(logical[1]), float64(logical[2])}))
		delta := logicalCenter.Sub(point)
		if (delta.Dot(delta) < .05 || surfaceSpanDistanceSquared(surfaceBeltSpan{ends: [2]placement.Vector{point, point}}, span) < .09) && (!terminal || math.Abs(direction.Dot(forward)) > .95) {
			return true
		}
	}
	return false
}

// validateBeltBuildingOccupancy 职责：用带节点球、游戏缩短胶囊及连接/喷涂豁免检查建筑占用；使用：动态预检；边界：保守离线规则，未复现场景碰撞恢复、合并已有带或科技限制。
func (encoding *surfaceEncoding) validateBeltBuildingOccupancy(ctx context.Context, records []blueprint.Building) error {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	// 游戏预览按记录顺序用带 output 回填 input；原始蓝图只写单向带连接，不能据此误判末端和塔口跳数。
	records = append([]blueprint.Building(nil), records...)
	for _, record := range records {
		if target, exists := byIndex[record.OutputObjIdx]; exists && surfaceIsBelt(record.ItemID) && surfaceIsBelt(records[target].ItemID) {
			records[target].InputObjIdx = record.Index
		}
	}
	boxes := []placement.OwnedBox{}
	occupied := map[[3]int][]int{}
	geometries := map[int]placement.PrefabGeometry{}
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带建筑占用检查已取消或超时"}
		}
		if surfaceIsBelt(record.ItemID) || record.ItemID >= 2011 && record.ItemID <= 2014 {
			continue
		}
		shapes, err := catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		if record.ItemID == 2313 {
			geometry, err := catalog.Geometry(record.ItemID, record.ModelIndex)
			if err != nil {
				return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
			}
			geometries[i] = geometry
		}
		for _, shape := range shapes {
			box, err := encoding.colliderBox(positions[i], record.Yaw, shape)
			if err != nil {
				return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
			}
			low, high := box.Bounds()
			for x := int(math.Floor(low[0] / 8)); x <= int(math.Floor(high[0]/8)); x++ {
				for y := int(math.Floor(low[1] / 8)); y <= int(math.Floor(high[1]/8)); y++ {
					for z := int(math.Floor(low[2] / 8)); z <= int(math.Floor(high[2]/8)); z++ {
						key := [3]int{x, y, z}
						occupied[key] = append(occupied[key], len(boxes))
					}
				}
			}
			boxes = append(boxes, placement.OwnedBox{Owner: i, Geometry: box})
		}
	}
	const radius = .23
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带建筑占用检查已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		point := placement.Vector{positions[i].x, positions[i].y, positions[i].z}
		radial := encoding.localUp(point).Scale(.2)
		center, first, second := point.Add(radial), point, point
		input, hasInput := byIndex[record.InputObjIdx]
		output, hasOutput := byIndex[record.OutputObjIdx]
		if hasInput && hasOutput && surfaceIsBelt(records[input].ItemID) && surfaceIsBelt(records[output].ItemID) {
			first = point.Add(placement.Vector{positions[input].x, positions[input].y, positions[input].z}.Sub(point).Scale(.65))
			second = point.Add(placement.Vector{positions[output].x, positions[output].y, positions[output].z}.Sub(point).Scale(.65))
			delta := second.Sub(first).Scale(.45)
			if encoding.planar {
				first, second = first.Add(delta), second.Sub(delta)
			} else {
				length := math.Sqrt(point.Dot(point))
				first, second = surfaceUnit(first.Add(delta)).Scale(length), surfaceUnit(second.Sub(delta)).Scale(length)
			}
		}
		first, second = first.Add(radial), second.Add(radial)
		seen := map[int]bool{}
		for x := int(math.Floor((center[0] - radius) / 8)); x <= int(math.Floor((center[0]+radius)/8)); x++ {
			for y := int(math.Floor((center[1] - radius) / 8)); y <= int(math.Floor((center[1]+radius)/8)); y++ {
				for z := int(math.Floor((center[2] - radius) / 8)); z <= int(math.Floor((center[2]+radius)/8)); z++ {
					for _, index := range occupied[[3]int{x, y, z}] {
						if seen[index] {
							continue
						}
						seen[index] = true
						box := boxes[index]
						if box.Geometry.SegmentDistanceSquared(center, center) >= radius*radius-1e-6 || box.Geometry.SegmentDistanceSquared(first, second) >= radius*radius-1e-6 || surfaceBeltBuildingLink(i, box.Owner, records, byIndex) {
							continue
						}
						if geometry, exists := geometries[box.Owner]; exists && encoding.surfaceAddonBeltPass(i, box.Owner, records, positions, byIndex, geometry) {
							continue
						}
						return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("传送带 %d（局部 %v）侵占建筑 %d（物品 %d，局部 %v）的建造盒且不满足连接/附加通道规则", record.Index, record.LocalOffset, records[box.Owner].Index, records[box.Owner].ItemID, records[box.Owner].LocalOffset)}
					}
				}
			}
		}
	}
	return nil
}
