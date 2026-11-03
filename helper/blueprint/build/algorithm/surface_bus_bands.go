package build

// 文件职责：将超出单纬带容量的共享总线迁移到多纬带货架并逐带边重接。
// 使用：共享总线单纬带折返的容量回退；边界：不以 Area 数量增加经向周期容量，最终所有静态和动态预检仍不可跳过。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// surfaceBusBandSection 职责：保存一段源总线的刚体变换和目标货架；使用：多纬带迁移；边界：经向装配坐标统一以赤道步长计量，记录写出时恢复区域自身的格距。
type surfaceBusBandSection struct {
	surfaceBusSlice
	// row 职责：引用规划货架；使用：确定物理纬带；边界：不是 Area 下标，同一 Area 可以含多行。
	row int
	// x 职责：保存所属 Area 原生格距下的货架经向起点；使用：整体变换；边界：装配时才除以 longitudeScales，不对设施自身做非刚体缩放。
	x float64
	// flipped 和 bottom 职责：保存经实体盒预检的区段朝向及纬向起点；使用：两端刚体变换与接驳切面；边界：只能整体旋转和平移，不拉伸设施。
	flipped bool
	bottom  int
}

// surfaceBusBandRow 职责：描述纬带内的一行折返货架；使用：限制实体与接驳净空；边界：相邻行独立预留南北通道，环向不回绕。
type surfaceBusBandRow struct {
	// band 职责：引用所在恒段纬带；使用：区域父链与格距；边界：不能跨越段数变化。
	band surfacePackedBand
	// low/high 职责：限定本货架的绝对纬向细格闭区间；使用：包含实体与接驳净空；边界：同一区域的货架区间不重叠。
	low, high int
	// reverse 职责：反转整行货架的列顺序；使用：跨货架沿外缘接驳；边界：不是对单个设施的镜像，设施只可整体旋转。
	reverse bool
}

// planSurfaceBusBandSections 职责：按可用纬带高度自适应选择不截断端点及非带连接的切面；使用：跨纬带总线规划；边界：不移动原对象，二维 footprint 不作为实体盒的替代，最终几何预检失败仍拒绝。
func planSurfaceBusBandSections(ctx context.Context, all []*placed, segment, padding, heightLimit int) ([]surfaceBusBandSection, []surfaceBusBandRow, error) {
	if len(all) == 0 || segment != 200 || padding < 1 || heightLimit < 1 {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "总线跨纬带规划的建筑集合、网格或净空参数无效"}
	}
	minimum, maximum := bounds(all)
	low, last := int(math.Floor(minimum.y)), int(math.Ceil(maximum.y))
	forbidden := map[int]bool{}
	for _, p := range all {
		if ctx.Err() != nil {
			return nil, nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "总线跨纬带切面规划已取消或超时"}
		}
		if surfaceIsBelt(p.record.ItemID) {
			continue
		}
		first, end := math.Min(p.a.y, p.b.y)-.25, math.Max(p.a.y, p.b.y)+.25
		for _, other := range []*placed{p.input, p.output} {
			if other != nil {
				first, end = math.Min(first, other.a.y-.25), math.Max(end, other.a.y+.25)
			}
		}
		for cut := int(math.Ceil(first - .5)); float64(cut)+.5 <= end; cut++ {
			forbidden[cut] = true
		}
	}
	rows := []surfaceBusBandRow{}
	// 从北向南保持相邻物理纬带，主带分两行；较窄纬带的切片高度随可用行数缩小。
	bands := surfaceLatitudeBands(segment)
	for i := len(bands) - 1; i >= 0; i-- {
		band := bands[i]
		if band.segments < 160 {
			continue
		}
		count := 1
		if band.segments == 200 {
			count = 2
		}
		height := (band.high - band.low + 1) / count
		for row := count - 1; row >= 0; row-- {
			bottom := band.low + row*height
			top := bottom + height - 1
			if row == count-1 {
				top = band.high
			}
			rows = append(rows, surfaceBusBandRow{band: band, low: bottom, high: top, reverse: len(rows)%2 != 0})
		}
	}
	sections := []surfaceBusBandSection{}
	for ri, row := range rows {
		maxHeight := min(heightLimit, row.high-row.low+1-2*padding)
		cursor := float64(2*padding + 4)
		for low <= last && maxHeight > 0 {
			high := min(last, low+maxHeight-1)
			for high < last && high >= low && forbidden[high] {
				high--
			}
			if high < low {
				break
			}
			members := []*placed{}
			for _, p := range all {
				if p.a.y >= float64(low)-.5 && p.a.y < float64(high)+.5 {
					members = append(members, p)
				}
			}
			if len(members) == 0 {
				return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带切面包含空区段"}
			}
			minPoint, maxPoint := bounds(members)
			width := math.Ceil(maxPoint.x-minPoint.x) + 1
			if cursor+width > float64(row.band.segments*5-2*padding-4) {
				break
			}
			x := cursor
			if row.reverse {
				x = float64(row.band.segments*5) - cursor - width
			}
			candidate := []surfaceBusBandSection{{surfaceBusSlice: surfaceBusSlice{low: low, high: high, minimum: minPoint, maximum: maxPoint, buildings: members, relaxed: true}, row: ri, x: x}}
			if err := chooseSurfaceBusBandTransforms(ctx, candidate, rows, padding); err != nil {
				if !surfaceAnchorRetryable(err) {
					return nil, nil, err
				}
				// 当前纬带曲率不容纳这组实体时，停止该货架并在下一货架重试同一源区段，不删机器也不压缩间距。
				break
			}
			sections = append(sections, candidate[0])
			cursor += width + 10
			low = high + 1
		}
		if low > last {
			return sections, rows[:ri+1], nil
		}
	}
	return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("共享总线在相邻纬带的安全货架中仍无法完整装入，源纬向格 %d 尚未布局；请拆分目标或使用独立产线模式", low)}
}

