package build

// 文件职责：在全场接线之前把同一生产者的完整短机器组组合成L/U凹形单元。
// 使用：私有密铺候选显式启用；边界：默认关闭，不拆开机器组、不修改物料份额或设备，已跨单元接线的实体不参与折行。

import (
	"context"
	"fmt"

	"DysonSphereBox/helper/blueprint"
)

// denseBentRowsKey 职责：隔离完整短run折行候选；使用：配合denseMaxSpan12或18的私有context；边界：原直线候选缺省不启用。
type denseBentRowsKey struct{}

// denseBentGroupKey 职责：禁止不同生产者、配方或设备混入同一折行单元；使用：短run分组；边界：同物品不代表同一生产身份。
type denseBentGroupKey struct{ owner, recipe, machine int }

// prepareDenseBentRows 职责：按原顺序将同身份的两三个完整短run折成L或U；使用：裁尾后、全场落位和原生纬带映射前；边界：只移动闭合局部实体，原规范和辅助物流仍由公共路由器接通。
func (b *builder) prepareDenseBentRows(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
	enabled, _ := ctx.Value(denseBentRowsKey{}).(bool)
	if !enabled || !b.denseSpatial || b.adapter == nil || b.adapter.density != densityDense {
		return units, nil
	}
	if b.polarFrame != nil || b.nativeLatitudeScaled {
		return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "折行必须在原生格距映射及全场接线之前完成"}
	}
	owners := map[*placed]int{}
	for index, unit := range units {
		for _, entity := range unit.entities {
			if _, exists := owners[entity]; exists {
				return nil, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "折行单元存在重复实体归属"}
			}
			owners[entity] = index
		}
	}
	open := map[int]bool{}
	for _, entity := range b.all {
		owner, owned := owners[entity]
		for _, linked := range []*placed{entity.input, entity.output} {
			if linked == nil {
				continue
			}
			other, otherOwned := owners[linked]
			if owned != otherOwned || owned && owner != other {
				if owned {
					open[owner] = true
				}
				if otherOwned {
					open[other] = true
				}
			}
		}
	}
	groups := map[denseBentGroupKey][]int{}
	order := []denseBentGroupKey{}
	for index, unit := range units {
		row := unit.row
		if row == nil || len(unit.rows) != 0 || len(unit.entities) == 0 || row.line == nil {
			continue
		}
		columns := map[[2]float64]bool{}
		for _, machine := range row.line.machines {
			columns[[2]float64{machine.a.x, machine.a.y}] = true
		}
		if len(columns) < 2 {
			continue
		}
		if open[index] {
			continue
		}
		key := denseBentGroupKey{row.owner, row.recipe.ID, row.info.MachineID}
		if len(groups[key]) == 0 {
			order = append(order, key)
		}
		groups[key] = append(groups[key], index)
	}
	replacements := map[int]denseSpatialUnit{}
	consumed := map[int]bool{}
	lShapes, uShapes, runs := 0, 0, 0
	for _, key := range order {
		indices := groups[key]
		for len(indices) >= 2 {
			if err := b.checkGeneration("真实短机器组折行"); err != nil {
				return nil, err
			}
			count := min(3, len(indices))
			var combined denseSpatialUnit
			var accepted bool
			for ; count >= 2; count-- {
				parts := make([]denseSpatialUnit, count)
				for i := range parts {
					parts[i] = units[indices[i]]
				}
				var err error
				combined, accepted, err = b.composeDenseBentRows(ctx, parts)
				if err != nil {
					return nil, err
				}
				if accepted {
					break
				}
			}
			if !accepted {
				indices = indices[1:]
				continue
			}
			replacements[indices[0]] = combined
			for _, index := range indices[:count] {
				consumed[index] = true
			}
			runs += count
			if count == 3 {
				uShapes++
			} else {
				lShapes++
			}
			indices = indices[count:]
		}
	}
	if runs == 0 {
		return nil, &blueprint.Error{Code: "GEN_I_DENSITY_EQUIVALENT", Message: "本折行候选没有可组成L/U的完整短机器组，保留独立直线候选"}
	}
	result := make([]denseSpatialUnit, 0, len(units)-runs+lShapes+uShapes)
	for index, unit := range units {
		if replacement, exists := replacements[index]; exists {
			result = append(result, replacement)
		} else if !consumed[index] {
			result = append(result, unit)
		}
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-bent-rows", Status: "passed", Message: fmt.Sprintf("%d个同生产者/同配方完整短run按真实轮廓组成%d个L形及%d个U形凹单元；各run保留原机器、层数、臂及物流身份，角内空隙不设矩形禁占，跨run供料和出货随后共同接线与终检", runs, lShapes, uShapes)})
	}
	return result, nil
}
