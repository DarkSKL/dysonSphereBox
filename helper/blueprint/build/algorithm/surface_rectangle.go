package build

// 文件职责：将完整矩形按安全设施切面编码成相邻原生纬带 Area。
// 使用：单区容量不足且允许邻圈扩展时，先于闭合产线块装箱尝试。
// 边界：保持经度与连接索引，最终几何预检仍为必需，不隐式切开设施或重接总线。

import (
	"context"
	"fmt"
	"math"
	"sort"

	"DysonSphereBox/helper/blueprint"
)

// surfaceRectChunk 职责：记录矩形布局在源纬向和目标纬带中的一段对应关系；使用：跨 Area 保留原始拓扑；边界：段内经度分段必须恒定，且不能切开单个设施。
type surfaceRectChunk struct {
	sourceLow, sourceHigh int
	targetLow             int
	segments              int
}

// prepareMultiBandRectangleEncoding 职责：把无法放入单一纬圈的完整矩形切成相邻原生 Area；使用：自动球面布局的容量回退；边界：只在设施自身不跨切面时切分，不伪造跨纬带总线重连能力。
// 逻辑流：1. 找到不会切开设施的源切面 => 2. 从候选纬圈逐行匹配恒定分段纬带 => 3. 建立父链并整体平移每段 => 4. 将原建筑连接保留在原索引上。
func (b *builder) prepareMultiBandRectangleEncoding(ctx context.Context, layout *SurfaceLayout, width, height, segment int) (*surfaceEncoding, error) {
	if ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "多纬带矩形布局已取消或超时"}
	}
	if layout.Mode == "explicitTropic" && !layout.ExpandAdjacent {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "指定纬圈禁止邻圈扩展，无法承载超出单圈容量的矩形"}
	}
	if width <= 0 || height <= 0 || width > 32767 || height > 32767 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "多纬带矩形占地必须为正数"}
	}
	bands := surfaceLatitudeBands(segment)
	if len(bands) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "当前行星没有可用纬带"}
	}
	safeCuts := surfaceRectangleSafeCuts(b.all, height)
	// 自动模式以赤道为中心试探；显式扩展模式仍优先接近用户指定纬圈。
	candidates := make([]int, 0, len(bands))
	for _, band := range bands {
		for row := band.low; row <= band.high; row++ {
			// 源 X 以最终主区的经度步长计量；首段可能比主区窄，不能用未缩放宽度在这里提前拒绝。
			candidates = append(candidates, row)
		}
	}
	preferred := layout.LatitudeGridIndex
	if layout.Mode == "autoTropic" {
		preferred = 0
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if layout.Mode == "autoTropic" {
			return absInt(2*candidates[i]+height-1) < absInt(2*candidates[j]+height-1)
		}
		return absInt(candidates[i]-preferred) < absInt(candidates[j]-preferred)
	})
	original, _, _, err := b.records()
	if err != nil {
		return nil, err
	}
	var lastError error
	for _, start := range candidates {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "多纬带矩形布局已取消或超时"}
		}
		chunks, ok := planSurfaceRectangleChunks(bands, safeCuts, start, width, height, segment)
		if !ok || len(chunks) == 0 || len(chunks) > surfaceMaxAreas {
			continue
		}
		primary := 0
		for i := 1; i < len(chunks); i++ {
			if chunks[i].segments > chunks[primary].segments {
				primary = i
			}
		}
		anchor := chunks[primary].targetLow
		encoding := &surfaceEncoding{
			// 矩形跨纬带只是按安全切面平移，不是闭合产线块装箱；保留 packed=false 让最终碰撞失败时还能回退到块装箱。
			packed: false, areas: make([]blueprint.Area, len(chunks)), latitudeOffsets: make([]int, len(chunks)), longitudeScales: make([]float64, len(chunks)),
			byBuilding: make(map[*placed]int, len(b.all)), anchorCandidates: []int{anchor},
			plan: makeSurfacePlan(layout.Mode, segment, preferred, anchor, width, height),
		}
		encoding.plan.AnchorLatitudeGrid, encoding.plan.PrimaryAreaIndex = anchor, primary
		encoding.plan.Bands = make([]SurfaceBandPlan, len(chunks))
		for i, chunk := range chunks {
			scale := float64(chunk.segments) / float64(chunks[primary].segments)
			areaWidth := int(math.Ceil(float64(width-1)*scale)) + 1
			parent, offsetY := -1, 0
			if i < primary {
				parent = i + 1
			} else if i > primary {
				parent = i - 1
			}
			if parent >= 0 {
				offsetY = chunk.targetLow - chunks[parent].targetLow
			}
			encoding.areas[i] = blueprint.Area{
				Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(chunk.segments),
				AnchorLocalOffsetY: int16(offsetY), Width: int16(areaWidth), Height: int16(chunk.sourceHigh - chunk.sourceLow),
			}
			encoding.latitudeOffsets[i] = chunk.sourceLow
			encoding.longitudeScales[i] = scale
			encoding.plan.Bands[i] = SurfaceBandPlan{
				AreaIndex: i, MinLatitudeGrid: chunk.targetLow, MaxLatitudeGrid: chunk.targetLow + chunk.sourceHigh - chunk.sourceLow - 1,
				LongitudeSegments: chunk.segments,
			}
		}
		for _, p := range b.all {
			source := int(math.Floor(p.a.y))
			index := -1
			for i, chunk := range chunks {
				if source >= chunk.sourceLow && source < chunk.sourceHigh {
					index = i
					break
				}
			}
			if index < 0 {
				ok = false
				break
			}
			encoding.byBuilding[p] = index
			encoding.plan.Bands[index].BuildingCount++
		}
		if !ok {
			continue
		}
		encoding.plan.LayoutWidth, encoding.plan.LayoutHeight = width, height
		encoding.plan.LongitudeSegments = chunks[primary].segments
		encoding.plan.RingGridCells = chunks[primary].segments * 5
		// 所有候选都在副本上按最终 float32 记录预检，失败不污染装配坐标或后续闭合块装箱。
		records := append([]blueprint.Building(nil), original...)
		if err := encoding.applyRecords(b.all, records); err != nil {
			continue
		}
		if err := encoding.validateAnchorCandidates(ctx, records); err != nil {
			if !surfaceAnchorRetryable(err) {
				return nil, err
			}
			lastError = err
			continue
		}
		b.warnings = append(b.warnings, fmt.Sprintf("完整矩形超出单纬圈容量，已沿相邻纬圈拆分为 %d 个原生 Area；跨 Area 连接保留原建筑索引并需最终预检", len(chunks)))
		return encoding, nil
	}
	if lastError != nil {
		return nil, lastError
	}
	return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("矩形占地 %dx%d 无法在 %d 个相邻纬带中安全拆分", width, height, surfaceMaxAreas)}
}

