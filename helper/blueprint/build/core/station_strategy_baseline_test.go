package build

// 文件职责：冻结重构前站点候选实现，作为临时差分验收基线。
// 使用：与具名策略的完整候选顺序、几何和观察计数对照；边界：仅测试引用旧编码，主线最终统一清理。
import (
	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/candidates"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/quant"
	"context"
	"math"
)

const baselineDenseStationGroundTracksVariantOffset = 51200
const baselineIndependentStationVariantOffset = 25600
const baselineDenseSpatialHolesBentVariantOffset = 12800
const baselineDenseSpatialHolesVariantOffset = 6400
const baselineDenseStationProducerVariantOffset = 3200
const baselineDenseStationPairedOutputVariantOffset = 1600
const baselineDenseSpatialCompactVariantOffset = 800
const baselineDenseStationSharedOutputVariantOffset = 400
const baselineDenseBalancedFrameVariantOffset = 200
const baselineDenseStationSharedInputVariantOffset = 100

// baselineStationCandidateSpec 职责：保存一个塔候选实际消费的全部策略值；使用：跨步骤只读交接；边界：无建筑指针、回调或工作流身份，SourceVariant不参与装配。
type baselineStationCandidateSpec struct {
	// InputFingerprint 职责：绑定规范输入；使用：阻止跨任务误用；边界：不代替实体终检。
	InputFingerprint string `json:"inputFingerprint"`
	// SourceVariant 职责：保存归一后的旧编号以便反查；使用：迁移诊断；边界：单次装配只读取具名字段。
	SourceVariant int `json:"sourceVariant"`
	// Kind 职责：选择blackbox或field实体拓扑；使用：单次求解；边界：未知值拒绝。
	Kind string `json:"kind"`
	// Density 职责：绑定普通或密铺硬规则；使用：一致性校验；边界：不得改写输入密度。
	Density string `json:"density"`
	// StationID 职责：绑定真实物流塔原型；使用：共享设备查询；边界：不允许改变输入模式的塔。
	StationID int `json:"stationId"`
	// BaseLayout 职责：保留基础布局选择；使用：资格及追踪；边界：不编码修饰位。
	BaseLayout int `json:"baseLayout"`
	// Order 职责：选择机器行空间排序；使用：单次装箱；边界：不改变机器数。
	Order int `json:"order"`
	// MaxSpan 职责：限制独立机器行工作格跨度；使用：实际拆行；边界：不扩大带容量。
	MaxSpan float64 `json:"maxSpan"`
	// Refine 职责：启用局部小数位姿搜索；使用：单次几何策略；边界：不放宽碰撞。
	Refine bool `json:"refine"`
	// Street 职责：启用连续街道；使用：有资格的机器行；边界：不产生另一个候选。
	Street bool `json:"street"`
	// Rotation 职责：选择街道四分之一转朝向；使用：装配；边界：无街道时归零。
	Rotation int `json:"rotation"`
	// Groups 职责：选择街道容量分组；使用：局部构造；边界：保持既有0、2、-1策略。
	Groups int `json:"groups"`
	// LatitudeSpan 职责：保存已取整的纬向工作预算；使用：装箱与最终范围核对；边界：零表示未显式限制。
	LatitudeSpan float64 `json:"latitudeSpan"`
	// MaterialOrder 职责：启用物料相邻排序及供剂网络；使用：大厂布局；边界：不改规范流图。
	MaterialOrder bool `json:"materialOrder"`
	// ConsumerFirst 职责：选择消费者优先；使用：物料排序；边界：与枢纽优先保持不同身份。
	ConsumerFirst bool `json:"consumerFirst"`
	// LinearTowers 职责：选择塔阵列单行；使用：外部塔布局；边界：仍满足真实塔距。
	LinearTowers bool `json:"linearTowers"`
	// HeightChoice 职责：选择真实占位测量的高度方案；使用：单次装配前测量；边界：不是候选调度。
	HeightChoice int `json:"heightChoice"`
	// GroundTracks 职责：启用地面机器轨；使用：独立实体策略；边界：不改变密度。
	GroundTracks bool `json:"groundTracks"`
	// IndependentStations 职责：使真实塔单独落位；使用：全场占位；边界：塔距不放宽。
	IndependentStations bool `json:"independentStations"`
	// HoleSpan 职责：启用20或18格空洞填充；使用：短行拓扑；边界：零为关闭。
	HoleSpan float64 `json:"holeSpan"`
	// BentRows 职责：启用真实L/U接角；使用：18格空洞策略；边界：不使用虚假矩形重叠。
	BentRows bool `json:"bentRows"`
	// ProducerSplits 职责：启用真实产物分流树；使用：有限拓扑；边界：仍需逐边证明。
	ProducerSplits bool `json:"producerSplits"`
	// SplitterModel 职责：指定真实分流器形态；使用：生产者分流；边界：当前只覆盖Model39。
	SplitterModel int16 `json:"splitterModel"`
	// PairedOutputs 职责：启用双侧过滤；使用：多去向输出；边界：与原单侧保持独立候选。
	PairedOutputs bool `json:"pairedOutputs"`
	// Compact 职责：启用空间平移靠拢；使用：完整真实轮廓；边界：不缩碰撞盒。
	Compact bool `json:"compact"`
	// SharedOutputs 职责：启用同物料边界合流；使用：共享出口；边界：无合流资格时枚举移除此修饰。
	SharedOutputs bool `json:"sharedOutputs"`
	// BalancedFrame 职责：启用均衡矩形及预留供电；使用：合资格原生厂；边界：不扩大用户纬带。
	BalancedFrame bool `json:"balancedFrame"`
	// SharedInputs 职责：启用共享原料轨；使用：边界聚合；边界：保留物料角色和容量。
	SharedInputs bool `json:"sharedInputs"`
	// RefineryCollinear 职责：启用同侧炼油轨；使用：真实炼油资格；边界：不替代其它机器模板。
	RefineryCollinear bool `json:"refineryCollinear"`
	// ChemicalFour 职责：启用四端化工模板；使用：既有资格；边界：炼油旧编号包含此修饰，保持原解码语义。
	ChemicalFour bool `json:"chemicalFour"`
	// MaterialNetworks 职责：启用全场物料网络；使用：基础布局4至6；边界：不代表拓扑已验证。
	MaterialNetworks bool `json:"materialNetworks"`
	// FieldVariant 职责：选择塔场四种放置策略；使用：field装配；边界：不含旧offset。
	FieldVariant int `json:"fieldVariant"`
	// NearbyInputPorts 职责：标识获选规格派生的就近口精修；使用：引擎显式新候选；边界：不携带winner实体。
	NearbyInputPorts bool `json:"nearbyInputPorts"`
}

