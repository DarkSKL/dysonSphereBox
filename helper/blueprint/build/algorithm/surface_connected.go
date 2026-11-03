package build

// 文件职责：在保留逐物料带图的前提下，为直连工厂插入安全纬向空行并重接多 Area 边。
// 使用：mode 1 的完整矩形无法沿天然纬带边界切分时，由球面编码回退调用。
// 边界：不移动单个机器相对于其分拣器的几何，不支持无安全切面的设施；候选必须通过全部最终数值规则。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// connectedSurfaceBridge 职责：保留一条原始带边及其新增接驳链；使用：同步机器行路线证据；边界：空接驳链代表原边原样保留。
type connectedSurfaceBridge struct {
	// target 职责：保存重排前的下游带；使用：识别路线中的原始相邻节点；边界：必须在同一装配图内。
	target *placed
	// belts 职责：保存两端之间实际新增的传送带；使用：替换连接及容量证据；边界：不包含两端本身。
	belts []*placed
}

// connectedSurfacePathRefs 职责：枚举跨区补带需要同步的有向路线字段；使用：候选快照恢复和成功接驳共用；边界：本地回流仍为独立毛流，不加入规范路线，source保持机器出货口之后的原始直连顺序。
func (b *builder) connectedSurfacePathRefs() []*[]*placed {
	refs := make([]*[]*placed, 0, 2*len(b.transportRoutes)+5*len(b.machineRecycles))
	seen := map[*[]*placed]bool{}
	addPath := func(ref *[]*placed) {
		if !seen[ref] {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	addRoute := func(route *transportRoute) {
		if route != nil {
			addPath(&route.path)
			addPath(&route.sharedPath)
		}
	}
	for _, route := range b.transportRoutes {
		addRoute(route)
	}
	for _, entry := range b.machineRecycles {
		if entry == nil {
			continue
		}
		addRoute(entry.route)
		addPath(&entry.source)
		addPath(&entry.intake)
		addPath(&entry.surplus)
	}
	return refs
}

// connectedSurfaceCuts 职责：在端点安全切面上额外保留不可拆分的设施连接；使用：直连工厂跨纬带规划；边界：带与带可以切分，非带连接不可跨切面。
func connectedSurfaceCuts(ctx context.Context, all []*placed, height int) ([]bool, error) {
	cuts := surfaceRectangleSafeCuts(all, height)
	blocked := make([]int, height+2)
	for _, p := range all {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "直连图安全切面检查已取消或超时"}
		}
		for _, other := range []*placed{p.input, p.output} {
			if other == nil || surfaceIsBelt(p.record.ItemID) && surfaceIsBelt(other.record.ItemID) {
				continue
			}
			low := min(p.a.y, p.b.y, other.a.y, other.b.y)
			high := max(p.a.y, p.b.y, other.a.y, other.b.y)
			first, last := max(1, int(math.Floor(low))+1), min(height-1, int(math.Ceil(high)))
			if first <= last {
				blocked[first]++
				blocked[last+1]--
			}
		}
	}
	count := 0
	for cut := 1; cut < height; cut++ {
		count += blocked[cut]
		if count > 0 {
			cuts[cut] = false
		}
	}
	return cuts, nil
}

// planConnectedSurfaceChunks 职责：把源图按安全切面分配到连续原生纬带；使用：原边重接之前；边界：提前切分留下的空行必须由调用方真实铺带补齐，不将其视为免费连接。
func planConnectedSurfaceChunks(bands []surfacePackedBand, cuts []bool, start, width, height int) ([]surfaceRectChunk, bool) {
	chunks := []surfaceRectChunk{}
	source, target := 0, start
	for source < height {
		band := surfacePackedBand{}
		found := false
		for _, candidate := range bands {
			if target >= candidate.low && target <= candidate.high {
				band, found = candidate, true
				break
			}
		}
		if !found || len(chunks) >= surfaceMaxAreas {
			return nil, false
		}
		cut := min(height, source+band.high-target+1)
		for cut > source && cut < height && !cuts[cut] {
			cut--
		}
		if cut <= source {
			return nil, false
		}
		chunks = append(chunks, surfaceRectChunk{sourceLow: source, sourceHigh: cut, targetLow: target, segments: band.segments})
		source, target = cut, band.high+1
	}
	primary := 0
	for _, chunk := range chunks {
		primary = max(primary, chunk.segments)
	}
	for _, chunk := range chunks {
		areaWidth := int(math.Ceil(float64(width-1)*float64(chunk.segments)/float64(primary))) + 1
		if areaWidth > chunk.segments*5 {
			return nil, false
		}
	}
	return chunks, len(chunks) > 1
}

