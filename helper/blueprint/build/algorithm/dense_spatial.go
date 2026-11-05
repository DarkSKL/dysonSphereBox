package build

// 文件职责：在同一占位场中搜索密铺位置，使用实际实体轮廓交错填充，取消产线矩形独占区和固定行列间隔。
// 使用：局部连接装配后、全局运输布线前；边界：只保留功能实体的刚性关系，最终仍执行球面、连通、节点合并与容量核验。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
	"sort"
)

// denseLayoutShapeObjective 职责：为参考图约3:1的合理矩形保留面积收益，继续惩罚更细长的布局；使用：密铺最终择优；边界：普通模式沿用原2:1软目标，不把空白率当密度。
func denseLayoutShapeObjective(metrics LayoutMetrics) float64 {
	return metrics.EnvelopeArea * math.Max(1, metrics.EnvelopeAspectRatio/3)
}

// betterDenseCandidate 职责：按带长条惩罚的面积和真实线路代价比较完整方案；使用：各超密度模式；边界：允许参考图的合理长矩形，不以2:1一票淘汰更紧凑布局，不奖励带填空。
func betterDenseCandidate(a, c LayoutMetrics) bool {
	if first, second := denseLayoutShapeObjective(a), denseLayoutShapeObjective(c); math.Abs(first-second) > 1e-6 {
		return first < second
	}
	if a.EnvelopeArea != c.EnvelopeArea {
		return a.EnvelopeArea < c.EnvelopeArea
	}
	if math.Abs(a.TotalBeltLengthMeters-c.TotalBeltLengthMeters) > 1e-6 {
		return a.TotalBeltLengthMeters < c.TotalBeltLengthMeters
	}
	if a.BeltCount != c.BeltCount {
		return a.BeltCount < c.BeltCount
	}
	if a.InternalBeltLength != c.InternalBeltLength {
		return a.InternalBeltLength < c.InternalBeltLength
	}
	if a.CrossLayerConnections != c.CrossLayerConnections {
		return a.CrossLayerConnections < c.CrossLayerConnections
	}
	return a.EnvelopeAspectRatio < c.EnvelopeAspectRatio
}

// denseSpatialUnit 职责：保存有局部连接约束的实体集合；使用：全场位置搜索；边界：集合外包矩形不是独占区，内部空隙可以容纳其它集合。
type denseSpatialUnit struct {
	// entities 职责：保留实际机器、带和臂；使用：整体姿态变换；边界：不包括尚未创建的全局路线。
	entities []*placed
	// row 职责：保留逐物料端口及份额；使用：邻近关联产线与旋转出口；边界：闭合设施集合可以为空。
	row *transportRow
	// rows/ports 职责：保存连续街道的多个容量组与真实朝向；使用：复合集合整体落位；边界：每组保留独立物流身份，集合内部不重新拆块。
	rows  []*transportRow
	ports []denseStreetPort
}

// denseSpatialWire 职责：保存当前集合对已落位对象的一条真实连接；使用：候选位置的距离代价；边界：端点指针随集合旋转更新，不冻结旧坐标。
type denseSpatialWire struct {
	moving, fixed *placed
}

// transportRows 职责：统一取得单行或复合街道的容量组；使用：位置代价与累积旋转；边界：不复制transportRow实体。
func (unit denseSpatialUnit) transportRows() []*transportRow {
	if len(unit.rows) > 0 {
		return unit.rows
	}
	if unit.row != nil {
		return []*transportRow{unit.row}
	}
	return nil
}

// denseSpatialUnitWires 职责：一次筛出当前集合与已放置对象的真实连接；使用：四向位置搜索前；边界：只缓存身份，保留原遍历顺序和重复权重，不计未落位对象或集合内部路线。
func (b *builder) denseSpatialUnitWires(unit denseSpatialUnit, placedEntities map[*placed]bool) []denseSpatialWire {
	members := make(map[*placed]bool, len(unit.entities))
	for _, entity := range unit.entities {
		members[entity] = true
	}
	wires := []denseSpatialWire{}
	for _, routes := range [][]*transportRoute{b.transportRoutes, b.denseStreetRoutes, b.denseMaterialNetworkRoutes()} {
		for _, route := range routes {
			if route.start == nil || route.end == nil {
				continue
			}
			if members[route.start] && placedEntities[route.end] {
				wires = append(wires, denseSpatialWire{route.start, route.end})
			}
			if members[route.end] && placedEntities[route.start] {
				wires = append(wires, denseSpatialWire{route.end, route.start})
			}
		}
	}
	return wires
}

