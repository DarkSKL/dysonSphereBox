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

// surfaceBeltBuildingLink 职责：识别非分流器建筑的原生三跳连接及监测器宿主；使用：带与建筑盒相交后的豁免；边界：只沿input或output同向追溯，分流器必须另按真实槽位几何复核。
func surfaceBeltBuildingLink(belt, building int, records []blueprint.Building, byIndex map[int32]int) bool {
	// 分流器另按真实模型槽位、方向及高度复核短接头，不继承此通用三跳规则。
	if records[building].ItemID == 2020 {
		return false
	}
	// 流速监测器按游戏规则直接落在宿主带节点；只豁免同 Area、同坐标且同朝向的这一对实体。
	if records[building].ItemID == 2030 {
		host := records[building]
		segment := records[belt]
		if host.AreaIndex == segment.AreaIndex && math.Abs(float64(host.LocalOffset.X-segment.LocalOffset.X)) < 1e-3 && math.Abs(float64(host.LocalOffset.Y-segment.LocalOffset.Y)) < 1e-3 && math.Abs(float64(host.LocalOffset.Z-segment.LocalOffset.Z)) < 1e-3 && math.Abs(math.Remainder(float64(host.Yaw-segment.Yaw), 360)) < 1e-2 {
			return true
		}
	}
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
	// 原生首轮AddonPass前会清空belt→belt的input；回填仅供后续胶囊/恢复阶段使用。
	// 因而同批只有上游、没有output的供剂端在首轮没有末端朝向限制，不能把后期指针提前套用。
	if hasInput && surfaceIsBelt(records[input].ItemID) {
		hasInput = false
	}
	if hasInput != hasOutput {
		terminal = true
		other := input
		if hasOutput {
			other = output
		}
		direction = surfaceUnit(placement.Vector{positions[other].x, positions[other].y, positions[other].z}.Sub(point))
	}
	for i := range geometry.AddonAreaColPoses {
		channel := addonBeltChannel(pose, geometry, i)
		if channel.contains(point) && (!terminal || math.Abs(direction.Dot(channel.forward)) > .95) {
			return true
		}
	}
	return false
}

// beltChannel 保存原版附着通道在粘贴位置的逻辑中心、轴线和方向。
type beltChannel struct {
	logical, forward placement.Vector
	span             surfaceBeltSpan
}

// addonBeltChannel 还原附着碰撞通道，与原生带节点占位校验和施工分图共同使用同一坐标。
func addonBeltChannel(pose surfacePose, geometry placement.PrefabGeometry, i int) beltChannel {
	channel := geometry.AddonAreaColPoses[i]
	position := func(v [3]float32) placement.Vector {
		return pose.position.Add(pose.transform(placement.Vector{float64(v[0]), float64(v[1]), float64(v[2])}))
	}
	center := position(channel.Position)
	forward := pose.transform(rotateSurfaceQuaternion(placement.Vector{0, 0, 1}, channel.Rotation))
	length := float64(geometry.AddonAreaSize[i][2]) * 3
	span := surfaceBeltSpan{ends: [2]placement.Vector{center.Sub(forward.Scale(length)), center.Add(forward.Scale(length))}}
	return beltChannel{position(geometry.AddonAreaPoses[i].Position), forward, span}
}

// contains 使用原版节点和通道中心线的距离阈值，在末端朝向检查前识别可能的宿主带。
func (channel beltChannel) contains(point placement.Vector) bool {
	delta := channel.logical.Sub(point)
	return delta.Dot(delta) < .05 || surfaceSpanDistanceSquared(surfaceBeltSpan{ends: [2]placement.Vector{point, point}}, channel.span) < .09
}