// surfaceRectangleSafeCuts 职责：标记两端都能落入同一 Area 坐标闭区间的切面；使用：矩形跨纬带规划；边界：切面 c 的南区最大坐标是 c-1，小数端点落在 (c-1,c) 也不能归入任一区。
func surfaceRectangleSafeCuts(all []*placed, height int) []bool {
	safeCuts := make([]bool, height+1)
	blockedDiff := make([]int, height+2)
	for cut := 1; cut < height; cut++ {
		safeCuts[cut] = true
	}
	for _, p := range all {
		low, high := math.Min(p.a.y, p.b.y), math.Max(p.a.y, p.b.y)
		first := max(1, int(math.Floor(low))+1)
		last := min(height-1, int(math.Ceil(high)))
		if first <= last {
			blockedDiff[first]++
			blockedDiff[last+1]--
		}
	}
	blocked := 0
	for cut := 1; cut < height; cut++ {
		blocked += blockedDiff[cut]
		if blocked > 0 {
			safeCuts[cut] = false
		}
	}
	return safeCuts
}

// planSurfaceRectangleChunks 职责：按目标纬圈的恒定经度段把源矩形切成连续小段；使用：多 Area 矩形编码；边界：safeCuts=false 的位置不可作为 Area 边界。
func planSurfaceRectangleChunks(bands []surfacePackedBand, safeCuts []bool, start, width, height, segment int) ([]surfaceRectChunk, bool) {
	chunks := make([]surfaceRectChunk, 0, 8)
	source, target := 0, start
	for source < height {
		bandIndex := -1
		for i, band := range bands {
			if target >= band.low && target <= band.high {
				bandIndex = i
				break
			}
		}
		if bandIndex < 0 {
			return nil, false
		}
		band := bands[bandIndex]
		if longitudeSegmentCount(target, segment) != band.segments {
			return nil, false
		}
		maxSource := min(height, source+band.high-target+1)
		cut := maxSource
		if cut <= source || cut < height && !safeCuts[cut] {
			return nil, false
		}
		chunks = append(chunks, surfaceRectChunk{sourceLow: source, sourceHigh: cut, targetLow: target, segments: band.segments})
		target += cut - source
		source = cut
	}
	// 先确定最大段数的主区，再逐 Area 按编码器相同的 ceil 规则核对宽度；面积比例不能掩盖整数边界超圈。
	primarySegments := 0
	for _, chunk := range chunks {
		primarySegments = max(primarySegments, chunk.segments)
	}
	for _, chunk := range chunks {
		scale := float64(chunk.segments) / float64(primarySegments)
		areaWidth := int(math.Ceil(float64(width-1)*scale)) + 1
		if areaWidth > chunk.segments*5 {
			return nil, false
		}
	}
	return chunks, true
}
