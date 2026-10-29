package build

// 文件职责：沿无实体切面折返共享总线，并显式重连所有跨切面的带边。
// 使用：超长共享总线的球面重排；边界：不拆分分拣器，不改变产率需求或假定容量通过即无碰撞。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// surfaceBusSlice 职责：保存连续纬向区段的建筑及平面边界；使用：总线折返规划；边界：不是连接闭合块，不能直接传给普通产线装箱器。
type surfaceBusSlice struct {
	low, high        int
	minimum, maximum vector
	buildings        []*placed
	// relaxed 表示切面避让只使用端点/连接净空，实体盒由后续最终静态预检复核。
	relaxed bool
}

// surfaceBusEdge 职责：记录一次切面上的原始有向带边；使用：折返后逐边重连；边界：只接受相邻切面及同高、同经向列的带边。
type surfaceBusEdge struct {
	from, to *placed
	x        float64
}

// foldSurfaceBus 职责：在矩形超容量时将连续总线折成相邻往返列并保留所有有向连接；使用：mode 6 收尾前；边界：优先单纬带，超出环向容量且允许扩展时交给多纬带迁移和逐边重接，最终预检始终保留。
func (b *builder) foldSurfaceBus(ctx context.Context, layout *SurfaceLayout) error {
	segment := packingSegment(layout)
	minimum, maximum := bounds(b.all)
	width, height := int(math.Ceil(maximum.x-minimum.x))+1, int(math.Ceil(maximum.y-minimum.y))+1
	if len(surfaceAnchorCandidates(layout.Mode, segment, layout.LatitudeGridIndex, layout.ExpandAdjacent, width, height)) > 0 {
		return nil
	}
	bandHeight, bandWidth := 0, 0
	for _, band := range surfaceLatitudeBands(segment) {
		low := band.low
		if layout.Mode == "explicitTropic" && !layout.ExpandAdjacent {
			if layout.LatitudeGridIndex < band.low || layout.LatitudeGridIndex > band.high {
				continue
			}
			low = layout.LatitudeGridIndex
		}
		if h := band.high - low + 1; h > bandHeight {
			bandHeight, bandWidth = h, band.segments*5
		}
	}
	// 满纬带的长切片会放大球面弯曲导致的实体盒交叠；先限制百格，并预留折返净空，最终仍须真实盒预检。
	sliceHeight := min(100, bandHeight-22)
	slices, byBuilding, err := surfaceBusSlices(ctx, b.all, sliceHeight)
	if err != nil {
		return err
	}
	edges := make([][]surfaceBusEdge, len(slices)-1)
	for _, p := range b.all {
		if p.output == nil || byBuilding[p] == byBuilding[p.output] {
			continue
		}
		other := p.output
		first, second := byBuilding[p], byBuilding[other]
		if absInt(first-second) != 1 || p.a.x != other.a.x || p.a.z != other.a.z || math.Abs(p.a.y-other.a.y) > 1.01 {
			return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线切面包含尚未适配的跨层或斜向连接"}
		}
		cut := min(first, second)
		edges[cut] = append(edges[cut], surfaceBusEdge{from: p, to: other, x: p.a.x})
	}
	maxEdges := 0
	for i := range edges {
		sort.SliceStable(edges[i], func(a, c int) bool { return edges[i][a].x < edges[i][c].x })
		maxEdges = max(maxEdges, len(edges[i]))
		for j := 1; j < len(edges[i]); j++ {
			if edges[i][j].x == edges[i][j-1].x {
				return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线切面包含重复列，不能使用平面折返通道"}
			}
		}
	}
	if maxEdges > 10 {
		return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "总线切面的并行通道超过当前折返净空"}
	}
	starts := make([]float64, len(slices))
	nextX := 4.
	for i, slice := range slices {
		starts[i] = nextX + 2
		nextX += math.Ceil(slice.maximum.x-slice.minimum.x) + 1 + 4 + 6
	}
	if nextX > float64(bandWidth-4) {
		if layout.Mode == "autoTropic" || layout.ExpandAdjacent {
			return b.foldSurfaceBusBands(ctx, layout)
		}
		return &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("共享总线折返后仍需要 %.0f 格经向容量；当前尚不能跨纬带重连该总线，请拆分混带目标或使用其他布局", nextX)}
	}
	baseY := float64(maxEdges + 1)
	for i, slice := range slices {
		for _, p := range slice.buildings {
			for _, point := range []*vector{&p.a, &p.b} {
				if i%2 == 0 {
					point.x = starts[i] + point.x - slice.minimum.x
					point.y = baseY + point.y - float64(slice.low)
				} else {
					point.x = starts[i] + slice.maximum.x - point.x
					point.y = baseY + float64(slice.high) - point.y
				}
			}
			if i%2 != 0 {
				p.record.Yaw = float32(math.Mod(float64(p.record.Yaw)+180, 360))
				p.record.Yaw2 = float32(math.Mod(float64(p.record.Yaw2)+180, 360))
			}
		}
	}
	b.owner = -1
	for cut, group := range edges {
		for i, edge := range group {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "总线折返路由已取消或超时"}
			}
			y := baseY - float64(i+1)
			if cut%2 == 0 {
				y = baseY + float64(sliceHeight+len(group)-i)
			}
			first := vector{edge.from.a.x, y, edge.from.a.z}
			second := vector{edge.to.a.x, y, edge.to.a.z}
			points := manhattan(edge.from.a, first, false)
			points = append(points, manhattan(first, second, false)[1:]...)
			points = append(points, manhattan(second, edge.to.a, false)[1:]...)
			if len(b.all)+len(points)-2 > blueprint.MaxBuildingCount {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线折返接驳后超过 20 万建筑，请拆分目标"}
			}
			bridge := b.byPath(points[1 : len(points)-1])
			connect(edge.from, bridge[0])
			connect(bridge[len(bridge)-1], edge.to)
			if edge.to.input == edge.from {
				edge.to.input = bridge[len(bridge)-1]
			}
			for _, p := range bridge {
				delta := vector{p.output.a.x - p.a.x, p.output.a.y - p.a.y, 0}
				p.record.Yaw = float32(math.Mod(math.Atan2(delta.x, delta.y)*180/math.Pi+360, 360))
				p.record.Yaw2 = p.record.Yaw
			}
		}
	}
	b.warnings = append(b.warnings, fmt.Sprintf("共享总线已沿 %d 个安全区段折返，并逐条重连跨切面传送带；完整产线和过滤器保留，带宽不足告警仍有效", len(slices)))
	for _, slice := range slices {
		if slice.relaxed {
			b.warnings = append(b.warnings, "部分总线切面采用端点净空回退；最终静态碰撞预检已重新核对实体建造盒")
			break
		}
	}
	return nil
}

