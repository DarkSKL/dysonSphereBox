package build

// 文件职责：定义物流塔具名候选、纯枚举和单次联合装配的领域边界。
// 使用：工作流工具按只读输入枚举规格，再独立求解、验证和编码。
// 边界：单次求解不调度、不择优、不编码；枚举、归一化与装配只消费具名策略。

import (
	"context"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/contract"
	"DysonSphereBox/helper/blueprint/build/shared/stationlayout"
)

// StationSearchOptions 职责：选择有限候选族；使用：工具默认参数或显式枚举；边界：空族等于all，不能覆盖设备事实或删除不合资格保护。
type StationSearchOptions struct {
	// Family 职责：选择all、blackbox或field；使用：显式候选组；边界：field仅返回真实合资格的密铺塔场。
	Family string `json:"family" enum:",all,blackbox,field"`
	// StandardRowSpans 职责：显式选择普通短行装箱；使用：普通塔枚举；边界：空表保留旧默认，密铺不消费。
	StandardRowSpans []float64 `json:"standardRowSpans"`
	// StandardRowPacking 职责：显式启用普通功能行阵；使用：生产方案；边界：默认关闭，不影响密铺和旧差分基线。
	StandardRowPacking bool `json:"standardRowPacking"`
}

// DefaultStationSearchOptions 职责：集中提供塔候选默认策略；使用：工作流工具注册；边界：不按模式密度复制默认值。
func DefaultStationSearchOptions() StationSearchOptions { return StationSearchOptions{Family: "all"} }

// ValidateStationSearchOptions 职责：校验有限塔候选族枚举；使用：ToolSpec 静态参数及核心资格入口；边界：纯函数，空字符串保留既有 all 语义，不读取设备或装配。
func ValidateStationSearchOptions(options StationSearchOptions) error {
	if err := validateStandardRowSpans(options.StandardRowSpans); err != nil {
		return err
	}
	switch options.Family {
	case "", "all", "blackbox", "field":
		return nil
	default:
		return &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "未知物流塔候选族"}
	}
}

// StationCandidateSpec 职责：保存一个塔候选实际消费的全部策略值；使用：跨步骤只读交接；边界：无建筑指针、回调或工作流身份，不携带旧编号。
type StationCandidateSpec struct {
	// RowPacking 职责：启用普通功能行阵；使用：显式近方形候选；边界：与共享塔场的FieldPacking配置独立。
	RowPacking bool `json:"rowPacking"`
	// InputFingerprint 职责：绑定规范输入；使用：阻止跨任务误用；边界：不代替实体终检。
	InputFingerprint string `json:"inputFingerprint"`
	// Kind 职责：选择blackbox或field实体拓扑；使用：单次求解；边界：未知值拒绝。
	Kind string `json:"kind"`
	// Density 职责：绑定普通或密铺硬规则；使用：一致性校验；边界：不得改写输入密度。
	Density string `json:"density"`
	// StationID 职责：绑定真实物流塔原型；使用：共享设备查询；边界：不允许改变输入模式的塔。
	StationID int `json:"stationId"`
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
	// ChemicalFour 职责：启用四端化工模板；使用：既有资格；边界：炼油候选同时保留该能力以支持混合工厂。
	ChemicalFour bool `json:"chemicalFour"`
	// MaterialNetworks 职责：启用全场物料网络；使用：物料布局族；边界：不代表拓扑已验证。
	MaterialNetworks bool `json:"materialNetworks"`
	// FieldPacking 职责：保存塔场阵列或普通功能行阵的实际放置策略；使用：单次装配；边界：不包含编号，不改变拓扑或设备。
	FieldPacking StationFieldPacking `json:"fieldPacking"`
	// NearbyInputPorts 职责：标识获选规格派生的就近口精修；使用：引擎显式新候选；边界：不携带winner实体。
	NearbyInputPorts bool `json:"nearbyInputPorts"`
}