// surfaceBusBandEdges 职责：验证全体归属并保留每条相邻区段的原始有向边；使用：变换之前；边界：非带引用不得跨区，蓝图外引用和单端错区必须拒绝。
func surfaceBusBandEdges(all []*placed, sections []surfaceBusBandSection) ([][]surfaceBusEdge, map[*placed]int, error) {
	owners := make(map[*placed]int, len(all))
	for i, s := range sections {
		for _, p := range s.buildings {
			if _, exists := owners[p]; exists || p.b.y < float64(s.low)-.5 || p.b.y > float64(s.high)+.5 {
				return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带区段重复包含或截断建筑"}
			}
			owners[p] = i
		}
	}
	if len(owners) != len(all) {
		return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带区段遗漏建筑"}
	}
	edges := make([][]surfaceBusEdge, len(sections)-1)
	for _, p := range all {
		for _, other := range []*placed{p.input, p.output} {
			if other != nil {
				j, exists := owners[other]
				if !exists || j != owners[p] && (!surfaceIsBelt(p.record.ItemID) || !surfaceIsBelt(other.record.ItemID)) {
					return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带存在不可拆分的设施连接"}
				}
			}
		}
		if p.output == nil || owners[p] == owners[p.output] {
			continue
		}
		other := p.output
		if absInt(owners[p]-owners[other]) != 1 || p.a.x != other.a.x || p.a.z != other.a.z || math.Abs(p.a.y-other.a.y) > 1.01 {
			return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带切面含斜向或跨层带连接"}
		}
		cut := min(owners[p], owners[other])
		edges[cut] = append(edges[cut], surfaceBusEdge{from: p, to: other, x: p.a.x})
	}
	for _, group := range edges {
		sort.SliceStable(group, func(i, j int) bool { return group[i].x < group[j].x })
		for i := 1; i < len(group); i++ {
			if group[i].x == group[i-1].x {
				return nil, nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带切面有重复带列"}
			}
		}
	}
	return edges, owners, nil
}