// prepareConnectedSurfaceEncoding 职责：以有界候选补齐直连图的多纬带适配；使用：完整矩形或其锚点规则失败回退；边界：失败恢复所有位置、引用和路线证据，不发布部分结果。
// 逻辑流：1. 保存原图 => 2. 规划不拆设施的安全切面 => 3. 按纬带补空行并逐带重接 => 4. 验证最终 float32 记录 => 5. 仅提交完整可行候选。
func (b *builder) prepareConnectedSurfaceEncoding(ctx context.Context, layout *SurfaceLayout) (*surfaceEncoding, error) {
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "直连图跨纬带适配已取消或超时"}
	}
	if len(b.all) == 0 || len(b.transportRoutes) == 0 || layout == nil || layout.Mode == "explicitTropic" && !layout.ExpandAdjacent {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "直连图跨纬带需要完整路线证据及邻圈扩展"}
	}
	minimum, maximum := bounds(b.all)
	width, height := int(math.Ceil(maximum.x-minimum.x))+1, int(math.Ceil(maximum.y-minimum.y))+1
	if width <= 0 || height <= 0 || height > 32767 || width > 32767 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "直连图占地超出原生区域编码范围"}
	}
	original := append([]*placed(nil), b.all...)
	states := make([]placed, len(original))
	for i, p := range original {
		states[i] = *p
	}
	pathRefs := b.connectedSurfacePathRefs()
	paths := make([][]*placed, len(pathRefs))
	for i, ref := range pathRefs {
		paths[i] = *ref
	}
	owner := b.owner
	restore := func() {
		b.all, b.owner = original, owner
		for i, p := range original {
			*p = states[i]
		}
		for i, ref := range pathRefs {
			*ref = paths[i]
		}
	}
	shift(b.all, -minimum.x, -minimum.y, 0)
	cuts, err := connectedSurfaceCuts(ctx, original, height)
	if err != nil {
		restore()
		return nil, err
	}
	bands := surfaceLatitudeBands(packingSegment(layout))
	starts := []int{}
	for _, band := range bands {
		for start := band.low; start <= band.high; start++ {
			starts = append(starts, start)
		}
	}
	preferred := layout.LatitudeGridIndex
	if layout.Mode == "autoTropic" {
		preferred = -height / 2
	}
	sort.SliceStable(starts, func(i, j int) bool { return absInt(starts[i]-preferred) < absInt(starts[j]-preferred) })
	var lastError error
	checked := 0
	for _, start := range starts {
		if ctx.Err() != nil {
			restore()
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "直连图跨纬带适配已取消或超时"}
		}
		chunks, ok := planConnectedSurfaceChunks(bands, cuts, start, width, height)
		if !ok {
			continue
		}
		// 硬规则验证候选最多 32 个；低成本切面容量检查不计入昂贵候选预算。
		if checked >= 32 {
			break
		}
		checked++
		restore()
		shift(b.all, -minimum.x, -minimum.y, 0)
		encoding, err := b.applyConnectedSurfaceChunks(ctx, layout, chunks, width, original)
		if err == nil {
			var records []blueprint.Building
			records, _, _, err = b.records()
			if err == nil {
				err = encoding.applyRecords(b.all, records)
			}
			if err == nil {
				err = encoding.validateAnchorCandidates(ctx, records)
			}
			if err == nil {
				_, err = validateFinalPlanetRules(ctx, Request{Mode: 1, Setting: b.setting}, encoding, records)
			}
		}
		if err == nil {
			b.owner = owner
			b.warnings = append(b.warnings, fmt.Sprintf("直连图按安全切面扩展到 %d 个原生纬带，逐边补齐新增空行中的传送带；检查 %d 个候选，所有内部连接保留", len(chunks), checked))
			return encoding, nil
		}
		lastError = err
		if !surfaceAnchorRetryable(err) {
			restore()
			return nil, err
		}
	}
	restore()
	if lastError != nil {
		return nil, lastError
	}
	return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("直连图 %dx%d 无法在连续原生纬带中找到不截断设施的完整布局", width, height)}
}

