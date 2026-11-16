package build

// 文件职责：为超密度箱罐保留竖向扩建空间，并从最终带段独立复核。
// 使用：局部装箱、三维寻路和最终规则出口；边界：只限制带路，不改变建筑真实碰撞盒或库存旁路。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
)

// expandableStorage 职责：识别可堆叠的箱体与储液罐；使用：顶部预留；边界：不把物流塔视为储物仓。
func expandableStorage(item int16) bool { return item == 2101 || item == 2102 || item == 2106 }

// storageRoofCollider 职责：按真实原型横截面生成箱罐上方的预留体；使用：规划和终检共用尺寸；边界：从现有建造盒顶面开始，覆盖生成器最大走线高度。
func storageRoofCollider(catalog *placement.Catalog, item, model int16) (placement.Collider, error) {
	shapes, err := catalog.Shapes(item, model)
	if err != nil {
		return placement.Collider{}, err
	}
	low, high := placement.Vector{math.Inf(1), math.Inf(1), math.Inf(1)}, placement.Vector{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	encoding := newPlanarRuleEncoding()
	for _, shape := range shapes {
		box, err := encoding.colliderBox(vector{}, 0, shape)
		if err != nil {
			return placement.Collider{}, err
		}
		a, c := box.Bounds()
		for axis := 0; axis < 3; axis++ {
			low[axis] = math.Min(low[axis], a[axis])
			high[axis] = math.Max(high[axis], c[axis])
		}
	}
	height := float32(transportMaxHeight*1.3333333 + 4)
	return placement.Collider{Position: [3]float32{float32((low[0] + high[0]) / 2), float32(high[1]) + height/2, float32((low[2] + high[2]) / 2)}, Extents: [3]float32{float32((high[0] - low[0]) / 2), height / 2, float32((high[2] - low[2]) / 2)}, Rotation: [4]float32{0, 0, 0, 1}}, nil
}

// storageRoofContours 职责：将库顶预留体加入局部轮廓；使用：超密度旋转与平移；边界：普通布局不附加该生成约束。
func (b *builder) storageRoofContours(entities []*placed) ([]packingShape, error) {
	if b.policy == nil || b.policy.density != densityDense {
		return nil, nil
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	encoding := newPlanarRuleEncoding()
	out := []packingShape{}
	for _, entity := range entities {
		if !expandableStorage(entity.record.ItemID) {
			continue
		}
		collider, err := storageRoofCollider(catalog, entity.record.ItemID, entity.record.ModelIndex)
		if err != nil {
			return nil, err
		}
		box, err := encoding.colliderBox(vector{entity.a.x * planarGridLength, entity.a.z*1.3333333 + .2, entity.a.y * planarGridLength}, entity.record.Yaw, collider)
		if err != nil {
			return nil, err
		}
		out = append(out, boxPackingShape("storage-roof", box))
	}
	return out, nil
}

// storageRoofConflict 职责：仅阻止带段和带节点进入顶部预留体；使用：局部布局窄相位；边界：允许箱体自身的配送器附加设施，不把竖向预留扩成地面空地。
func storageRoofConflict(a, c *packingShape) bool {
	if a.kind != "storage-roof" {
		a, c = c, a
	}
	if c.kind != "belt" && c.kind != "dense-node" {
		return false
	}
	return a.box.SegmentDistanceSquared(c.span.ends[0], c.span.ends[1]) < .23*.23-1e-6
}

// validateStorageRoofClearance 职责：从最终浮点记录检查所有箱罐顶面上方的完整带段；使用：超密度导出前；边界：使用真实球面坐标及空间桶，不以两个端点在体外代替整段检查。
func (encoding *surfaceEncoding) validateStorageRoofClearance(ctx context.Context, records []blueprint.Building) error {
	count := 0
	for _, record := range records {
		if expandableStorage(record.ItemID) {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	positions, err := encoding.surfaceWorldPositions(records)
	if err != nil {
		return err
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	boxes := []packingShape{}
	owners := []int32{}
	cells := map[[3]int][]int{}
	byID := make(map[int32]int, len(records))
	for i, record := range records {
		byID[record.Index] = i
		if !expandableStorage(record.ItemID) {
			continue
		}
		collider, err := storageRoofCollider(catalog, record.ItemID, record.ModelIndex)
		if err != nil {
			return err
		}
		box, err := encoding.colliderBox(positions[i], record.Yaw, collider)
		if err != nil {
			return err
		}
		shape := boxPackingShape("storage-roof", box)
		number := len(boxes)
		boxes = append(boxes, shape)
		owners = append(owners, record.Index)
		packingShapeCells(shape, func(cell [3]int) bool { cells[cell] = append(cells[cell], number); return true })
	}
	marks := collisionQueryMarks{}
	for i, record := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		first := placement.Vector{positions[i].x, positions[i].y, positions[i].z}
		// 世界坐标已包含游戏带基准高度，和动态带建筑占用检查采用同一中心。
		if encoding.planar {
			first[1] += .2
		} else {
			normal := first.Scale(1 / math.Sqrt(first.Dot(first)))
			first = first.Add(normal.Scale(.2))
		}
		second := first
		if j, exists := byID[record.OutputObjIdx]; exists && surfaceIsBelt(records[j].ItemID) {
			second = placement.Vector{positions[j].x, positions[j].y, positions[j].z}
			if encoding.planar {
				second[1] += .2
			} else {
				normal := second.Scale(1 / math.Sqrt(second.Dot(second)))
				second = second.Add(normal.Scale(.2))
			}
		}
		span := beltPackingShape(first, second)
		marks.begin(len(boxes))
		hit := -1
		packingShapeCells(span, func(cell [3]int) bool {
			for _, n := range cells[cell] {
				if marks.seen(n) {
					continue
				}
				if storageRoofConflict(&boxes[n], &span) {
					hit = n
					return false
				}
			}
			return true
		})
		if hit >= 0 {
			return &blueprint.Error{Code: "GEN_E_STORAGE_CLEARANCE", Message: fmt.Sprintf("传送带%d经过箱罐%d顶部扩建空间", record.Index, owners[hit])}
		}
	}
	return nil
}
