package build

// 文件职责：在全场装箱后、全局接线前，以真实轮廓尝试有限纯平移压缩。
// 使用：私有候选通过context显式启用；边界：不旋转或拆分单元，不增加包络或接线距离，错误和取消恢复原始坐标。

import (
	"context"
	"fmt"
	"math"
	"sort"
)

// denseSpatialCompactKey 职责：隔离平移压缩候选；使用：候选context；边界：缺省关闭，不影响既有装箱策略。
type denseSpatialCompactKey struct{}

// denseSpatialCompactVariantOffset 职责：区分整体平移与原布局候选；使用：内部塔候选组合；边界：保留原方案，不是公开模式编号。
const denseSpatialCompactVariantOffset = 800

// denseCompactPosition 职责：保存一项纯平移和完整布局下界；使用：有限候选排序；边界：通过真实占位前不移动实体。
type denseCompactPosition struct {
	// delta 职责：保存XY格位移；使用：整组平移；边界：z始终为零。
	delta vector
	// shape/area/wire 职责：保存包络形状、面积和受影响真实流边长度；使用：单调择优；边界：不代替最终接线结果。
	shape, area, wire float64
	// hole 职责：记录生产设备和塔的最大空矩形；使用：独立填洞候选排序；边界：旧平移候选不启用该评分。
	hole float64
}

// denseCompactObjective 职责：按当前全厂格包络计算与最终密铺一致的形状目标；使用：试探和提交比较；边界：不以增加带节点填充空白获得奖励。
func denseCompactObjective(low, high vector) (float64, float64) {
	w, h := math.Ceil(high.x)-math.Floor(low.x)+1, math.Ceil(high.y)-math.Floor(low.y)+1
	area := w * h
	return area * math.Max(1, math.Max(w, h)/math.Max(1, math.Min(w, h))/3), area
}

// denseCompactBetter 职责：仅接受面积、形状和真实边长均不增加的变化；使用：压缩及内部移近；边界：至少一项严格改善，避免等价位置循环。
func denseCompactBetter(candidate, current denseCompactPosition) bool {
	const epsilon = 1e-7
	return candidate.area <= current.area+epsilon && candidate.shape <= current.shape+epsilon && candidate.wire <= current.wire+epsilon &&
		(candidate.shape < current.shape-epsilon || candidate.area < current.area-epsilon || candidate.wire < current.wire-epsilon)
}

// denseCompactEdges 职责：提取真实轮廓的有限有序平面边缘；使用：凹槽和相邻实体对齐；边界：均匀保留内外边缘，不把单元外包矩形作为排他区。
func denseCompactEdges(contours []connectedBlockContour, axis, limit int) []float64 {
	values := []float64{}
	for _, contour := range contours {
		for _, shape := range contour.shapes {
			values = append(values, shape.low[axis]/planarGridLength, shape.high[axis]/planarGridLength)
		}
	}
	sort.Float64s(values)
	unique := values[:0]
	for _, value := range values {
		if len(unique) == 0 || value-unique[len(unique)-1] > 1e-7 {
			unique = append(unique, value)
		}
	}
	if len(unique) <= limit {
		return unique
	}
	selected := make([]float64, limit)
	for i := range selected {
		selected[i] = unique[i*(len(unique)-1)/(limit-1)]
	}
	return selected
}

