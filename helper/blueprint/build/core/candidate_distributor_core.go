package build

// 文件职责：提供普通、密铺和内部 mode5 配送的具名枚举与单次装配边界。
// 使用：领域工具先枚举，再由外部引擎按组调度并对 GeometrySnapshot 终检与评分。
// 边界：不依赖工作流、YAML 或反射，不编码、不选择获胜者；局部预算探针不产生成功蓝图。

import (
	"context"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	distributor "DysonSphereBox/helper/blueprint/build/planning/distributor"
)

// DistributorTopology 职责：标识真实配送装配拓扑；使用：枚举和单次求解；边界：内部 mode5 不扩大公开模式目录。
type DistributorTopology string

// 职责：维护配送拓扑身份；使用：规格与装配分派；边界：不得用不透明变体编号代替。
const (
	// DistributorStandardField 职责：标识普通间距全场黑盒；使用：mode2 standard；边界：仍使用普通碰撞规则。
	DistributorStandardField DistributorTopology = "standard-field"
	// DistributorDenseBlackbox 职责：标识密铺内部直连黑盒；使用：mode2 dense；边界：保留真实反馈及边界设施。
	DistributorDenseBlackbox DistributorTopology = "dense-blackbox"
	// DistributorDenseDetached 职责：标识机器与配送设施分离拓扑；使用：合资格密铺请求；边界：不支持黑盒专属回流和缓存。
	DistributorDenseDetached DistributorTopology = "dense-detached"
	// DistributorInternalMode5 职责：保留内部逐行配送布局；使用：mode5 兼容流程；边界：不可当作 mode2 失败退路。
	DistributorInternalMode5 DistributorTopology = "internal-mode5"
)

// DistributorFallback 职责：解释备用组的失败触发条件；使用：引擎在组归约后决定是否调度；边界：枚举与求解均不执行触发逻辑。
type DistributorFallback struct {
	// AfterGroups 职责：列出可触发此组的前置组；使用：任意一个前置组满足下面条件即可；边界：同一备用组只运行一次。
	AfterGroups []string `json:"afterGroups"`
	// RequireNoSuccess 职责：要求该前置组没有完整终检成功；使用：备用调度；边界：几何定稿不等于成功。
	RequireNoSuccess bool `json:"requireNoSuccess"`
	// RequireFailure 职责：要求真实装配、定稿或终检失败；使用：排除空组和纯优化剪枝；边界：取消不触发备用。
	RequireFailure bool `json:"requireFailure"`
	// ExcludedFailureCode 职责：排除设施严格劣解证明；使用：失败归类；边界：此类剪枝不证明几何不可行。
	ExcludedFailureCode string `json:"excludedFailureCode"`
}

