package build

// 文件职责：把不可拆分产线块按有限容量装入游戏恒定经度段纬带。
// 使用：球面生产编码器和装箱测试按块尺寸调用，允许邻圈扩展时分散到多 Area。
// 边界：只依据纬圈分段表与保守余量做几何装箱；实际坐标、碰撞与跨块连接仍由编码出口校验。
// 逻辑流：1. 校验布局与块 => 2. 按经度段合并纬带 => 3. 选择目标纬带并按距离排列扩展顺序 => 4. 逐块确定性 shelf 装箱 => 5. 按南到北重映射输出。

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"DysonSphereBox/helper/blueprint"
)

// surfaceBandEdgeMargin 职责：指定纬带南北两端最少空出的细格行数；使用：装箱可用高度；边界：块间距更大时按半个间距放大。
const surfaceBandEdgeMargin = 2

// surfaceRingMargin 职责：指定环向首尾最少空出的细格数；使用：防止最后一块回绕贴住第一块；边界：块间距更大时取块间距。
const surfaceRingMargin = 4

// surfaceMaxAreas 职责：限制一次装箱可使用的纬带数；使用：对应蓝图 Area 上限；边界：超出直接失败，不合并纬带。
const surfaceMaxAreas = 64

// surfaceBlockSize 职责：描述一个不可拆分产线块的整数占地；使用：planSurfaceBlocks 输入；边界：单位为细格，宽为经向、高为纬向，均须为正。
type surfaceBlockSize struct{ width, height int }

// surfacePackedBand 职责：描述一段连续且经度段数恒定的纬带；使用：生成原生 Area；边界：low/high 为绝对纬向细格闭区间，不含极点行，安全余量在区间内部预留。
type surfacePackedBand struct{ low, high, segments int }

// surfacePackedBlock 职责：记录产线块的落点；使用：整块平移到所属 Area；边界：band 为 surfacePacking.bands 下标，x/y 为相对纬带 low 行与环向起点的局部整数，指块占地左下格。
type surfacePackedBlock struct{ band, x, y int }

// surfacePackPlacement 职责：描述待提交的绝对纬带块落点；使用：装箱跨块几何筛选；边界：不引用可变 shelf 状态，previous 仅包含本次试装已经提交的块。
type surfacePackPlacement struct {
	index int
	band  surfacePackedBand
	x, y  int
}

// surfacePacking 职责：汇总有限容量纬带装箱结果；使用：生产编码器映射块到 Area；边界：几何容量通过不代表游戏碰撞通过。
type surfacePacking struct {
	// segment 职责：保存实际使用的主网格分段数；使用：复现纬圈容量；边界：缺省请求补为 200。
	segment int
	// preferred 职责：保存用户选择的纬向细格；使用：解释偏移；边界：autoTropic 固定为 0。
	preferred int
	// selected 职责：保存第一块实际纬带的建议中心；使用：前端展示；边界：explicit 且首块仍在目标纬带时保留 preferred。
	selected int
	// bands 职责：保存实际使用的纬带；使用：多 Area 编码；边界：按南到北排序且数量不超过 64。
	bands []surfacePackedBand
	// blocks 职责：按输入顺序保存每块落点；使用：整块平移；边界：长度与输入一致。
	blocks []surfacePackedBlock
}

// surfacePackRules 职责：保存一次装箱的间距与余量；使用：shelf 装箱与容量判断；边界：由 gap 推导，调用期间不变。
type surfacePackRules struct {
	gap, edgeMargin, ringMargin int
	// ctx 职责：传播生产取消；使用：逐块、逐行与逐格候选检查；边界：纯装箱测试可以为空。
	ctx context.Context
	// accept 职责：按块检查候选南缘的实际几何；使用：生产装箱；边界：nil 时仅验证矩形容量。
	accept func(int, surfacePackedBand, int) bool
	// acceptPlacement 职责：核对候选与同次试装前序块之间的空间约束；使用：球面塔间距筛选；边界：回调不得修改候选或已放置块。
	acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool
}

