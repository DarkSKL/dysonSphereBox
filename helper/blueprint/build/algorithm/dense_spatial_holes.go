package build

// 文件职责：以生产设备和物流塔足迹的最大空矩形引导有限填洞候选。
// 使用：独立塔候选的空间启发式评分；边界：离散网格仅评分，不能替代机器、臂、分流器及塔距真实净空。

import (
	"math"

	"DysonSphereBox/helper/quant"
)

// denseSpatialHolesKey 职责：保存独立非街道机器组的短行跨度；使用：填洞候选context；边界：零值关闭，不改变长街道及公开模式。
type denseSpatialHolesKey struct{}

// denseSpatialHolesVariantOffset 职责：隔离20格独立短行填洞候选；使用：塔候选内部编码；边界：保留原始候选并沿用共享输入输出。
const denseSpatialHolesVariantOffset = 6400

// denseSpatialHolesBentVariantOffset 职责：隔离18格可折行填洞候选；使用：与20格独立行对照；边界：两项均保留同一基础街道及高度预算。
const denseSpatialHolesBentVariantOffset = 12800

// denseStationHoleEligible 职责：在建立额外候选前确认独立机器行的分组确有变化；使用：塔候选登记；边界：纯低轨街道及等价小厂不增加完整生成成本，不修改既有候选集合。
func (b *builder) denseStationHoleEligible(req Request, dataset *quant.Dataset, span float64) (bool, error) {
	for _, p := range req.Plan.Productions {
		r, err := datasetRecipeForProduction(dataset, p)
		if err != nil {
			return false, err
		}
		if p.MachineCount < 2 || referenceLowRecipe(p, r) {
			continue
		}
		counts, err := layoutLineCounts(p, r, b.setting, dataset.ByID[p.MachineID].Speed, req.Density)
		if err != nil {
			return false, err
		}
		original, err := denseSpatialRowCounts(p, r, b.setting, counts, 32, false)
		if err != nil {
			return false, err
		}
		short, err := denseSpatialRowCounts(p, r, b.setting, counts, span, true)
		if err != nil {
			return false, err
		}
		if len(original) != len(short) {
			return true, nil
		}
		for i := range original {
			if original[i] != short[i] {
				return true, nil
			}
		}
	}
	return false, nil
}

// denseEquipmentHole 职责：保存最大设备空矩形及其格面积；使用：填洞对齐与评分；边界：不包含传送带、电力塔或分拣器，不能通过加带缩小空洞。
type denseEquipmentHole struct {
	// low/high/area 职责：保存格网空矩形两角与面积；使用：位置对齐与单调比较；边界：不作为排他禁占区。
	low, high vector
	area      float64
}

// denseEquipmentMark 职责：缓存一项设备旋转后的原型足迹；使用：同次压缩的网格试探；边界：仅持有实体只读引用，不修改实体的布局预算。
type denseEquipmentMark struct {
	// entity/low/high 职责：关联真实位置与当前朝向的局部足迹；使用：纯平移评分；边界：不缓存实体位置或写回布局预算。
	entity    *placed
	low, high vector
}

// denseEquipmentField 职责：固定同次压缩的评分原点、步长与原型足迹；使用：各纯平移候选；边界：不随试探包络缩放，避免栅格相位变化制造假改善。
type denseEquipmentField struct {
	// low/high/step 职责：固定本次试探的评分坐标系；使用：全部单元同尺度比较；边界：不随候选包络重采样。
	low, high vector
	step      float64
	// columns/rows/marks 职责：限定网格大小并保存只含设备和塔的足迹；使用：最大空矩形扫描；边界：单轴最多96格，带和臂不参与填充。
	columns, rows int
	marks         []denseEquipmentMark
}