// DistributorCandidateSpec 职责：完整解释一个配送候选；使用：外部隔离候选及单次求解；边界：纯值参数，不保存实体、Builder 或已有获胜者。
type DistributorCandidateSpec struct {
	// ID 职责：提供枚举内稳定身份；使用：诊断；边界：不参与几何运算。
	ID string `json:"id"`
	// GroupID 职责：标识同拓扑策略组；使用：引擎归约及备用条件；边界：与街道容量组数不同。
	GroupID string `json:"groupId"`
	// Topology 职责：选择具名装配器；使用：单次求解；边界：必须匹配输入模式及密度。
	Topology DistributorTopology `json:"topology" enum:"standard-field,dense-blackbox,dense-detached,internal-mode5"`
	// Scale 职责：设置实际模块高度倍率；使用：全场装配；边界：mode5 为零，其余有限正数。
	Scale float64 `json:"scale" min:"0" unit:"ratio"`
	// Order 职责：设置全场稳定排序；使用：全场装配；边界：0..3，不改变设备事实。
	Order int `json:"order" min:"0" max:"3"`
	// LatitudeFactor 职责：设置普通布局纬向预算因子；使用：普通全场装配；边界：零保留未限高策略。
	LatitudeFactor float64 `json:"latitudeFactor" min:"0" unit:"ratio"`
	// RowSpan 职责：显式限制普通机器行跨度；使用：短行装箱；边界：零沿用默认，不能用于密铺或内部mode5。
	RowSpan float64 `json:"rowSpan" min:"0" max:"10000" unit:"grid"`
	// RowPacking 职责：保留普通功能行阵及其接线间距；使用：显式普通策略；边界：不改变碰撞或球面许可。
	RowPacking bool `json:"rowPacking"`
	// MaterialOrder 职责：启用物料邻接排序；使用：普通配送；边界：不改变生产计划。
	MaterialOrder bool `json:"materialOrder"`
	// ConsumerFirst 职责：选择消费者优先物料排序；使用：MaterialOrder；边界：单独为真无效并拒绝。
	ConsumerFirst bool `json:"consumerFirst"`
	// Ground 职责：显式要求独立地面机器轨；使用：黑盒备用；边界：为假仍须服从外层球面规格的必要地面轨约束，不追加候选。
	Ground bool `json:"ground"`
	// Street 职责：选择共享街道机器核心；使用：合资格密铺产线；边界：仍按真实附件资格筛选。
	Street bool `json:"street"`
	// Groups 职责：限制旧街道容量分组；使用：street 备用；边界：零使用原策略，不能与 GroupID 混淆。
	Groups int `json:"groups" min:"0"`
	// Clearance 职责：设置街道折叠间隙；使用：street 拓扑；边界：零由枚举归一为真实默认 .04。
	Clearance float64 `json:"clearance" min:"0" unit:"grid"`
	// NativeReserve 职责：启用原生纬带容量预留；使用：密铺原生策略；边界：去重必须保留不同预留。
	NativeReserve bool `json:"nativeReserve"`
	// HeightLimit 职责：指定 mode5 列高；使用：逐行布局；边界：有限正数，不在求解时推导其他列高。
	HeightLimit float64 `json:"heightLimit" min:"0" unit:"grid"`
	// GroupRows 职责：保留 mode5 整组基线；使用：逐行装配；边界：不改变机器数量。
	GroupRows bool `json:"groupRows"`
	// Compact 职责：启用 mode5 真实轮廓压缩；使用：逐行装配；边界：不选择其他候选。
	Compact bool `json:"compact"`
	// SurfacePairs 职责：启用 mode5 球面反向交错；使用：自动球面布局；边界：仍须后续完整终检。
	SurfacePairs bool `json:"surfacePairs"`
	// Fallback 职责：解释延后组触发关系；使用：外部引擎；边界：单次求解只执行显式规格，不检查其他组结果。
	Fallback DistributorFallback `json:"fallback"`
}

// DistributorSearchOptions 职责：集中配置配送枚举参数；使用：工具默认值和显式配置覆盖；边界：零值不偷偷填充默认表，空切片表示不枚举对应策略。
type DistributorSearchOptions struct {
	// Standard 职责：提供普通全场策略；使用：mode2 standard；边界：返回规格不引用此切片。
	Standard []distributor.FieldStrategy `json:"standard"`
	// StandardRows 职责：提供追加的普通功能行阵策略；使用：显式生产参数；边界：不替代Standard原候选，空表不追加。
	StandardRows []distributor.FieldStrategy `json:"standardRows"`
	// StandardRowSpans 职责：提供有限普通行跨度；使用：与StandardRows追加策略组合；边界：空表使用原行长，不影响Standard或密铺。
	StandardRowSpans []float64 `json:"standardRowSpans"`
	// StandardRowPacking 职责：显式启用普通功能行阵；使用：生产方案；边界：默认关闭，不影响旧候选。
	StandardRowPacking bool `json:"standardRowPacking"`
	// Dense 职责：提供非原生密铺策略；使用：mode2 dense；边界：不隐式增加尺度。
	Dense []distributor.FieldStrategy `json:"dense"`
	// NativeDense 职责：提供原生纬带密铺策略；使用：原生上下文；边界：与 Dense 分别显式配置。
	NativeDense []distributor.FieldStrategy `json:"nativeDense"`
	// DenseTopologies 职责：声明密铺主拓扑；使用：参数展开；边界：仅消费拓扑、Ground、Street、Groups 和 Clearance。
	DenseTopologies []DistributorCandidateSpec `json:"denseTopologies"`
	// GroundFallback 职责：枚举参考轨失败的地面轨备用；使用：引擎调度；边界：只附元数据，不执行备用。
	GroundFallback bool `json:"groundFallback"`
	// StreetFallback 职责：枚举旧 street 的容量分组备用；使用：引擎调度；边界：仍须通过旧拓扑资格。
	StreetFallback bool `json:"streetFallback"`
	// StreetFallbackTopologies 职责：显式配置旧 street 备用容量与间隙；使用：StreetFallback；边界：默认表只在默认选项函数定义。
	StreetFallbackTopologies []DistributorCandidateSpec `json:"streetFallbackTopologies"`
	// InternalMode5 职责：显式覆盖内部 mode5 空间策略；使用：兼容方案；边界：与 AutoMode5Heights 互斥。
	InternalMode5 []distributor.LayoutVariant `json:"internalMode5"`
	// AutoMode5Heights 职责：用真实局部面积推导 mode5 高度表；使用：默认兼容方案；边界：探针不终检、不编码。
	AutoMode5Heights bool `json:"autoMode5Heights"`
	// Mode5SurfacePairs 职责：允许自动球面交错策略；使用：自动 mode5 高度表；边界：还须输入具备此能力。
	Mode5SurfacePairs bool `json:"mode5SurfacePairs"`
}

