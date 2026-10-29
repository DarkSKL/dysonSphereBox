package build

// 文件职责：把原型建筑的三维建造盒转换为塔接驳的保守障碍格。
// 使用：高架寻路前登记本组建筑；边界：局部平面路由只是候选，球面最终预检仍不可省略。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// stationBuildingObstacles 职责：防止低层高架穿过熔炉、炼油厂等实体；使用：塔策略高架寻路；边界：以保守 1.2 米格距构建候选，不用原来的二维 footprint 代替建筑高度。
func stationBuildingObstacles(ctx context.Context, existing []*placed) (map[vector]bool, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	const grid, height, radius = 1.2, 1.3333333, .25
	blocked := map[vector]bool{}
	for _, p := range existing {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔三维障碍登记已取消或超时"}
		}
		if surfaceIsBelt(p.record.ItemID) || p.record.ItemID >= 2011 && p.record.ItemID <= 2014 || p.record.ItemID == 2313 {
			continue
		}
		shapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
		if err != nil {
			return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		angle := float64(p.record.Yaw) * math.Pi / 180
		pose := surfacePose{position: placement.Vector{p.a.x * grid, p.a.z * height, p.a.y * grid}, axes: [3]placement.Vector{{math.Cos(angle), 0, -math.Sin(angle)}, {0, 1, 0}, {math.Sin(angle), 0, math.Cos(angle)}}}
		for _, shape := range shapes {
			box := placement.Box{Center: pose.position.Add(pose.transform(placement.Vector{float64(shape.Position[0]), float64(shape.Position[1]), float64(shape.Position[2])})), HalfSize: placement.Vector{float64(shape.Extents[0]), float64(shape.Extents[1]), float64(shape.Extents[2])}}
			for axis, unit := range [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
				box.Axes[axis] = pose.transform(rotateSurfaceQuaternion(unit, shape.Rotation))
			}
			if !box.Valid() {
				return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: "塔接驳的原型三维障碍无效"}
			}
			low, high := box.Bounds()
			for x := math.Floor((low[0] - radius) / grid); x <= math.Ceil((high[0]+radius)/grid); x++ {
				for y := math.Floor((low[2] - radius) / grid); y <= math.Ceil((high[2]+radius)/grid); y++ {
					for z := max(0., math.Floor((low[1]-radius-.2)/height)); z <= math.Ceil((high[1]+radius-.2)/height); z++ {
						point := placement.Vector{x * grid, z*height + .2, y * grid}
						if box.SegmentDistanceSquared(point, point) < radius*radius {
							blocked[vector{x, y, z}] = true
						}
					}
				}
			}
		}
	}
	return blocked, nil
}
