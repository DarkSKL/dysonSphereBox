package build

// 文件职责：在真实极区球面中消除局部单元之间的交叠与被封闭带口。
// 使用：单元格距补偿后、布线前；边界：仅在所选极区内调整完整单元的位置与格距，不逐行剪切实体，仍须完整终检。

import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"context"
	"fmt"
	"math"
	"sort"
)

// polarUnitOccupancy 职责：保存单元的原生静态盒、带节点与分拣器端点；使用：经向位置搜索；边界：不把包络矩形当作独占区域。
type polarUnitOccupancy struct {
	unit  denseSpatialUnit
	boxes []placement.Box
	// selections标记boxes中必须与真实臂盒互斥的分流器选择体积。
	selections map[int]bool
	nodes      []surfaceBeltNodeProbe
	// belts 职责：保存普通单元内部的完整带段；使用：跨单元装箱；边界：密铺仍沿用节点规则。
	belts []surfaceBeltSpan
	// selectionBelts只参与分流器选择体积检查，不把密铺带段改成普通带间规则。
	selectionBelts []surfaceBeltSpan
	arms           []surfaceBeltSpan
	// armBoxes/verifiedArms 职责：保存完整单元经正式槽位修正后的动态盒；使用：细化阶段替代双端点粗略胶囊；边界：待接线单元仍沿用粗筛。
	armBoxes     []placement.Box
	verifiedArms bool
	stations     []polarStationOccupancy
	low, high    vector
}

// polarStationOccupancy 职责：保存物流塔或电力节点的世界中心及原型；使用：跨单元设施间距；边界：两类设施分别按真实规则检查，不能只检查模型盒。
type polarStationOccupancy struct {
	point placement.Vector
	item  int16
}

// rotatePolarPoint 职责：绕行星轴平移经度；使用：单元刚性搜索；边界：不会旋转机器相对本地经纬线的Yaw。
func rotatePolarPoint(p placement.Vector, s, c float64) placement.Vector {
	return placement.Vector{c*p[0] - s*p[2], p[1], s*p[0] + c*p[2]}
}

// repairPolarUnitOccupancy 职责：以球面真实占位修复平面候选在高纬变换后的相邻冲突；使用：极区原生布线入口；边界：按单元数分配有界重排预算，服从总截止时间，不忽略实际实体。
func (b *builder) repairPolarUnitOccupancy(ctx context.Context, units []denseSpatialUnit) error {
	err := b.repairPolarUnitGeometry(ctx, units, false)
	if err == nil || ctx.Err() != nil || !denseNativeGeometryFailure(err) {
		return err
	}
	// 保留既有装箱及完整纬向修复序列，失败后才使用与最终出口一致的节点球及缩短胶囊。
	// 直接替换预筛选会改变后续贪心选择，可能增加原本已可用工厂的塔数、占地或绕线。
	return b.repairPolarUnitGeometry(ctx, units, true)
}

// repairPolarUnitGeometry 职责：按同一节点策略完成有限装箱与纬向修复；使用：既有策略优先，全部失败后细化；边界：服从总时限，不用失败的部分装箱代替完整结果。
func (b *builder) repairPolarUnitGeometry(ctx context.Context, units []denseSpatialUnit, preciseNodes bool) error {
	visited := map[*placed]map[int]bool{}
	limit := max(128, min(2048, len(units)*8))
	for attempt := 0; attempt < limit; attempt++ {
		blocked, err := b.packPolarUnitGeometry(ctx, units, preciseNodes)
		if err == nil {
			length := len(b.all)
			if err := b.resamplePolarLocalBelts(ctx, units); err != nil {
				return err
			}
			if len(b.all) == length {
				return nil
			}
			continue
		}
		if len(units) >= 64 || b.polarFrame.compactLongitude {
			return err
		}
		if blocked == nil || len(blocked.entities) == 0 {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		low, high := bounds(blocked.entities)
		key := blocked.entities[0]
		if visited[key] == nil {
			visited[key] = map[int]bool{}
		}
		visited[key][int(math.Round(low.y*4))] = true
		dy, found := 0., false
		for distance := 1; distance < b.polarFrame.height && !found; distance++ {
			for _, direction := range []float64{-math.Copysign(1, float64(b.polarFrame.start)), math.Copysign(1, float64(b.polarFrame.start))} {
				candidate := float64(distance) * direction
				if low.y+candidate >= 0 && high.y+candidate <= float64(b.polarFrame.height-1) && !visited[key][int(math.Round((low.y+candidate)*4))] {
					dy, found = candidate, true
					break
				}
			}
		}
		if !found {
			return err
		}
		oldScale := 0.
		for _, p := range blocked.entities {
			if p.nativeUnitScale > 0 {
				oldScale = p.nativeUnitScale
				break
			}
		}
		if oldScale <= 0 {
			return err
		}
		movedLow, movedHigh := low, high
		movedLow.y += dy
		movedHigh.y += dy
		scale := b.polarFrame.unitScale(*blocked, movedLow, movedHigh)
		ratio, center := scale/oldScale, (low.x+high.x)/2
		for _, p := range blocked.entities {
			p.a.x = center + (p.a.x-center)*ratio
			p.b.x = center + (p.b.x-center)*ratio
			p.a.y += dy
			p.b.y += dy
			p.densityCargo.relative.x *= ratio
			p.nativeUnitScale = scale
		}
	}
	return &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "极区局部单元在本次球面位置搜索内未全部落位"}
}

