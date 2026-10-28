package build

// 文件职责：用游戏原型的数值建造盒检查球面静态建筑之间的三维碰撞。
// 使用：最终 float32 记录通过 Area 和接线校验后调用。
// 边界：不覆盖传送带动态形状、分拣器端口修正与扫掠、地形或运行时 Mod；通过仍为 partial。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// validateStaticColliders 职责：把实际原型盒放到球面世界坐标并查询冲突；使用：球面编码出口；边界：动态对象单独留待适配，未知静态模型不放行。
func (encoding *surfaceEncoding) validateStaticColliders(ctx context.Context, records []blueprint.Building) error {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	boxes := []placement.OwnedBox{}
	descriptions := make(map[int32]string, len(records))
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "静态建筑碰撞检查已取消或超时"}
		}
		if surfaceIsBelt(record.ItemID) || record.ItemID >= 2011 && record.ItemID <= 2014 {
			continue
		}
		descriptions[record.Index] = fmt.Sprintf("物品 %d / 模型 %d / Area %d / 局部(%.3f,%.3f,%.3f)", record.ItemID, record.ModelIndex, record.AreaIndex, record.LocalOffset.X, record.LocalOffset.Y, record.LocalOffset.Z)
		shapes, err := catalog.Shapes(record.ItemID, record.ModelIndex)
		if err != nil {
			return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
		}
		for _, shape := range shapes {
			box, err := surfaceColliderBox(positions[i], record.Yaw, shape)
			if err != nil {
				return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: fmt.Sprintf("建筑 %d：%v", record.Index, err)}
			}
			boxes = append(boxes, placement.OwnedBox{Owner: int(record.Index), Geometry: box})
		}
	}
	left, right, err := placement.FirstConflict(ctx, boxes)
	if err != nil {
		return &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	if left >= 0 {
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("静态建筑三维建造盒冲突：建筑 %d（%s）与建筑 %d（%s）（原型规则 %s；检查覆盖原版静态建造盒，不含传送带/分拣器动态扫掠）", left, descriptions[int32(left)], right, descriptions[int32(right)], catalog.Version())}
	}
	encoding.plan.StaticCollidersChecked = true
	encoding.plan.ColliderGameVersion = catalog.Version()
	encoding.plan.CollisionCoverage = "staticBuildColliders"
	return nil
}

// surfaceColliderBox 职责：将 Unity 局部建造盒按球面切线朝向变换；使用：静态占位登记；边界：只采用最终首端中心和 yaw，输入几何不合法时拒绝。
func surfaceColliderBox(origin vector, yaw float32, collider placement.Collider) (placement.Box, error) {
	center := placement.Vector{origin.x, origin.y, origin.z}
	length := math.Sqrt(center.Dot(center))
	if length < 1 {
		return placement.Box{}, fmt.Errorf("球面中心无效")
	}
	up := center.Scale(1 / length)
	east := up.Cross(placement.Vector{0, 1, 0})
	length = math.Sqrt(east.Dot(east))
	if length < .0001 {
		return placement.Box{}, fmt.Errorf("极点朝向尚未验证")
	}
	east = east.Scale(1 / length)
	north := east.Cross(up)
	angle := float64(yaw) * math.Pi / 180
	cosine, sine := math.Cos(angle), math.Sin(angle)
	basis := [3]placement.Vector{east.Scale(cosine).Sub(north.Scale(sine)), up, east.Scale(sine).Add(north.Scale(cosine))}
	toWorld := func(local placement.Vector) placement.Vector {
		return basis[0].Scale(local[0]).Add(basis[1].Scale(local[1])).Add(basis[2].Scale(local[2]))
	}
	quaternion := collider.Rotation
	q := placement.Vector{float64(quaternion[0]), float64(quaternion[1]), float64(quaternion[2])}
	qw := float64(quaternion[3])
	if norm := q.Dot(q) + qw*qw; math.IsNaN(norm) || math.Abs(norm-1) > 1e-4 {
		return placement.Box{}, fmt.Errorf("原型碰撞盒旋转无效")
	}
	rotateLocal := func(v placement.Vector) placement.Vector {
		cross := q.Cross(v)
		return v.Add(cross.Scale(2 * qw)).Add(q.Cross(cross).Scale(2))
	}
	box := placement.Box{Center: center.Add(toWorld(placement.Vector{float64(collider.Position[0]), float64(collider.Position[1]), float64(collider.Position[2])})), HalfSize: placement.Vector{float64(collider.Extents[0]), float64(collider.Extents[1]), float64(collider.Extents[2])}}
	for i, axis := range [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}} {
		box.Axes[i] = toWorld(rotateLocal(axis))
	}
	if !box.Valid() {
		return placement.Box{}, fmt.Errorf("原型碰撞盒尺寸或世界旋转无效")
	}
	return box, nil
}