// algorithmContext 职责：将显式塔策略适配到现有几何原语；使用：单候选局部计算；边界：上下文仅在本次调用内存在，不跨步骤保存。
func (spec StationCandidateSpec) algorithmContext(ctx context.Context, req Request) context.Context {
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

// checkStationInput 职责：核对塔模式和共享设备事实；使用：枚举与单解入口；边界：零值或其它模式不回退为塔。
func checkStationInput(input *PreparedInput) (contract.StationConfig, error) {
	if input == nil || input.dataset == nil || input.graph == nil || input.policy == nil || input.policy.station == nil || (input.request.Mode != 3 && input.request.Mode != 4) {
		return contract.StationConfig{}, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔核心需要完整的行星塔或星际塔输入"}
	}
	config := input.policy.station.Config
	if err := stationlayout.ValidateConfiguration(config); err != nil {
		return config, err
	}
	if config.Mode != input.request.Mode {
		return config, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔事实与输入模式不一致"}
	}
	return config, nil
}

// EnumerateStationCandidates 职责：返回普通或密铺塔所有有效具名候选；使用：引擎建立隔离候选组；边界：不装配实体、不执行候选、不选择结果。
func EnumerateStationCandidates(ctx context.Context, input *PreparedInput, options StationSearchOptions) ([]StationCandidateSpec, error) {
	if input == nil {
		return nil, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔输入为空"}
	}
	ctx = input.algorithmContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	config, err := checkStationInput(input)
	if err != nil {
		return nil, err
	}
	if options.Family == "" {
		options.Family = DefaultStationSearchOptions().Family
	}
	if err := ValidateStationSearchOptions(options); err != nil {
		return nil, err
	}
	b, err := input.newAssembly(ctx, nil)
	if err != nil {
		return nil, err
	}
	req := input.request
	plans, fieldBudget := stationRowPlans(), 0
	if req.Density == densityDense {
		prepared, err := prepareStationLayoutInput(ctx, req, input.dataset, b.setting, input.graph)
		if err != nil {
			return nil, err
		}
		plans, fieldBudget, err = b.enumerateDenseStationPlans(ctx, req, input.dataset, prepared)
		if err != nil {
			return nil, err
		}
	} else {
		for _, plan := range stationRowPlans() {
			plan.spec.IndependentStations = true
			plans = append(plans, plan)
		}
		for _, plan := range standardStationRowPlans(options.StandardRowSpans) {
			plan.spec.RowPacking = options.StandardRowPacking
			plans = append(plans, plan)
		}
	}
	specs, err := b.stationCandidateSpecs(ctx, req, input.dataset, "station-"+req.Density, plans)
	if err != nil {
		return nil, err
	}
	result := []StationCandidateSpec{}
	if options.Family != "field" {
		result = append(result, specs...)
	}
	if options.Family != "blackbox" {
		// 四种真实放置组合先保留独立行，再保留有资格的前两种连续街道。
		packing := []StationFieldPacking{{}, {NarrowColumns: true}, {Staggered: true}, {Staggered: true, NarrowColumns: true}}
		for index := 0; index < fieldBudget; index++ {
			choice := packing[index%len(packing)]
			span := 24.
			if choice.NarrowColumns {
				span = 32
			}
			result = append(result, StationCandidateSpec{Kind: "field", FieldPacking: choice, Street: index >= len(packing), MaxSpan: span})
		}
	}
	for i := range result {
		result[i].InputFingerprint, result[i].Density, result[i].StationID = input.fingerprint, req.Density, config.StationID
	}
	return prioritizeLargeStandardStationRows(ctx, input, options, result)
}

// RefineStationCandidate 职责：由获选黑盒规格派生独立就近塔口规格；使用：引擎显式精修候选；边界：不读取winner实体、不分配预算或运行求解。
func RefineStationCandidate(spec StationCandidateSpec) (StationCandidateSpec, error) {
	if spec.Kind != "blackbox" || spec.NearbyInputPorts {
		return StationCandidateSpec{}, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "就近塔口精修需要尚未精修的黑盒规格"}
	}
	spec.NearbyInputPorts = true
	return spec, nil
}