// JoinPlanetCoaterBelts 将喷涂机与经过两条实体通道的传送带归入同张施工图。纬度是整星锚点格；原版并不将这些附着关系写成建筑输入/输出。
func JoinPlanetCoaterBelts(ctx context.Context, doc blueprint.Document, latitude int, join func(addon, belt int)) error {
	if doc.PrimaryAreaIndex < 0 || int(doc.PrimaryAreaIndex) >= len(doc.Areas) {
		return fmt.Errorf("喷涂机所需的主粘贴区域无效")
	}
	encoding := &surfaceEncoding{areas: doc.Areas, plan: &SurfacePlan{GridSegment: 200, AnchorLatitudeGrid: latitude, PrimaryAreaIndex: int(doc.PrimaryAreaIndex)}}
	positions, err := encoding.surfaceWorldPositions(doc.Buildings)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	var belts worldSpatialIndex
	for i, b := range doc.Buildings {
		if surfaceIsBelt(b.ItemID) {
			belts.addPoint(positions[i], 4, i)
		}
	}
	for i, b := range doc.Buildings {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.ItemID != 2313 {
			continue
		}
		model, err := catalog.Geometry(b.ItemID, b.ModelIndex)
		if err != nil || len(model.AddonAreaColPoses) != 2 || len(model.AddonAreaSize) != 2 || len(model.AddonAreaPoses) != 2 {
			return fmt.Errorf("无法识别喷涂机 #%d 的物料和供剂通道", i+1)
		}
		pose, err := encoding.recordPose(positions[i], b.Yaw, 0, 0)
		if err != nil {
			return err
		}
		for n := range model.AddonAreaPoses {
			channel := addonBeltChannel(pose, model, n)
			low, high := channel.logical, channel.logical
			for axis := range low {
				low[axis] = min(low[axis], channel.span.ends[0][axis], channel.span.ends[1][axis]) - .3
				high[axis] = max(high[axis], channel.span.ends[0][axis], channel.span.ends[1][axis]) + .3
			}
			found := false
			belts.any(worldSpatialBounds(low, high, 4), func(belt int) bool {
				p := positions[belt]
				if channel.contains(placement.Vector{p.x, p.y, p.z}) {
					join(i, belt)
					found = true
				}
				return false
			})
			if !found {
				return fmt.Errorf("未能确定喷涂机 #%d 第 %d 个通道的宿主传送带，无法安全分图", i+1, n+1)
			}
		}
	}
	return nil
}

// validateBeltBuildingOccupancy 职责：用带节点球、游戏缩短胶囊及连接/喷涂豁免检查建筑占用；使用：动态预检；边界：保守离线规则，未复现场景碰撞恢复、合并已有带或科技限制。
func (encoding *surfaceEncoding) validateBeltBuildingOccupancy(ctx context.Context, records []blueprint.Building) error {
	var candidates, narrow uint64
	defer func() { recordPerformanceCollision(ctx, "beltBuildingOccupancy", candidates, narrow) }()
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
	occupied := worldSpatialIndex{}
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
			occupied.add(worldSpatialBounds(low, high, 8), len(boxes))
			boxes = append(boxes, placement.OwnedBox{Owner: i, Geometry: box})
		}
	}
	const radius = surfaceBeltNodeRadius
	for i, record := range records {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "传送带建筑占用检查已取消或超时"}
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		point := placement.Vector{positions[i].x, positions[i].y, positions[i].z}
		input, hasInput := byIndex[record.InputObjIdx]
		output, hasOutput := byIndex[record.OutputObjIdx]
		before, after := point, point
		through := hasInput && hasOutput && surfaceIsBelt(records[input].ItemID) && surfaceIsBelt(records[output].ItemID)
		if through {
			before = placement.Vector{positions[input].x, positions[input].y, positions[input].z}
			after = placement.Vector{positions[output].x, positions[output].y, positions[output].z}
		}
		probe := encoding.beltNodeProbe(point, before, after, through)
		center, first, second := probe.center, probe.span.ends[0], probe.span.ends[1]
		margin := placement.Vector{radius, radius, radius}
		var collision error
		occupied.any(worldSpatialBounds(center.Sub(margin), center.Add(margin), 8), func(index int) bool {
			candidates++
			box := boxes[index]
			narrow++
			if box.Geometry.SegmentDistanceSquared(center, center) >= radius*radius-1e-6 {
				return false
			}
			narrow++
			linked := surfaceBeltBuildingLink(i, box.Owner, records, byIndex)
			if records[box.Owner].ItemID == 2020 {
				end := i
				if hasOutput && surfaceIsBelt(records[output].ItemID) {
					end = output
				}
				linked = encoding.splitterShortConnectionAllowed(i, box.Owner, end, records, positions, byIndex)
			}
			if box.Geometry.SegmentDistanceSquared(first, second) >= radius*radius-1e-6 || linked {
				return false
			}
			if geometry, exists := geometries[box.Owner]; exists && encoding.surfaceAddonBeltPass(i, box.Owner, records, positions, byIndex, geometry) {
				return false
			}

			collision = &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("传送带 %d（局部 %v）侵占建筑 %d（物品 %d，局部 %v）的建造盒且不满足连接/附加通道规则；带输入%d:%d、输出%d:%d，建筑朝向%.2f°", record.Index, record.LocalOffset, records[box.Owner].Index, records[box.Owner].ItemID, records[box.Owner].LocalOffset, record.InputObjIdx, record.InputFromSlot, record.OutputObjIdx, record.OutputToSlot, records[box.Owner].Yaw)}
			return true
		})
		if collision != nil {
			return collision
		}
	}
	return nil
}