// DefaultDistributorSearchOptions 职责：提供唯一配送默认组合；使用：工具注册及独立核心调用；边界：每次返回独立切片，配置工具应显式覆盖差异。
func DefaultDistributorSearchOptions() DistributorSearchOptions {
	return DistributorSearchOptions{
		Standard: distributor.StandardFieldStrategies(), Dense: distributor.DenseFieldStrategies(false), NativeDense: distributor.DenseFieldStrategies(true),
		DenseTopologies: []DistributorCandidateSpec{{Topology: DistributorDenseBlackbox}, {Topology: DistributorDenseBlackbox, Street: true}, {Topology: DistributorDenseDetached}, {Topology: DistributorDenseDetached, Street: true}},
		GroundFallback:  true, StreetFallback: true,
		StreetFallbackTopologies: []DistributorCandidateSpec{
			{Topology: DistributorDenseDetached, Street: true, Groups: 3, Clearance: .04},
			{Topology: DistributorDenseDetached, Street: true, Groups: 2, Clearance: .2},
			{Topology: DistributorDenseDetached, Street: true, Groups: 2, Clearance: .4},
			{Topology: DistributorDenseDetached, Street: true, Groups: 2, Clearance: .6},
			{Topology: DistributorDenseDetached, Street: true, Groups: 1, Clearance: .04},
		},
		AutoMode5Heights: true, Mode5SurfacePairs: true,
	}
}

// ValidateDistributorSearchOptions 职责：纯校验配送策略、拓扑字段及mode5互斥参数；使用：ToolSpec 和核心参数入口；边界：空列表主动禁用对应策略，未激活策略表保留，不读取请求密度或设备资格。
func ValidateDistributorSearchOptions(options DistributorSearchOptions) error {
	if err := validateStandardRowSpans(options.StandardRowSpans); err != nil {
		return err
	}
	for _, group := range []struct {
		name     string
		topology DistributorTopology
		values   []distributor.FieldStrategy
	}{
		{"standard", DistributorStandardField, options.Standard}, {"standardRows", DistributorStandardField, options.StandardRows}, {"dense", DistributorDenseBlackbox, options.Dense}, {"nativeDense", DistributorDenseBlackbox, options.NativeDense},
	} {
		for index, strategy := range group.values {
			if err := validateDistributorFieldStrategy(strategy, group.topology); err != nil {
				return distributorSpecError(fmt.Sprintf("%s[%d]: %v", group.name, index, err))
			}
		}
	}
	for _, group := range []struct {
		name   string
		backup bool
		values []DistributorCandidateSpec
	}{
		{"denseTopologies", false, options.DenseTopologies}, {"streetFallbackTopologies", true, options.StreetFallbackTopologies},
	} {
		for index, topology := range group.values {
			if err := validateDistributorTopologyFields(topology); err != nil {
				return distributorSpecError(fmt.Sprintf("%s[%d]: %v", group.name, index, err))
			}
			if topology.ID != "" || topology.GroupID != "" || topology.Scale != 0 || topology.Order != 0 || topology.LatitudeFactor != 0 || topology.RowSpan != 0 || topology.RowPacking || topology.MaterialOrder || topology.ConsumerFirst || topology.NativeReserve || topology.HeightLimit != 0 || topology.GroupRows || topology.Compact || topology.SurfacePairs || len(topology.Fallback.AfterGroups) != 0 || topology.Fallback.RequireNoSuccess || topology.Fallback.RequireFailure || topology.Fallback.ExcludedFailureCode != "" {
				return distributorSpecError(fmt.Sprintf("%s[%d] 只消费 topology/ground/street/groups/clearance，其余字段不能配置", group.name, index))
			}
			if group.backup && (topology.Topology != DistributorDenseDetached || !topology.Street || topology.Ground || topology.Groups <= 0) {
				return distributorSpecError("streetFallbackTopologies 必须为独立设施街道且 groups 为正")
			}
		}
	}
	if options.AutoMode5Heights && len(options.InternalMode5) > 0 {
		return distributorSpecError("mode5 显式高度与自动高度不能同时配置")
	}
	for index, variant := range options.InternalMode5 {
		if err := distributor.ValidateLayoutVariant(variant); err != nil {
			return distributorSpecError(fmt.Sprintf("internalMode5[%d]: %v", index, err))
		}
	}
	return nil
}