// denseSpatialWireCost 职责：按当前姿态汇总已筛出的真实连接距离；使用：整数落位和四分之一格微调；边界：不替代实际布线，不缓存旋转前坐标。
func denseSpatialWireCost(wires []denseSpatialWire, dx, dy float64) float64 {
	cost := 0.
	for _, wire := range wires {
		cost += math.Abs(wire.moving.a.x+dx-wire.fixed.a.x) + math.Abs(wire.moving.a.y+dy-wire.fixed.a.y)
	}
	return cost
}

// denseSpatialConflict 职责：按单次密铺规则检查实际占位；使用：全场空间索引；边界：带段相交不直接禁止，但节点保留0.35米间距，机器及分拣器规则不缩小。
func denseSpatialConflict(a, c *connectedPackingShape) bool {
	if a.kind == "dense-monitor" || c.kind == "dense-monitor" {
		monitor, arm := a, c
		if monitor.kind != "dense-monitor" {
			monitor, arm = c, a
		}
		return arm.kind == "dense-monitor-arm" && monitor.box.SegmentDistanceSquared(arm.span.ends[0], arm.span.ends[1]) < .55*.55
	}
	if a.kind == "dense-monitor-arm" || c.kind == "dense-monitor-arm" {
		return false
	}
	if a.kind == "dense-node" || c.kind == "dense-node" {
		if a.kind != "dense-node" || c.kind != "dense-node" {
			return false
		}
		delta := a.span.ends[0].Sub(c.span.ends[0])
		return delta.Dot(delta) < .35*.35
	}
	if a.kind == "belt" && c.kind == "belt" {
		return false
	}
	return connectedShapesConflict(a, c)
}

