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
	nodes []surfaceBeltNodeProbe
	arms  []surfaceBeltSpan
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
		if len(units) >= 64 {
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
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	encoding := &surfaceEncoding{}
	previous := map[*placed]*placed{}
	if preciseNodes {
		for _, entity := range b.all {
			if surfaceIsBelt(entity.record.ItemID) && entity.output != nil && surfaceIsBelt(entity.output.record.ItemID) {
				previous[entity.output] = entity
			}
		}
	}
	// 大厂在每个单元内完整搜索纬度，避免已落位单元被反复移动而耗尽重排预算。
	flexibleLatitude := len(units) >= 64
	makePart := func(unit denseSpatialUnit, dynamic bool) (polarUnitOccupancy, error) {
		part := polarUnitOccupancy{unit: unit}
		part.low, part.high = bounds(unit.entities)
		members := make(map[*placed]bool, len(unit.entities))
		if preciseNodes {
			for _, entity := range unit.entities {
				members[entity] = true
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
			shapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
			if err != nil {
				return part, err
			}
			point := b.polarFrame.world(p.a, 0)
			if surfaceIsStation(p.record.ItemID) || surfaceIsPowerNode(p.record.ItemID) {
				part.stations = append(part.stations, polarStationOccupancy{point, p.record.ItemID})
			}
			for _, shape := range shapes {
				box, err := encoding.colliderBox(vector{point[0], point[1], point[2]}, p.record.Yaw, shape)
				if err != nil {
					return part, err
				}
				part.boxes = append(part.boxes, box)
			}
		}
		if preciseNodes && dynamic {
			part.armBoxes, part.verifiedArms, err = b.polarUnitInserterBoxes(ctx, unit)
			if err != nil {
				return part, err
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
			limit := b.polarFrame.longitudeLimit()
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
	arms := []surfaceBeltSpan{}
	verifiedArms := []bool{}
	armBoxes := []placement.Box{}
	stations := []polarStationOccupancy{}
	boxIndex, nodeIndex, armIndex := &polarSpatialIndex{}, &polarSpatialIndex{}, &polarSpatialIndex{}
	armBoxIndex := &polarSpatialIndex{}
	for _, seed := range parts {
		found := false
		// 保存当前最短右界的完整单元状态；后续纬度只读已提交占位，最后提交一次。
		var bestCommit func()
		bestRight := math.Inf(1)
		latitudeCount := 1
		if flexibleLatitude {
			latitudeCount = b.polarFrame.height
		}
		for latitude := 0; latitude < latitudeCount && !found; latitude++ {
			part := seed
			if flexibleLatitude {
				low, high := bounds(seed.unit.entities)
				target := float64(latitude)
				if b.polarFrame.position != "" {
					target = b.polarFrame.targetLatitude(latitude, high.y-low.y)
				} else if b.polarFrame.start < 0 {
					target = float64(b.polarFrame.height-1-latitude) - (high.y - low.y)
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
				if high.x-low.x >= b.polarFrame.longitudeLimit() {
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
			rotatedArms := make([]surfaceBeltSpan, len(part.arms))
			rotatedArmBoxes := make([]placement.Box, len(part.armBoxes))
			for attempt := 0; attempt <= b.polarFrame.segments*40; attempt++ {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				dx := float64((attempt+1)/2) / 4
				if attempt%2 == 1 {
					dx = -dx
				}
				if flexibleLatitude {
					dx = float64(attempt) / 4
					if part.high.x+dx >= b.polarFrame.longitudeLimit() || part.high.x+dx >= bestRight {
						break
					}
				}
				if part.low.x+dx < 0 || part.high.x+dx >= b.polarFrame.longitudeLimit() {
					continue
				}
				s, c := math.Sincos(dx * float64(b.polarFrame.longitudeStep))
				stationConflict := false
				for _, station := range part.stations {
					point := rotatePolarPoint(station.point, s, c)
					for _, other := range stations {
						delta := point.Sub(other.point)
						limit := float64(0)
						if surfaceIsStation(station.item) && surfaceIsStation(other.item) {
							limit = float64(surfaceStationDistanceLimit(station.item, other.item))
						}
						if surfaceIsPowerNode(station.item) && surfaceIsPowerNode(other.item) {
							d := math.Sqrt(float64(surfacePowerDistanceLimit(station.item, other.item))) + 1e-4
							limit = d * d
						}
						if delta.Dot(delta) < limit {
							stationConflict = true
							break
						}
					}
					if stationConflict {
						break
					}
				}
				if stationConflict {
					continue
				}
				for i, box := range part.boxes {
					box.Center = rotatePolarPoint(box.Center, s, c)
					for j := range box.Axes {
						box.Axes[j] = rotatePolarPoint(box.Axes[j], s, c)
					}
					rotatedBoxes[i] = box
				}
				for i, p := range part.nodes {
					rotatedNodes[i] = surfaceBeltNodeProbe{center: rotatePolarPoint(p.center, s, c), span: surfaceBeltSpan{ends: [2]placement.Vector{rotatePolarPoint(p.span.ends[0], s, c), rotatePolarPoint(p.span.ends[1], s, c)}}}
				}
				for i, a := range part.arms {
					rotatedArms[i] = surfaceBeltSpan{ends: [2]placement.Vector{rotatePolarPoint(a.ends[0], s, c), rotatePolarPoint(a.ends[1], s, c)}}
				}
				for i, box := range part.armBoxes {
					box.Center = rotatePolarPoint(box.Center, s, c)
					for axis := range box.Axes {
						box.Axes[axis] = rotatePolarPoint(box.Axes[axis], s, c)
					}
					rotatedArmBoxes[i] = box
				}
				conflict := false
				for _, a := range rotatedBoxes {
					low, high := a.Bounds()
					conflict = boxIndex.any(low, high, func(i int) bool { return a.Overlaps(boxes[i]) })
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
					continue
				}
				for _, p := range rotatedNodes {
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
				for _, a := range rotatedArms {
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
				for _, box := range rotatedArmBoxes {
					low, high := box.Bounds()
					conflict = armBoxIndex.any(low, high, func(i int) bool { return box.Overlaps(armBoxes[i]) })
					if conflict {
						break
					}
				}
				if conflict {
					continue
				}
				commit := func() {
					shift(part.unit.entities, dx, 0, 0)
					for _, box := range rotatedBoxes {
						low, high := box.Bounds()
						boxIndex.add(low, high)
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
						stations = append(stations, polarStationOccupancy{rotatePolarPoint(station.point, s, c), station.item})
					}
					found = true
				}
				if flexibleLatitude && b.polarFrame.compactLongitude {
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
	}
	return nil, nil
}