// assembleStationCandidate 职责：在独立短时内存中装配一个具名规格；使用：新核心及临时兼容桥；边界：不终检、不编码、不构造或调用其它候选。
func assembleStationCandidate(ctx context.Context, input *PreparedInput, spec StationCandidateSpec, preview *generationPreview) (*builder, error) {
	config, err := checkStationInput(input)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.InputFingerprint != input.fingerprint || spec.Density != input.request.Density || spec.StationID != config.StationID {
		return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: "物流塔候选规格不属于此规范输入"}
	}
	if spec.Kind != "blackbox" && spec.Kind != "field" {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "未知物流塔实体拓扑"}
	}
	if spec.RowPacking && (spec.Kind != "blackbox" || spec.Density != densityStandard || spec.MaterialOrder) {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "功能行阵仅接受普通非物料排序黑盒"}
	}
	if spec.Order < 0 || spec.Order > 3 || spec.Rotation < 0 || spec.Rotation > 3 || spec.Groups != -1 && spec.Groups != 0 && spec.Groups != 2 || spec.HeightChoice < 0 || spec.HeightChoice > 1 || !finitePositive(spec.MaxSpan) || spec.MaxSpan > 10000 || spec.LatitudeSpan < 0 || math.IsNaN(spec.LatitudeSpan) || math.IsInf(spec.LatitudeSpan, 0) || spec.HoleSpan != 0 && spec.HoleSpan != 18 && spec.HoleSpan != 20 || spec.ProducerSplits && spec.SplitterModel != 39 || spec.ConsumerFirst && !spec.MaterialOrder || spec.BentRows != (spec.HoleSpan == 18) {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "物流塔候选几何或分流器参数无效"}
	}
	if spec.Kind == "field" && (spec.GroundTracks || spec.IndependentStations || spec.HoleSpan > 0 || spec.ProducerSplits || spec.PairedOutputs || spec.Compact || spec.SharedOutputs || spec.BalancedFrame || spec.SharedInputs || spec.RefineryCollinear || spec.ChemicalFour || spec.MaterialNetworks || spec.MaterialOrder || spec.NearbyInputPorts) {
		return nil, &blueprint.Error{Code: "GEN_E_LAYOUT", Message: "塔场规格不能夹带黑盒专属拓扑策略"}
	}
	ctx = spec.algorithmContext(ctx, input.request)
	if spec.RowPacking {
		ctx = context.WithValue(ctx, denseBalancedFrameKey{}, false)
	}
	b, err := input.newAssembly(ctx, preview)
	if err != nil {
		return nil, err
	}
	b.beginPreview()
	if spec.Kind == "blackbox" {
		if spec.RowPacking {
			ctx = b.configureStandardStationRows(ctx)
			b.generationCtx = ctx
		}
		if spec.Density == densityStandard {
			// 普通行的局部估算没有塔和边界带；复用裁尾后全部实体模块的预算，保留普通碰撞与带间距。
			b.directBoundaryExtentScale = 1.25
		}
		err = b.assembleStationBlackbox(ctx, input.request, input.dataset, config, spec)
	} else {
		err = b.assembleStationField(ctx, input, config, spec)
	}
	return b, err
}

// AssembleStationCandidate 职责：只装配一个显式规格并冻结装配图；使用：工作流装配节点；边界：后续定位、补电、绑定、规则和编码由独立节点执行。
func AssembleStationCandidate(ctx context.Context, input *PreparedInput, spec StationCandidateSpec) (AssemblySnapshot, error) {
	if input == nil {
		return AssemblySnapshot{}, &blueprint.Error{Code: "GEN_E_MODE", Message: "物流塔输入为空"}
	}
	ctx = input.algorithmContext(ctx)
	b, err := assembleStationCandidate(ctx, input, spec, previewFromContext(ctx).fork())
	if err != nil {
		if b != nil {
			b.rollbackPreview()
		}
		return AssemblySnapshot{}, err
	}
	geometry, err := freezeAssembly(b.generationCtx, b, input.request, input.dataset)
	if err != nil {
		b.rollbackPreview()
	}
	return geometry, err
}

