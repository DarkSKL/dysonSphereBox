package build

// 文件职责：为保持旧平面排版的完整蓝图选择单 Area 粘贴锚点。
// 使用：finish 在统一编号之前计算占地，在最终预检时尝试候选锚点。
// 边界：只编码原生格网，不写入地心坐标；尚未完成的游戏碰撞验证由 SurfacePlan 单独披露。

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"

	"DysonSphereBox/helper/blueprint"
)

// surfaceEncoding 职责：保存整块平面蓝图的区域和候选锚点；使用：finish 的共同出口；边界：建筑仍按原顺序编号。
type surfaceEncoding struct {
	// areas 职责：保存按南到北排列的原生区域；使用：文档写出；边界：父区域朝主区域连接。
	areas []blueprint.Area
	// byBuilding 职责：保存建筑所属区域；使用：最终 float32 记录转换；边界：每个建筑只归属一区。
	byBuilding map[*placed]int
	// latitudeOffsets 职责：保存区域相对主区域的纬向原点；使用：从装配视图转为 Area 局部 Y；边界：单位为细格。
	latitudeOffsets []int
	// plan 职责：保存建议锚点和已用纬带；使用：前端及独立游戏校验；边界：不宣称 Physics 已通过。
	plan *SurfacePlan
	// anchorCandidates 职责：保存按优先级排列的完整平面锚点候选；使用：最终碰撞预检失败时回退；边界：候选只改变粘贴纬圈，不改变建筑坐标。
	anchorCandidates []int
}

// prepareSurfaceEncoding 职责：保持旧平面坐标并生成单一原生 Area 的粘贴锚点建议；使用：球面模式收尾；边界：不拆分产线块、不伪造绝对纬度。
// 逻辑流：1. 计算完整平面 footprint => 2. 选择可容纳的粘贴纬圈 => 3. 仅平移到旧平面原点 => 4. 生成单 Area 与局部坐标映射。
func (b *builder) prepareSurfaceEncoding(layout *SurfaceLayout) (*surfaceEncoding, error) {
	// 球面策略只选择整块蓝图的粘贴锚点；建筑仍沿用旧的全局平面 X/Y 排版。
	// 只做与旧流程相同的整体归零；具体建筑冲突仍须按最终姿态检查，不能由编号推断原因。
	minimum, maximum := bounds(b.all)
	shift(b.all, -minimum.x, -minimum.y, 0)
	width := int(math.Ceil(maximum.x-minimum.x)) + 1
	height := int(math.Ceil(maximum.y-minimum.y)) + 1
	if width <= 0 || height <= 0 || width > 32767 || height > 32767 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "平面蓝图占地必须为正数"}
	}
	segment := layout.GridSegment
	preferred := layout.LatitudeGridIndex
	if layout.Mode == "autoTropic" {
		preferred = 0
	}
	candidates := surfaceAnchorCandidates(layout.Mode, segment, preferred, layout.ExpandAdjacent, width, height)
	if len(candidates) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("平面蓝图占地 %dx%d 无法放入目标纬圈；请启用邻圈扩展或缩小蓝图", width, height)}
	}
	selected := candidates[0]
	longitudeSegments := longitudeSegmentCount(selected, segment)
	encoded := &surfaceEncoding{
		areas:      []blueprint.Area{{Index: 0, ParentIndex: -1, AreaSegments: int16(longitudeSegments), Width: int16(width), Height: int16(height)}},
		byBuilding: make(map[*placed]int, len(b.all)), latitudeOffsets: []int{0}, anchorCandidates: candidates,
	}
	encoded.plan = makeSurfacePlan(layout.Mode, segment, preferred, selected, width, height)
	encoded.plan.AnchorLatitudeGrid, encoded.plan.PrimaryAreaIndex = selected, 0
	encoded.plan.Bands = []SurfaceBandPlan{{AreaIndex: 0, MinLatitudeGrid: selected, MaxLatitudeGrid: selected + height - 1, LongitudeSegments: longitudeSegments, BuildingCount: len(b.all)}}
	encoded.plan.LayoutWidth, encoded.plan.LayoutHeight = width, height
	if !candidateFitsRing(selected, 0, height-1, width, segment) {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "最终平面占地跨越纬圈段或超出环向容量"}
	}
	for _, p := range b.all {
		encoded.byBuilding[p] = 0
	}
	return encoded, nil
}