// baselineDecodeStationVariant 职责：唯一解释旧编号全部修饰；使用：枚举和迁移兼容；边界：不装配、不写上下文，非法余数拒绝。
func baselineDecodeStationVariant(variant int) (baselineStationCandidateSpec, error) {
	spec := baselineStationCandidateSpec{Kind: "blackbox", SourceVariant: variant}
	for _, entry := range []struct {
		offset  int
		enabled *bool
	}{
		{baselineDenseStationGroundTracksVariantOffset, &spec.GroundTracks}, {baselineIndependentStationVariantOffset, &spec.IndependentStations},
		{baselineDenseSpatialHolesBentVariantOffset, &spec.BentRows}, {baselineDenseSpatialHolesVariantOffset, nil},
		{baselineDenseStationProducerVariantOffset, &spec.ProducerSplits}, {baselineDenseStationPairedOutputVariantOffset, &spec.PairedOutputs},
		{baselineDenseSpatialCompactVariantOffset, &spec.Compact}, {baselineDenseStationSharedOutputVariantOffset, &spec.SharedOutputs},
		{baselineDenseBalancedFrameVariantOffset, &spec.BalancedFrame}, {baselineDenseStationSharedInputVariantOffset, &spec.SharedInputs},
	} {
		if variant >= entry.offset {
			variant -= entry.offset
			if entry.enabled != nil {
				*entry.enabled = true
			}
			if entry.offset == baselineDenseSpatialHolesBentVariantOffset {
				spec.HoleSpan = 18
			}
			if entry.offset == baselineDenseSpatialHolesVariantOffset {
				spec.HoleSpan = 20
			}
		}
	}
	if variant >= 10 {
		spec.RefineryCollinear = true
		variant -= 3
	}
	if variant >= 7 {
		spec.ChemicalFour = true
		variant -= 3
	}
	if variant < 0 || variant > 6 {
		return baselineStationCandidateSpec{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔候选编号不在已核对的布局范围"}
	}
	spec.BaseLayout, spec.MaterialNetworks = variant, variant >= 4
	if spec.ProducerSplits {
		spec.SplitterModel = 39
	}
	return spec, nil
}

// baselineLegacyVariant 职责：把具名基础修饰还原为旧编号；使用：反查及兼容归一化；边界：派生几何和精修不伪装成另一个旧编号。
func (spec baselineStationCandidateSpec) baselineLegacyVariant() int {
	value := spec.BaseLayout
	for _, entry := range []struct {
		enabled bool
		offset  int
	}{
		{spec.GroundTracks, baselineDenseStationGroundTracksVariantOffset}, {spec.IndependentStations, baselineIndependentStationVariantOffset},
		{spec.HoleSpan == 18, baselineDenseSpatialHolesBentVariantOffset}, {spec.HoleSpan == 20, baselineDenseSpatialHolesVariantOffset},
		{spec.ProducerSplits, baselineDenseStationProducerVariantOffset}, {spec.PairedOutputs, baselineDenseStationPairedOutputVariantOffset},
		{spec.Compact, baselineDenseSpatialCompactVariantOffset}, {spec.SharedOutputs, baselineDenseStationSharedOutputVariantOffset},
		{spec.BalancedFrame, baselineDenseBalancedFrameVariantOffset}, {spec.SharedInputs, baselineDenseStationSharedInputVariantOffset},
		{spec.RefineryCollinear, 3}, {spec.ChemicalFour, 3},
	} {
		if entry.enabled {
			value += entry.offset
		}
	}
	return value
}

// baselineAlgorithmContext 职责：将显式塔策略适配到现有几何原语；使用：单候选局部计算；边界：上下文仅在本次调用内存在，不跨步骤保存。
func (spec baselineStationCandidateSpec) baselineAlgorithmContext(ctx context.Context, req Request) context.Context {
	for _, entry := range []struct {
		key     any
		enabled bool
	}{
		{denseGroundTracksKey{}, spec.GroundTracks}, {independentStationPlacementKey{}, spec.IndependentStations},
		{denseBentRowsKey{}, spec.BentRows}, {denseProducerSplittersKey{}, spec.ProducerSplits},
		{denseShortPairedOutputsKey{}, spec.PairedOutputs}, {denseSpatialCompactKey{}, spec.Compact || spec.HoleSpan > 0},
		{denseStationSharedOutputsKey{}, spec.SharedOutputs}, {denseStationSharedInputsKey{}, spec.SharedInputs},
		{refineryCollinearKey{}, spec.RefineryCollinear}, {chemicalFourKey{}, spec.ChemicalFour},
		{denseMaterialNetworksKey{}, spec.MaterialNetworks}, {stationInputAssignmentKey{}, spec.NearbyInputPorts},
	} {
		if entry.enabled {
			ctx = context.WithValue(ctx, entry.key, true)
		}
	}
	if spec.HoleSpan > 0 {
		ctx = context.WithValue(ctx, denseSpatialHolesKey{}, spec.HoleSpan)
	}
	if spec.ProducerSplits {
		ctx = context.WithValue(ctx, splitterGateModelKey{}, spec.SplitterModel)
	}
	if spec.BalancedFrame {
		ctx = denseBalancedFrameContext(ctx)
		ctx = context.WithValue(ctx, denseUnitPowerRequestedKey{}, req.Setting.NeedPower == nil || *req.Setting.NeedPower)
	}
	if root, _ := ctx.Value(densityPolarRootKey{}).(bool); root && spec.SharedInputs {
		ctx = context.WithValue(ctx, densityPrototypeFootprintKey{}, true)
	}
	return ctx
}

// baselineStationSpecForVariant 职责：把统一解码和有效几何组合为只读规格；使用：正式枚举及去重；边界：所有有效策略参与身份，不按结果面积剪枝。
func (b *builder) baselineStationSpecForVariant(ctx context.Context, req Request, dataset *quant.Dataset, variant int) (baselineStationCandidateSpec, error) {
	spec, err := baselineDecodeStationVariant(variant)
	if err != nil {
		return spec, err
	}
	geometry, err := b.baselineStationGeometry(spec.baselineAlgorithmContext(ctx, req), req, dataset, spec.BaseLayout)
	if err != nil {
		return spec, err
	}
	spec.Order, spec.MaxSpan, spec.Refine = geometry.order, geometry.span, geometry.refine
	spec.Street, spec.Rotation, spec.Groups = geometry.street, geometry.rotation, geometry.groups
	spec.LatitudeSpan, spec.MaterialOrder, spec.ConsumerFirst = geometry.latitude, geometry.materialOrder, geometry.consumerFirst
	spec.LinearTowers, spec.HeightChoice = geometry.linearTowers, geometry.heightChoice
	return spec, nil
}

// baselineStationCandidateGeometry 职责：保存黑盒构造器真正消费的布局参数；使用：规划键与装配赋值共用；边界：高度索引保守区分，未试装前不假设两种高度相同。
type baselineStationCandidateGeometry struct {
	// order/span/refine 职责：保存机器行及全场搜索策略；使用：赋值给候选；边界：跨度相同也不合并不同排序或微调。
	order  int
	span   float64
	refine bool
	// street/rotation/groups 职责：保存有效街道计划；使用：街道装配；边界：无合资格街道时旋转和分组均不消费。
	street   bool
	rotation int
	groups   int
	// latitude 职责：保存取整、限高后的纬向预算；使用：装箱和最终范围核验；边界：保留17格窄候选规则。
	latitude float64
	// materialOrder/consumerFirst 职责：保存物料相邻排序及供剂回调选择；使用：大厂分支；边界：消费者优先与枢纽优先不能互换。
	materialOrder bool
	consumerFirst bool
	// linearTowers/heightChoice 职责：保留塔阵列列数策略和试装高度选择；使用：边界构造及装配；边界：不能仅以机器数或限高相同省略这些差异。
	linearTowers bool
	heightChoice int
}

// baselineStationGeometry 职责：计算塔黑盒实际参数而不装配实体；使用：候选去重与构造共用；边界：variant必须是上下文修饰解码后的0至6，未知配方沿用原错误。
func (b *builder) baselineStationGeometry(ctx context.Context, req Request, dataset *quant.Dataset, variant int) (baselineStationCandidateGeometry, error) {
	geometry := baselineStationCandidateGeometry{order: variant % 4, span: []float64{18, 24, 32, 24}[variant%4], linearTowers: variant%2 == 1}
	if variant >= 2 {
		geometry.heightChoice = 1
	}
	needsRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return geometry, err
	}
	if needsRecycle {
		geometry.span = []float64{8, 8, 8, 24}[variant%4]
		geometry.refine = variant%2 == 1
	}
	if b.setting.EnableIntermediateBuffer {
		geometry.span = 32
	}
	streetSupported := false
	// 资格检查会读取generationCtx；用当前候选上下文，避免地面轨等修饰被父builder状态遮蔽。
	probe := &builder{setting: b.setting, policy: b.policy, generationCtx: ctx}
	for _, production := range req.Plan.Productions {
		recipe, err := datasetRecipeForProduction(dataset, production)
		if err != nil {
			return geometry, err
		}
		streetSupported = streetSupported || probe.denseStationStreetEligible(production, recipe)
	}
	geometry.street = variant%2 == 1 && streetSupported
	geometry.rotation = (variant / 2) % 2
	if variant >= 4 {
		choice := variant - 4
		scale := math.Max(1, math.Sqrt(float64(req.Plan.MachineCount)/96))
		geometry.latitude = math.Ceil([]float64{24, 32, 40}[choice]*scale/4) * 4
		geometry.latitude = denseLatitudeHeightLimit(ctx, geometry.latitude)
		zone, _ := ctx.Value(densityLatitudeKey{}).(string)
		selection, selected := densityLatitudeSelectionFor(zone)
		expand, _ := ctx.Value(denseAdjacentLatitudeKey{}).(bool)
		if selected && selection.tier >= 3 && choice == 0 && !expand {
			geometry.latitude = math.Min(geometry.latitude, 17)
		}
		geometry.order = 3
		geometry.span = []float64{12, 18, 32}[choice]
		compacted, _ := ctx.Value(denseSpatialCompactKey{}).(bool)
		if scale > 1 && !compacted {
			geometry.span = 32
		}
		geometry.groups = []int{0, 2, -1}[choice]
		geometry.street, geometry.rotation = streetSupported, 0
		geometry.materialOrder, geometry.consumerFirst = true, choice == 1
	}
	if span, _ := ctx.Value(denseSpatialHolesKey{}).(float64); span > 0 {
		geometry.span = 32
	}
	if !geometry.street {
		geometry.rotation, geometry.groups = 0, 0
	}
	return geometry, nil
}

