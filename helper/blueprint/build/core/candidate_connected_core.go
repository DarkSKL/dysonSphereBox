package build

// 文件职责：提供直连候选的显式枚举、单次装配及只读评分边界。
// 使用：工作流直接枚举和消费同一种完整规格，不经过旧变体结构或编号转换。
// 边界：不依赖工作流、YAML 或反射，不选择胜者，不编码，不保留跨步骤可变装配器。

import (
	"context"
	"fmt"
	"math"

	mode1common "DysonSphereBox/helper/blueprint/build/planning/connected"
)

// ConnectedInputGroup 职责：标识就近与集中入口两组；使用：引擎决定备用条件；边界：枚举不自动执行备用。
type ConnectedInputGroup string

// 入口组职责：给工作流提供稳定业务值；使用：显式分组；边界：不表示候选已成功。
const (
	// ConnectedNearbyInputs 职责：标识就近原料入口；使用：首选组；边界：是否优先由引擎决定。
	ConnectedNearbyInputs ConnectedInputGroup = "nearby"
	// ConnectedConcentratedInputs 职责：标识集中兼容入口；使用：备用组；边界：不在 Solve 内自动切换。
	ConnectedConcentratedInputs ConnectedInputGroup = "concentrated"
)

// ConnectedObjectiveBound 职责：保存引擎显式提供的已验证目标下界；使用：单候选安全剪枝；边界：按值复制，不引用共享可变胜者，也不代表当前候选通过。
type ConnectedObjectiveBound struct {
	// Enabled 职责：声明本次是否应用目标界；使用：已存在完整可行候选时；边界：默认不剪枝。
	Enabled bool `yaml:"enabled"`
	// Metrics 职责：保存已验证候选的指标值；使用：固定单区域面积下界比较；边界：来源及是否允许提前终止由引擎负责。
	Metrics LayoutMetrics `yaml:"metrics"`
}

// ConnectedCandidateSpec 职责：描述一个完整且可解释的直连空间策略；使用：单次求解；边界：只包含值，不持有实体、胜者或调度回调。
type ConnectedCandidateSpec struct {
	// SpatialRows 职责：启用共同的独立短行装箱；使用：普通模式近方形候选；边界：物理间距仍由原密度策略决定。
	SpatialRows bool `json:"spatialRows"`
	// ObjectiveBound 职责：保存当前步骤的明确剪枝界；使用：可选安全剪枝；边界：不从全局胜者读取。
	ObjectiveBound ConnectedObjectiveBound `yaml:"objectiveBound"`
	// Limit 职责：候选软高度；使用：当前候选；边界：不得改变机器或流量。
	Limit float64 `yaml:"limit"`
	// Compact 职责：真实轮廓压缩；使用：当前候选；边界：保留碰撞规则。
	Compact bool `yaml:"compact"`
	// Horizontal 职责：水平轮廓回填；使用：当前候选；边界：仅自动纬带生效。
	Horizontal bool `yaml:"horizontal"`
	// GlobalPower 职责：统一供电策略；使用：当前候选；边界：不改变供电开关。
	GlobalPower bool `yaml:"globalPower"`
	// LegacyInputs 职责：集中兼容入口；使用：当前候选；边界：与就近入口分组执行。
	LegacyInputs bool `yaml:"legacyInputs"`
	// DenseOrder 职责：选择面积降序或依赖顺序、面积或接线距离代价；使用：0面积/面积，1依赖/面积，2面积/接线，3依赖/接线；边界：是真实排序策略，不是复合变体编号。
	DenseOrder int `yaml:"denseOrder"`
	// DenseSpan 职责：密铺行跨度；使用：当前候选；边界：不扩大单带容量。
	DenseSpan float64 `yaml:"denseSpan"`
	// Refine 职责：小数位姿搜索；使用：当前候选；边界：不绕过终检。
	Refine bool `yaml:"refine"`
	// Street 职责：连续容量街道；使用：当前候选；边界：须满足真实资格。
	Street bool `yaml:"street"`
	// StreetRotation 职责：街道朝向；使用：当前候选；边界：取值为零至三。
	StreetRotation int `yaml:"streetRotation"`
	// StreetGroupLimit 职责：街道容量组约束；使用：当前候选；边界：保留自动及单组语义。
	StreetGroupLimit int `yaml:"streetGroupLimit"`
	// PairedOutputs 职责：双侧过滤；使用：当前候选；边界：不改变产物数量。
	PairedOutputs bool `yaml:"pairedOutputs"`
	// CapacityFill 职责：容量优先分组；使用：当前候选；边界：不改变设备档位。
	CapacityFill bool `yaml:"capacityFill"`
	// ProducerSplits 职责：独立产物分流树；使用：当前候选；边界：保留真实端口验证。
	ProducerSplits bool `yaml:"producerSplits"`
	// SplitterModel 职责：真实分流器模型；使用：当前候选；边界：零表示默认模型。
	SplitterModel int16 `yaml:"splitterModel"`
	// BentRows 职责：短行折行；使用：当前候选；边界：保留真实净空。
	BentRows bool `yaml:"bentRows"`
	// TranslateCompact 职责：独立行组移近；使用：当前候选；边界：不缩减碰撞盒。
	TranslateCompact bool `yaml:"translateCompact"`
	// MaterialNetworks 职责：原生大厂物料网络；使用：当前候选；边界：资格来自实际输入。
	MaterialNetworks bool `yaml:"materialNetworks"`
}