// selectSurfaceAnchor 职责：为完整平面 footprint 选择首个粘贴纬圈；使用：兼容旧调用和单元测试；边界：不改变建筑坐标，不把邻圈拆成多个 Area。
func selectSurfaceAnchor(mode string, segment, preferred int, expand bool, width, height int) (int, bool) {
	candidates := surfaceAnchorCandidates(mode, segment, preferred, expand, width, height)
	if len(candidates) == 0 {
		return 0, false
	}
	return candidates[0], true
}

// surfaceAnchorCandidates 职责：按硬容量约束生成确定性的整块锚点候选；使用：初选和碰撞失败回退；边界：不以软权重放行跨圈或重排。
func surfaceAnchorCandidates(mode string, segment, preferred int, expand bool, width, height int) []int {
	maxLatitude := segment * 5 / 4
	candidates := make([]int, 0, maxLatitude*2+1)
	seen := make(map[int]struct{}, maxLatitude*2+1)
	for delta := 0; delta <= maxLatitude*2; delta++ {
		for _, candidate := range []int{preferred - delta, preferred + delta} {
			if candidate < -maxLatitude || candidate > maxLatitude {
				continue
			}
			if _, exists := seen[candidate]; exists {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	if mode == "autoTropic" {
		// 自动模式从赤道向两极搜索，不受指定锚点的禁止回退选项限制。
		candidates = candidates[:0]
		for candidate := -maxLatitude; candidate <= maxLatitude; candidate++ {
			candidates = append(candidates, candidate)
		}
		slices.SortStableFunc(candidates, func(left, right int) int {
			return cmp.Or(cmp.Compare(absInt(left), absInt(right)), cmp.Compare(left, right))
		})
	}
	valid := make([]int, 0, len(candidates))
	for i, candidate := range candidates {
		if i > 0 && mode == "explicitTropic" && !expand {
			break
		}
		if candidateFitsRing(candidate, 0, height-1, width, segment) {
			valid = append(valid, candidate)
		}
	}
	return valid
}

// reanchorSurfaceEncoding 职责：同步切换建议纬圈、区域段数和预检状态；使用：候选静态碰撞失败时重试；边界：全部建筑局部坐标、连接与区域尺寸保持不变。
func (encoding *surfaceEncoding) reanchorSurfaceEncoding(latitude int) {
	encoding.plan.AnchorLatitudeGrid = latitude
	encoding.plan.SelectedLatitudeGrid = latitude
	encoding.plan.LatitudeDegrees = float64(latitude) * 360 / float64(encoding.plan.GridSegment*5)
	encoding.plan.ShiftedGridRows = latitude - encoding.plan.PreferredLatitudeGrid
	if len(encoding.plan.Bands) > 0 {
		encoding.plan.Bands[0].MinLatitudeGrid = latitude
		encoding.plan.Bands[0].MaxLatitudeGrid = latitude + int(encoding.areas[0].Height) - 1
		encoding.plan.Bands[0].LongitudeSegments = longitudeSegmentCount(latitude, encoding.plan.GridSegment)
	}
	encoding.plan.LongitudeSegments = longitudeSegmentCount(latitude, encoding.plan.GridSegment)
	encoding.areas[encoding.plan.PrimaryAreaIndex].AreaSegments = int16(encoding.plan.LongitudeSegments)
	encoding.plan.RingGridCells = encoding.plan.LongitudeSegments * 5
	encoding.plan.StaticCollidersChecked = false
	encoding.plan.BeltConnectionsChecked = false
	encoding.plan.CollisionCoverage = "none"
	encoding.plan.ColliderGameVersion = ""
}

// validateAnchorCandidates 职责：对完整平面蓝图逐个执行锚点硬约束预检；使用：finish 的编码出口；边界：空候选、取消或未知规则立即拒绝，只有几何冲突和接线距离失败可以换圈。
func (encoding *surfaceEncoding) validateAnchorCandidates(ctx context.Context, records []blueprint.Building) error {
	var lastError error = &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "没有可验证的整块蓝图锚点"}
	for _, latitude := range encoding.anchorCandidates {
		if ctx.Err() != nil {
			return &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "锚点预检已取消或超时"}
		}
		encoding.reanchorSurfaceEncoding(latitude)
		lastError = encoding.validateBeltDistancesContext(ctx, records)
		if lastError == nil {
			encoding.plan.BeltConnectionsChecked = true
			lastError = encoding.validateStaticColliders(ctx, records)
		}
		if lastError == nil || !surfaceAnchorRetryable(lastError) {
			return lastError
		}
	}
	return lastError
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