// baselineStationCandidateIdentity 职责：组合有效几何和剩余全部上下文修饰；使用：稳定去重；边界：化工、炼油、原生占地、压缩、地面轨等修饰均不能丢失。
type baselineStationCandidateIdentity struct {
	modifiers int
	geometry  baselineStationCandidateGeometry
}

// baselineStationCandidateVariants 职责：先剥离无效修饰，再按实际执行参数保留首个计划；使用：普通与密铺塔派发前；边界：不试装、不缓存失败、不按实体数量或面积剪枝。
func (b *builder) baselineStationCandidateVariants(ctx context.Context, req Request, dataset *quant.Dataset, scope string, variants []int) ([]int, error) {
	normalized, inactive := b.baselineNormalizeStationVariants(ctx, req, variants)
	keys := make(map[int]baselineStationCandidateIdentity, len(normalized))
	for _, variant := range normalized {
		spec, err := baselineDecodeStationVariant(variant)
		if err != nil {
			return nil, err
		}
		base := spec.BaseLayout
		geometry, err := b.baselineStationGeometry(spec.baselineAlgorithmContext(ctx, req), req, dataset, base)
		if err != nil {
			return nil, err
		}
		keys[variant] = baselineStationCandidateIdentity{modifiers: variant - base, geometry: geometry}
	}
	unique := candidates.StableUnique(normalized, func(variant int) baselineStationCandidateIdentity { return keys[variant] })
	recordCandidatePlans(ctx, scope, len(variants), len(unique), inactive)
	return unique, nil
}