// newSurfacePackRules 由块间距推导纬带边缘与环向首尾余量。
// 职责：保证跨纬带相邻块、环向回绕相邻块至少保持块间距。
// 使用：planSurfaceBlocks 校验 gap 后调用。
// 边界：只放大余量，不缩小常量下限。
func newSurfacePackRules(gap int) surfacePackRules {
	return surfacePackRules{gap: gap, edgeMargin: max(surfaceBandEdgeMargin, (gap+1)/2), ringMargin: max(surfaceRingMargin, gap)}
}

// usableWidth 职责：返回纬带扣除环向首尾余量后的可用格数；使用：容量判断；边界：可能为非正数，表示不可用。
func (rules surfacePackRules) usableWidth(band surfacePackedBand) int {
	return band.segments*5 - 2*rules.ringMargin
}

// usableHeight 职责：返回纬带扣除南北余量后的可用行数；使用：容量判断；边界：可能为非正数，表示不可用。
func (rules surfacePackRules) usableHeight(band surfacePackedBand) int {
	return band.high - band.low + 1 - 2*rules.edgeMargin
}

// fitsEmpty 职责：判断空纬带能否容纳单块；使用：单块过大检测与自动候选筛选；边界：不考虑已放置块。
func (rules surfacePackRules) fitsEmpty(band surfacePackedBand, size surfaceBlockSize) bool {
	return size.width <= rules.usableWidth(band) && size.height <= rules.usableHeight(band)
}

// surfaceShelfState 职责：保存单纬带的 shelf 装箱游标；使用：packSurfaceBands 逐块推进；边界：只向前推进，满后不回绕覆盖已放置块。
type surfaceShelfState struct {
	used                         bool
	shelfY, shelfHeight, cursorX int
}

// place 在当前纬带按 shelf 顺序尝试放置一块。
// 职责：先放当前行，放不下再另起一行；成功时推进游标。
// 使用：packSurfaceBands。
// 边界：失败时不修改状态；坐标相对纬带 low 行，已扣除余量。
func (state *surfaceShelfState) place(band surfacePackedBand, size surfaceBlockSize, rules surfacePackRules) (int, int, bool) {
	width, top := rules.usableWidth(band), rules.usableHeight(band)
	if !rules.fitsEmpty(band, size) {
		return 0, 0, false
	}
	if !state.used {
		*state = surfaceShelfState{used: true, shelfHeight: size.height, cursorX: size.width}
		return rules.ringMargin, rules.edgeMargin, true
	}
	if x := state.cursorX + rules.gap; x+size.width <= width && state.shelfY+size.height <= top {
		state.cursorX = x + size.width
		state.shelfHeight = max(state.shelfHeight, size.height)
		return x + rules.ringMargin, state.shelfY + rules.edgeMargin, true
	}
	if y := state.shelfY + state.shelfHeight + rules.gap; y+size.height <= top {
		state.shelfY, state.shelfHeight, state.cursorX = y, size.height, size.width
		return rules.ringMargin, y + rules.edgeMargin, true
	}
	return 0, 0, false
}

// surfacePackAttempt 职责：保存一次按指定顺序装箱的中间结果；使用：自动候选比较与最终输出；边界：band 下标指向全部纬带表，尚未重映射。
type surfacePackAttempt struct {
	blocks []surfacePackedBlock
	states []surfaceShelfState
	opened int
}

