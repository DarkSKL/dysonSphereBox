package build

// 文件职责：在原生单元落位时复用正式分拣器动态盒。
// 使用：既有装箱失败后的几何细化；边界：仅核对连接两端均属于同一刚性单元的分拣器，跨单元待接线保持原有粗筛，不猜测未来端点。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
)

// polarUnitInserterBoxes 职责：按当前球面位置计算完整单元的真实分拣器动态盒；使用：细化装箱的每个纬向候选；边界：未闭合单元返回未核对标记，未知几何及实际槽位/跨度错误均不能当作可用盒。
func (b *builder) polarUnitInserterBoxes(ctx context.Context, unit denseSpatialUnit) ([]placement.Box, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if b.polarFrame == nil {
		return nil, false, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "原生分拣器检查缺少目标球面坐标"}
	}
	indices := make(map[*placed]int32, len(unit.entities))
	for i, entity := range unit.entities {
		indices[entity] = int32(i)
	}
	hasArms := false
	for _, entity := range unit.entities {
		if entity.record.ItemID < 2011 || entity.record.ItemID > 2014 {
			continue
		}
		hasArms = true
		if _, ok := indices[entity.input]; !ok {
			return nil, false, nil
		}
		if _, ok := indices[entity.output]; !ok {
			return nil, false, nil
		}
	}
	if !hasArms {
		return nil, true, nil
	}
	records := make([]blueprint.Building, len(unit.entities))
	first, second := make([]vector, len(records)), make([]vector, len(records))
	for i, entity := range unit.entities {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		record := entity.record
		record.Index, record.AreaIndex = int32(i), 0
		record.InputObjIdx, record.OutputObjIdx = -1, -1
		if index, ok := indices[entity.input]; ok {
			record.InputObjIdx = index
		}
		if index, ok := indices[entity.output]; ok {
			record.OutputObjIdx = index
		}
		records[i] = record
		a, c := b.polarFrame.world(entity.a, 0), b.polarFrame.world(entity.b, 0)
		first[i], second[i] = vector{a[0], a[1], a[2]}, vector{c[0], c[1], c[2]}
	}
	encoding := &surfaceEncoding{}
	owned, err := encoding.surfaceInserterBoxesAtWorld(ctx, records, first, second, nil)
	if err != nil {
		return nil, false, err
	}
	left, right, _, err := placement.FirstConflictWithStatistics(ctx, owned)
	if err != nil {
		return nil, false, err
	}
	if left >= 0 || right >= 0 {
		return nil, false, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "当前纬度的单元内部真实分拣器动态盒相交"}
	}
	boxes := make([]placement.Box, len(owned))
	for i, box := range owned {
		boxes[i] = box.Geometry
	}
	return boxes, true, nil
}