// assembleStationField 职责：装配一次共享塔场并连接真实路线；使用：合资格密铺field规格；边界：无完整证据能力的输入拒绝，不降级为其它拓扑。
func (b *builder) assembleStationField(ctx context.Context, input *PreparedInput, config contract.StationConfig, spec StationCandidateSpec) error {
	req := input.request
	if req.Density != densityDense || spec.NearbyInputPorts || b.setting.EnableIntermediateBuffer || needsByproductPriority(req.Plan, b.flowGraph, input.dataset) || req.SurfaceLayout != nil && req.SurfaceLayout.polarRoot {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "此输入不具备共享塔场的完整实体能力"}
	}
	recycle, err := denseBlackboxHasMachineRecycle(req.Plan, input.dataset)
	if err != nil {
		return err
	}
	if recycle {
		return &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: "共享塔场不支持本地毛流回收"}
	}
	prepared, err := prepareStationLayoutInput(ctx, req, input.dataset, b.setting, b.flowGraph)
	if err != nil {
		return err
	}
	plan, err := stationlayout.PlanField(ctx, prepared, config)
	if err != nil {
		return err
	}
	if err := stationlayout.Validate(ctx, prepared, plan, config); err != nil {
		return err
	}
	b.denseSpatial, b.connectedGlobalPower = true, spec.Street
	rows, routes, err := b.assembleDenseStationRows(ctx, prepared, plan, config, spec.MaxSpan, spec.Street)
	if err != nil {
		return err
	}
	towers, err := planDenseStationTowers(ctx, rows, routes, config.ItemSlots)
	if err != nil {
		return err
	}
	if err := b.placeDenseStationField(ctx, rows, routes, towers, config, spec.FieldPacking); err != nil {
		return err
	}
	if err := b.connectDenseStationField(ctx, rows, routes, towers, config); err != nil {
		return err
	}
	b.surfaceBlocks = nil
	b.zones = make([]Zone, 0, len(plan.Zones))
	for _, zone := range plan.Zones {
		b.zones = append(b.zones, zoneFromLayout(zone.Zone))
	}
	if req.SurfaceLayout != nil {
		err = b.completeConnectedSurfacePower(ctx, req)
	} else {
		err = b.completeBoundaryFlowMonitors(ctx, req.Mode)
		if err == nil {
			err = b.completeConnectedPower(ctx)
		}
	}
	if err != nil {
		return err
	}
	for _, link := range b.stationLinks {
		previous := link.head
		for _, belt := range link.path {
			a, c := previous.a, belt.a
			b.logistics.Metrics.InternalBeltLength += math.Sqrt((a.x-c.x)*(a.x-c.x) + (a.y-c.y)*(a.y-c.y) + (a.z-c.z)*(a.z-c.z))
			previous = belt
		}
	}
	return nil
}

// BetterStationCandidate 职责：将各密度及塔策略的最终比较交给共同形状规则；使用：引擎稳定归约完整终检结果；边界：先比较1.8达标档，同档再节省真实塔数，规格不改变评分，同分返回false。
func BetterStationCandidate(aSpec StationCandidateSpec, aTowers int, a LayoutMetrics, bSpec StationCandidateSpec, bTowers int, b LayoutMetrics) bool {
	return BetterLayoutCandidate(a, aTowers, b, bTowers)
}

// BetterStationRefinement 职责：在原有就近口约束内按共同形状规则批准精修；使用：引擎归约可选精修；边界：塔数不变、面积不增、带长严格减少，且不能令已达标布局变狭长；双侧均缺形状的历史探针仅保留旧成本判断。
func BetterStationRefinement(candidateTowers int, candidate LayoutMetrics, baseTowers int, base LayoutMetrics) bool {
	if candidateTowers != baseTowers || candidate.EnvelopeArea > base.EnvelopeArea+1e-6 || candidate.TotalBeltLengthMeters >= base.TotalBeltLengthMeters-1e-6 {
		return false
	}
	if candidate.EnvelopeAspectRatio == 0 && base.EnvelopeAspectRatio == 0 {
		candidate.EnvelopeAspectRatio, base.EnvelopeAspectRatio = 1, 1
		return validLayoutCandidateCost(candidate, candidateTowers) && validLayoutCandidateCost(base, baseTowers)
	}
	return BetterLayoutCandidate(candidate, candidateTowers, base, baseTowers)
}

// StationCandidateTowerCount 职责：直接读取不可变定稿中的真实塔数；使用：最终验证后的评分；边界：不返回内部记录，不把零值快照当作可行结果。
func StationCandidateTowerCount(geometry GeometrySnapshot) (int, error) {
	if geometry.data == nil {
		return 0, &blueprint.Error{Code: "GEN_E_FINAL_SNAPSHOT", Message: "物流塔评分缺少几何定稿"}
	}
	count := 0
	for _, record := range geometry.data.document.Buildings {
		if surfaceIsStation(record.ItemID) {
			count++
		}
	}
	return count, nil
}