// validateDistributorFieldStrategy 职责：复用规划纯约束并核对字段消费范围；使用：参数表及单个规格校验；边界：不决定真实回流、喷涂或街道资格。
func validateDistributorFieldStrategy(strategy distributor.FieldStrategy, topology DistributorTopology) error {
	if err := distributor.ValidateFieldStrategy(strategy); err != nil {
		return distributorSpecError(err.Error())
	}
	switch topology {
	case DistributorStandardField:
		if strategy.NativeReserve {
			return distributorSpecError("普通配送不消费 nativeReserve")
		}
	case DistributorDenseBlackbox, DistributorDenseDetached:
		if strategy.LatitudeFactor != 0 || strategy.MaterialOrder || strategy.ConsumerFirst {
			return distributorSpecError("密铺配送不消费 latitudeFactor/materialOrder/consumerFirst")
		}
	default:
		return distributorSpecError("未知全场配送拓扑")
	}
	return nil
}

// validateDistributorTopologyFields 职责：共享密铺拓扑的纯字段关系；使用：拓扑表和单个候选规格；边界：不接受mode5或普通字段，不读取密度与实际能力。
func validateDistributorTopologyFields(spec DistributorCandidateSpec) error {
	if spec.Topology != DistributorDenseBlackbox && spec.Topology != DistributorDenseDetached {
		return distributorSpecError("密铺拓扑必须为 dense-blackbox 或 dense-detached")
	}
	if spec.Groups < 0 || math.IsNaN(spec.Clearance) || math.IsInf(spec.Clearance, 0) || spec.Clearance < 0 {
		return distributorSpecError("groups 与工作格 clearance 必须有限且非负")
	}
	if spec.Ground && spec.Topology != DistributorDenseBlackbox {
		return distributorSpecError("ground 仅适用于密铺黑盒")
	}
	if !spec.Street && (spec.Groups != 0 || spec.Clearance != 0) {
		return distributorSpecError("非街道拓扑不能携带 groups/clearance")
	}
	if spec.Topology == DistributorDenseBlackbox && spec.Groups != 0 {
		return distributorSpecError("黑盒街道不消费旧街道 groups 限制")
	}
	return nil
}