// applyConnectedSurfaceChunks 职责：平移纬向切片并为每条跨片原带边实际插带；使用：候选验证前；边界：不放行蓝图外引用、跨片非带连接或超预算接驳。
func (b *builder) applyConnectedSurfaceChunks(ctx context.Context, layout *SurfaceLayout, chunks []surfaceRectChunk, width int, original []*placed) (*surfaceEncoding, error) {
	primary := 0
	for i := range chunks {
		if chunks[i].segments > chunks[primary].segments {
			primary = i
		}
	}
	anchor := chunks[primary].targetLow
	encoding := &surfaceEncoding{packed: true, areas: make([]blueprint.Area, len(chunks)), latitudeOffsets: make([]int, len(chunks)), longitudeScales: make([]float64, len(chunks)), byBuilding: make(map[*placed]int, len(original)), anchorCandidates: []int{anchor}, plan: makeSurfacePlan(layout.Mode, packingSegment(layout), layout.LatitudeGridIndex, anchor, width, 0)}
	encoding.plan.PrimaryAreaIndex, encoding.plan.AnchorLatitudeGrid = primary, anchor
	encoding.plan.Bands = make([]SurfaceBandPlan, len(chunks))
	for i, chunk := range chunks {
		parent, offset := -1, 0
		if i < primary {
			parent = i + 1
		} else if i > primary {
			parent = i - 1
		}
		if parent >= 0 {
			offset = chunk.targetLow - chunks[parent].targetLow
		}
		high := chunk.targetLow + chunk.sourceHigh - chunk.sourceLow - 1
		if i+1 < len(chunks) {
			high = chunks[i+1].targetLow - 1
		}
		scale := float64(chunk.segments) / float64(chunks[primary].segments)
		areaWidth := int(math.Ceil(float64(width-1)*scale)) + 1
		encoding.areas[i] = blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(chunk.segments), AnchorLocalOffsetY: int16(offset), Width: int16(areaWidth), Height: int16(high - chunk.targetLow + 1)}
		encoding.latitudeOffsets[i], encoding.longitudeScales[i] = chunk.targetLow, scale
		encoding.plan.Bands[i] = SurfaceBandPlan{AreaIndex: i, MinLatitudeGrid: chunk.targetLow, MaxLatitudeGrid: high, LongitudeSegments: chunk.segments}
		encoding.plan.LayoutWidth = max(encoding.plan.LayoutWidth, areaWidth)
	}
	encoding.plan.LayoutHeight = encoding.plan.Bands[len(chunks)-1].MaxLatitudeGrid - chunks[0].targetLow + 1
	for _, p := range original {
		index := sort.Search(len(chunks), func(i int) bool { return float64(chunks[i].sourceHigh) > p.a.y })
		if index >= len(chunks) || p.a.y > float64(chunks[index].sourceHigh-1) || p.b.y < float64(chunks[index].sourceLow) || p.b.y > float64(chunks[index].sourceHigh-1) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "直连图安全切面截断了设施端点"}
		}
		encoding.byBuilding[p] = index
		encoding.plan.Bands[index].BuildingCount++
		shift([]*placed{p}, 0, float64(chunks[index].targetLow-chunks[index].sourceLow), 0)
	}
	bridges := map[*placed]connectedSurfaceBridge{}
	b.owner = -1
	for _, p := range original {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "直连图跨纬带接驳已取消或超时"}
		}
		for _, other := range []*placed{p.input, p.output} {
			if other == nil {
				continue
			}
			area, exists := encoding.byBuilding[other]
			if !exists || area != encoding.byBuilding[p] && (!surfaceIsBelt(p.record.ItemID) || !surfaceIsBelt(other.record.ItemID)) {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "直连图跨纬带存在悬空或不可拆分的设施连接"}
			}
		}
		other := p.output
		if other == nil || encoding.byBuilding[p] == encoding.byBuilding[other] {
			continue
		}
		if absInt(encoding.byBuilding[p]-encoding.byBuilding[other]) != 1 || p.a.x != other.a.x || p.a.z != other.a.z {
			return nil, b.connectedSurfaceRoutingError(p, other, "安全切面含不支持的斜向、跨层或跳区带边，请缩小目标规模或改用平面布局")
		}
		points := surfaceBusBandPath([]vector{p.a, other.a})
		if len(b.all)+len(points)-2 > blueprint.MaxBuildingCount {
			return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "直连图跨纬带接驳超过 20 万建筑预算"}
		}
		if len(points) <= 2 {
			continue
		}
		added := b.byPath(points[1 : len(points)-1])
		connect(p, added[0])
		connect(added[len(added)-1], other)
		if other.input == p {
			other.input = added[len(added)-1]
		}
		for _, belt := range added {
			area := sort.Search(len(chunks), func(i int) bool { return float64(encoding.plan.Bands[i].MaxLatitudeGrid) >= belt.a.y })
			if area >= len(chunks) || belt.a.y < float64(chunks[area].targetLow) {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "直连图接驳带未落在原生纬向网格"}
			}
			encoding.byBuilding[belt] = area
			encoding.plan.Bands[area].BuildingCount++
			if other.a.y < p.a.y {
				belt.record.Yaw, belt.record.Yaw2 = 180, 180
			} else {
				belt.record.Yaw, belt.record.Yaw2 = 0, 0
			}
		}
		bridges[p] = connectedSurfaceBridge{target: other, belts: added}
	}
	for _, ref := range b.connectedSurfacePathRefs() {
		*ref = expandConnectedSurfacePath(*ref, bridges)
	}
	return encoding, nil
}