// newDenseEquipmentField 职责：在压缩开始时缓存各模型真实足迹及当前Yaw；使用：独立填洞候选；边界：只写本地副本，不改变真实p.layoutFootprint或碰撞规则。
func newDenseEquipmentField(all []*placed, low, high vector) (*denseEquipmentField, error) {
	w, h := high.x-low.x, high.y-low.y
	step := math.Max(.5, math.Max(w, h)/96)
	columns, rows := max(1, int(math.Ceil(w/step))), max(1, int(math.Ceil(h/step)))
	field := &denseEquipmentField{low: low, high: high, step: step, columns: columns, rows: rows}
	cache := map[[2]int16]*[2]vector{}
	for _, entity := range all {
		if entity.record.RecipeID <= 0 && !surfaceIsStation(entity.record.ItemID) {
			continue
		}
		key := [2]int16{entity.record.ItemID, entity.record.ModelIndex}
		footprint := cache[key]
		if footprint == nil {
			var err error
			footprint, err = prototypeLayoutFootprint(key[0], key[1])
			if err != nil {
				return nil, err
			}
			cache[key] = footprint
		}
		copy := *entity
		copy.layoutFootprint = footprint
		lo, hi := placedFootprint(&copy)
		field.marks = append(field.marks, denseEquipmentMark{entity: entity, low: lo, high: hi})
	}
	return field, nil
}

// hole 职责：在固定栅格上求一次纯平移后的最大设备空矩形；使用：仅限有界压缩评分；边界：不移动真实实体，带和臂永远不能充当设备填充。
func (field *denseEquipmentField) hole(moving map[*placed]bool, delta vector) denseEquipmentHole {
	low, high, step, columns, rows := field.low, field.high, field.step, field.columns, field.rows
	occupied := make([]bool, columns*rows)
	for _, mark := range field.marks {
		lo, hi := mark.low, mark.high
		x, y := mark.entity.a.x-low.x, mark.entity.a.y-low.y
		if moving[mark.entity] {
			x, y = x+delta.x, y+delta.y
		}
		x0, x1 := max(0, int(math.Floor((x+lo.x)/step))), min(columns, int(math.Ceil((x+hi.x)/step)))
		y0, y1 := max(0, int(math.Floor((y+lo.y)/step))), min(rows, int(math.Ceil((y+hi.y)/step)))
		for iy := y0; iy < y1; iy++ {
			for ix := x0; ix < x1; ix++ {
				occupied[iy*columns+ix] = true
			}
		}
	}
	heights := make([]int, columns)
	starts, levels := make([]int, 0, columns), make([]int, 0, columns)
	best := denseEquipmentHole{}
	for y := 0; y < rows; y++ {
		for x := range heights {
			if occupied[y*columns+x] {
				heights[x] = 0
			} else {
				heights[x]++
			}
		}
		starts, levels = starts[:0], levels[:0]
		for x := 0; x <= columns; x++ {
			level, start := 0, x
			if x < columns {
				level = heights[x]
			}
			for len(levels) > 0 && levels[len(levels)-1] > level {
				last := len(levels) - 1
				left, height := starts[last], levels[last]
				starts, levels, start = starts[:last], levels[:last], left
				lo := vector{x: low.x + float64(left)*step, y: low.y + float64(y-height+1)*step}
				hi := vector{x: math.Min(high.x, low.x+float64(x)*step), y: math.Min(high.y, low.y+float64(y+1)*step)}
				area := (hi.x - lo.x) * (hi.y - lo.y)
				if area > best.area {
					best = denseEquipmentHole{low: lo, high: hi, area: area}
				}
			}
			if len(levels) == 0 || levels[len(levels)-1] < level {
				starts, levels = append(starts, start), append(levels, level)
			}
		}
	}
	return best
}

// denseHoleAlignments 职责：把单元整体对齐当前最大设备空洞的两边与中心；使用：双轴位置优先队列；边界：仅产生九项四分之一格试探，真实轮廓不合则拒绝。
func denseHoleAlignments(hole denseEquipmentHole, low, high vector) []vector {
	positions := make([]vector, 0, 9)
	for _, x := range []float64{hole.low.x - low.x, hole.high.x - high.x, (hole.low.x + hole.high.x - low.x - high.x) / 2} {
		for _, y := range []float64{hole.low.y - low.y, hole.high.y - high.y, (hole.low.y + hole.high.y - low.y - high.y) / 2} {
			positions = append(positions, vector{x: math.Round(x*4) / 4, y: math.Round(y*4) / 4})
		}
	}
	return positions
}
