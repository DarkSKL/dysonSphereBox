package build

// 文件职责：枚举和应用显式球面候选规格，使纬带/镜像/占地选择脱离生成调度。
// 使用：工作流的球面规划及输入适配工具；边界：不运行布局、不重试、不选择获胜结果，真实可行性仍需实体终检。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// SurfaceCandidateSpec 职责：完整描述一次球面输入变换；使用：实例保存并传给单候选核心；边界：只允许所选区域内的窗口或子环，不扩大用户跨带授权。
type SurfaceCandidateSpec struct {
	// OffsetWindow 职责：显式选择许可内偏移窗；使用：扩带中心大dense1；边界：false保持旧居中行为。
	OffsetWindow bool `json:"offsetWindow,omitempty"`
	// Strategy 职责：区分严格选区、原生窗口和原生子环；使用：显式调度分支；边界：未知值拒绝。
	Strategy string `json:"strategy"`
	// ExtentRows 职责：限定窗口纬向细格行数；使用：原生窗口；边界：不是世界长度。
	ExtentRows int `json:"extentRows"`
	// AdjacentDepth 职责：保存显式邻带层数；使用：候选许可与指纹；边界：0/1沿用旧depth1。
	AdjacentDepth int `json:"adjacentDepth,omitempty"`
	// LocalRectangleAspect 职责：独立非极区矩形候选的布局比例目标；使用：真实宽度预算；边界：零保留旧策略，不替代最终形状规则。
	LocalRectangleAspect float64 `json:"localRectangleAspect,omitempty"`
	// TurnLongRows 职责：独立比较封闭生产长行竖放；使用：窄经向候选；边界：不旋转共享或跨单元接线。
	TurnLongRows bool `json:"turnLongRows,omitempty"`
	// Mirror/SameSideRefinery 职责：保存设施镜像及炼油方向策略；使用：单次装配；边界：不改变物料或生产设备。
	Mirror           bool `json:"mirror"`
	SameSideRefinery bool `json:"sameSideRefinery"`
	// PreciseFootprint 职责：启用真实原型占位；使用：明确的几何备用组；边界：不替代最终碰撞规则。
	PreciseFootprint bool `json:"preciseFootprint"`
	// ExpandAdjacent 职责：保存本规格使用的跨带许可；使用：范围检查；边界：不能超过原请求授权。
	ExpandAdjacent bool `json:"expandAdjacent"`
	// BandLow/BandHigh/BandSegments 职责：标识真实子环范围；使用：native-band策略；边界：来自共同网格表，不能自定义物理格距。
	BandLow      int `json:"bandLow"`
	BandHigh     int `json:"bandHigh"`
	BandSegments int `json:"bandSegments"`
}

// SurfaceSearchOptions 职责：接受方案中的球面枚举策略参数；使用：枚举工具显式传参；边界：没有隐藏执行、回退或候选择优。
type SurfaceSearchOptions struct {
	// LargestOffsetFirst 职责：先尝试当前许可内最大的偏移窗口；使用：跨带大厂容量策略；边界：默认false，只重排已有窗口，不增加范围或省略单带验证。
	LargestOffsetFirst bool `json:"largestOffsetFirst"`
	// AllowOffsetWindows 职责：启用许可内偏移窗口候选；使用：显式YAML参数；边界：默认false，不能扩大原跨带授权。
	AllowOffsetWindows bool `json:"allowOffsetWindows"`
	// LocalRectangleAspects 职责：列出独立矩形布局的软宽高目标；使用：YAML调节非极区直连大厂；边界：0保留原策略，不能覆盖最终物理形状阈值。
	LocalRectangleAspects []float64 `json:"localRectangleAspects"`
	// CompareTurnedRows 职责：在原水平行之后比较完整封闭长行转向；使用：矩形装箱；边界：不旋转跨单元连接，不省略实体校验。
	CompareTurnedRows bool `json:"compareTurnedRows"`
	// AdjacentDepths 职责：给出有界邻带备用层数；使用：扩带候选轮转；边界：缺省[1]，未授权扩带时不消费，仅接受1至3。
	AdjacentDepths []int `json:"adjacentDepths"`
	// Strategy 职责：选择严格选区或原生候选；使用：静态工具参数；边界：未知值不回退。
	Strategy string `json:"strategy" enum:"strict,native"`
	// ExpandAdjacent 职责：显式选择原生相邻纬带；使用：native 策略；边界：还须真实输入授权，strict 不消费此字段。
	ExpandAdjacent bool `json:"expandAdjacent"`
	// PreciseFootprint 职责：启用真实原型占位；使用：两个策略均可；边界：不改变物理能力。
	PreciseFootprint bool `json:"preciseFootprint"`
	// IncludeBandFallback 职责：声明原生子环备用；使用：未扩带的 native；边界：strict 中作为未激活策略保留。
	IncludeBandFallback bool `json:"includeBandFallback"`
	// PolarWindowRows 职责：提供极区窗口细格行数；使用：原生极区且未扩带时；边界：每项6..112，空表是否缺失由真实纬带决定。
	PolarWindowRows []int `json:"polarWindowRows" unit:"grid-row"`
}