// connectedSurfaceRoutingError 职责：把不支持的跨区原带边关联回物品和计划流量；使用：有界适配失败诊断；边界：无法匹配规范 lane 时仍报告两端坐标，不伪造物料归属。
func (b *builder) connectedSurfaceRoutingError(from, to *placed, reason string) error {
	endpoints := fmt.Sprintf("端点(%.3f,%.3f,%.3f)→(%.3f,%.3f,%.3f)", from.a.x, from.a.y, from.a.z, to.a.x, to.a.y, to.a.z)
	for _, route := range b.transportRoutes {
		for _, path := range [][]*placed{route.path, route.sharedPath} {
			for i := 1; i < len(path); i++ {
				if path[i-1] == from && path[i] == to {
					return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("mode1物品%d、边%s、%.6f/min：%s；%s", route.edge.ItemID, route.edge.ID, route.rate, reason, endpoints)}
				}
			}
		}
	}
	return &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: fmt.Sprintf("mode1跨Area带边：%s；%s", reason, endpoints)}
}

// expandConnectedSurfacePath 职责：把逐原始带边的新增实体补入路线证据；使用：多 Area 重接后；边界：只扩展与原目标相邻的边，不把不相关带误计入流量。
func expandConnectedSurfacePath(path []*placed, bridges map[*placed]connectedSurfaceBridge) []*placed {
	if len(path) == 0 {
		return path
	}
	out := make([]*placed, 0, len(path))
	for i, p := range path {
		out = append(out, p)
		if bridge, exists := bridges[p]; exists && i+1 < len(path) && path[i+1] == bridge.target {
			out = append(out, bridge.belts...)
		}
	}
	return out
}