// InputGroup 职责：从显式入口策略导出稳定候选组；使用：引擎分组；边界：不改变候选。
func (spec ConnectedCandidateSpec) InputGroup() ConnectedInputGroup {
	if spec.LegacyInputs {
		return ConnectedConcentratedInputs
	}
	return ConnectedNearbyInputs
}

// ConnectedSearchOptions 职责：提供可由配置完整传入的候选生成参数；使用：先取默认工厂再显式覆盖；边界：零值不等同默认，核心不偷偷补值。
type ConnectedSearchOptions struct {
	// SurfaceHeightSeeds 职责：为原生矩形大厂追加实际窗口高度的基础种子；使用：显式配置；边界：默认关闭，保留旧候选，不展开拓扑组合。
	SurfaceHeightSeeds bool `json:"surfaceHeightSeeds" yaml:"surfaceHeightSeeds"`
	// SharedRows 职责：将已配置跨度交给共同短行装箱；使用：普通模式的有限形状候选；边界：不展开密铺专属拓扑或修改设备。
	SharedRows bool `json:"sharedRows"`
	// HeightLimits 职责：明确高度列表；使用：枚举阶段；边界：空时仅在允许测量时计算。
	HeightLimits []float64 `json:"heightLimits" yaml:"heightLimits" unit:"grid"`
	// MeasureHeightLimits 职责：允许按真实局部装配测量高度；使用：枚举阶段；边界：不是第二次完整求解。
	MeasureHeightLimits bool `json:"measureHeightLimits" yaml:"measureHeightLimits"`
	// Layout 职责：普通布局生成参数；使用：枚举阶段；边界：不补隐含默认。
	Layout mode1common.SearchOptions `json:"layout" yaml:"layout"`
	// InputGroups 职责：入口候选组及顺序；使用：枚举阶段；边界：备用触发由引擎决定。
	InputGroups []ConnectedInputGroup `json:"inputGroups" yaml:"inputGroups"`
	// DenseOrders 职责：密铺排序循环；使用：枚举阶段；边界：空时拒绝密铺枚举。
	DenseOrders []int `json:"denseOrders" yaml:"denseOrders"`
	// DenseSpans 职责：密铺跨度循环；使用：枚举阶段；边界：不修改额定容量。
	DenseSpans []float64 `json:"denseSpans" yaml:"denseSpans" unit:"grid"`
	// DenseSpanRepeat 职责：相邻排序共享跨度的次数；使用：密铺枚举；边界：密铺循环必须为正，普通请求未配置该循环时可为零。
	DenseSpanRepeat int `json:"denseSpanRepeat" yaml:"denseSpanRepeat" min:"0"`
	// RefineOrders 职责：额外微调排序；使用：枚举阶段；边界：空时不增加微调。
	RefineOrders []int `json:"refineOrders" yaml:"refineOrders"`
	// Street 职责：开启合资格街道；使用：枚举阶段；边界：资格仍由真实能力判断。
	Street bool `json:"street" yaml:"street"`
	// StreetGroups 职责：街道分组策略；使用：枚举阶段；边界：不合并未知容量键。
	StreetGroups []int `json:"streetGroups" yaml:"streetGroups"`
	// StreetRotations 职责：街道朝向；使用：枚举阶段；边界：取值零至三。
	StreetRotations []int `json:"streetRotations" yaml:"streetRotations"`
	// CapacityFill 职责：生成容量优先候选；使用：枚举阶段；边界：保留均衡候选。
	CapacityFill bool `json:"capacityFill" yaml:"capacityFill"`
	// CapacityStreetPreference 职责：容量街道优先组选取顺序；使用：枚举阶段；边界：每朝向仅选首个有效组。
	CapacityStreetPreference []int `json:"capacityStreetPreference" yaml:"capacityStreetPreference"`
	// ProducerSplits 职责：生成产物分流树候选；使用：枚举阶段；边界：只扩展容量优先候选。
	ProducerSplits bool `json:"producerSplits" yaml:"producerSplits"`
	// SplitterModels 职责：分流器模型列表；使用：枚举阶段；边界：必须是真实模型。
	SplitterModels []int16 `json:"splitterModels" yaml:"splitterModels"`
	// SplitterCapacityLimit 职责：分流树扩展容量候选数上限；使用：枚举阶段；边界：零不扩展。
	SplitterCapacityLimit int `json:"splitterCapacityLimit" yaml:"splitterCapacityLimit" min:"0"`
	// BentRows 职责：生成折行候选；使用：枚举阶段；边界：只适用密铺直连。
	BentRows bool `json:"bentRows" yaml:"bentRows"`
	// BentSpans 职责：折行短行跨度；使用：枚举阶段；边界：须为正数。
	BentSpans []float64 `json:"bentSpans" yaml:"bentSpans" unit:"grid"`
	// BentCapacityModes 职责：折行容量策略；使用：枚举阶段；边界：保留配置顺序。
	BentCapacityModes []bool `json:"bentCapacityModes" yaml:"bentCapacityModes"`
	// PairedOutputs 职责：追加合资格双侧过滤候选；使用：枚举阶段；边界：不替换单侧候选。
	PairedOutputs bool `json:"pairedOutputs" yaml:"pairedOutputs"`
	// TranslateCompact 职责：追加独立移近候选；使用：枚举阶段；边界：保留原候选。
	TranslateCompact bool `json:"translateCompact" yaml:"translateCompact"`
	// MaterialNetworkThreshold 职责：原生直连物料网络机器阈值；使用：枚举阶段；边界：只影响策略资格。
	MaterialNetworkThreshold int `json:"materialNetworkThreshold" yaml:"materialNetworkThreshold" min:"0" unit:"machine"`
	// FlowAwareRowThreshold 职责：控制多单元大厂的物料图排序优先；使用：基础候选的真实最少分行数达到阈值才启用；边界：零关闭，只调整优先级，不删除策略或改变物理约束。
	FlowAwareRowThreshold int `json:"flowAwareRowThreshold" yaml:"flowAwareRowThreshold" min:"0" unit:"row"`
}