// surfaceBusBandPath 职责：按最多一格的平面步长和半层的爬升步长生成接驳；使用：不同经度段之间的统一赤道坐标系；边界：保留小数端点，不产生零长度连续节点，最终仍检查球面实际距离。
func surfaceBusBandPath(corners []vector) []vector {
	points := []vector{corners[0]}
	for _, end := range corners[1:] {
		start := points[len(points)-1]
		length := max(math.Abs(end.x-start.x), math.Abs(end.y-start.y), 2*math.Abs(end.z-start.z))
		steps := int(math.Ceil(length))
		for i := 1; i <= steps; i++ {
			weight := float64(i) / float64(steps)
			points = append(points, vector{start.x + (end.x-start.x)*weight, start.y + (end.y-start.y)*weight, start.z + (end.z-start.z)*weight})
		}
	}
	return points
}

// chooseSurfaceBusBandTransforms 职责：按实际静态盒选择各区段的朝向和货架内余量；使用：迁移前；边界：只探测副本，整块内实体都不合法时拒绝，最终跨区与动态检查仍必须执行。
func chooseSurfaceBusBandTransforms(ctx context.Context, sections []surfaceBusBandSection, rows []surfaceBusBandRow, padding int) error {
	for i := range sections {
		s := &sections[i]
		row := rows[s.row]
		var lastError error
		found := false
		for _, flip := range []bool{i%2 != 0, i%2 == 0} {
			for offset := 0; offset <= row.high-row.low+1-2*padding-(s.high-s.low+1); offset++ {
				records := []blueprint.Building{}
				for _, p := range s.buildings {
					if surfaceIsBelt(p.record.ItemID) || p.record.ItemID >= 2011 && p.record.ItemID <= 2014 {
						continue
					}
					x, y := p.a.x-s.minimum.x, p.a.y-float64(s.low)
					yaw := p.record.Yaw
					if flip {
						x, y = s.maximum.x-p.a.x, float64(s.high)-p.a.y
						yaw = float32(math.Mod(float64(yaw)+180, 360))
					}
					r := p.record
					r.Index = int32(len(records))
					r.AreaIndex = 0
					r.Yaw = yaw
					r.LocalOffset = blueprint.Point{X: float32(s.x + x), Y: float32(padding+offset) + float32(y), Z: float32(p.a.z)}
					records = append(records, r)
				}
				probe := &surfaceEncoding{areas: []blueprint.Area{{AreaSegments: int16(row.band.segments)}}, plan: &SurfacePlan{GridSegment: 200, AnchorLatitudeGrid: row.low}}
				lastError = probe.validateStaticColliders(ctx, records)
				if lastError == nil {
					s.flipped = flip
					s.bottom = row.low + padding + offset
					found = true
					break
				}
				if !surfaceAnchorRetryable(lastError) {
					return lastError
				}
			}
			if found {
				break
			}
		}
		if !found {
			return &blueprint.Error{Code: "GEN_E_COLLISION", Message: fmt.Sprintf("总线源区段 %d..%d 在目标纬向 %d..%d 的刚体朝向未通过实体盒检查：%v", s.low, s.high, row.low, row.high, lastError)}
		}
	}
	return nil
}