// DefaultSurfaceSearchOptions 职责：集中声明原生球面搜索的默认策略；使用：工具默认值与独立核心调用；边界：调用者修改返回切片不会污染其它任务。
func DefaultSurfaceSearchOptions() SurfaceSearchOptions {
	return SurfaceSearchOptions{LocalRectangleAspects: []float64{4.5, 3, 2.4, 1.8, 1.5, 1.2, 0}, CompareTurnedRows: true, AdjacentDepths: []int{1}, Strategy: "native", IncludeBandFallback: true, PolarWindowRows: []int{24, 32, 40, 48, 56, 64}}
}

// ValidateSurfaceSearchOptions 职责：纯校验球面策略与窗口行数；使用：ToolSpec 及枚举参数入口；边界：不读取行星或权限，空窗口表留给极区真实输入判定，未激活备用表不丢弃。
func ValidateSurfaceSearchOptions(options SurfaceSearchOptions) error {
	if options.Strategy != "strict" && options.Strategy != "native" {
		return fmt.Errorf("strategy 只接受 strict 或 native")
	}
	if options.Strategy == "strict" && options.ExpandAdjacent {
		return fmt.Errorf("strict 策略不消费 expandAdjacent，扩带必须使用 native")
	}
	for _, depth := range options.AdjacentDepths {
		if depth < 1 || depth > 3 {
			return fmt.Errorf("adjacentDepths只接受1至3")
		}
	}
	if len(options.LocalRectangleAspects) > 16 {
		return fmt.Errorf("localRectangleAspects最多16项")
	}
	for _, aspect := range options.LocalRectangleAspects {
		if aspect != 0 && (!finitePositive(aspect) || aspect > 8) {
			return fmt.Errorf("localRectangleAspects必须为0或不超过8的正有限数")
		}
	}
	for _, rows := range options.PolarWindowRows {
		if err := validatePolarWindowRows(rows); err != nil {
			return err
		}
	}
	return nil
}

// validatePolarWindowRows 职责：校验方案提供的极区搜索预算；使用：静态参数和极区显式规格；边界：不用于从真实非极区选区推导的窗口，最终实体仍须在许可区域内。
func validatePolarWindowRows(rows int) error {
	if rows < 6 || rows > 112 {
		return fmt.Errorf("球面窗口行数 %d 超出6..112细格", rows)
	}
	return nil
}