// validateDistributorCandidateParameters 职责：集中校验具名规格的静态策略关系；使用：单规格校验复用；边界：不读取输入或改变装配行为。
func validateDistributorCandidateParameters(spec DistributorCandidateSpec) error {
	if spec.RowPacking && spec.Topology != DistributorStandardField {
		return distributorSpecError("rowPacking仅适用于普通配送")
	}
	if spec.RowSpan != 0 && (spec.Topology != DistributorStandardField || !finitePositive(spec.RowSpan) || spec.RowSpan > 10000) {
		return distributorSpecError("rowSpan必须为普通配送的有限正工作格跨度且不超过10000")
	}
	if spec.Topology == DistributorInternalMode5 {
		if err := distributor.ValidateLayoutVariant(distributor.LayoutVariant{HeightLimit: spec.HeightLimit}); err != nil {
			return distributorSpecError(err.Error())
		}
		if spec.Scale != 0 || spec.Order != 0 || spec.LatitudeFactor != 0 || spec.MaterialOrder || spec.ConsumerFirst || spec.NativeReserve || spec.Ground || spec.Street || spec.Groups != 0 || spec.Clearance != 0 {
			return distributorSpecError("mode5 不消费全场配送字段")
		}
		return nil
	}
	strategy := distributor.FieldStrategy{Scale: spec.Scale, Order: spec.Order, LatitudeFactor: spec.LatitudeFactor, MaterialOrder: spec.MaterialOrder, ConsumerFirst: spec.ConsumerFirst, NativeReserve: spec.NativeReserve}
	if err := validateDistributorFieldStrategy(strategy, spec.Topology); err != nil {
		return err
	}
	if spec.HeightLimit != 0 || spec.GroupRows || spec.Compact || spec.SurfacePairs {
		return distributorSpecError("全场配送不消费 mode5 空间策略")
	}
	if spec.Topology == DistributorStandardField {
		if spec.Ground || spec.Street || spec.Groups != 0 || spec.Clearance != 0 {
			return distributorSpecError("普通配送不能携带密铺拓扑字段")
		}
		return nil
	}
	return validateDistributorTopologyFields(spec)
}

// EnumerateDistributorCandidates 职责：筛选资格、展开参数并按实际取整预算去重；使用：引擎创建隔离候选之前；边界：不运行成功候选或备用调度，不修改输入和选项。
// 逻辑流：恢复显式几何上下文 => 资格筛选 => 具名主组及条件备用组 => 局部测量去重 => 稳定编号。
func EnumerateDistributorCandidates(ctx context.Context, input *PreparedInput, options DistributorSearchOptions) ([]DistributorCandidateSpec, error) {
	if input == nil || input.dataset == nil || input.graph == nil {
		return nil, distributorSpecError("配送输入快照为空")
	}
	ctx = input.algorithmContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ValidateDistributorSearchOptions(options); err != nil {
		return nil, err
	}
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		return nil, err
	}
	req, dataset := input.request, input.dataset
	var specs []DistributorCandidateSpec
	if req.Mode == 5 {
		variants := options.InternalMode5
		if options.AutoMode5Heights {
			limits, err := b.layoutHeightCandidates(ctx, req, dataset, false)
			if err != nil {
				return nil, err
			}
			surface := distributor.SurfacePlanar
			if req.SurfaceLayout != nil && req.SurfaceLayout.Mode == "autoTropic" {
				surface = distributor.SurfaceAutoTropic
			}
			variants = distributor.Variants(b.setting.BlockWidthLimit, limits, surface, options.Mode5SurfacePairs && b.policy != nil && b.policy.flySurfacePairs)
		}
		for _, variant := range variants {
			specs = append(specs, DistributorCandidateSpec{Topology: DistributorInternalMode5, GroupID: "internal-mode5", HeightLimit: variant.HeightLimit, GroupRows: variant.GroupRows, Compact: variant.Compact, SurfacePairs: variant.SurfacePairs})
		}
	} else if req.Mode != 2 {
		return nil, distributorSpecError("配送核心只接受 mode2 或内部 mode5")
	} else if req.Density != densityDense {
		for _, strategy := range options.Standard {
			specs = append(specs, distributorStrategySpec(DistributorCandidateSpec{Topology: DistributorStandardField, GroupID: "standard-field"}, strategy))
		}
		for _, strategy := range options.StandardRows {
			spans := options.StandardRowSpans
			if len(spans) == 0 {
				spans = []float64{0}
			}
			for _, span := range spans {
				specs = append(specs, distributorStrategySpec(DistributorCandidateSpec{Topology: DistributorStandardField, GroupID: "standard-field", RowSpan: span, RowPacking: options.StandardRowPacking}, strategy))
			}
		}
	} else {
		recycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
		if err != nil {
			return nil, err
		}
		detached := !recycle && !b.setting.EnableIntermediateBuffer && !needsByproductPriority(req.Plan, b.flowGraph, dataset)
		street := b.hasDenseFlyStreet(req, dataset)
		strategies := options.Dense
		if denseNativeLatitude(req) {
			strategies = options.NativeDense
		}
		var topologies []DistributorCandidateSpec
		var groundAfter []string
		var streetAfter string
		seenGroups := map[string]bool{}
		for _, topology := range options.DenseTopologies {
			if topology.Topology == DistributorDenseDetached && !detached || topology.Street && !street {
				continue
			}
			// 主组只消费拓扑字段，避免混入上一次枚举的编号、参数和触发元数据。
			topology = DistributorCandidateSpec{Topology: topology.Topology, Ground: topology.Ground, Street: topology.Street, Groups: topology.Groups, Clearance: topology.Clearance}
			if topology.Street && topology.Clearance == 0 {
				topology.Clearance = .04
			}
			topology.GroupID = distributorGroupID(topology)
			if seenGroups[topology.GroupID] {
				continue
			}
			seenGroups[topology.GroupID] = true
			topologies = append(topologies, topology)
			if topology.Topology == DistributorDenseBlackbox && !topology.Ground {
				groundAfter = append(groundAfter, topology.GroupID)
			}
			if topology.Topology == DistributorDenseDetached && topology.Street && topology.Groups == 0 {
				streetAfter = topology.GroupID
			}
		}
		if options.GroundFallback && len(groundAfter) > 0 {
			topology := DistributorCandidateSpec{Topology: DistributorDenseBlackbox, Ground: true, Fallback: distributorFallback(groundAfter)}
			topology.GroupID = distributorGroupID(topology)
			if !seenGroups[topology.GroupID] {
				seenGroups[topology.GroupID] = true
				topologies = append(topologies, topology)
			}
		}
		if options.StreetFallback && streetAfter != "" {
			for _, backup := range options.StreetFallbackTopologies {
				clearance := backup.Clearance
				if clearance == 0 {
					clearance = .04
				}
				topology := DistributorCandidateSpec{Topology: DistributorDenseDetached, Street: true, Groups: backup.Groups, Clearance: clearance, Fallback: distributorFallback([]string{streetAfter})}
				topology.GroupID = distributorGroupID(topology)
				if !seenGroups[topology.GroupID] {
					seenGroups[topology.GroupID] = true
					topologies = append(topologies, topology)
				}
			}
		}
		for _, topology := range topologies {
			for _, strategy := range strategies {
				specs = append(specs, distributorStrategySpec(topology, strategy))
			}
		}
	}
	for _, spec := range specs {
		if err := validateDistributorSpec(input, spec); err != nil {
			return nil, err
		}
	}
	specs, err = normalizeDistributorCandidateSpecs(ctx, input, specs)
	if err != nil {
		return nil, err
	}
	for i := range specs {
		specs[i].ID = fmt.Sprintf("%s/%d", specs[i].GroupID, i)
		specs[i].Fallback.AfterGroups = append([]string(nil), specs[i].Fallback.AfterGroups...)
	}
	return specs, nil
}

