package build

// 文件职责：在物流塔真实间距约束下，按实际实体占位填入跨产线机器阵列。
// 使用：密铺塔统一布线之前；边界：不分配独占矩形，不减小任何建造盒或塔间距。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"math"
	"sort"
)

// denseStationOuter 职责：提供各塔口最短的整数地面接驳点；使用：密铺路径与预留；边界：最后两节点沿用已经核对的原版塔口微调。
func denseStationOuter(center vector, slot int) vector {
	point := slotOffset(slot)[len(slotOffset(slot))-1]
	switch {
	case slot < 3:
		point.y = 4
	case slot < 6:
		point.x = -4
	case slot < 9:
		point.y = -4
	default:
		point.x = 4
	}
	point.x += center.x
	point.y += center.y
	return point
}

// denseStationInner 职责：保留原版真实塔口最后两节带的精确姿态；使用：短径向接驳；边界：只去掉塔外阶梯，不变更端口槽位与最终小数坐标。
func denseStationInner(center vector, slot int) []vector {
	points := stationInner(center, slot)
	return append([]vector(nil), points[len(points)-2:]...)
}

// denseStationTowerContour 职责：登记塔的实体及十二口实际短接驳净空；使用：机器空隙填充；边界：不把塔间最小间距变成禁止其它建筑的圆形空地。
func (b *builder) denseStationTowerContour(ctx context.Context, tower *placed) (connectedBlockContour, error) {
	contour, err := b.connectedBlockShape(ctx, &transportRow{}, []*placed{tower})
	if err != nil {
		return contour, err
	}
	for slot := 0; slot < 12; slot++ {
		for _, point := range append(denseStationInner(tower.a, slot), denseStationOuter(tower.a, slot)) {
			world := placement.Vector{point.x * planarGridLength, .2, point.y * planarGridLength}
			box := placement.Box{Center: world.Add(placement.Vector{0, transportMaxHeight * 1.3333333 / 2, 0}), HalfSize: placement.Vector{.31, transportMaxHeight*1.3333333/2 + .3, .31}, Axes: [3]placement.Vector{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
			contour.shapes = append(contour.shapes, connectedBoxShape("port", box))
		}
	}
	return contour, nil
}

// registerDenseStationContour 职责：提交获选几何到共同占位索引；使用：固定塔与机器行；边界：候选失败时不提交。
func registerDenseStationContour(index *connectedPacking, contour connectedBlockContour, dx, dy float64) {
	for _, original := range contour.shapes {
		shape := shiftedConnectedShape(original, dx, dy)
		number := len(index.shapes)
		index.shapes = append(index.shapes, shape)
		connectedShapeCells(shape, func(cell [3]int) bool { index.cells[cell] = append(index.cells[cell], number); return true })
	}
}

// placeDenseStationField 职责：在紧凑塔格点间排布并旋转真实机器轮廓；使用：密铺塔空间阶段；边界：十二塔口仅各留一条必要接线柱，没有配方庭院。
func (b *builder) placeDenseStationField(ctx context.Context, rows []*denseStationRow, routes []denseStationRoute, towers []denseStationTower, config contract.StationConfig, variant int) error {
	if len(rows) == 0 || len(towers) == 0 {
		return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "密铺塔缺少机器或真实库存"}
	}
	for _, row := range rows {
		if row == nil || row.line == nil || len(row.line.machines) == 0 || len(row.entities) == 0 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "密铺塔空间规划缺少真实机器行及实体，无法判定必要朝向"}
		}
	}
	packing := &connectedPacking{dense: true, cells: map[[3]int][]int{}}
	pitch := math.Ceil(math.Sqrt(float64(surfaceStationDistanceLimit(int16(config.StationID), int16(config.StationID))))/planarGridLength) + 1
	if variant >= 2 {
		for _, row := range rows {
			if row.line.info.MachineType == 4 && !row.line.referenceLowTracks && len(row.line.machines) > 1 {
				// 南北长轴候选可能覆盖更宽纬度范围；多一个真实格距作为独立塔距候选，最终仍核对球面15/29米阈值。
				pitch++
				break
			}
		}
	}
	latitudeHeight := denseLatitudeHeightLimit(ctx, math.Inf(1))
	rowPitch := pitch
	if variant >= 2 {
		rowPitch = math.Ceil(pitch * math.Sqrt(3) / 2)
	}
	columns := max(1, int(math.Ceil(math.Sqrt(float64(len(towers))))))
	if variant%2 == 1 {
		columns = max(1, int(math.Floor(math.Sqrt(float64(len(towers))))))
	}
	if !math.IsInf(latitudeHeight, 1) {
		if latitudeHeight < 10 {
			return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "所选纬带无法容纳物流塔的必要建造与带口范围"}
		}
		// 只改变塔的行列安排，保持真实塔间距；窄纬带优先横向分配，不能先堆高再默默换区。
		maximumRows := max(1, int(math.Floor((latitudeHeight-10)/rowPitch))+1)
		columns = max(columns, (len(towers)+maximumRows-1)/maximumRows)
	}
	for i := range towers {
		x, y := float64(i%columns)*pitch, float64(i/columns)*pitch
		if variant >= 2 {
			y = float64(i/columns) * rowPitch
			if (i/columns)%2 == 1 {
				x += math.Floor(pitch / 2)
			}
		}
		b.owner = -1
		towers[i].tower = b.createStation(vector{x + 5, y + 5, 0}, towers[i].items, config)
		contour, err := b.denseStationTowerContour(ctx, towers[i].tower)
		if err != nil {
			return err
		}
		registerDenseStationContour(packing, contour, 0, 0)
		packing.high.x = math.Max(packing.high.x, x+10)
		packing.high.y = math.Max(packing.high.y, y+10)
	}
	packing.ready = true
	units := denseStationPlacementUnits(rows)
	area := 0.
	for _, unit := range units {
		low, high := bounds(unit.entities)
		area += (high.x - low.x + 1) * (high.y - low.y + 1)
	}
	extent := math.Ceil(math.Sqrt(area+float64(len(towers))*80) * (1.1 + float64(variant%2)*.2))
	widthLimit, heightLimit := math.Max(packing.high.x+5, extent*1.2), math.Max(packing.high.y+5, extent)
	heightLimit = denseLatitudeHeightLimit(ctx, heightLimit)
	ordered := append([]*denseStationUnit(nil), units...)
	if variant%2 == 0 {
		sort.SliceStable(ordered, func(i, j int) bool {
			lo, hi := bounds(ordered[i].entities)
			a, c := bounds(ordered[j].entities)
			return (hi.x-lo.x)*(hi.y-lo.y) > (c.x-a.x)*(c.y-a.y)
		})
	}
	settled := map[*placed]bool{}
	for rowNumber, unit := range ordered {
		members := map[*placed]bool{}
		for _, entity := range unit.entities {
			members[entity] = true
		}
		ports := map[int]*placed{}
		unitRoutes := []int{}
		for index, route := range routes {
			if members[route.head] {
				ports[len(ports)] = route.head
				unitRoutes = append(unitRoutes, index)
			}
		}
		neighbors := [][2]*placed{}
		for _, link := range unit.links {
			if members[link.start] && !members[link.end] && settled[link.end] {
				neighbors = append(neighbors, [2]*placed{link.start, link.end})
			} else if members[link.end] && !members[link.start] && settled[link.start] {
				neighbors = append(neighbors, [2]*placed{link.end, link.start})
			}
		}
		spatialUnit := denseSpatialUnit{entities: unit.entities, ports: unit.ports}
		if len(unit.ports) == 0 {
			spatialUnit.row = &transportRow{inputs: ports}
		}
		bestScore, bestArea := math.Inf(1), math.Inf(1)
		bestTurns := -1
		dxBest, dyBest := 0., 0.
		var chosen connectedBlockContour
		var chosenHigh vector
		meridian := false
		if variant >= 2 {
			for _, row := range unit.rows {
				meridian = meridian || row.line.info.MachineType == 4 && !row.line.referenceLowTracks && len(row.line.machines) > 1
			}
		}
		for turns := 0; turns < 4; turns++ {
			// 非参考化工长轴在纬向收缩后可能相交；保留独立南北走向候选，仍以最终球面建造盒判定。
			if meridian && turns%2 == 0 {
				rotatePlacedQuarter(unit.entities, 1)
				continue
			}
			contour, err := b.denseSpatialContour(ctx, spatialUnit, turns)
			if err != nil {
				rotatePlacedQuarter(unit.entities, 4-turns)
				return err
			}
			low, high := bounds(unit.entities)
			width, height := math.Ceil(high.x)-math.Floor(low.x)+1, math.Ceil(high.y)-math.Floor(low.y)+1
			if !math.IsInf(latitudeHeight, 1) && height > heightLimit {
				rotatePlacedQuarter(unit.entities, 1)
				continue
			}
			xLimit, yLimit := int(math.Max(widthLimit, width)-width), int(math.Max(heightLimit, height)-height)
			attempts := 0
			for y := 0; y <= yLimit && attempts < 16384; y++ {
				for x := 0; x <= xLimit && attempts < 16384; x++ {
					attempts++
					if ctx.Err() != nil {
						rotatePlacedQuarter(unit.entities, 4-turns)
						return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "密铺塔全场位置搜索已取消"}
					}
					dx, dy := float64(x)-math.Floor(low.x), float64(y)-math.Floor(low.y)
					mx, my := math.Max(packing.high.x, float64(x)+width), math.Max(packing.high.y, float64(y)+height)
					area := mx * my
					aspect := math.Max(mx, my) / math.Max(1, math.Min(mx, my))
					wire := 0.
					for _, routeIndex := range unitRoutes {
						route := routes[routeIndex]
						tower := towers[route.tower].tower
						wire += math.Abs(route.head.a.x+dx-tower.a.x) + math.Abs(route.head.a.y+dy-tower.a.y)
					}
					for _, neighbor := range neighbors {
						wire += math.Abs(neighbor[0].a.x+dx-neighbor[1].a.x) + math.Abs(neighbor[0].a.y+dy-neighbor[1].a.y)
					}
					score := area*math.Max(1, aspect/1.8) + wire*2
					if score > bestScore || score == bestScore && area >= bestArea || !packing.fits(ctx, contour, dx, dy) {
						continue
					}
					bestScore, bestArea, bestTurns, dxBest, dyBest, chosen, chosenHigh = score, area, turns, dx, dy, contour, vector{mx, my, 0}
				}
			}
			rotatePlacedQuarter(unit.entities, 1)
		}
		if bestTurns < 0 {
			return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "密铺塔全场矩形内没有满足真实实体与端口净空的位置"}
		}
		rotatePlacedQuarter(unit.entities, bestTurns)
		shift(unit.entities, dxBest, dyBest, 0)
		for _, row := range unit.rows {
			row.turns = (row.turns + bestTurns) % 4
		}
		for _, entity := range unit.entities {
			settled[entity] = true
		}
		registerDenseStationContour(packing, chosen, dxBest, dyBest)
		packing.high = chosenHigh
		reportProgress(ctx, "dense-stations", "正在利用塔间真实空隙排布机器", rowNumber+1, len(ordered))
	}
	return nil
}

// denseStationPlacementUnits 职责：把街道关联的容量组收拢为同一刚性集合；使用：塔场位置搜索；边界：原有单行保持独立，不增加矩形排他范围。
func denseStationPlacementUnits(rows []*denseStationRow) []*denseStationUnit {
	units := []*denseStationUnit{}
	seen := map[*denseStationUnit]bool{}
	for _, row := range rows {
		unit := row.unit
		if unit == nil {
			units = append(units, &denseStationUnit{entities: row.entities, rows: []*denseStationRow{row}})
		} else if !seen[unit] {
			units = append(units, unit)
			units = append(units, unit.accessories...)
			seen[unit] = true
		}
	}
	return units
}