// DefaultConnectedSearchOptions 职责：集中定义与原有完整搜索一致的全部策略默认；使用：工具参数默认工厂；边界：每次返回独立切片，不执行任何求解。
func DefaultConnectedSearchOptions() ConnectedSearchOptions {
	return ConnectedSearchOptions{
		MeasureHeightLimits: true,
		Layout:              mode1common.SearchOptions{CompactModes: []bool{false, true}, WideLimitFactor: 2, Horizontal: true, HorizontalFactor: 1.6, GlobalPower: true, PowerFactor: 4.0 / 3, PowerQuantum: 10, MinimumPowerLimit: 10},
		InputGroups:         []ConnectedInputGroup{ConnectedNearbyInputs, ConnectedConcentratedInputs},
		DenseOrders:         []int{0, 1, 2, 3}, DenseSpans: []float64{12, 18, 24, 32}, DenseSpanRepeat: 2, RefineOrders: []int{2, 3},
		Street: true, StreetGroups: []int{0, 2, -1}, StreetRotations: []int{0, 1, 2, 3},
		CapacityFill: true, CapacityStreetPreference: []int{2, 0}, ProducerSplits: true, SplitterModels: []int16{38, 39, 40}, SplitterCapacityLimit: 8,
		BentRows: true, BentSpans: []float64{12, 18}, BentCapacityModes: []bool{false, true}, PairedOutputs: true, TranslateCompact: true, MaterialNetworkThreshold: 90,
		FlowAwareRowThreshold: 128,
	}
}

