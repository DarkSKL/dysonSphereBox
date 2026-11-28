package build

// 文件职责：把原型建筑的三维建造盒转换为塔接驳的保守障碍格。
// 使用：高架寻路前登记本组建筑；边界：局部平面路由只是候选，球面最终预检仍不可省略。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// newLocalTransportEnvironment 建立一次性副产物或自产剂接线环境。塔外部交付口可以扩大边界，各调用方再登记自己的尾链或未来端口。
func newLocalTransportEnvironment(ctx context.Context, existing []*placed, end *vector) (*transportEnvironment, error) {
	low, high := bounds(existing)
	blocked, err := stationBuildingObstacles(ctx, existing, true)
	if err != nil {
		return nil, err
	}
	if end != nil {
		low.x, low.y = math.Min(low.x, end.x), math.Min(low.y, end.y)
		high.x, high.y = math.Max(high.x, end.x), math.Max(high.y, end.y)
	}
	return &transportEnvironment{blocked: blocked,
		minimum: vector{math.Floor(low.x) - 8, math.Floor(low.y) - 8, 0},
		maximum: vector{math.Ceil(high.x) + 8, math.Ceil(high.y) + 8, transportMaxHeight}}, nil
}

// stationBuildingObstacles 注册真实建筑盒。局部接线同时登记既有带和臂，全厂路由再按其精确布线策略登记运输占位。
func stationBuildingObstacles(ctx context.Context, existing []*placed, includeTransport bool) (map[vector]bool, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	// 平面障碍仅用于筛选候选；球面曲率及切线旋转会吞掉建筑顶部附近的余隙。
	const grid, height, radius = 1.2, 1.3333333, .35
	blocked := map[vector]bool{}
	for _, p := range existing {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物流塔三维障碍登记已取消或超时"}
		}
		if surfaceIsBelt(p.record.ItemID) {
			if includeTransport {
				blocked[beltCell(p.a)] = true
			}
			continue
		}
		if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
			if includeTransport {
				blockTransportInserter(blocked, p)
			}
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

// blockTransportInserter 按分拣器两端的格网包络保留原离散臂通道。
func blockTransportInserter(blocked map[vector]bool, arm *placed) {
	for x := math.Floor(math.Min(arm.a.x, arm.b.x)); x <= math.Ceil(math.Max(arm.a.x, arm.b.x)); x++ {
		for y := math.Floor(math.Min(arm.a.y, arm.b.y)); y <= math.Ceil(math.Max(arm.a.y, arm.b.y)); y++ {
			blocked[vector{x, y, math.Round(arm.a.z)}] = true
		}
	}
}