// baselineNormalizeStationVariants 职责：移除已证明不可用的修饰后按有效编号稳定去重；使用：塔候选装配之前；边界：共享出口无合流原本返回错误，此处只去掉该修饰，保留压缩等其余能力并重新完整验证。
func (b *builder) baselineNormalizeStationVariants(ctx context.Context, req Request, variants []int) ([]int, int) {
	sharedOutputs := b.bufferStationExportCanMerge(req)
	root, _ := ctx.Value(densityPolarRootKey{}).(bool)
	noOutputBranches := b.setting.EnableIntermediateBuffer || b.stationSingleOutputRow(req)
	result := make([]int, 0, len(variants))
	inactive := 0
	balanced := []int{}
	preferBalanced := b.setting.EnableIntermediateBuffer && root && denseNativeLargeRequest(ctx, req)
	for _, original := range variants {
		spec, err := baselineDecodeStationVariant(original)
		if err != nil {
			result = append(result, original)
			continue
		}
		isBalanced := spec.BalancedFrame
		for _, entry := range []struct {
			enabled *bool
			remove  bool
		}{
			{&spec.ProducerSplits, noOutputBranches}, {&spec.PairedOutputs, noOutputBranches},
			{&spec.SharedOutputs, !sharedOutputs}, {&spec.SharedInputs, b.setting.EnableIntermediateBuffer && !root},
			{&spec.IndependentStations, root},
		} {
			if *entry.enabled && entry.remove {
				*entry.enabled = false
				inactive++
			}
		}
		value := spec.baselineLegacyVariant()
		if preferBalanced && isBalanced {
			balanced = append(balanced, value)
		} else {
			result = append(result, value)
		}
	}
	return candidates.StableUnique(append(balanced, result...), func(value int) int { return value }), inactive
}