// surfaceBusSlices 职责：选择不切开建筑、分拣器和非带连接的半格切面；使用：共享总线重排；边界：保留所有跨区带边，任一区段过高或非带连接跨区均失败关闭。
func surfaceBusSlices(ctx context.Context, all []*placed, maximumHeight int) ([]surfaceBusSlice, map[*placed]int, error) {
	if len(all) == 0 || maximumHeight < 1 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "总线分段高度或建筑集合无效"}
	}
	minimum, maximum := bounds(all)
	first, last := int(math.Floor(minimum.y)), int(math.Ceil(maximum.y))
	forbidden := map[int]bool{}
	relaxedForbidden := map[int]bool{}
	for _, p := range all {
		if ctx.Err() != nil {
			return nil, nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "总线分段规划已取消或超时"}
		}
		if surfaceIsBelt(p.record.ItemID) {
			continue
		}
		endpointLow, endpointHigh := math.Min(p.a.y, p.b.y)-.25, math.Max(p.a.y, p.b.y)+.25
		low, high := endpointLow, endpointHigh
		relaxedLow, relaxedHigh := endpointLow, endpointHigh
		if p.record.ItemID < 2011 || p.record.ItemID > 2014 {
			lo, hi := footprint(p.record.ItemID)
			// 二维经验 footprint 的边缘不能充当实体切面；额外两格为真实盒及弯曲留出净空。
			low, high = math.Min(low, p.a.y+lo.y-2), math.Max(high, p.a.y+hi.y+2)
		}
		for _, other := range []*placed{p.input, p.output} {
			if other != nil {
				low, high = math.Min(low, other.a.y-.25), math.Max(high, other.a.y+.25)
				relaxedLow, relaxedHigh = math.Min(relaxedLow, other.a.y-.25), math.Max(relaxedHigh, other.a.y+.25)
			}
		}
		for cut := int(math.Ceil(relaxedLow - .5)); float64(cut)+.5 <= relaxedHigh; cut++ {
			relaxedForbidden[cut] = true
		}
		for cut := int(math.Ceil(low - .5)); float64(cut)+.5 <= high; cut++ {
			forbidden[cut] = true
		}
	}
	slices := []surfaceBusSlice{}
	for low := first; low <= last; {
		high := min(last, low+maximumHeight-1)
		if high < last {
			for high >= low && forbidden[high] {
				high--
			}
		}
		relaxed := false
		if high < low {
			// 超大总线的实体 footprint 可能连续覆盖整个平面高度；允许在端点之外切面，
			// 将真实 OBB 交叠交给最终球面静态预检，而不是把保守二维 footprint 当成硬容量。
			high = min(last, low+maximumHeight-1)
			for high >= low && relaxedForbidden[high] {
				high--
			}
			if high < low {
				return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("总线在纬向格 %d 附近没有不切开设施的安全切面", low)}
			}
			relaxed = true
		}
		slices = append(slices, surfaceBusSlice{low: low, high: high, relaxed: relaxed})
		low = high + 1
	}
	byBuilding := make(map[*placed]int, len(all))
	for _, p := range all {
		index := sort.Search(len(slices), func(i int) bool { return float64(slices[i].high)+.5 > p.a.y })
		if index >= len(slices) || p.b.y < float64(slices[index].low)-.5 || p.b.y > float64(slices[index].high)+.5 {
			return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线切面截断了建筑端点"}
		}
		byBuilding[p] = index
		slices[index].buildings = append(slices[index].buildings, p)
	}
	for i := range slices {
		if len(slices[i].buildings) == 0 {
			return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线存在不含节点的分段"}
		}
		slices[i].minimum, slices[i].maximum = bounds(slices[i].buildings)
	}
	for p, index := range byBuilding {
		for _, other := range []*placed{p.input, p.output} {
			if other == nil {
				continue
			}
			target, exists := byBuilding[other]
			if !exists || target != index && (!surfaceIsBelt(p.record.ItemID) || !surfaceIsBelt(other.record.ItemID)) {
				return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("总线存在不能拆开的跨区设施连接：%d(%d,y=%.2f)→%d(%d,y=%.2f)", p.record.Index, p.record.ItemID, p.a.y, other.record.Index, other.record.ItemID, other.a.y)}
			}
		}
	}
	return slices, byBuilding, nil
}