// denseCompactCandidates 职责：生成边缘对齐、端点对齐和小步的有限平移；使用：每个单元的坐标下降；边界：位移量化为四分之一格，最终必须逐项通过真实轮廓。
func denseCompactCandidates(contour connectedBlockContour, others []connectedBlockContour, low, high, fixedLow, fixedHigh vector, wires []denseSpatialWire) []vector {
	axes := [2][]float64{}
	for axis, worldAxis := range []int{0, 2} {
		seen := map[float64]bool{}
		add := func(value float64) {
			value = math.Round(value*4) / 4
			if !seen[value] {
				seen[value] = true
				axes[axis] = append(axes[axis], value)
			}
		}
		for _, step := range []float64{0, -.25, .25, -1, 1, -2, 2, -4, 4, -8, 8, -16, 16} {
			add(step)
		}
		movingLow, movingHigh, otherLow, otherHigh := low.x, high.x, fixedLow.x, fixedHigh.x
		if axis == 1 {
			movingLow, movingHigh, otherLow, otherHigh = low.y, high.y, fixedLow.y, fixedHigh.y
		}
		for _, value := range []float64{otherLow - movingLow, otherHigh - movingHigh, otherLow - movingHigh, otherHigh - movingLow} {
			add(value)
		}
		for _, wire := range wires {
			value := wire.fixed.a.x - wire.moving.a.x
			if axis == 1 {
				value = wire.fixed.a.y - wire.moving.a.y
			}
			add(value)
		}
		movingEdges, fixedEdges := denseCompactEdges([]connectedBlockContour{contour}, worldAxis, 8), denseCompactEdges(others, worldAxis, 32)
		for _, moving := range movingEdges {
			for _, fixed := range fixedEdges {
				// 同时保留两侧四分之一格，避免取整把相切候选推进真实盒。
				for _, offset := range []float64{-.25, 0, .25} {
					add(fixed - moving + offset)
				}
			}
		}
	}
	positions := []vector{}
	for _, x := range axes[0] {
		if x != 0 {
			positions = append(positions, vector{x: x})
		}
	}
	for _, y := range axes[1] {
		if y != 0 {
			positions = append(positions, vector{y: y})
		}
	}
	// 少量双轴步可进入凹形空间，其余候选逐轴推进，避免笛卡尔积放大预算。
	for _, x := range axes[0][:min(9, len(axes[0]))] {
		for _, y := range axes[1][:min(9, len(axes[1]))] {
			if x != 0 && y != 0 {
				positions = append(positions, vector{x: x, y: y})
			}
		}
	}
	return positions
}

// denseCompactStationClear 职责：补齐轮廓索引未表达的物流塔间距；使用：移动含塔单元；边界：内部塔相对距离不变，其它单元和未归属塔全部检查。
func (b *builder) denseCompactStationClear(unit denseSpatialUnit, members map[*placed]bool, delta vector) bool {
	for _, moving := range unit.entities {
		if !surfaceIsStation(moving.record.ItemID) {
			continue
		}
		for _, fixed := range b.all {
			if members[fixed] || !surfaceIsStation(fixed.record.ItemID) {
				continue
			}
			dx, dy := (moving.a.x+delta.x-fixed.a.x)*planarGridLength, (moving.a.y+delta.y-fixed.a.y)*planarGridLength
			if dx*dx+dy*dy < float64(surfaceStationDistanceLimit(moving.record.ItemID, fixed.record.ItemID))-1e-7 {
				return false
			}
		}
	}
	return true
}