// packPolarUnitGeometry 职责：执行指定节点预筛选策略的球面装箱；使用：既有路径优先、失败后细化节点形状；边界：两种策略均保留静态盒、塔距、分拣器预筛选和最终完整规则。
func (b *builder) packPolarUnitGeometry(ctx context.Context, units []denseSpatialUnit, preciseNodes bool) (*denseSpatialUnit, error) {
	longitudeLimit := b.polarFrame.longitudeLimit()
	quarterAngle := float64(b.polarFrame.longitudeStep) / 4
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	encoding := &surfaceEncoding{}
	strictBelts := b.policy != nil && b.policy.density == densityStandard
	hasSelections := false
	for _, entity := range b.all {
		hasSelections = hasSelections || entity.record.ItemID == 2020
	}
	previous := map[*placed]*placed{}
	if preciseNodes {
		for _, entity := range b.all {
			if surfaceIsBelt(entity.record.ItemID) && entity.output != nil && surfaceIsBelt(entity.output.record.ItemID) {
				previous[entity.output] = entity
			}
		}
	}
	// 大厂在每个单元内完整搜索纬度，避免已落位单元被反复移动而耗尽重排预算。
	flexibleLatitude := len(units) >= 64 || b.polarFrame.compactLongitude
	zone, _ := ctx.Value(densityLatitudeKey{}).(string)
	selection, _ := densityLatitudeSelectionFor(zone)
	// 大厂先复用已经通过平面全场装箱的完整单元排布，并以真实球面几何复核；局部经向避让失败才完整扫描经纬。
	// 均衡原生方案也须保留这个起点，不能无条件丢弃第一次装箱结果，重复从零压缩整个工厂。
	seedFirst := flexibleLatitude && len(units) >= 64 && b.policy != nil && b.polarFrame.position != ""
	makePart := func(unit denseSpatialUnit, dynamic bool) (polarUnitOccupancy, error) {
		part := polarUnitOccupancy{unit: unit}
		part.low, part.high = bounds(unit.entities)
		members := make(map[*placed]bool, len(unit.entities))
		if preciseNodes || strictBelts || hasSelections {
			for _, entity := range unit.entities {
				members[entity] = true
			}
		}
		if strictBelts || hasSelections {
			for _, p := range unit.entities {
				if !surfaceIsBelt(p.record.ItemID) {
					continue
				}
				next := p
				if p.output != nil && members[p.output] && surfaceIsBelt(p.output.record.ItemID) {
					next = p.output
				}
				span := surfaceBeltSpan{ends: [2]placement.Vector{b.polarFrame.world(p.a, .2), b.polarFrame.world(next.a, .2)}}
				if strictBelts {
					part.belts = append(part.belts, span)
				}
				if hasSelections {
					part.selectionBelts = append(part.selectionBelts, span)
				}
			}
		}
		for _, p := range unit.entities {
			if surfaceIsBelt(p.record.ItemID) {
				if !preciseNodes {
					part.nodes = append(part.nodes, surfaceBeltNodeProbe{center: b.polarFrame.world(p.a, .2)})
					continue
				}
				point := b.polarFrame.world(p.a, 0)
				before, after := point, point
				input, output := previous[p], p.output
				// 只使用同一刚性单元内已存在的邻居；跨单元路线尚未装配时不推测胶囊方向。
				through := input != nil && output != nil && members[input] && members[output] && surfaceIsBelt(output.record.ItemID)
				if through {
					before, after = b.polarFrame.world(input.a, 0), b.polarFrame.world(output.a, 0)
				}
				part.nodes = append(part.nodes, encoding.beltNodeProbe(point, before, after, through))
				continue
			}
			if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
				part.arms = append(part.arms, surfaceBeltSpan{ends: [2]placement.Vector{b.polarFrame.world(p.a, 0), b.polarFrame.world(p.b, 0)}})
				continue
			}
			// 原生装箱与路由共用分流器完整选择体积，只检查跨单元净空，保留同单元真实槽口短接。
			shapes, err := splitterClearanceShapes(catalog, p.record.ItemID, p.record.ModelIndex)
			if err != nil {
				return part, err
			}
			point := b.polarFrame.world(p.a, 0)
			if surfaceIsStation(p.record.ItemID) || surfaceIsPowerNode(p.record.ItemID) {
				part.stations = append(part.stations, polarStationOccupancy{point, p.record.ItemID})
			}
			for shapeNumber, shape := range shapes {
				box, err := encoding.colliderBox(vector{point[0], point[1], point[2]}, p.record.Yaw, shape)
				if err != nil {
					return part, err
				}
				if p.record.ItemID == 2020 && shapeNumber == len(shapes)-1 {
					if part.selections == nil {
						part.selections = map[int]bool{}
					}
					part.selections[len(part.boxes)] = true
				}
				part.boxes = append(part.boxes, box)
			}
		}
		if dynamic && b.policy != nil && b.policy.density == densityStandard {
			if err := b.validateNativeUnitBelts(ctx, unit); err != nil {
				return part, err
			}
		}
		if (preciseNodes || hasSelections) && dynamic {
			part.armBoxes, part.verifiedArms, err = b.polarUnitInserterBoxes(ctx, unit)
			if err != nil {
				return part, err
			}
		}
		for selection := range part.selections {
			for _, arm := range part.armBoxes {
				if arm.Overlaps(part.boxes[selection]) {
					return part, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "球面单元内真实分拣器动态盒侵占分流器选择体积"}
				}
			}
		}
		return part, nil
	}
	parts := make([]polarUnitOccupancy, 0, len(units))
	for _, unit := range units {
		part, err := makePart(unit, false)
		if err != nil {
			return nil, err
		}
		if !flexibleLatitude {
			period := float64(b.polarFrame.segments * 5)
			limit := longitudeLimit
			if part.high.x-part.low.x >= limit {
				return &unit, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("极区单元在当前纬度的经向跨度 %.1f 格超过可接线范围 %.1f 格", part.high.x-part.low.x, limit)}
			}
			dx := -math.Floor(part.low.x/period) * period
			if part.high.x+dx >= limit {
				dx = limit - part.high.x - .001
			}
			shift(unit.entities, dx, 0, 0)
			part, err = makePart(unit, true)
			if err != nil {
				return &unit, err
			}
		}
		parts = append(parts, part)
	}

	sort.SliceStable(parts, func(i, j int) bool {
		area := func(part polarUnitOccupancy) float64 {
			scale := 1.
			if flexibleLatitude {
				for _, p := range part.unit.entities {
					if p.nativeUnitScale > 0 {
						scale = p.nativeUnitScale
						break
					}
				}
			}
			return (part.high.x - part.low.x) * (part.high.y - part.low.y) / scale
		}
		return area(parts[i]) > area(parts[j])
	})
	boxes := []placement.Box{}
	nodes := []surfaceBeltNodeProbe{}
	belts := []surfaceBeltSpan{}
	beltIndex := &polarSpatialIndex{}
	arms := []surfaceBeltSpan{}
	verifiedArms := []bool{}
	armBoxes := []placement.Box{}
	stationIndex := &polarStationIndex{}
	boxIndex, nodeIndex, armIndex := &polarSpatialIndex{}, &polarSpatialIndex{}, &polarSpatialIndex{}
	armBoxIndex := &polarSpatialIndex{}
	selectionBoxes := []placement.Box{}
	selectionIndex := &polarSpatialIndex{}
	selectionBelts := []surfaceBeltSpan{}
	selectionBeltIndex := &polarSpatialIndex{}
	reportProgress(ctx, "surface-pack", "正在校验并装箱球面生产单元", 0, len(parts))
	for partNumber, seed := range parts {
		found := false
		// 保存经向紧凑的完整单元状态；最终近方形比较使用真实物理包络，不能用极区根网格比例替代。
		var bestCommit func()
		bestRight := math.Inf(1)
		latitudeCount := 1
		if flexibleLatitude {
			latitudeCount = b.polarFrame.height
			if seedFirst {
				latitudeCount++
			}
			// 非极区边缘请求保留一个完整单元锚定在该边缘；否则只缩短经向会把所有单元移向赤道，最后被位置规则全部拒绝。
			if partNumber == 0 && b.polarFrame.compactLongitude && selection.tier < 5 && (b.polarFrame.position == "north" || b.polarFrame.position == "south") {
				latitudeCount = min(latitudeCount, 5)
			}

		}
		for latitude := 0; latitude < latitudeCount && !found; latitude++ {
			part := seed
			seedAttempt := seedFirst && latitude == 0
			if flexibleLatitude {
				low, high := bounds(seed.unit.entities)
				scanLatitude := latitude
				if seedFirst {
					scanLatitude--
				}
				target := float64(scanLatitude)
				if b.polarFrame.position != "" {
					target = b.polarFrame.targetLatitude(scanLatitude, high.y-low.y)
				} else if b.polarFrame.start < 0 {
					target = float64(b.polarFrame.height-1-latitude) - (high.y - low.y)
				}
				if seedAttempt {
					target = seed.low.y
				}
				dy := target - low.y
				if low.y+dy < 0 || high.y+dy > float64(b.polarFrame.height-1) {
					continue
				}
				oldScale := 0.
				for _, p := range seed.unit.entities {
					if p.nativeUnitScale > 0 {
						oldScale = p.nativeUnitScale
						break
					}
				}
				if oldScale <= 0 {
					return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "极区单元缺少格距证据"}
				}
				movedLow, movedHigh := low, high
				movedLow.y += dy
				movedHigh.y += dy
				scale := b.polarFrame.unitScale(seed.unit, movedLow, movedHigh)
				ratio, center := scale/oldScale, (low.x+high.x)/2
				for _, p := range seed.unit.entities {
					p.a.x = center + (p.a.x-center)*ratio
					p.b.x = center + (p.b.x-center)*ratio
					p.a.y += dy
					p.b.y += dy
					p.densityCargo.relative.x *= ratio
					p.nativeUnitScale = scale
				}
				low, high = bounds(seed.unit.entities)
				if high.x-low.x >= longitudeLimit {
					continue
				}
				shift(seed.unit.entities, -low.x, 0, 0)
				part, err = makePart(seed.unit, true)
				if err != nil {
					if ctx.Err() == nil && denseNativeGeometryFailure(err) {
						continue
					}
					return nil, err
				}
			}
			rotatedBoxes := make([]placement.Box, len(part.boxes))
			rotatedNodes := make([]surfaceBeltNodeProbe, len(part.nodes))
			rotatedBelts := make([]surfaceBeltSpan, len(part.belts))
			rotatedSelectionBelts := make([]surfaceBeltSpan, len(part.selectionBelts))
			rotatedArms := make([]surfaceBeltSpan, len(part.arms))
			rotatedArmBoxes := make([]placement.Box, len(part.armBoxes))
			for attempt := 0; attempt <= b.polarFrame.segments*40; attempt++ {
				if seedAttempt && attempt > 128 {
					break
				}
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				dx := float64((attempt+1)/2) / 4
				if attempt%2 == 1 {
					dx = -dx
				}
				if seedAttempt {
					dx += seed.low.x
				} else if flexibleLatitude {
					dx = float64(attempt) / 4
					if part.high.x+dx >= longitudeLimit || part.high.x+dx >= bestRight {
						break
					}
				}
				if part.low.x+dx < 0 || part.high.x+dx >= longitudeLimit {
					continue
				}
				s, c := math.Sincos(dx * float64(b.polarFrame.longitudeStep))
				stationConflict := false
				skipSteps := 0
				for _, station := range part.stations {
					point := rotatePolarPoint(station.point, s, c)
					if margin, blocked := stationIndex.conflictMargin(station.item, point); blocked {
						if flexibleLatitude && !seedAttempt {
							skipSteps = polarBlockedSteps(margin, math.Hypot(point[0], point[2]), quarterAngle)
						}
						stationConflict = true
						break
					}
				}
				if stationConflict {
					attempt += skipSteps
					continue
				}
				conflict := false
				for i, span := range part.belts {
					span.ends = [2]placement.Vector{rotatePolarPoint(span.ends[0], s, c), rotatePolarPoint(span.ends[1], s, c)}
					rotatedBelts[i] = span
					low, high := polarSpanBounds(span, .46)
					if beltIndex.any(low, high, func(i int) bool { return surfaceSpanDistanceSquared(span, belts[i]) < .46*.46-1e-6 }) {
						conflict = true
						break
					}
					if boxIndex.any(low, high, func(i int) bool { return boxes[i].SegmentDistanceSquared(span.ends[0], span.ends[1]) < .23*.23-1e-6 }) {
						conflict = true
						break
					}
				}
				if conflict {
					continue
				}
				for i, span := range part.selectionBelts {
					span.ends = [2]placement.Vector{rotatePolarPoint(span.ends[0], s, c), rotatePolarPoint(span.ends[1], s, c)}
					rotatedSelectionBelts[i] = span
					low, high := polarSpanBounds(span, .23)
					if selectionIndex.any(low, high, func(n int) bool {
						return selectionBoxes[n].SegmentDistanceSquared(span.ends[0], span.ends[1]) < .23*.23-1e-6
					}) {
						conflict = true
						break
					}
				}
				if conflict {
					continue
				}
				// 按既有检测次序惰性变换；早期碰撞时不旋转本次位置中尚未检查的实体。
				for i, a := range part.boxes {
					a.Center = rotatePolarPoint(a.Center, s, c)
					for axis := range a.Axes {
						a.Axes[axis] = rotatePolarPoint(a.Axes[axis], s, c)
					}
					rotatedBoxes[i] = a
					low, high := a.Bounds()
					conflict = boxIndex.any(low, high, func(i int) bool {
						if !a.Overlaps(boxes[i]) {
							return false
						}
						if flexibleLatitude && !seedAttempt {
							skipSteps = polarBoxBlockedSteps(a, boxes[i], quarterAngle)
						}
						return true
					})
					if !conflict && part.selections[i] {
						conflict = armBoxIndex.any(low, high, func(n int) bool { return a.Overlaps(armBoxes[n]) })
					}
					if !conflict && strictBelts {
						margin := placement.Vector{.23, .23, .23}
						conflict = beltIndex.any(low.Sub(margin), high.Add(margin), func(n int) bool {
							span := belts[n]
							return a.SegmentDistanceSquared(span.ends[0], span.ends[1]) < .23*.23-1e-6
						})
					}
					if !conflict && part.selections[i] {
						margin := placement.Vector{.23, .23, .23}
						conflict = selectionBeltIndex.any(low.Sub(margin), high.Add(margin), func(n int) bool {
							span := selectionBelts[n]
							return a.SegmentDistanceSquared(span.ends[0], span.ends[1]) < .23*.23-1e-6
						})
					}

					if !conflict {
						radius := .25
						if preciseNodes {
							radius = surfaceBeltNodeRadius
						}
						margin := placement.Vector{radius, radius, radius}
						conflict = nodeIndex.any(low.Sub(margin), high.Add(margin), func(i int) bool {
							if preciseNodes {
								return nodes[i].overlapsBox(a)
							}
							return polarPointBoxDistance(a, nodes[i].center) < .25*.25
						})
					}
					if conflict {
						break
					}
				}
				if conflict {
					attempt += skipSteps
					continue
				}
				for i, p := range part.nodes {
					p = surfaceBeltNodeProbe{center: rotatePolarPoint(p.center, s, c), span: surfaceBeltSpan{ends: [2]placement.Vector{rotatePolarPoint(p.span.ends[0], s, c), rotatePolarPoint(p.span.ends[1], s, c)}}}
					rotatedNodes[i] = p
					radius := .25
					if preciseNodes {
						radius = surfaceBeltNodeRadius
					}
					margin := placement.Vector{radius, radius, radius}
					conflict = boxIndex.any(p.center.Sub(margin), p.center.Add(margin), func(i int) bool {
						if preciseNodes {
							return p.overlapsBox(boxes[i])
						}
						return polarPointBoxDistance(boxes[i], p.center) < .25*.25
					})
					if !conflict {
						margin = placement.Vector{.37, .37, .37}
						conflict = nodeIndex.any(p.center.Sub(margin), p.center.Add(margin), func(i int) bool { d := p.center.Sub(nodes[i].center); return d.Dot(d) < .37*.37 })
					}
					if conflict {
						break
					}
				}
				if conflict {
					continue
				}
				for i, a := range part.arms {
					a = surfaceBeltSpan{ends: [2]placement.Vector{rotatePolarPoint(a.ends[0], s, c), rotatePolarPoint(a.ends[1], s, c)}}
					rotatedArms[i] = a
					low, high := polarSpanBounds(a, .5)
					conflict = armIndex.any(low, high, func(i int) bool {
						if part.verifiedArms && verifiedArms[i] {
							return false
						}
						return surfaceSpanDistanceSquared(a, arms[i]) < .5*.5
					})
					if conflict {
						break
					}
				}
				if conflict {
					continue
				}
				for i, box := range part.armBoxes {
					box.Center = rotatePolarPoint(box.Center, s, c)
					for axis := range box.Axes {
						box.Axes[axis] = rotatePolarPoint(box.Axes[axis], s, c)
					}
					rotatedArmBoxes[i] = box
					low, high := box.Bounds()
					conflict = armBoxIndex.any(low, high, func(i int) bool { return box.Overlaps(armBoxes[i]) })
					if !conflict {
						conflict = selectionIndex.any(low, high, func(i int) bool { return box.Overlaps(selectionBoxes[i]) })
					}
					if conflict {
						break
					}
				}
				if conflict {
					continue
				}
				commit := func() {
					shift(part.unit.entities, dx, 0, 0)
					for _, span := range rotatedBelts {
						low, high := polarSpanBounds(span, 0)
						beltIndex.add(low, high)
					}
					belts = append(belts, rotatedBelts...)
					for _, span := range rotatedSelectionBelts {
						low, high := polarSpanBounds(span, 0)
						selectionBeltIndex.add(low, high)
					}
					selectionBelts = append(selectionBelts, rotatedSelectionBelts...)
					for number, box := range rotatedBoxes {
						low, high := box.Bounds()
						boxIndex.add(low, high)
						if part.selections[number] {
							selectionIndex.add(low, high)
							selectionBoxes = append(selectionBoxes, box)
						}
					}
					for _, point := range rotatedNodes {
						nodeIndex.add(point.center, point.center)
					}
					for _, arm := range rotatedArms {
						low, high := polarSpanBounds(arm, 0)
						armIndex.add(low, high)
						verifiedArms = append(verifiedArms, part.verifiedArms)
					}
					for _, box := range rotatedArmBoxes {
						low, high := box.Bounds()
						armBoxIndex.add(low, high)
					}
					boxes = append(boxes, rotatedBoxes...)
					nodes = append(nodes, rotatedNodes...)
					arms = append(arms, rotatedArms...)
					armBoxes = append(armBoxes, rotatedArmBoxes...)
					for _, station := range part.stations {
						stationIndex.add(station.item, rotatePolarPoint(station.point, s, c))
					}
					found = true
				}
				if flexibleLatitude && b.polarFrame.compactLongitude && !seedAttempt {
					bestRight = part.high.x + dx
					snapshot := map[*placed]placed{}
					for _, p := range part.unit.entities {
						snapshot[p] = *p
					}
					bestCommit = func() {
						for p, old := range snapshot {
							*p = old
						}
						commit()
					}
				} else {
					commit()
				}

				break
			}
		}
		if !found && bestCommit != nil {
			bestCommit()
		}
		if !found {
			return &seed.unit, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "极区球面单元在本次经纬位置搜索内未找到无冲突位置"}
		}
		reportProgress(ctx, "surface-pack", "正在校验并装箱球面生产单元", partNumber+1, len(parts))
	}
	return nil, nil
}