// packSurfaceBands 按纬带优先顺序对全部块执行确定性 first-fit shelf 装箱。
// 职责：每块按输入顺序依次尝试 order 中的纬带，未开启的纬带只在区域数未满时开启。
// 使用：planSurfaceBlocks 的 explicit 与 auto 候选。
// 边界：任何块放不下即整体失败，不截断、不跳过；返回的错误说明首个失败块。
func packSurfaceBands(bands []surfacePackedBand, order []int, sizes []surfaceBlockSize, rules surfacePackRules, maxAreas int) (*surfacePackAttempt, error) {
	attempt := &surfacePackAttempt{blocks: make([]surfacePackedBlock, len(sizes)), states: make([]surfaceShelfState, len(bands))}
	previous := make([]surfacePackPlacement, 0, len(sizes))
	for i, size := range sizes {
		if rules.ctx != nil && rules.ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "产线块装箱已取消或超时"}
		}
		placed, limited := false, false
		for _, index := range order {
			state := &attempt.states[index]
			if !state.used && attempt.opened >= maxAreas {
				limited = limited || rules.fitsEmpty(bands[index], size)
				continue
			}
			wasUsed := state.used
			trial := *state
			x, y, ok := trial.place(bands[index], size, rules)
			if ok {
				initialX := x
				x, ok = rules.acceptedX(i, bands[index], x, y, size, previous)
				trial.cursorX += x - initialX
			}
			if !ok && rules.fitsEmpty(bands[index], size) && (rules.accept != nil || rules.acceptPlacement != nil) {
				// 同行向东仍无法满足真实几何时，再逐行尝试新 shelf；不修改失败候选的游标。
				start := 0
				if state.used {
					start = state.shelfY + state.shelfHeight + rules.gap
				}
				for row := start; row+size.height <= rules.usableHeight(bands[index]); row++ {
					if rules.ctx != nil && rules.ctx.Err() != nil {
						return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "产线块装箱已取消或超时"}
					}
					y = row + rules.edgeMargin
					if x, ok = rules.acceptedX(i, bands[index], rules.ringMargin, y, size, previous); ok {
						trial = surfaceShelfState{used: true, shelfY: row, shelfHeight: size.height, cursorX: x - rules.ringMargin + size.width}
						break
					}
				}
			}
			if ok {
				*state = trial
				if !wasUsed {
					attempt.opened++
				}
				attempt.blocks[i] = surfacePackedBlock{band: index, x: x, y: y}
				previous = append(previous, surfacePackPlacement{index: i, band: bands[index], x: x, y: y})
				placed = true
				break
			}
		}
		if placed {
			continue
		}
		if limited {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("第 %d 个产线块需要超过 %d 个纬带区域，请拆分布局", i+1, maxAreas)}
		}
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("可用纬带已装满，无法放置第 %d 个产线块（%dx%d），请启用邻圈扩展或拆分布局", i+1, size.width, size.height)}
	}
	if rules.ctx != nil && rules.ctx.Err() != nil {
		return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "产线块装箱已取消或超时"}
	}
	return attempt, nil
}

// acceptedX 职责：在当前 shelf 的剩余宽度内选择首个几何可行整数落点；使用：跨块塔间距候选；边界：行内几何失败不重复扫描经度，取消不发布落点。
func (rules surfacePackRules) acceptedX(index int, band surfacePackedBand, x, y int, size surfaceBlockSize, previous []surfacePackPlacement) (int, bool) {
	if rules.accept != nil && !rules.accept(index, band, y) {
		return x, false
	}
	for candidate := x; candidate+size.width <= rules.ringMargin+rules.usableWidth(band); candidate++ {
		if rules.ctx != nil && rules.ctx.Err() != nil {
			return x, false
		}
		if rules.acceptPlacement == nil || rules.acceptPlacement(surfacePackPlacement{index: index, band: band, x: candidate, y: y}, previous) {
			return candidate, true
		}
	}
	return x, false
}

// surfaceLatitudeBands 把经度段数相同的连续纬向细格行合并为纬带。
// 职责：给出可作为单个原生 Area 的恒定段数纬带候选。
// 使用：planSurfaceBlocks。
// 边界：排除南北极点行；结果按南到北排序，南北对称的同段数纬带保持为两段。
func surfaceLatitudeBands(segment int) []surfacePackedBand {
	maxLatitudeGrid := segment * 5 / 4
	var bands []surfacePackedBand
	for row := -maxLatitudeGrid + 1; row <= maxLatitudeGrid-1; row++ {
		segments := longitudeSegmentCount(row, segment)
		if last := len(bands) - 1; last >= 0 && bands[last].segments == segments && bands[last].high == row-1 {
			bands[last].high = row
			continue
		}
		bands = append(bands, surfacePackedBand{low: row, high: row, segments: segments})
	}
	return bands
}

// bandCenter 职责：返回纬带中心行；使用：自动选择与 selected 输出；边界：偶数行数取靠近赤道一行，保证南北对称纬带同距。
func bandCenter(band surfacePackedBand) int { return (band.low + band.high) / 2 }

// bandRowGap 职责：返回纬向细格到纬带最近行的距离；使用：扩展顺序同距离裁决；边界：位于纬带内时为 0。
func bandRowGap(band surfacePackedBand, row int) int {
	if row < band.low {
		return band.low - row
	}
	if row > band.high {
		return row - band.high
	}
	return 0
}