// ValidateConnectedSearchOptions 职责：拒绝非法排序、模型及非有限工作格预算；使用：ToolSpec 参数校验和核心入口复用；边界：纯函数，不推断密度、不补默认值，空扩展列表保持主动禁用语义。
func ValidateConnectedSearchOptions(options ConnectedSearchOptions) error {
	values := append([]float64{}, options.HeightLimits...)
	values = append(values, options.DenseSpans...)
	values = append(values, options.BentSpans...)
	for _, limit := range values {
		if limit <= 0 || math.IsNaN(limit) || math.IsInf(limit, 0) {
			return fmt.Errorf("直连候选高度或跨度必须为有限正数")
		}
	}
	if err := mode1common.ValidateSearchOptions(options.Layout); err != nil {
		return err
	}
	if options.DenseSpanRepeat < 0 {
		return fmt.Errorf("denseSpanRepeat 不能为负")
	}
	for _, order := range options.DenseOrders {
		if order < 0 || order > 3 {
			return fmt.Errorf("未知密铺排序 %d", order)
		}
	}
	for _, order := range options.RefineOrders {
		if order < 0 || order > 3 {
			return fmt.Errorf("未知微调排序 %d", order)
		}
	}
	for _, turn := range options.StreetRotations {
		if turn < 0 || turn > 3 {
			return fmt.Errorf("未知街道朝向 %d", turn)
		}
	}
	for _, group := range append(append([]int{}, options.StreetGroups...), options.CapacityStreetPreference...) {
		if group != 0 && group != 2 && group != -1 {
			return fmt.Errorf("未知街道分组 %d", group)
		}
	}
	for _, model := range options.SplitterModels {
		if model != 38 && model != 39 && model != 40 {
			return fmt.Errorf("未知分流器模型 %d", model)
		}
	}
	if options.SplitterCapacityLimit < 0 || options.MaterialNetworkThreshold < 0 || options.FlowAwareRowThreshold < 0 {
		return fmt.Errorf("直连策略预算及资格阈值不能为负")
	}
	for _, group := range options.InputGroups {
		if group != ConnectedNearbyInputs && group != ConnectedConcentratedInputs {
			return fmt.Errorf("未知直连入口组 %q", group)
		}
	}
	if options.ProducerSplits && !options.CapacityFill {
		return fmt.Errorf("producerSplits 需要启用 capacityFill；折行分流由 bentRows 独立控制")
	}
	return nil
}

// validateConnectedSearchOptions 职责：在纯参数校验后补充密铺必需输入；使用：已有枚举入口；边界：不复制静态规则，普通请求可不配置密铺循环。
func validateConnectedSearchOptions(options ConnectedSearchOptions, dense bool) error {
	if err := ValidateConnectedSearchOptions(options); err != nil {
		return err
	}
	if dense && (len(options.DenseOrders) == 0 || len(options.DenseSpans) == 0 || options.DenseSpanRepeat <= 0) {
		return fmt.Errorf("密铺直连必须明确排序、跨度及重复次数")
	}
	return nil
}