// AssembleDistributorCandidate 职责：只装配一个显式规格并冻结装配图；使用：工作流装配节点；边界：后续定位、补电、绑定、规则和编码由独立节点执行。
func AssembleDistributorCandidate(ctx context.Context, input *PreparedInput, spec DistributorCandidateSpec) (AssemblySnapshot, error) {
	if input == nil || input.dataset == nil || input.graph == nil {
		return AssemblySnapshot{}, distributorSpecError("配送输入快照为空")
	}
	ctx = input.algorithmContext(ctx)
	if err := ctx.Err(); err != nil {
		return AssemblySnapshot{}, err
	}
	if err := validateDistributorSpec(input, spec); err != nil {
		return AssemblySnapshot{}, err
	}
	b, ctx, err := newDistributorAssembly(ctx, input, spec)
	if err != nil {
		return AssemblySnapshot{}, err
	}
	if spec.Topology == DistributorDenseDetached {
		recycle, err := denseBlackboxHasMachineRecycle(input.request.Plan, input.dataset)
		if err != nil {
			return AssemblySnapshot{}, err
		}
		if recycle || b.setting.EnableIntermediateBuffer || needsByproductPriority(input.request.Plan, input.graph, input.dataset) {
			return AssemblySnapshot{}, distributorSpecError("回流、中转缓存或联产优先请求不能使用旧独立设施拓扑")
		}
	}
	if spec.Street && !b.hasDenseFlyStreet(input.request, input.dataset) {
		return AssemblySnapshot{}, distributorSpecError("本请求没有合资格的配送街道产线")
	}
	b.preview = previewFromContext(ctx).fork()
	b.beginPreview()
	if err := assembleDistributorSpec(ctx, b, input, spec); err != nil {
		b.rollbackPreview()
		if ctx.Err() != nil {
			return AssemblySnapshot{}, ctx.Err()
		}
		return AssemblySnapshot{}, err
	}
	geometry, err := freezeAssembly(ctx, b, input.request, input.dataset)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		b.rollbackPreview()
		return AssemblySnapshot{}, err
	}
	return geometry, nil
}