// surfaceBandOrder 生成以目标纬带为起点的逐圈扩展顺序。
// 职责：按纬带序号距离由近及远排列，同距离时更靠近参考行、更靠近赤道、更南者优先。
// 使用：explicit 扩展与 auto 候选试装。
// 边界：expand 为 false 时只返回目标纬带。
func surfaceBandOrder(bands []surfacePackedBand, target, reference int, expand bool) []int {
	if !expand {
		return []int{target}
	}
	order := make([]int, len(bands))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(left, right int) int {
		return cmp.Or(
			cmp.Compare(absInt(left-target), absInt(right-target)),
			cmp.Compare(bandRowGap(bands[left], reference), bandRowGap(bands[right], reference)),
			cmp.Compare(absInt(bandCenter(bands[left])), absInt(bandCenter(bands[right]))),
			cmp.Compare(bands[left].low, bands[right].low),
		)
	})
	return order
}

// planSurfaceBlocks 把产线块按有限容量装入恒定经度段纬带。
// 职责：校验布局、选择目标纬带，在禁止扩展时严格单纬带装箱，允许扩展时逐块向邻近纬带分散。
// 使用：生产编码器及独立算法测试；gap 由调用方按物流塔半径等给出。
// 边界：nil/planar 不应进入本函数并返回错误；单块超容量明确失败；不声明未知游戏碰撞规则已满足。
func planSurfaceBlocks(layout *SurfaceLayout, blocks []surfaceBlockSize, gap int) (*surfacePacking, error) {
	return planSurfaceBlocksAccepted(layout, blocks, gap, nil)
}

// planSurfaceBlocksAccepted 职责：在相同装箱硬容量上加入候选块几何筛选；使用：生产多 Area 编码；边界：回调不修改块与纬带状态。
func planSurfaceBlocksAccepted(layout *SurfaceLayout, blocks []surfaceBlockSize, gap int, accept func(int, surfacePackedBand, int) bool) (*surfacePacking, error) {
	return planSurfaceBlocksChecked(context.Background(), layout, blocks, gap, accept, nil)
}

// planSurfaceBlocksChecked 职责：统一行内与跨块几何筛选并传播取消；使用：生产球面块装箱；边界：失败候选不提交，原有无几何回调装箱顺序保持不变。
func planSurfaceBlocksChecked(ctx context.Context, layout *SurfaceLayout, blocks []surfaceBlockSize, gap int, accept func(int, surfacePackedBand, int) bool, acceptPlacement func(surfacePackPlacement, []surfacePackPlacement) bool) (*surfacePacking, error) {
	if layout == nil || layout.Mode == "" || layout.Mode == "planar" {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "平面布局不能进入纬带装箱"}
	}
	if layout.Mode != "autoTropic" && layout.Mode != "explicitTropic" {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "未知行星布局模式"}
	}
	segment := layout.GridSegment
	if segment == 0 {
		segment = defaultSurfaceGridSegment
	}
	if segment < 40 || segment > 512 || segment%4 != 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "目标行星主网格分段数必须为 40 到 512 的 4 的倍数"}
	}
	preferred := 0
	if layout.Mode == "explicitTropic" {
		preferred = layout.LatitudeGridIndex
	}
	if maxLatitudeGrid := segment * 5 / 4; preferred < -maxLatitudeGrid || preferred > maxLatitudeGrid {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬圈索引超出行星范围"}
	}
	if gap < 2 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "产线块间距至少为 2 格"}
	}
	if len(blocks) == 0 {
		return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: "纬带装箱至少需要一个产线块"}
	}
	rules := newSurfacePackRules(gap)
	rules.ctx, rules.accept, rules.acceptPlacement = ctx, accept, acceptPlacement
	bands := surfaceLatitudeBands(segment)
	for i, size := range blocks {
		if size.width <= 0 || size.height <= 0 {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE", Message: fmt.Sprintf("第 %d 个产线块占地必须为正数", i+1)}
		}
		if !slices.ContainsFunc(bands, func(band surfacePackedBand) bool { return rules.fitsEmpty(band, size) }) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("第 %d 个产线块（%dx%d）超过任一纬带的安全容量，不能截断机器，请拆分布局", i+1, size.width, size.height)}
		}
	}
	return packSurfaceLayout(layout.Mode, segment, preferred, layout.ExpandAdjacent, bands, blocks, rules, surfaceMaxAreas)
}

