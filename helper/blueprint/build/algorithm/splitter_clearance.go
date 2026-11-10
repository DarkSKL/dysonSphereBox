package build

// 文件职责：对三形态分流器执行独立、明确的保守选择体积净空检查。
// 使用：完整蓝图编码前，在原生建造盒检查之外验证实体和真实带段。
// 边界：不是Unity原生选择盒碰撞规则；仅正确槽位上的已验证短接头可穿入，不以任意三跳连接放行。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
)

// validateSplitterClearance 职责：拒绝无关设施或带段穿过分流器的保守选择体积；使用：最终共同规则出口；边界：原生细分建造盒继续独立执行，不把本检查报告成游戏内放置验收。
func (encoding *surfaceEncoding) validateSplitterClearance(ctx context.Context, original []blueprint.Building) error {
	count := 0
	for _, record := range original {
		if record.ItemID == 2020 {
			count++
		}
	}
	if count == 0 {
		return nil
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return err
	}
	positions, err := encoding.surfaceWorldPositions(original)
	if err != nil {
		return err
	}
	records := append([]blueprint.Building(nil), original...)
	byIndex := make(map[int32]int, len(records))
	for i, record := range records {
		byIndex[record.Index] = i
	}
	for _, record := range records {
		if target, exists := byIndex[record.OutputObjIdx]; exists && surfaceIsBelt(record.ItemID) && surfaceIsBelt(records[target].ItemID) {
			records[target].InputObjIdx = record.Index
		}
	}
	static := []placement.OwnedBox{}
	selections := []placement.OwnedBox{}
	cells := map[[3]int][]int{}
	for i, record := range records {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if surfaceIsBelt(record.ItemID) || record.ItemID >= 2011 && record.ItemID <= 2014 {
			continue
		}
		shapes, err := splitterClearanceShapes(catalog, record.ItemID, record.ModelIndex)
		if err != nil {
			return err
		}
		for shapeIndex, shape := range shapes {
			box, err := encoding.colliderBox(positions[i], record.Yaw, shape)
			if err != nil {
				return err
			}
			static = append(static, placement.OwnedBox{Owner: int(record.Index), Geometry: box})
			if record.ItemID == 2020 && shapeIndex == len(shapes)-1 {
				number := len(selections)
				selections = append(selections, placement.OwnedBox{Owner: i, Geometry: box})
				connectedShapeCells(connectedBoxShape("static", box), func(cell [3]int) bool {
					cells[cell] = append(cells[cell], number)
					return true
				})
			}
		}
	}
	left, right, err := placement.FirstConflict(ctx, static)
	if err != nil {
		return err
	}
	if left >= 0 {
		return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("建筑%d与%d侵占分流器保守选择体积；该净空在原生细分建造盒之外独立保留", left, right)}
	}
	world := func(i int) placement.Vector {
		position := placement.Vector{positions[i].x, positions[i].y, positions[i].z}
		return position.Add(encoding.localUp(position).Scale(.2))
	}
	for i, record := range records {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !surfaceIsBelt(record.ItemID) {
			continue
		}
		end := i
		if next, exists := byIndex[record.OutputObjIdx]; exists && surfaceIsBelt(records[next].ItemID) {
			end = next
		}
		first, second := world(i), world(end)
		seen := map[int]bool{}
		var collision error
		connectedShapeCells(connectedBeltShape(first, second), func(cell [3]int) bool {
			for _, number := range cells[cell] {
				if seen[number] {
					continue
				}
				seen[number] = true
				selection := selections[number]
				if selection.Geometry.SegmentDistanceSquared(first, second) >= surfaceBeltNodeRadius*surfaceBeltNodeRadius-1e-6 {
					continue
				}
				if encoding.splitterShortConnectionAllowed(i, selection.Owner, end, records, positions, byIndex) {
					continue
				}
				gate := records[selection.Owner]
				collision = &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("传送带%d至%d侵占分流器%d（模型%d）的保守选择体积，且不是正确槽位上的原生短接头", record.Index, records[end].Index, gate.Index, gate.ModelIndex)}
				return false
			}
			return true
		})
		if collision != nil {
			return collision
		}
	}
	return nil
}