// EnumerateSurfaceCandidates 职责：按稳定次序列出许可范围内的显式球面规格；使用：引擎枚举前；边界：不运行生产候选，也不将预算可用视为布局成功。
func enumerateSurfaceCandidatesAtDepth(ctx context.Context, input *PreparedInput, options SurfaceSearchOptions) ([]SurfaceCandidateSpec, error) {
	if input == nil {
		return nil, fmt.Errorf("缺少规范生成输入")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateSurfaceSearchOptions(options); err != nil {
		return nil, err
	}
	req := input.request
	if options.ExpandAdjacent && (req.SurfaceLayout == nil || !req.SurfaceLayout.ExpandAdjacent) {
		return nil, fmt.Errorf("球面策略不能扩大请求的跨纬带许可")
	}
	if options.Strategy == "strict" {
		return []SurfaceCandidateSpec{{Strategy: "strict", PreciseFootprint: options.PreciseFootprint}}, nil
	}
	if options.Strategy != "native" || req.SurfaceLayout == nil || req.SurfaceLayout.GridSegment != 200 {
		return nil, fmt.Errorf("原生球面策略需要已核对的行星选区")
	}
	selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
	if !ok {
		return nil, fmt.Errorf("原生球面策略缺少有效纬带")
	}
	depth, tagDepth := 1, 0
	if options.ExpandAdjacent && len(options.AdjacentDepths) > 0 {
		depth = options.AdjacentDepths[0]
	}
	if depth > 1 {
		tagDepth = depth
	}
	bands := denseLatitudeAllowedBands(selection, options.ExpandAdjacent, depth)
	if len(bands) == 0 {
		return nil, fmt.Errorf("所选区域没有原生纬带")
	}
	ctx = input.algorithmContext(ctx)
	var extents []int
	switch {
	case options.ExpandAdjacent:
		extents = denseAdjacentFrameExtents(selection, bands, req.Plan.MachineCount, req.shape.AdjacentWindowScales)
	case selection.tier == 5:
		if len(options.PolarWindowRows) == 0 {
			return nil, fmt.Errorf("极区窗口策略不能为空")
		}
		extents = append([]int(nil), options.PolarWindowRows...)
	default:
		extents = denseTargetFrameExtents(bands, req.search.NativeWindowRows)
	}
	// 普通大厂的完整带段与行间接线占地更宽；优先验证许可内宽窗口，避免先耗尽预算在细长的窄窗口结果上。
	// 仅改变既有窗口顺序，不增加许可区域、不删除原窗口，最终形状与物理规则仍逐项执行。
	if !options.ExpandAdjacent && denseNativeLargeRequest(ctx, req) || options.ExpandAdjacent && req.Density == densityStandard && req.Plan.MachineCount >= req.search.LargeLayoutThreshold {
		sort.Sort(sort.Reverse(sort.IntSlice(extents)))
	}
	seenExtents := map[int]bool{}
	specs := []SurfaceCandidateSpec{}
	orientations := []bool{false}
	largeEdge := selection.tier < 5 && (denseNativeLargeRequest(ctx, req) || options.ExpandAdjacent) && selection.position != "center"
	if req.Mode >= 2 && req.Mode <= 4 && (largeEdge || selection.hemisphere > 0 && selection.position == "north" || selection.hemisphere < 0 && selection.position == "south") {
		orientations = []bool{false, true}
		if selection.hemisphere > 0 {
			orientations = []bool{true, false}
		}
	}
	for _, rows := range extents {
		// 非极区和扩带窗口来自真实区域，不受极区参数上限约束；赤道原生环本身可宽于112格。
		if selection.tier != 5 || options.ExpandAdjacent {
			limit := bands[len(bands)-1].high - bands[0].low + 1
			if rows < 6 || rows > limit {
				return nil, fmt.Errorf("球面窗口行数 %d 超出所选区域6..%d细格", rows, limit)
			}
		}
		if seenExtents[rows] {
			continue
		}
		seenExtents[rows] = true
		for _, mirror := range orientations {
			base := SurfaceCandidateSpec{Strategy: "native-window", ExtentRows: rows, Mirror: mirror, PreciseFootprint: options.PreciseFootprint, ExpandAdjacent: options.ExpandAdjacent, AdjacentDepth: tagDepth}
			specs = append(specs, denseSurfaceWindowModifiers(req, selection, options, base)...)
		}
	}
	if options.IncludeBandFallback && !options.ExpandAdjacent {
		preferred := (bands[0].low + bands[len(bands)-1].high) / 2
		if selection.position == "south" {
			preferred = bands[0].low
		}
		if selection.position == "north" {
			preferred = bands[len(bands)-1].high
		}
		sort.SliceStable(bands, func(i, j int) bool {
			return absInt(bandCenter(bands[i])-preferred) < absInt(bandCenter(bands[j])-preferred)
		})
		for _, band := range bands {
			if band.high-band.low+1 < 6 {
				continue
			}
			for _, sameSide := range refineryLayoutVariants(req) {
				specs = append(specs, SurfaceCandidateSpec{Strategy: "native-band", SameSideRefinery: sameSide, PreciseFootprint: options.PreciseFootprint, BandLow: band.low, BandHigh: band.high, BandSegments: band.segments})
			}
		}
	}
	extra, err := denseOffsetSurfaceSpecs(req, selection, bands, options, tagDepth)
	if err != nil {
		return nil, err
	}
	return deduplicateDenseLocalRectangleBounds(req, denseInterleaveOffsetSpecs(specs, extra, denseAdjacentIdealRows(req.Plan.MachineCount) > maxDenseCenteredExtent(selection, bands)))
}

// EnumerateSurfaceCandidates 职责：按深度轮转保留每层主窗口，再轮转其余布局变体。
// 使用：显式球面枚举；边界：默认仅旧depth1，严格或无扩带许可不启用深度备用，任何深度仍须完整最终规则。
func EnumerateSurfaceCandidates(ctx context.Context, input *PreparedInput, options SurfaceSearchOptions) ([]SurfaceCandidateSpec, error) {
	if err := ValidateSurfaceSearchOptions(options); err != nil {
		return nil, err
	}
	if !options.ExpandAdjacent || options.Strategy == "strict" {
		return enumerateSurfaceCandidatesAtDepth(ctx, input, options)
	}
	depths := options.AdjacentDepths
	if len(depths) == 0 {
		depths = []int{1}
	}
	groups := [][]SurfaceCandidateSpec{}
	seen := map[int]bool{}
	for _, depth := range depths {
		if seen[depth] {
			continue
		}
		seen[depth] = true
		one := options
		one.AdjacentDepths = []int{depth}
		values, err := enumerateSurfaceCandidatesAtDepth(ctx, input, one)
		if err != nil {
			return nil, err
		}
		groups = append(groups, values)
	}
	result := []SurfaceCandidateSpec{}
	for i := 0; ; i++ {
		added := false
		for _, group := range groups {
			if i < len(group) {
				result = append(result, group[i])
				added = true
			}
		}
		if !added {
			break
		}
	}
	return result, nil
}

// PrepareSurfaceCandidate 职责：把一个合法规格应用为新的不可变领域输入；使用：候选工作区的输入步骤；边界：不共享可变请求，不运行生成，不持有父步骤context。
func PrepareSurfaceCandidate(ctx context.Context, input *PreparedInput, spec SurfaceCandidateSpec) (*PreparedInput, error) {
	if input == nil {
		return nil, fmt.Errorf("缺少规范生成输入")
	}
	if spec.AdjacentDepth < 0 || spec.AdjacentDepth > 3 {
		return nil, fmt.Errorf("非法邻带深度")
	}
	if spec.AdjacentDepth == 1 {
		spec.AdjacentDepth = 0
	}
	options := DefaultSurfaceSearchOptions()
	options.AllowOffsetWindows = spec.OffsetWindow
	if spec.OffsetWindow {
		if input.request.SurfaceLayout == nil {
			return nil, fmt.Errorf("偏移窗口缺少原选区")
		}
		selection, ok := densityLatitudeSelectionFor(input.request.SurfaceLayout.LatitudeZone)
		if !ok || spec.Strategy != "native-window" || !denseOffsetWindowEligible(input.request, selection, spec.ExpandAdjacent, spec.Mirror) {
			return nil, fmt.Errorf("偏移窗口不属于请求授权范围")
		}
	}
	options.LocalRectangleAspects = []float64{spec.LocalRectangleAspect}
	options.CompareTurnedRows = spec.TurnLongRows
	if spec.AdjacentDepth > 1 {
		options.AdjacentDepths = []int{spec.AdjacentDepth}
	}
	options.ExpandAdjacent, options.PreciseFootprint = spec.ExpandAdjacent, spec.PreciseFootprint
	if spec.Strategy == "strict" {
		options.Strategy = "strict"
	}
	if spec.Strategy == "native-window" && input.request.SurfaceLayout != nil {
		selection, ok := densityLatitudeSelectionFor(input.request.SurfaceLayout.LatitudeZone)
		if ok && selection.tier == 5 && !spec.ExpandAdjacent {
			options.PolarWindowRows = []int{spec.ExtentRows}
		}
	}
	allowed, err := EnumerateSurfaceCandidates(ctx, input, options)
	if err != nil {
		return nil, err
	}
	valid := false
	for _, value := range allowed {
		if value == spec {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("球面候选规格不属于请求许可的网格或方向")
	}
	request, err := cloneInputRequest(input.request)
	if err != nil {
		return nil, err
	}
	if request.SurfaceLayout != nil {
		request.SurfaceLayout.ExpandAdjacent = spec.ExpandAdjacent
		request.SurfaceLayout.adjacentDepth = spec.AdjacentDepth
		request.SurfaceLayout.offsetWindow = spec.OffsetWindow
		request.SurfaceLayout.nativeBand = nil
		request.SurfaceLayout.polarRoot = spec.Strategy == "native-window"
		if spec.Strategy == "native-band" {
			band := surfacePackedBand{low: spec.BandLow, high: spec.BandHigh, segments: spec.BandSegments}
			request.SurfaceLayout.nativeBand = &band
			request.SurfaceLayout.LatitudeGridIndex = bandCenter(band)
		}
	}
	result := *input
	result.request, result.geometry = request, spec
	raw, err := json.Marshal(struct {
		Parent   string
		Geometry SurfaceCandidateSpec
	}{input.fingerprint, spec})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	result.fingerprint = hex.EncodeToString(digest[:])
	result.bytes += int64(len(raw)) * 4
	return &result, nil
}

// algorithmContext 职责：从显式领域选项恢复一次调用的几何参数；使用：每个枚举和单次求解的入口；边界：仅派生当前ctx，不延长寿命、不调度、不跨步骤保存句柄。
func (input *PreparedInput) algorithmContext(ctx context.Context) context.Context {
	ctx = context.WithValue(ctx, denseCollisionKey{}, input.request.Density == densityDense)
	ctx = context.WithValue(ctx, denseNativeWorkloadKey{}, input.nativeWorkload)
	if input.request.SurfaceLayout != nil {
		ctx = context.WithValue(ctx, densityLatitudeKey{}, input.request.SurfaceLayout.LatitudeZone)
	}
	spec := input.geometry
	ctx = context.WithValue(ctx, denseAdjacentDepthKey{}, spec.AdjacentDepth)
	ctx = context.WithValue(ctx, denseOffsetWindowKey{}, spec.OffsetWindow)
	ctx = context.WithValue(ctx, denseLocalRectangleAspectKey{}, spec.LocalRectangleAspect)
	ctx = context.WithValue(ctx, denseLocalRectangleTurnKey{}, spec.TurnLongRows)
	ctx = context.WithValue(ctx, densityPrototypeFootprintKey{}, spec.PreciseFootprint)
	ctx = context.WithValue(ctx, densityPolarRootKey{}, spec.Strategy == "native-window")
	ctx = context.WithValue(ctx, densityPolarMirrorKey{}, spec.Mirror)
	ctx = context.WithValue(ctx, refinerySameSideKey{}, spec.SameSideRefinery)
	ctx = context.WithValue(ctx, denseAdjacentLatitudeKey{}, spec.ExpandAdjacent)
	if spec.Strategy == "native-window" {
		ctx = context.WithValue(ctx, densityNativeBandKey{}, surfacePackedBand{low: 0, high: spec.ExtentRows - 1, segments: 200})
	}
	if spec.Strategy == "native-band" {
		ctx = context.WithValue(ctx, densityNativeBandKey{}, surfacePackedBand{low: spec.BandLow, high: spec.BandHigh, segments: spec.BandSegments})
	}
	if spec.ExpandAdjacent {
		ctx = context.WithValue(ctx, denseBalancedFrameKey{}, true)
		ctx = context.WithValue(ctx, denseUnitPowerRequestedKey{}, input.request.Setting.NeedPower == nil || *input.request.Setting.NeedPower)
		if selected, ok := densityLatitudeSelectionFor(input.request.SurfaceLayout.LatitudeZone); ok && selected.tier >= 3 {
			ctx = context.WithValue(ctx, denseGroundTracksKey{}, true)
		}
	}
	return ctx
}