// DistributorCandidateFacilityCount 职责：读取定稿实体中的真实配送器数量；使用：完整终检后的外部评分；边界：不从理论端口估计，不签发验证证据。
func DistributorCandidateFacilityCount(geometry GeometrySnapshot) (int, error) {
	if geometry.data == nil {
		return 0, distributorSpecError("配送评分缺少几何定稿")
	}
	count := 0
	for _, record := range geometry.data.document.Buildings {
		if record.ItemID == 2107 {
			count++
		}
	}
	return count, nil
}

// DistributorCandidateMetrics 职责：返回完整终检后的指标值；使用：外部稳定归约；边界：不返回可变报告，不接受零值证据。
func DistributorCandidateMetrics(validation FinalValidation) (LayoutMetrics, error) {
	if validation.data == nil || validation.data.logistics == nil {
		return LayoutMetrics{}, distributorSpecError("配送评分缺少最终验证指标")
	}
	return validation.data.logistics.Metrics, nil
}

// BetterDistributorCandidate 职责：将所有配送拓扑的最终比较交给共同形状规则；使用：引擎归约完整终检结果；边界：规格不改变1.8验收线或排序次序，设施数必须来自真实定稿，不保存获胜者。
func BetterDistributorCandidate(spec DistributorCandidateSpec, facilities int, metrics LayoutMetrics, other DistributorCandidateSpec, otherFacilities int, otherMetrics LayoutMetrics) bool {
	return BetterLayoutCandidate(metrics, facilities, otherMetrics, otherFacilities)
}

// distributorStrategySpec 职责：把具名策略复制到拓扑规格；使用：参数展开；边界：不保留选项切片的引用。
func distributorStrategySpec(spec DistributorCandidateSpec, strategy distributor.FieldStrategy) DistributorCandidateSpec {
	spec.Scale, spec.Order, spec.LatitudeFactor = strategy.Scale, strategy.Order, strategy.LatitudeFactor
	spec.MaterialOrder, spec.ConsumerFirst, spec.NativeReserve = strategy.MaterialOrder, strategy.ConsumerFirst, strategy.NativeReserve
	return spec
}

// distributorFallback 职责：创建统一失败触发元数据；使用：备用组枚举；边界：实际判断留给引擎。
func distributorFallback(groups []string) DistributorFallback {
	return DistributorFallback{AfterGroups: append([]string(nil), groups...), RequireNoSuccess: true, RequireFailure: true, ExcludedFailureCode: "GEN_E_DENSITY_FACILITIES"}
}

// distributorGroupID 职责：给拓扑容量组建立可解释稳定身份；使用：触发关系和诊断；边界：不包含组内尺度及排序。
func distributorGroupID(spec DistributorCandidateSpec) string {
	id := string(spec.Topology)
	if spec.Ground {
		id += "-ground"
	}
	if spec.Street {
		id += fmt.Sprintf("-street-g%d-c%g", spec.Groups, spec.Clearance)
	}
	return id
}

// distributorSpecError 职责：返回配送参数边界错误；使用：枚举和求解拒绝非法规格；边界：不软化实际装配失败。
func distributorSpecError(message string) error {
	return &blueprint.Error{Code: "GEN_E_DISTRIBUTOR_SPEC", Message: message}
}