// EnumerateConnectedCandidates 职责：枚举可解释且容量等价去重后的两组规格；使用：工作流显式生成计划；边界：只探测局部容量，不执行完整候选、不编码、不选择备用。
func EnumerateConnectedCandidates(ctx context.Context, input *PreparedInput, options ConnectedSearchOptions) (out []ConnectedCandidateSpec, err error) {
	if input == nil || input.dataset == nil || input.graph == nil {
		return nil, fmt.Errorf("直连输入快照为空")
	}
	ctx = input.algorithmContext(ctx)
	defer func() {
		if cause := ctx.Err(); cause != nil {
			out = nil
			err = cause
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.request.Mode != 1 {
		return nil, fmt.Errorf("直连候选只支持模式1")
	}
	if err := validateConnectedSearchOptions(options, input.request.Density == densityDense); err != nil {
		return nil, err
	}
	if denseNativeLatitude(input.request) && input.request.Plan.MachineCount >= options.MaterialNetworkThreshold {
		ctx = context.WithValue(ctx, denseMaterialNetworksKey{}, true)
	}
	assembly, err := input.newAssembly(ctx, nil)
	if err != nil {
		return nil, err
	}
	limits := append([]float64{}, options.HeightLimits...)
	if len(limits) == 0 && options.MeasureHeightLimits {
		limits, err = assembly.layoutHeightCandidates(ctx, input.request, input.dataset, true)
		if err != nil {
			return nil, err
		}
	}
	var keys, capacityKeys map[int]string
	paired := false
	if input.request.Density == densityDense && options.Street && denseConnectedStreetRequestEligible(input.request) && len(limits) > 0 {
		keys, paired, err = assembly.denseConnectedCandidateFacts(ctx, input.request, input.dataset, false)
		if err != nil {
			return nil, err
		}
		if options.CapacityFill {
			capacityKeys, _, err = assembly.denseConnectedCandidateFacts(ctx, input.request, input.dataset, true)
			if err != nil {
				return nil, err
			}
		}
	}
	planned := planConnectedCandidates(input.request, limits, options, paired)
	if options.SurfaceHeightSeeds {
		planned = connectedSurfaceHeightSeeds(ctx, input, planned)
	}
	planned = deduplicateConnectedCandidates(ctx, input.request, planned, keys, capacityKeys)
	planned, err = deduplicateConnectedRows(ctx, input, planned, options.FlowAwareRowThreshold)
	if err != nil {
		return nil, err
	}
	specs := make([]ConnectedCandidateSpec, 0, len(planned)*len(options.InputGroups))
	seenGroups := map[ConnectedInputGroup]bool{}
	for _, group := range options.InputGroups {
		if seenGroups[group] {
			continue
		}
		seenGroups[group] = true
		for _, spec := range planned {
			spec.LegacyInputs = group == ConnectedConcentratedInputs
			spec.MaterialNetworks = denseNativeLatitude(input.request) && input.request.Plan.MachineCount >= options.MaterialNetworkThreshold
			specs = append(specs, spec)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return specs, nil
}

// assembleConnectedCandidate 职责：只创建一个短时装配器并执行一次指定装配；使用：单次 Solve 的局部装配；边界：不读取全局胜者，不枚举、不编码，失败撤销该候选预览。
func assembleConnectedCandidate(ctx context.Context, input *PreparedInput, spec ConnectedCandidateSpec, preview *generationPreview) (*builder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Limit <= 0 || math.IsNaN(spec.Limit) || math.IsInf(spec.Limit, 0) {
		return nil, fmt.Errorf("直连候选高度必须为有限正数")
	}
	if spec.DenseSpan < 0 || math.IsNaN(spec.DenseSpan) || math.IsInf(spec.DenseSpan, 0) || spec.DenseOrder < 0 || spec.DenseOrder > 3 || spec.StreetRotation < 0 || spec.StreetRotation > 3 || spec.StreetGroupLimit != 0 && spec.StreetGroupLimit != 2 && spec.StreetGroupLimit != -1 {
		return nil, fmt.Errorf("直连密铺参数无效")
	}
	if spec.SplitterModel != 0 && spec.SplitterModel != 38 && spec.SplitterModel != 39 && spec.SplitterModel != 40 {
		return nil, fmt.Errorf("直连分流器模型无效")
	}
	if spec.ObjectiveBound.Enabled {
		metrics := spec.ObjectiveBound.Metrics
		if metrics.EnvelopeArea <= 0 || metrics.EnvelopeAspectRatio < 1 || math.IsNaN(metrics.EnvelopeArea) || math.IsInf(metrics.EnvelopeArea, 0) || math.IsNaN(metrics.EnvelopeAspectRatio) || math.IsInf(metrics.EnvelopeAspectRatio, 0) {
			return nil, fmt.Errorf("直连目标界必须包含有限正面积及有效长宽比")
		}
	}
	candidate, err := input.newAssembly(ctx, preview)
	if err != nil {
		return nil, err
	}
	if spec.ObjectiveBound.Enabled {
		objective := spec.ObjectiveBound.Metrics
		candidate.connectedBestObjective = &objective
	}
	if spec.SpatialRows {
		if input.request.Density != densityStandard {
			return nil, fmt.Errorf("普通行阵不能改变密铺策略")
		}
		ctx = candidate.configureStandardRows(ctx)
	}
	candidate.connectedCompactBelts = true
	candidate.connectedLegacyInputs = spec.LegacyInputs
	candidate.connectedHorizontalPacking = spec.Horizontal
	candidate.connectedGlobalPower = spec.GlobalPower
	if input.request.Density == densityDense || input.geometry.Strategy == "native-window" || spec.SpatialRows {
		if spec.MaterialNetworks {
			ctx = context.WithValue(ctx, denseMaterialNetworksKey{}, true)
		}
		if spec.TranslateCompact {
			ctx = context.WithValue(ctx, denseSpatialCompactKey{}, true)
		}
		candidate.denseSpatial, candidate.denseSpatialOrder = true, spec.DenseOrder
		candidate.denseMaxSpan, candidate.denseRefine = spec.DenseSpan, spec.Refine
		candidate.denseStreetCandidate, candidate.denseStreetRotation = spec.Street, spec.StreetRotation
		candidate.denseStreetGroupLimit = spec.StreetGroupLimit
		candidate.densePairedOutputs, candidate.denseCapacityFill = spec.PairedOutputs, spec.CapacityFill
		if spec.ProducerSplits {
			ctx = context.WithValue(ctx, denseProducerSplittersKey{}, true)
		}
		if spec.SplitterModel != 0 {
			ctx = context.WithValue(ctx, splitterGateModelKey{}, spec.SplitterModel)
		}
		if spec.BentRows {
			ctx = context.WithValue(ctx, denseBentRowsKey{}, true)
		}
	}
	candidate.generationCtx = ctx
	candidate.beginPreview()
	if err := candidate.prepareConnectedRows(ctx, input.request, input.dataset, spec.Limit, spec.Compact); err != nil {
		candidate.rollbackPreview()
		return candidate, &ConnectedPreparationError{Cause: err}
	}
	if err := candidate.completePreparedConnected(ctx, input.request); err != nil {
		candidate.rollbackPreview()
		return candidate, err
	}
	if err := ctx.Err(); err != nil {
		candidate.rollbackPreview()
		return candidate, err
	}
	return candidate, nil
}

// AssembleConnectedCandidate 职责：只装配一个显式规格并冻结装配图；使用：工作流装配节点；边界：后续定位、补电、绑定、规则和编码由独立节点执行。
func AssembleConnectedCandidate(ctx context.Context, input *PreparedInput, spec ConnectedCandidateSpec) (AssemblySnapshot, error) {
	if input == nil || input.dataset == nil || input.graph == nil {
		return AssemblySnapshot{}, fmt.Errorf("直连输入快照为空")
	}
	ctx = input.algorithmContext(ctx)
	if err := ctx.Err(); err != nil {
		return AssemblySnapshot{}, err
	}
	if input.request.Mode != 1 {
		return AssemblySnapshot{}, fmt.Errorf("直连候选只支持模式1")
	}
	candidate, err := assembleConnectedCandidate(ctx, input, spec, previewFromContext(ctx).fork())
	if err != nil {
		if ctx.Err() != nil {
			return AssemblySnapshot{}, ctx.Err()
		}
		return AssemblySnapshot{}, err
	}
	geometry, err := freezeAssembly(candidate.generationCtx, candidate, input.request, input.dataset)
	if err != nil {
		candidate.rollbackPreview()
		if ctx.Err() != nil {
			return AssemblySnapshot{}, ctx.Err()
		}
		return AssemblySnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		candidate.rollbackPreview()
		return AssemblySnapshot{}, err
	}
	return geometry, nil
}

// ConnectedCandidateMetrics 职责：读取完成终检的候选指标值；使用：引擎评分；边界：不接受未验证几何，不暴露可变报告。
func ConnectedCandidateMetrics(validation FinalValidation) (LayoutMetrics, error) {
	if validation.data == nil || validation.data.logistics == nil {
		return LayoutMetrics{}, fmt.Errorf("直连候选缺少最终验证指标")
	}
	return validation.data.logistics.Metrics, nil
}

// BetterConnectedCandidate 职责：按现有普通或密铺目标及纬带边缘优先级比较；使用：引擎对完整可行候选稳定归约；边界：只读证据，不保存胜者，同分返回 false。
func BetterConnectedCandidate(input *PreparedInput, a, b FinalValidation) (bool, error) {
	if input == nil {
		return false, fmt.Errorf("直连输入快照为空")
	}
	first, err := ConnectedCandidateMetrics(a)
	if err != nil {
		return false, err
	}
	second, err := ConnectedCandidateMetrics(b)
	if err != nil {
		return false, err
	}
	if LayoutShapeAccepted(first) != LayoutShapeAccepted(second) {
		return LayoutShapeAccepted(first), nil
	}
	if edge := compareDenseLatitudeEdge(input.request.SurfaceLayout, a.data.surfacePlan, b.data.surfacePlan); edge != 0 {
		return edge < 0, nil
	}
	return BetterLayoutCandidate(first, 0, second, 0), nil
}