// foldSurfaceBusBands 职责：物理迁移连续总线区段到不同纬带并逐边重接；使用：单带容量失败回退；边界：不修改机器数量和物流语义，不跳过任何最终预检，当前使用 200/160 段相邻纬带。
// 逻辑流：1. 完整规划并验证图 => 2. 建立连续 Area => 3. 按区段刚体变换 => 4. 逐切面添加货架内或环缘接驳 => 5. 交还统一编码和最终预检。
func (b *builder) foldSurfaceBusBands(ctx context.Context, layout *SurfaceLayout) error {
	const padding = 10
	var sections []surfaceBusBandSection
	var rows []surfaceBusBandRow
	var edges [][]surfaceBusEdge
	var owners map[*placed]int
	var lastError error
	for _, limit := range []int{100, 40, 30, 20} {
		var err error
		sections, rows, err = planSurfaceBusBandSections(ctx, b.all, packingSegment(layout), padding, limit)
		if err != nil {
			if failure, ok := err.(*blueprint.Error); !ok || failure.Code != "GEN_E_SURFACE_CAPACITY" {
				return err
			}
			lastError = err
			continue
		}
		edges, owners, err = surfaceBusBandEdges(b.all, sections)
		if err != nil {
			return err
		}
		for _, group := range edges {
			if len(group) >= padding {
				err = &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "总线跨纬带并行切面超过货架预留通道"}
				break
			}
		}
		if err == nil {
			err = chooseSurfaceBusBandTransforms(ctx, sections, rows, padding)
		}
		if err == nil {
			lastError = nil
			break
		}
		if !surfaceAnchorRetryable(err) {
			return err
		}
		lastError = err
	}
	if lastError != nil {
		return lastError
	}
	used := map[surfacePackedBand]bool{}
	for _, s := range sections {
		used[rows[s.row].band] = true
	}
	allBands := surfaceLatitudeBands(packingSegment(layout))
	first, last := -1, -1
	for i, band := range allBands {
		if used[band] {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "总线跨纬带规划没有区域"}
	}
	// 即使某一中间纬带没有源设施，也保留其 Area，使游戏按相邻纬带边界逐段吸附；接驳带仍可能经过它。
	bands := allBands[first : last+1]
	primary := 0
	for i, band := range bands {
		if band.segments > bands[primary].segments {
			primary = i
		}
	}
	anchor := bands[primary].low
	encoding := &surfaceEncoding{packed: true, areas: make([]blueprint.Area, len(bands)), latitudeOffsets: make([]int, len(bands)), longitudeScales: make([]float64, len(bands)), byBuilding: make(map[*placed]int, len(b.all)), anchorCandidates: []int{anchor}, plan: makeSurfacePlan(layout.Mode, packingSegment(layout), layout.LatitudeGridIndex, anchor, 0, 0)}
	encoding.plan.PrimaryAreaIndex, encoding.plan.AnchorLatitudeGrid = primary, anchor
	encoding.plan.Bands = make([]SurfaceBandPlan, len(bands))
	bandIndex := map[surfacePackedBand]int{}
	for i, band := range bands {
		parent, offset := -1, 0
		if i < primary {
			parent = i + 1
		} else if i > primary {
			parent = i - 1
		}
		if parent >= 0 {
			offset = band.low - bands[parent].low
		}
		encoding.areas[i] = blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(band.segments), AnchorLocalOffsetY: int16(offset), Width: int16(band.segments * 5), Height: int16(band.high - band.low + 1)}
		encoding.latitudeOffsets[i] = band.low
		encoding.longitudeScales[i] = float64(band.segments) / float64(bands[primary].segments)
		encoding.plan.Bands[i] = SurfaceBandPlan{AreaIndex: i, MinLatitudeGrid: band.low, MaxLatitudeGrid: band.high, LongitudeSegments: band.segments}
		encoding.plan.LayoutWidth = max(encoding.plan.LayoutWidth, int(encoding.areas[i].Width))
		bandIndex[band] = i
	}
	encoding.plan.LayoutHeight = bands[len(bands)-1].high - bands[0].low + 1
	for _, s := range sections {
		row := rows[s.row]
		area := bandIndex[row.band]
		scale := encoding.longitudeScales[area]
		for _, p := range s.buildings {
			for _, point := range []*vector{&p.a, &p.b} {
				x, y := point.x-s.minimum.x, point.y-float64(s.low)
				if s.flipped {
					x, y = s.maximum.x-point.x, float64(s.high)-point.y
				}
				point.x = (s.x + x) / scale
				point.y = float64(s.bottom) + y
			}
			if s.flipped {
				p.record.Yaw = float32(math.Mod(float64(p.record.Yaw)+180, 360))
				p.record.Yaw2 = float32(math.Mod(float64(p.record.Yaw2)+180, 360))
			}
			encoding.byBuilding[p] = area
			encoding.plan.Bands[area].BuildingCount++
		}
	}
	b.owner = -1
	for cut, group := range edges {
		left, right := sections[cut], sections[cut+1]
		// 同行 U 型通道按实际嵌套宽度排序，不能假定所有货架都从左向右。
		sort.SliceStable(group, func(i, j int) bool {
			return math.Min(group[i].from.a.x, group[i].to.a.x) < math.Min(group[j].from.a.x, group[j].to.a.x)
		})
		for i, edge := range group {
			if ctx.Err() != nil {
				return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "总线跨纬带接驳已取消或超时"}
			}
			row := rows[left.row]
			rank := len(group) - i
			y := float64(min(left.bottom, right.bottom) - rank)
			if !left.flipped {
				y = float64(left.bottom + left.high - left.low + rank + 1)
				y = max(y, float64(right.bottom+right.high-right.low+rank+1))
			}
			corners := []vector{edge.from.a, {edge.from.a.x, y, edge.from.a.z}, {edge.to.a.x, y, edge.to.a.z}, edge.to.a}
			if left.row != right.row || left.flipped == right.flipped {
				// 跨货架只使用环缘预留走廊，不横穿已占用的中间产线。两端仍从各自切面正向延伸。
				start, end := edge.from, edge.to
				if owners[start] != cut {
					start, end = end, start
				}
				firstY := float64(left.bottom - padding)
				if !left.flipped {
					firstY = float64(left.bottom + padding + left.high - left.low)
				}
				secondY := float64(right.bottom - padding)
				if right.flipped {
					secondY = float64(right.bottom + padding + right.high - right.low)
				}
				x := float64(rank + 2)
				if !row.reverse {
					x = float64(bands[primary].segments*5 - rank - 3)
				}
				if left.row == right.row {
					scale := encoding.longitudeScales[bandIndex[row.band]]
					if left.x < right.x {
						x = (left.x + left.maximum.x - left.minimum.x + right.x) / scale / 2
					} else {
						x = (right.x + right.maximum.x - right.minimum.x + left.x) / scale / 2
					}
				}
				// 跨行会反转带束顺序，平面外缘通道可能必然相交；独立半层高度使带束在三维保持分离，切面两端以不超过半层/格的斜率爬升。
				height := start.a.z + .5 + float64(rank)*.5
				corners = []vector{start.a, {start.a.x, firstY, height}, {x, firstY, height}, {x, secondY, height}, {end.a.x, secondY, height}, end.a}
				if start != edge.from {
					for a, c := 0, len(corners)-1; a < c; a, c = a+1, c-1 {
						corners[a], corners[c] = corners[c], corners[a]
					}
				}
			}
			points := surfaceBusBandPath(corners)
			if len(points) < 3 || len(b.all)+len(points)-2 > blueprint.MaxBuildingCount {
				return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "总线跨纬带接驳超过建筑预算或路径无效"}
			}
			bridge := b.byPath(points[1 : len(points)-1])
			connect(edge.from, bridge[0])
			connect(bridge[len(bridge)-1], edge.to)
			if edge.to.input == edge.from {
				edge.to.input = bridge[len(bridge)-1]
			}
			for _, p := range bridge {
				area := -1
				for j, band := range bands {
					if p.a.y >= float64(band.low) && p.a.y <= float64(band.high) {
						area = j
						break
					}
				}
				if area < 0 {
					return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "总线跨纬带接驳节点未落在原生纬向行"}
				}
				encoding.byBuilding[p] = area
				encoding.plan.Bands[area].BuildingCount++
				delta := vector{p.output.a.x - p.a.x, p.output.a.y - p.a.y, 0}
				p.record.Yaw = float32(math.Mod(math.Atan2(delta.x, delta.y)*180/math.Pi+360, 360))
				p.record.Yaw2 = p.record.Yaw
			}
		}
	}
	b.surfaceBusEncoding = encoding
	b.warnings = append(b.warnings, fmt.Sprintf("共享总线已实际迁移为 %d 个区段、%d 个原生纬带区域，并逐边重接；带宽不足告警仍有效", len(sections), len(bands)))
	return nil
}