// packSurfaceLayout 按模式选择目标纬带并输出南到北重映射结果。
// 职责：explicit 固定包含首选行的纬带；auto 对每个可容纳首块的纬带试装并比较。
// 使用：planSurfaceBlocks 校验后调用；测试可注入合成纬带验证区域上限。
// 边界：输入已校验；不修改 bands 与 blocks。
func packSurfaceLayout(mode string, segment, preferred int, expand bool, bands []surfacePackedBand, blocks []surfaceBlockSize, rules surfacePackRules, maxAreas int) (*surfacePacking, error) {
	var attempt *surfacePackAttempt
	target := -1
	if mode == "explicitTropic" {
		// 首选行落在极点等不属于任何纬带时，回落到行距最近的纬带，preferred 仍保留用户输入。
		for i, band := range bands {
			if target < 0 || bandRowGap(band, preferred) < bandRowGap(bands[target], preferred) {
				target = i
			}
		}
		if !expand && !rules.fitsEmpty(bands[target], blocks[0]) {
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "指定纬带无法容纳首个产线块，请启用邻圈扩展或拆分布局"}
		}
		var err error
		if attempt, err = packSurfaceBands(bands, surfaceBandOrder(bands, target, preferred, expand), blocks, rules, maxAreas); err != nil {
			return nil, err
		}
	} else {
		// 自动模式按目标纬带安全容量、剩余行余量、所用纬带数、离赤道距离依次比较。
		bestCapacity, bestSlack := 0, 0
		for i, band := range bands {
			if !rules.fitsEmpty(band, blocks[0]) {
				continue
			}
			candidate, err := packSurfaceBands(bands, surfaceBandOrder(bands, i, bandCenter(band), expand), blocks, rules, maxAreas)
			if err != nil {
				if failure, ok := err.(*blueprint.Error); !ok || failure.Code != "GEN_E_SURFACE_CAPACITY" {
					return nil, err
				}
				continue
			}
			state := candidate.states[i]
			capacity := rules.usableWidth(band) * rules.usableHeight(band)
			slack := rules.edgeMargin + rules.usableHeight(band) - state.shelfY - state.shelfHeight
			better := attempt == nil || cmp.Or(
				cmp.Compare(bestCapacity, capacity),
				cmp.Compare(bestSlack, slack),
				cmp.Compare(candidate.opened, attempt.opened),
				cmp.Compare(absInt(bandCenter(band)), absInt(bandCenter(bands[target]))),
			) < 0
			if better {
				attempt, target, bestCapacity, bestSlack = candidate, i, capacity, slack
			}
		}
		if attempt == nil {
			if expand {
				return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: fmt.Sprintf("产线块总量超过 %d 个纬带区域的安全容量，请拆分布局", maxAreas)}
			}
			return nil, &blueprint.Error{Code: "GEN_E_SURFACE_CAPACITY", Message: "没有单一纬带可容纳全部产线块，请启用邻圈扩展或拆分布局"}
		}
	}

	// 只输出实际使用的纬带，并按南到北重映射块的纬带下标。
	used := make([]int, 0, attempt.opened)
	for i, state := range attempt.states {
		if state.used {
			used = append(used, i)
		}
	}
	slices.SortFunc(used, func(left, right int) int { return cmp.Compare(bands[left].low, bands[right].low) })
	remap := make(map[int]int, len(used))
	packing := &surfacePacking{segment: segment, preferred: preferred, bands: make([]surfacePackedBand, len(used)), blocks: make([]surfacePackedBlock, len(blocks))}
	for i, index := range used {
		remap[index] = i
		packing.bands[i] = bands[index]
	}
	for i, block := range attempt.blocks {
		packing.blocks[i] = surfacePackedBlock{band: remap[block.band], x: block.x, y: block.y}
	}
	first := attempt.blocks[0].band
	packing.selected = bandCenter(bands[first])
	if mode == "explicitTropic" && first == target && bandRowGap(bands[first], preferred) == 0 {
		packing.selected = preferred
	}
	return packing, nil
}