// compactDenseSpatialUnits 职责：在全部其它实体占位下执行有限单元平移下降；使用：私有全场压缩候选；边界：最多三轮和4096次真实位置检查，已跨单元连接的集合固定，取消或错误恢复全部坐标。
func (b *builder) compactDenseSpatialUnits(ctx context.Context, units []denseSpatialUnit, heightLimit float64) (err error) {
	if len(units) < 2 || len(b.all) == 0 {
		return nil
	}
	fillSpan, _ := ctx.Value(denseSpatialHolesKey{}).(float64)
	fillHoles := fillSpan > 0
	// 保存两端点而非逆向累加浮点位移，确保失败恢复与原布局逐值一致。
	type savedPosition struct {
		entity *placed
		a, b   vector
	}
	saved := []savedPosition{}
	owners, seen := map[*placed]int{}, map[*placed]bool{}
	for i, unit := range units {
		for _, entity := range unit.entities {
			if _, exists := owners[entity]; exists {
				return fmt.Errorf("平移压缩单元共享实体，不能独立移动")
			}
			owners[entity] = i
			if !seen[entity] {
				saved = append(saved, savedPosition{entity, entity.a, entity.b})
				seen[entity] = true
			}
		}
	}
	defer func() {
		if err != nil {
			for _, state := range saved {
				state.entity.a, state.entity.b = state.a, state.b
			}
		}
	}()
	frozen := make([]bool, len(units))
	if fillHoles {
		for i, unit := range units {
			// 先保持长街道、塔与共享附件原位，仅让独立非低轨短行填洞；避免为局部空洞重新挤压全部已可建核心。
			frozen[i] = !unit.bent && (unit.row == nil || referenceLowRecipe(unit.row.info, unit.row.recipe))
		}
	}
	for _, entity := range b.all {
		owner, owned := owners[entity]
		for _, linked := range []*placed{entity.input, entity.output} {
			other, otherOwned := owners[linked]
			if linked != nil && (owned != otherOwned || owned && owner != other) {
				if owned {
					frozen[owner] = true
				}
				if otherOwned {
					frozen[other] = true
				}
			}
		}
	}
	contours := make([]connectedBlockContour, len(units))
	for i, unit := range units {
		if frozen[i] {
			continue
		}
		turns := 0
		if unit.row != nil {
			turns = unit.row.quarterTurns
		}
		contours[i], err = b.denseSpatialContour(ctx, unit, turns)
		if err != nil {
			return err
		}
	}
	outside := []*placed{}
	for _, entity := range b.all {
		if _, owned := owners[entity]; !owned {
			outside = append(outside, entity)
		}
	}
	// 固定的跨单元实体按闭合指针集合建立只读轮廓，避免局部records误把外部连接判为悬空。
	fixedPorts := []denseStreetPort{}
	for i, unit := range units {
		if frozen[i] {
			outside = append(outside, unit.entities...)
			fixedPorts = append(fixedPorts, unit.ports...)
		}
	}
	fixedContour, err := b.denseSpatialContour(ctx, denseSpatialUnit{entities: outside, ports: fixedPorts}, 0)
	if err != nil {
		return err
	}
	for i, unit := range units {
		if frozen[i] && unit.row != nil {
			ports, portErr := b.denseSpatialContour(ctx, denseSpatialUnit{row: unit.row}, unit.row.quarterTurns)
			if portErr != nil {
				return portErr
			}
			fixedContour.shapes = append(fixedContour.shapes, ports.shapes...)
		}
	}
	originalLow, originalHigh := bounds(b.all)
	var holeField *denseEquipmentField
	if fillHoles {
		holeField, err = newDenseEquipmentField(b.all, originalLow, originalHigh)
		if err != nil {
			return err
		}
	}
	maximumY := originalHigh.y
	if b.denseLatitudeSpan > 0 {
		maximumY = math.Min(maximumY, heightLimit-1)
	}
	checks, moves := 0, 0
	for pass := 0; pass < 3 && checks < 4096; pass++ {
		changed := false
		for i, unit := range units {
			if checks >= 4096 {
				break
			}
			if err = b.checkGeneration("全场单元平移压缩"); err != nil {
				return err
			}
			if frozen[i] || len(unit.entities) == 0 {
				continue
			}
			members := map[*placed]bool{}
			for _, entity := range unit.entities {
				members[entity] = true
			}
			others, entities := []connectedBlockContour{fixedContour}, []*placed{}
			for j := range units {
				if j != i && !frozen[j] {
					others = append(others, contours[j])
				}
			}
			for _, entity := range b.all {
				if !members[entity] {
					entities = append(entities, entity)
				}
			}
			if len(entities) == 0 {
				continue
			}
			fixedLow, fixedHigh := bounds(entities)
			low, high := bounds(unit.entities)
			placed := map[*placed]bool{}
			for _, entity := range entities {
				placed[entity] = true
			}
			wires := b.denseSpatialUnitWires(unit, placed)
			for _, recycle := range b.machineRecycles {
				route := recycle.route
				if route == nil || route.start == nil || route.end == nil {
					continue
				}
				if members[route.start] && placed[route.end] {
					wires = append(wires, denseSpatialWire{route.start, route.end})
				}
				if members[route.end] && placed[route.start] {
					wires = append(wires, denseSpatialWire{route.end, route.start})
				}
			}
			globalLow, globalHigh := bounds(b.all)
			shape, area := denseCompactObjective(globalLow, globalHigh)
			current := denseCompactPosition{shape: shape, area: area, wire: denseSpatialWireCost(wires, 0, 0)}
			hole := denseEquipmentHole{}
			if fillHoles {
				hole = holeField.hole(nil, vector{})
				current.hole = hole.area
			}
			candidates := []denseCompactPosition{}
			deltas := denseCompactCandidates(contours[i], others, low, high, fixedLow, fixedHigh, wires)
			if fillHoles {
				// 最大空洞九点与真实边缘的少量双轴组合优先，仍共用原96次/单元和4096次总几何预算。
				aligned := denseHoleAlignments(hole, low, high)
				xEdges := denseCompactEdges(others, 0, 4)
				yEdges := denseCompactEdges(others, 2, 4)
				for _, x := range xEdges {
					for _, y := range yEdges {
						aligned = append(aligned, vector{x: math.Round((x-low.x)*4) / 4, y: math.Round((y-low.y)*4) / 4})
					}
				}
				deltas = append(aligned, deltas...)
			}
			for _, delta := range deltas {
				if low.x+delta.x < originalLow.x-1e-7 || low.y+delta.y < originalLow.y-1e-7 || high.x+delta.x > originalHigh.x+1e-7 || high.y+delta.y > maximumY+1e-7 {
					continue
				}
				lo := vector{x: math.Min(fixedLow.x, low.x+delta.x), y: math.Min(fixedLow.y, low.y+delta.y)}
				hi := vector{x: math.Max(fixedHigh.x, high.x+delta.x), y: math.Max(fixedHigh.y, high.y+delta.y)}
				shape, area := denseCompactObjective(lo, hi)
				candidate := denseCompactPosition{delta: delta, shape: shape, area: area, wire: denseSpatialWireCost(wires, delta.x, delta.y)}
				if fillHoles {
					if candidate.area <= current.area+1e-7 && candidate.shape <= current.shape+1e-7 && candidate.wire <= current.wire+math.Max(4, current.wire*.03) {
						candidates = append(candidates, candidate)
					}
				} else if denseCompactBetter(candidate, current) {
					candidates = append(candidates, candidate)
				}
			}
			if fillHoles {
				// 网格只对有限的不同平移评分，避免边缘轴笛卡尔积转化为无界设备栅格评估。
				scored, seen := make([]denseCompactPosition, 0, 96), map[vector]bool{}
				for _, candidate := range candidates {
					if err = b.checkGeneration("设备空矩形有限评分"); err != nil {
						return err
					}
					if seen[candidate.delta] {
						continue
					}
					seen[candidate.delta] = true
					if len(seen) > 96 {
						break
					}
					candidate.hole = holeField.hole(members, candidate.delta).area
					if candidate.hole <= current.hole+1e-7 && (candidate.hole < current.hole-1e-7 || denseCompactBetter(candidate, current)) {
						scored = append(scored, candidate)
					}
				}
				candidates = scored
			}
			sort.SliceStable(candidates, func(a, c int) bool {
				left, right := candidates[a], candidates[c]
				if left.shape != right.shape {
					return left.shape < right.shape
				}
				if left.area != right.area {
					return left.area < right.area
				}
				if fillHoles && left.hole != right.hole {
					return left.hole < right.hole
				}
				if left.wire != right.wire {
					return left.wire < right.wire
				}
				return math.Abs(left.delta.x)+math.Abs(left.delta.y) < math.Abs(right.delta.x)+math.Abs(right.delta.y)
			})
			packing := &connectedPacking{dense: b.adapter == nil || b.adapter.density == densityDense, cells: map[[3]int][]int{}}
			for _, contour := range others {
				registerDenseStationContour(packing, contour, 0, 0)
			}
			for number, candidate := range candidates {
				if number >= 96 || checks >= 4096 {
					break
				}
				checks++
				if !packing.fits(ctx, contours[i], candidate.delta.x, candidate.delta.y) || !b.denseCompactStationClear(unit, members, candidate.delta) {
					continue
				}
				shift(unit.entities, candidate.delta.x, candidate.delta.y, 0)
				turns := 0
				if unit.row != nil {
					turns = unit.row.quarterTurns
				}
				contours[i], err = b.denseSpatialContour(ctx, unit, turns)
				if err != nil {
					return err
				}
				changed, moves = true, moves+1
				break
			}
		}
		if !changed {
			break
		}
	}
	if ctx.Err() != nil {
		return previewContextError(ctx.Err())
	}
	if b.logistics != nil {
		message := fmt.Sprintf("装箱后按全部真实轮廓纯平移压缩：%d次有效移动、%d次位置检查；保留单元连接和朝向，包络形状、面积与真实流边距离均不增加", moves, checks)
		if fillHoles {
			message = fmt.Sprintf("设备空洞引导的纯平移：%d次有效移动、%d次真实位置检查；包络与最大设备空矩形不增加，每步受影响流边只允许至多3%%或4格的有限增长；不按新增带/臂奖励填充，完整结果仍比较实际带长", moves, checks)
		}
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-spatial-translation-compact", Status: "passed", Message: message})
	}
	return nil
}
