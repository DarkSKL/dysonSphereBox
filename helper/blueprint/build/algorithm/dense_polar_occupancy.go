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
	unit      denseSpatialUnit
	boxes     []placement.Box
	nodes     []placement.Vector
	arms      []surfaceBeltSpan
	stations  []polarStationOccupancy
	low, high vector
}

// polarStationOccupancy 职责：保存独立物流塔的世界中心及原型；使用：跨单元最小塔距；边界：不能用模型盒互不重叠代替游戏塔距。
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
	visited := map[*placed]map[int]bool{}
	limit := max(128, min(2048, len(units)*8))
	for attempt := 0; attempt < limit; attempt++ {
		blocked, err := b.packPolarUnitOccupancy(ctx, units)
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

// packPolarUnitOccupancy 职责：沿纬线选择互不冲突的经向位置；使用：极区二维修复的一次完整尝试；边界：返回阻塞单元供调用方重新选择纬度，不据有限搜索声称数学容量不足。
func (b *builder) packPolarUnitOccupancy(ctx context.Context, units []denseSpatialUnit) (*denseSpatialUnit, error) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		return nil, err
	}
	encoding := &surfaceEncoding{}
	parts := make([]polarUnitOccupancy, 0, len(units))
	for _, unit := range units {
		part := polarUnitOccupancy{unit: unit}
		part.low, part.high = bounds(unit.entities)
		period := float64(b.polarFrame.segments * 5)
		limit := b.polarFrame.longitudeLimit()
		if part.high.x-part.low.x >= limit {
			return &unit, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: fmt.Sprintf("极区单元在当前纬度的经向跨度 %.1f 格超过可接线范围 %.1f 格", part.high.x-part.low.x, limit)}
		}
		// 每个纬线上的整周平移是同一球面位置；先归入一周，避免有限搜索围绕多周外的坐标空转。
		dx := -math.Floor(part.low.x/period) * period
		if part.high.x+dx >= limit {
			dx = limit - part.high.x - .001
		}
		shift(unit.entities, dx, 0, 0)
		part.low.x += dx
		part.high.x += dx
		for _, p := range unit.entities {
			if surfaceIsBelt(p.record.ItemID) {
				part.nodes = append(part.nodes, b.polarFrame.world(p.a, .2))
				continue
			}
			if p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
				part.arms = append(part.arms, surfaceBeltSpan{ends: [2]placement.Vector{b.polarFrame.world(p.a, 0), b.polarFrame.world(p.b, 0)}})
				continue
			}
			shapes, err := catalog.Shapes(p.record.ItemID, p.record.ModelIndex)
			if err != nil {
				return nil, err
			}
			point := b.polarFrame.world(p.a, 0)
			if surfaceIsStation(p.record.ItemID) {
				part.stations = append(part.stations, polarStationOccupancy{point, p.record.ItemID})
			}
			for _, shape := range shapes {
				box, err := encoding.colliderBox(vector{point[0], point[1], point[2]}, p.record.Yaw, shape)
				if err != nil {
					return nil, err
				}
				part.boxes = append(part.boxes, box)
			}
		}
		parts = append(parts, part)
	}
	sort.SliceStable(parts, func(i, j int) bool {
		return (parts[i].high.x-parts[i].low.x)*(parts[i].high.y-parts[i].low.y) > (parts[j].high.x-parts[j].low.x)*(parts[j].high.y-parts[j].low.y)
	})
	boxes := []placement.Box{}
	nodes := []placement.Vector{}
	arms := []surfaceBeltSpan{}
	stations := []polarStationOccupancy{}
	for _, part := range parts {
		found := false
		for attempt := 0; attempt <= b.polarFrame.segments*40; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			dx := float64((attempt+1)/2) / 4
			if attempt%2 == 1 {
				dx = -dx
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
					if delta.Dot(delta) < float64(surfaceStationDistanceLimit(station.item, other.item)) {
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
			rotatedBoxes := make([]placement.Box, len(part.boxes))
			rotatedNodes := make([]placement.Vector, len(part.nodes))
			rotatedArms := make([]surfaceBeltSpan, len(part.arms))
			for i, box := range part.boxes {
				box.Center = rotatePolarPoint(box.Center, s, c)
				for j := range box.Axes {
					box.Axes[j] = rotatePolarPoint(box.Axes[j], s, c)
				}
				rotatedBoxes[i] = box
			}
			for i, p := range part.nodes {
				rotatedNodes[i] = rotatePolarPoint(p, s, c)
			}
			for i, a := range part.arms {
				rotatedArms[i] = surfaceBeltSpan{ends: [2]placement.Vector{rotatePolarPoint(a.ends[0], s, c), rotatePolarPoint(a.ends[1], s, c)}}
			}
			conflict := false
			for _, a := range rotatedBoxes {
				for _, c := range boxes {
					if a.Overlaps(c) {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
				for _, p := range nodes {
					if a.SegmentDistanceSquared(p, p) < .25*.25 {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
			}
			if conflict {
				continue
			}
			for _, p := range rotatedNodes {
				for _, box := range boxes {
					if box.SegmentDistanceSquared(p, p) < .25*.25 {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
				for _, q := range nodes {
					d := p.Sub(q)
					if d.Dot(d) < .37*.37 {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
			}
			if conflict {
				continue
			}
			for _, a := range rotatedArms {
				for _, c := range arms {
					if surfaceSpanDistanceSquared(a, c) < .5*.5 {
						conflict = true
						break
					}
				}
				if conflict {
					break
				}
			}
			if conflict {
				continue
			}
			shift(part.unit.entities, dx, 0, 0)
			boxes = append(boxes, rotatedBoxes...)
			nodes = append(nodes, rotatedNodes...)
			arms = append(arms, rotatedArms...)
			for _, station := range part.stations {
				stations = append(stations, polarStationOccupancy{rotatePolarPoint(station.point, s, c), station.item})
			}
			found = true
			break
		}
		if !found {
			return &part.unit, &blueprint.Error{Code: "GEN_E_DENSITY_LAYOUT", Message: "极区球面单元在本次环向搜索内未找到无冲突位置"}
		}
	}
	return nil, nil
}