// baselineEnumerateDenseStationVariants 职责：保留所有真实黑盒和塔场资格；使用：具名枚举的唯一原始计划来源；边界：不装配、不派发、不择优。
func (b *builder) baselineEnumerateDenseStationVariants(ctx context.Context, req Request, dataset *quant.Dataset, input contract.StationLayoutInput) ([]int, int, error) {
	budget := 4
	for index, prepared := range input.Productions {
		if b.denseStationStreetEligible(input.Plan.Productions[index], prepared.Recipe) {
			budget = 6
			break
		}
	}
	fieldBudget := budget
	needsLocalRecycle, err := denseBlackboxHasMachineRecycle(req.Plan, dataset)
	if err != nil {
		return nil, 0, err
	}
	// 塔场路径尚未建立原生单元坐标；根球面候选即使完成平面接线，也会被最终编码明确拒绝。
	// 提前排除不具备目标坐标能力的候选，把同一总时限留给实际可编码的黑盒与后续单环搜索。
	if b.setting.EnableIntermediateBuffer || needsByproductPriority(req.Plan, b.flowGraph, dataset) || needsLocalRecycle || req.SurfaceLayout != nil && req.SurfaceLayout.polarRoot {
		fieldBudget = 0
	}
	blackboxBudget := 4
	narrowTier := false
	if req.SurfaceLayout != nil {
		selection, ok := densityLatitudeSelectionFor(req.SurfaceLayout.LatitudeZone)
		narrowTier = ok && selection.tier >= 2 && selection.tier <= 5
	}
	// 窄纬中的中等规模厂也需要低纬向高度候选，避免仅按机器数量决定候选集合而回退到多塔转发。
	if req.Plan.MachineCount >= 80 || narrowTier {
		blackboxBudget = 7
	}
	blackboxIndices := make([]int, blackboxBudget)
	for index := range blackboxIndices {
		blackboxIndices[index] = index
	}
	if !b.setting.EnableFlowMonitor {
		for index, production := range input.Productions {
			p, r := input.Plan.Productions[index], production.Recipe
			if p.MachineType == 4 && denseReferenceSource(p.MachineID) == 2317 && p.IncreasePoints == 0 && len(r.Items) == 3 && len(r.Results) == 1 && !recipeHasMachineRecycle(r) && referenceSingleArmsSufficient(p, r, b.setting.InserterLv) {
				blackboxIndices = append(blackboxIndices, 7, 8, 9)
				break
			}
		}
	}
	for index, production := range input.Productions {
		if b.denseRefineryEligible(input.Plan.Productions[index], production.Recipe) {
			blackboxIndices = append(blackboxIndices, 10, 11, 12)
			break
		}
	}
	// 共享原料使用独立候选；保留既有成功方案，按实际塔数、占地及线长比较完成终检的结果。
	if len(req.Plan.RawInputs) > 0 {
		base := 4
		if blackboxIndices[len(blackboxIndices)-1] >= 10 {
			base = 10
		} else if blackboxIndices[len(blackboxIndices)-1] >= 7 {
			base = 7
		}
		for variant := base; variant < base+3; variant++ {
			blackboxIndices = append(blackboxIndices, baselineDenseStationSharedInputVariantOffset+variant)
		}
	}
	if denseBalancedFrameEligible(ctx, req) && len(req.Plan.RawInputs) > 0 {
		balanced := baselineDenseBalancedFrameVariantOffset + blackboxIndices[len(blackboxIndices)-1]
		blackboxIndices = append(blackboxIndices, balanced)
		if req.Setting.EnableIntermediateBuffer {
			blackboxIndices = append(blackboxIndices, baselineDenseStationGroundTracksVariantOffset+balanced)
		}
	}
	// 共享出口同时保留基础容量候选，避免最后一个特殊布局失败时失去全部共享出口方案。
	lastLayout := blackboxIndices[len(blackboxIndices)-1]
	if lastLayout >= baselineDenseStationGroundTracksVariantOffset {
		lastLayout -= baselineDenseStationGroundTracksVariantOffset
	}
	sharedBase := 4
	if len(req.Plan.RawInputs) > 0 {
		sharedBase += baselineDenseStationSharedInputVariantOffset
	}
	blackboxIndices = append(blackboxIndices, baselineDenseStationSharedOutputVariantOffset+lastLayout)
	if lastLayout != sharedBase {
		blackboxIndices = append(blackboxIndices, baselineDenseStationSharedOutputVariantOffset+sharedBase)
	}
	if req.Density == densityDense {
		// 独立塔可围绕生产核心落位；保留固定阵列候选并比较完整路由后的实际代价。
		for _, variant := range []int{0, 2, sharedBase, sharedBase + 1, sharedBase + 2} {
			blackboxIndices = append(blackboxIndices, baselineIndependentStationVariantOffset+baselineDenseStationSharedOutputVariantOffset+variant)
		}
		for variant := sharedBase; variant < sharedBase+3; variant++ {
			blackboxIndices = append(blackboxIndices, baselineIndependentStationVariantOffset+baselineDenseSpatialCompactVariantOffset+baselineDenseStationSharedOutputVariantOffset+variant)
		}
		// 较短单元必须同时比较共享输入/输出，避免仅因分组变细而增加塔数；原成功候选继续保留。
		for variant := sharedBase; variant < sharedBase+3; variant++ {
			blackboxIndices = append(blackboxIndices, baselineDenseSpatialCompactVariantOffset+variant, baselineDenseSpatialCompactVariantOffset+baselineDenseStationSharedOutputVariantOffset+variant)
		}
		if lastLayout < sharedBase || lastLayout >= sharedBase+3 {
			blackboxIndices = append(blackboxIndices, baselineDenseSpatialCompactVariantOffset+lastLayout, baselineDenseSpatialCompactVariantOffset+baselineDenseStationSharedOutputVariantOffset+lastLayout)
		}
		for variant := sharedBase; variant < sharedBase+3; variant++ {
			blackboxIndices = append(blackboxIndices, baselineDenseStationPairedOutputVariantOffset+variant, baselineDenseStationPairedOutputVariantOffset+baselineDenseSpatialCompactVariantOffset+baselineDenseStationSharedOutputVariantOffset+variant)
		}
		for variant := sharedBase; variant < sharedBase+3; variant++ {
			blackboxIndices = append(blackboxIndices, baselineDenseStationProducerVariantOffset+baselineDenseSpatialCompactVariantOffset+variant, baselineDenseStationProducerVariantOffset+baselineDenseSpatialCompactVariantOffset+baselineDenseStationSharedOutputVariantOffset+variant)
		}
		// 保留最长街道的基础布局，仅在独立机器行确实改变时比较20格整列短行和18格真实L/U接角。
		for _, option := range []struct {
			offset int
			span   float64
		}{{baselineDenseSpatialHolesVariantOffset, 20}, {baselineDenseSpatialHolesBentVariantOffset, 18}} {
			eligible, err := b.denseStationHoleEligible(req, dataset, option.span)
			if err != nil {
				return nil, 0, err
			}
			if eligible {
				blackboxIndices = append(blackboxIndices, option.offset+baselineDenseStationSharedOutputVariantOffset+sharedBase+2)
			}
		}
	}
	return blackboxIndices, fieldBudget, nil
}
