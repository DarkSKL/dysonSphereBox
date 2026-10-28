package build

// 文件职责：把有限纬带装箱结果转换为原生多 Area 元数据与建筑局部坐标。
// 使用：Generate 保留完整产线块，finish 在统一编号之前执行。
// 边界：只编码原生格网，不写入地心坐标；尚未完成的游戏碰撞验证由 SurfacePlan 单独披露。

import (
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
)

// surfaceEncoding 职责：保存装箱到蓝图记录的映射；使用：finish 的共同出口；边界：建筑仍按原顺序编号。
type surfaceEncoding struct {
	// areas 职责：保存按南到北排列的原生区域；使用：文档写出；边界：父区域朝主区域连接。
	areas []blueprint.Area
	// byBuilding 职责：保存建筑所属区域；使用：最终 float32 记录转换；边界：每个建筑只归属一区。
	byBuilding map[*placed]int
	// latitudeOffsets 职责：保存区域相对主区域的纬向原点；使用：从装配视图转为 Area 局部 Y；边界：单位为细格。
	latitudeOffsets []int
	// plan 职责：保存建议锚点和已用纬带；使用：前端及独立游戏校验；边界：不宣称 Physics 已通过。
	plan *SurfacePlan
}

// prepareSurfaceEncoding 职责：在保持块内拓扑的前提下装箱并生成原生 Area；使用：球面模式收尾；边界：跨块引用、漏记对象和单块超容量均拒绝。
// 逻辑流：1. 验证块与引用 => 2. 规划有限纬带 => 3. 整块平移 => 4. 生成父子 Area 与局部坐标映射。
func (b *builder) prepareSurfaceEncoding(layout *SurfaceLayout) (*surfaceEncoding, error) {
	blocks := b.surfaceBlocks
	if len(blocks) == 0 || b.setting.NeedCoaterNet {
		// 总线和公共供剂网络目前保持整个连通结构，不能拆开以后伪造跨纬带接线。
		blocks = [][]*placed{b.all}
	}
	sizes := make([]surfaceBlockSize, len(blocks))
	minimums := make([]vector, len(blocks))
	owners := make(map[*placed]int, len(b.all))
	for i, block := range blocks {
		if len(block) == 0 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬带装箱不能包含空产线块"}
		}
		minimum, maximum := bounds(block)
		minimums[i] = minimum
		sizes[i] = surfaceBlockSize{width: int(math.Ceil(maximum.x-minimum.x)) + 1, height: int(math.Ceil(maximum.y-minimum.y)) + 1}
		for _, p := range block {
			if _, exists := owners[p]; exists {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬带装箱的建筑重复归属"}
			}
			owners[p] = i
		}
	}
	if len(owners) != len(b.all) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬带装箱遗漏了辅助建筑"}
	}
	for _, p := range b.all {
		owner, exists := owners[p]
		if !exists {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "建筑未登记到纬带装箱"}
		}
		for _, connected := range []*placed{p.input, p.output} {
			if connected == nil {
				continue
			}
			other, exists := owners[connected]
			if !exists || other != owner {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_ROUTING", Message: "产线块包含尚未适配的跨块连接，不能分配到不同纬带"}
			}
		}
	}
	packing, err := planSurfaceBlocks(layout, sizes, max(2, int(math.Ceil(b.setting.LocalStationHitRadius))))
	if err != nil {
		return nil, err
	}
	if len(packing.bands) == 1 {
		// 单 Area 不会触发游戏 SnapTropic，应收紧到实际占用并围绕目标纬度居中。
		band := &packing.bands[0]
		low, high := int(^uint(0)>>1), 0
		for i, block := range packing.blocks {
			low = min(low, block.y)
			high = max(high, block.y+sizes[i].height)
		}
		height := high - low + 4
		start := max(band.low, min(packing.selected-(height-1)/2, band.high-height+1))
		for i := range packing.blocks {
			packing.blocks[i].y = packing.blocks[i].y - low + 2
		}
		band.low, band.high = start, start+height-1
	}
	primary := packing.blocks[0].band
	rootLatitude := packing.bands[primary].low
	encoded := &surfaceEncoding{areas: make([]blueprint.Area, len(packing.bands)), byBuilding: make(map[*placed]int, len(b.all)), latitudeOffsets: make([]int, len(packing.bands))}
	encoded.plan = makeSurfacePlan(layout.Mode, packing.segment, packing.preferred, packing.selected, 0, 0)
	encoded.plan.AnchorLatitudeGrid, encoded.plan.PrimaryAreaIndex = rootLatitude, primary
	encoded.plan.Bands = make([]SurfaceBandPlan, len(packing.bands))
	for i, band := range packing.bands {
		parent := i - 1
		if i < primary {
			parent = i + 1
		}
		offset := 0
		if i == primary {
			parent = -1
		} else {
			offset = band.low - packing.bands[parent].low
		}
		encoded.areas[i] = blueprint.Area{Index: int8(i), ParentIndex: int8(parent), AreaSegments: int16(band.segments), AnchorLocalOffsetY: int16(offset), Width: 1, Height: int16(band.high - band.low + 1)}
		encoded.latitudeOffsets[i] = band.low - rootLatitude
		encoded.plan.Bands[i] = SurfaceBandPlan{AreaIndex: i, MinLatitudeGrid: band.low, MaxLatitudeGrid: band.high, LongitudeSegments: band.segments}
	}
	for i, position := range packing.blocks {
		band := position.band
		shift(blocks[i], float64(position.x)-minimums[i].x, float64(position.y+encoded.latitudeOffsets[band])-minimums[i].y, 0)
		width := position.x + sizes[i].width
		if width > int(encoded.areas[band].Width) {
			encoded.areas[band].Width = int16(width)
		}
		for _, p := range blocks[i] {
			encoded.byBuilding[p] = band
		}
		encoded.plan.Bands[band].BuildingCount += len(blocks[i])
	}
	for _, area := range encoded.areas {
		encoded.plan.LayoutWidth = max(encoded.plan.LayoutWidth, int(area.Width))
		if !candidateFitsRing(packing.bands[area.Index].low, 0, int(area.Height)-1, int(area.Width), packing.segment) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("区域 %d 的最终占地跨越纬带或超出环向容量", area.Index)}
		}
	}
	encoded.plan.LayoutHeight = packing.bands[len(packing.bands)-1].high - packing.bands[0].low + 1
	return encoded, nil
}

// applyRecords 职责：将最终编号记录转换为所属 Area 的 float32 局部坐标；使用：编码前；边界：不改变连接编号或原始装配视图，量化后越界拒绝。
func (encoding *surfaceEncoding) applyRecords(all []*placed, records []blueprint.Building) error {
	for i, p := range all {
		band, exists := encoding.byBuilding[p]
		if !exists {
			return &blueprint.Error{Code: "GEN_E_SURFACE", Message: "建筑缺少所属区域"}
		}
		r := &records[i]
		r.AreaIndex = int8(band)
		r.LocalOffset.Y = float32(p.a.y - float64(encoding.latitudeOffsets[band]))
		r.LocalOffset2.Y = float32(p.b.y - float64(encoding.latitudeOffsets[band]))
		area := encoding.areas[band]
		for _, point := range []blueprint.Point{r.LocalOffset, r.LocalOffset2} {
			if point.X < 0 || point.Y < 0 || point.X > float32(area.Width-1) || point.Y > float32(area.Height-1) {
				return &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("建筑 %d 的最终坐标超出区域 %d", i, band)}
			}
		}
	}
	return nil
}
