package planet

// 文件职责：在矿区用实际分拣器拉伸盒核对露出矿物。使用：完整工区选址和整图终检。
// 边界：保留采集端点和矿物本体，地基保护格不作为悬空碰撞体。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	buildcore "DysonSphereBox/helper/blueprint/build/core"
	"DysonSphereBox/helper/blueprint/build/geometry"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// inserterVeinConflict 返回首个拉伸盒触矿的分拣器下标。先筛附近矿点，再复用正式的端口、朝向和跨度修正。
func (req *Request) inserterVeinConflict(ctx context.Context, objects []Object) (int, error) {
	bodies := req.veinBodies.volume
	if len(bodies) == 0 {
		return -1, nil
	}
	var byIndex map[int32]int
	for i, arm := range objects {
		b := arm.Building
		if b.ItemID < 2011 || b.ItemID > 2014 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		start, err := world(arm.First)
		if err != nil {
			return -1, err
		}
		// 原版修正后最长臂为 7.5 米。一个 16 米体素的半径也容得下端口修正、带端延伸及盒体高度。
		if !overlapsObstacle(bodies, start, 16) {
			continue
		}
		if byIndex == nil {
			byIndex = make(map[int32]int, len(objects))
			for j, obj := range objects {
				byIndex[obj.Building.Index] = j
			}
		}
		records := [3]blueprint.Building{b}
		starts := [3]geometry.WorldPoint{start}
		for end, id := range [2]int32{b.InputObjIdx, b.OutputObjIdx} {
			j, ok := byIndex[id]
			if !ok {
				return -1, fmt.Errorf("分拣器 %d 缺少连接的建筑 %d", b.Index, id)
			}
			records[end+1] = objects[j].Building
			starts[end+1], err = world(objects[j].First)
			if err != nil {
				return -1, err
			}
		}
		finish, err := world(arm.Second)
		if err != nil {
			return -1, err
		}
		boxes, err := buildcore.PlanetInserterBoxes(ctx, records, starts, finish)
		if err != nil {
			return -1, err
		}
		for _, box := range boxes {
			if req.veinBodies.overlapsBox(box.Geometry) {
				return i, nil
			}
		}
	}
	return -1, nil
}

// overlapsBox 用定向盒到球心的最短距离核对矿点。体素只缩小查询范围。
func (idx obstacleIndex) overlapsBox(box placement.Box) bool {
	low, high := box.Bounds()
	for x := obstacleCell(low[0]); x <= obstacleCell(high[0]); x++ {
		for y := obstacleCell(low[1]); y <= obstacleCell(high[1]); y++ {
			for z := obstacleCell(low[2]); z <= obstacleCell(high[2]); z++ {
				for _, sphere := range idx.volume[[3]int{x, y, z}] {
					p := placement.Vector{float64(sphere.center.X), float64(sphere.center.Y), float64(sphere.center.Z)}
					if box.SegmentDistanceSquared(p, p) <= sphere.radius*sphere.radius {
						return true
					}
				}
			}
		}
	}
	return false
}

// siteOccupied 核验完整候选的普通占位及分拣器跨矿。局部抽样仍只检查其自带的端点。
func (req *Request) siteOccupied(ctx context.Context, objects []Object, index obstacleIndex, catalog *placement.Catalog) bool {
	if placementConflict(objects, index, catalog) {
		return true
	}
	arm, err := req.inserterVeinConflict(ctx, objects)
	return arm >= 0 || err != nil
}