// validateDistributorSpec 职责：拒绝密度错配、无效数值及未消费字段；使用：枚举出口和单次求解入口；边界：不能绕过实际附件和终检规则。
func validateDistributorSpec(input *PreparedInput, spec DistributorCandidateSpec) error {
	if err := validateDistributorCandidateParameters(spec); err != nil {
		return err
	}
	if spec.Topology == DistributorInternalMode5 {
		if input.request.Mode != 5 {
			return distributorSpecError("内部 mode5 规格与输入模式不符")
		}
		if spec.SurfacePairs && (input.request.SurfaceLayout == nil || input.request.SurfaceLayout.Mode != "autoTropic" || input.policy == nil || !input.policy.flySurfacePairs) {
			return distributorSpecError("mode5 球面交错需要自动球面能力")
		}
		return nil
	}
	if input.request.Mode != 2 {
		return distributorSpecError("全场配送规格与输入模式不符")
	}
	if spec.Topology == DistributorStandardField {
		if input.request.Density == densityDense {
			return distributorSpecError("普通配送规格不能使用密铺拓扑")
		}
	} else if input.request.Density != densityDense {
		return distributorSpecError("密铺规格与输入密度不符")
	}
	return nil
}

// newDistributorAssembly 职责：应用唯一规格并创建短时装配内存；使用：单次求解与局部测量共用；边界：禁用历史获胜者剪枝，不共享可变实体。
func newDistributorAssembly(ctx context.Context, input *PreparedInput, spec DistributorCandidateSpec) (*builder, context.Context, error) {
	ctx = context.WithValue(ctx, denseFlyFacilityPruneDisabledKey{}, true)
	if spec.RowPacking {
		ctx = context.WithValue(ctx, denseBalancedFrameKey{}, false)
	}
	if spec.Ground {
		ctx = context.WithValue(ctx, denseGroundTracksKey{}, true)
	}
	if spec.Street {
		clearance := spec.Clearance
		if clearance == 0 {
			clearance = .04
		}
		ctx = context.WithValue(ctx, denseStreetFoldClearanceKey{}, clearance)
	}
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		return nil, ctx, err
	}
	if spec.Topology == DistributorInternalMode5 {
		b.flyCompact, b.flySurfacePairs = spec.Compact, spec.SurfacePairs
		return b, ctx, nil
	}
	b.denseSpatial, b.denseSpatialOrder, b.flyCompact = true, spec.Order, true
	if spec.RowSpan > 0 {
		ctx = context.WithValue(ctx, standardRowSpanKey{}, spec.RowSpan)
	}
	b.nativeCapacityReserve, b.denseStreetGroupLimit = spec.NativeReserve, spec.Groups
	if spec.LatitudeFactor > 0 {
		b.denseLatitudeSpan = math.Max(24, math.Ceil(math.Sqrt(float64(input.request.Plan.MachineCount)*32)*spec.LatitudeFactor))
	}
	if spec.MaterialOrder {
		b.denseUnitOrder = func(ctx context.Context, units []denseSpatialUnit) ([]denseSpatialUnit, error) {
			return b.denseStationMaterialUnitOrder(ctx, units, spec.ConsumerFirst)
		}
	}
	if spec.RowPacking {
		ctx = b.configureStandardRows(ctx)
		b.generationCtx = ctx
	}
	return b, ctx, nil
}

// assembleDistributorSpec 职责：执行唯一规格的真实装配；使用：求解和探针；边界：不终检、不编码、不构造第二个完整候选。
func assembleDistributorSpec(ctx context.Context, b *builder, input *PreparedInput, spec DistributorCandidateSpec) error {
	switch spec.Topology {
	case DistributorStandardField, DistributorDenseBlackbox:
		return b.assembleDenseFlyBlackbox(ctx, input.request, input.dataset, spec.Scale, spec.Street)
	case DistributorDenseDetached:
		return b.assembleDenseFlyLayout(ctx, input.request, input.dataset, spec.Scale, spec.Street)
	case DistributorInternalMode5:
		variant := flyLayoutVariant{HeightLimit: spec.HeightLimit, GroupRows: spec.GroupRows, Compact: spec.Compact, SurfacePairs: spec.SurfacePairs}
		if err := b.assembleFlyLayout(ctx, input.request, input.dataset, variant); err != nil {
			return err
		}
		if spec.SurfacePairs {
			return b.pairFlySurfaceBlocks(ctx)
		}
		return nil
	default:
		return distributorSpecError("未知配送拓扑")
	}
}