// denseSpatialContour 职责：提取当前姿态的真实轮廓并仅预留端口接线柱；使用：四向候选；边界：不为整条产线保留走廊或独占外包矩形。
func (b *builder) denseSpatialContour(ctx context.Context, unit denseSpatialUnit, turns int) (connectedBlockContour, error) {
	contour, err := b.connectedBlockShape(ctx, &transportRow{}, unit.entities)
	if err != nil {
		return contour, err
	}
	ordinarySpacing := b.adapter != nil && b.adapter.density == densityStandard
	world := func(p vector) placement.Vector {
		return placement.Vector{p.x * planarGridLength, p.z*1.3333333 + .4, p.y * planarGridLength}
	}
	if !ordinarySpacing {
		for _, entity := range unit.entities {
			if !surfaceIsBelt(entity.record.ItemID) {
				continue
			}
			point := world(entity.a)
			contour.shapes = append(contour.shapes, connectedPackingShape{kind: "dense-node", span: surfaceBeltSpan{ends: [2]placement.Vector{point, point}}, low: point.Sub(placement.Vector{.35, .35, .35}), high: point.Add(placement.Vector{.35, .35, .35})})
		}
	}
	if !ordinarySpacing && b.setting.EnableFlowMonitor {
		catalog, err := placement.LoadCatalog()
		if err != nil {
			return contour, err
		}
		encoding := newPlanarRuleEncoding()
		for _, entity := range unit.entities {
			if entity.record.ItemID == 2030 {
				shapes, err := catalog.Shapes(entity.record.ItemID, entity.record.ModelIndex)
				if err != nil {
					return contour, err
				}
				origin := world(entity.a)
				origin[1] -= .2
				for _, shape := range shapes {
					box, err := encoding.colliderBox(vector{origin[0], origin[1], origin[2]}, entity.record.Yaw, shape)
					if err != nil {
						return contour, err
					}
					contour.shapes = append(contour.shapes, connectedBoxShape("dense-monitor", box))
				}
			} else if entity.record.ItemID >= 2011 && entity.record.ItemID <= 2014 {
				first, second := world(entity.a), world(entity.b)
				first[1] -= .2
				second[1] -= .2
				shape := connectedBeltShape(first, second)
				shape.kind = "dense-monitor-arm"
				shape.low = shape.low.Sub(placement.Vector{.32, .32, .32})
				shape.high = shape.high.Add(placement.Vector{.32, .32, .32})
				contour.shapes = append(contour.shapes, shape)
			}
		}
	}
	if unit.row != nil {
		ports := unit.row.allInputEndpoints()
		for _, route := range unit.row.outputs {
			if route.to != nil {
				ports = append(ports, route.start)
			}
		}
		for _, recycle := range unit.row.recycles {
			ports = append(ports, recycle.route.start, recycle.route.end)
		}
		offset := denseRotateVector(vector{x: -1}, turns)
		for _, port := range ports {
			point := world(vector{port.a.x + offset.x, port.a.y + offset.y, 0})
			box := placement.Box{Center: point.Add(placement.Vector{0, transportMaxHeight * 1.3333333 / 2, 0}), HalfSize: placement.Vector{.31, transportMaxHeight*1.3333333/2 + .3, .31}, Axes: [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
			shape := connectedBoxShape("port", box)
			shape.gridPort = true
			contour.shapes = append(contour.shapes, shape)
		}
	}
	for _, port := range unit.ports {
		direction := port.outward()
		// 仅保留真实向外半格，跨高度路径由portal及普通带段避碰共同判定。
		a := world(port.head.a)
		c := world(vector{port.head.a.x + direction.x*.5, port.head.a.y + direction.y*.5, port.head.a.z})
		contour.shapes = append(contour.shapes, connectedBeltShape(a, c))
	}
	priority := func(kind string) int {
		switch kind {
		case "static":
			return 0
		case "inserter":
			return 1
		case "port":
			return 2
		case "dense-node":
			return 3
		default:
			return 4
		}
	}
	sort.SliceStable(contour.shapes, func(i, j int) bool { return priority(contour.shapes[i].kind) < priority(contour.shapes[j].kind) })
	return contour, nil
}

// placeDenseSpatialUnits 职责：全场搜索实体交错位置和四向姿态；使用：超密度装配；边界：有界搜索不宣称数学全局最优，不以普通区块布局作成功退路。
func (b *builder) placeDenseSpatialUnits(ctx context.Context, units []denseSpatialUnit, heightLimit float64) error {
	heightLimit = denseLatitudeHeightLimit(ctx, heightLimit)
	widthLimit := heightLimit
	strictHeight := b.denseLatitudeSpan > 0
	if strictHeight {
		heightLimit = math.Min(heightLimit, b.denseLatitudeSpan)
		area, maximumWidth := 0., 0.
		for _, unit := range units {
			low, high := bounds(unit.entities)
			w, h := math.Ceil(high.x)-math.Floor(low.x)+1, math.Ceil(high.y)-math.Floor(low.y)+1
			area += w * h
			maximumWidth = math.Max(maximumWidth, math.Max(w, h))
		}
		// 经向只给搜索预算，实际候选仍按真实包络和线路评分；不能让纬向预算同时封死合法长矩形。
		widthLimit = math.Max(widthLimit, math.Max(maximumWidth, math.Ceil(area/math.Max(1, heightLimit)*1.25)))
	}
	ordered := append([]denseSpatialUnit(nil), units...)
	if b.denseSpatialOrder%2 == 0 {
		sort.SliceStable(ordered, func(i, j int) bool {
			a, c := bounds(ordered[i].entities)
			d, e := bounds(ordered[j].entities)
			return (c.x-a.x)*(c.y-a.y) > (e.x-d.x)*(e.y-d.y)
		})
	}
	packing := &connectedPacking{dense: b.adapter == nil || b.adapter.density == densityDense, cells: map[[3]int][]int{}}
	placedEntities := map[*placed]bool{}
	for number, unit := range ordered {
		wires := b.denseSpatialUnitWires(unit, placedEntities)
		bestArea, bestWire, bestAspect := math.Inf(1), math.Inf(1), math.Inf(1)
		bestScore := math.Inf(1)
		bestX, bestY, bestTurns := 0., 0., -1
		bestHigh := vector{}
		var bestContour connectedBlockContour
		for turns := 0; turns < 4; turns++ {
			contour, err := b.denseSpatialContour(ctx, unit, turns)
			if err != nil {
				rotatePlacedQuarter(unit.entities, 4-turns)
				return err
			}
			low, high := bounds(unit.entities)
			width, height := math.Ceil(high.x)-math.Floor(low.x)+1, math.Ceil(high.y)-math.Floor(low.y)+1
			if strictHeight && height > heightLimit+1e-6 {
				rotatePlacedQuarter(unit.entities, 1)
				continue
			}
			maxX, maxY := int(math.Ceil(packing.high.x)+1), max(0, int(math.Max(heightLimit, height)-height))
			maxWidth := math.Max(widthLimit, math.Max(packing.high.x, width))
			if !packing.ready {
				maxX, maxY = 0, 0
			}
			attempts := 0
			for y := 0; y <= maxY && attempts < 8192; y++ {
				for x := 0; x <= maxX && attempts < 8192; x++ {
					attempts++
					if err := b.checkGeneration("全场实体密铺位置搜索"); err != nil {
						rotatePlacedQuarter(unit.entities, 4-turns)
						return err
					}
					dx, dy := float64(x)-math.Floor(low.x), float64(y)-math.Floor(low.y)
					xHigh, yHigh := math.Max(packing.high.x, float64(x)+width), math.Max(packing.high.y, float64(y)+height)
					if xHigh > maxWidth {
						continue
					}
					area := xHigh * yHigh
					wire := denseSpatialWireCost(wires, dx, dy)
					aspect := math.Max(xHigh, yHigh) / math.Max(1, math.Min(xHigh, yHigh))
					score := area * math.Max(1, aspect/1.8)
					if b.denseSpatialOrder >= 2 {
						score += wire * 2
					}
					if score > bestScore || score == bestScore && (area > bestArea || area == bestArea && (wire > bestWire || wire == bestWire && aspect >= bestAspect)) {
						continue
					}
					if !packing.fits(ctx, contour, dx, dy) {
						continue
					}
					bestArea, bestWire, bestAspect = area, wire, aspect
					bestScore = score
					bestX, bestY, bestTurns = dx, dy, turns
					bestHigh, bestContour = vector{xHigh, yHigh, 0}, contour
				}
			}
			rotatePlacedQuarter(unit.entities, 1)
		}
		if bestTurns < 0 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "全场密铺在本次空间预算内未找到可接线的实体位置"}
		}
		if b.denseRefine && packing.ready {
			rotatePlacedQuarter(unit.entities, bestTurns)
			low, high := bounds(unit.entities)
			baseX, baseY := bestX, bestY
			for iy := -4; iy <= 4; iy++ {
				for ix := -4; ix <= 4; ix++ {
					if err := b.checkGeneration("密铺四分之一格微调"); err != nil {
						return err
					}
					dx, dy := baseX+float64(ix)/4, baseY+float64(iy)/4
					if low.x+dx < 0 || low.y+dy < 0 {
						continue
					}
					xHigh, yHigh := math.Max(packing.high.x, math.Ceil(high.x)+dx+1), math.Max(packing.high.y, math.Ceil(high.y)+dy+1)
					if yHigh > math.Max(heightLimit, packing.high.y) || xHigh > math.Max(widthLimit, packing.high.x) {
						continue
					}
					area := xHigh * yHigh
					aspect := math.Max(xHigh, yHigh) / math.Max(1, math.Min(xHigh, yHigh))
					wire := denseSpatialWireCost(wires, dx, dy)
					score := area * math.Max(1, aspect/1.8)
					if b.denseSpatialOrder >= 2 {
						score += wire * 2
					}
					if score > bestScore || score == bestScore && (area > bestArea || area == bestArea && wire >= bestWire) {
						continue
					}
					if !packing.fits(ctx, bestContour, dx, dy) {
						continue
					}
					bestArea, bestWire, bestAspect, bestScore = area, wire, aspect, score
					bestX, bestY, bestHigh = dx, dy, vector{xHigh, yHigh, 0}
				}
			}
			rotatePlacedQuarter(unit.entities, 4-bestTurns)
		}
		rotatePlacedQuarter(unit.entities, bestTurns)
		shift(unit.entities, bestX, bestY, 0)
		for _, entity := range unit.entities {
			placedEntities[entity] = true
		}
		for _, row := range unit.transportRows() {
			row.quarterTurns = (row.quarterTurns + bestTurns) % 4
		}
		for _, source := range bestContour.shapes {
			shape := shiftedConnectedShape(source, bestX, bestY)
			index := len(packing.shapes)
			packing.shapes = append(packing.shapes, shape)
			connectedShapeCells(shape, func(cell [3]int) bool { packing.cells[cell] = append(packing.cells[cell], index); return true })
		}
		packing.high, packing.ready = bestHigh, true
		reportProgress(ctx, "dense-placement", "正在按真实占位交错排布生产设备", number+1, len(ordered))
	}
	if b.logistics != nil {
		b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "dense-global-occupancy", Status: "passed", Message: "使用全场实体轮廓与四向姿态搜索，取消产线矩形独占和固定行列间隔；局部空隙可交错利用，全部跨线运输仍需统一布线与最终检查"})
	}
	return nil
}
