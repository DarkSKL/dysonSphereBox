package build

// 文件职责：定义必须逐项完成的封闭规则证据与静态依赖。
// 使用：纯核心单规则入口及外部独立工具登记；边界：不依赖工作流、YAML或反射，未知或遗漏规则不能获得最终批准。

import (
	"DysonSphereBox/helper/blueprint"
	"context"
	"fmt"
	"unsafe"
)

// RuleKind 职责：标识一个真实规则或度量职责；使用：独立工具登记；边界：不作为可关闭硬约束的策略参数。
type RuleKind string

// 规则标识职责：固定最终批准必须具备的全部证明；使用：静态依赖和工具目录；边界：不适用规则仍须按可信请求形成明确证据。
const (
	// RuleReferences 职责：标识references检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleReferences RuleKind = "references"
	// RuleLatitude 职责：标识latitude检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleLatitude RuleKind = "latitude"
	// RuleMachines 职责：标识machines检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleMachines RuleKind = "machines"
	// RulePastedSlots 职责：标识pasted-slots检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RulePastedSlots RuleKind = "pasted-slots"
	// RuleBeltDistance 职责：标识belt-distance检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleBeltDistance RuleKind = "belt-distance"
	// RuleStaticColliders 职责：标识static-colliders检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleStaticColliders RuleKind = "static-colliders"
	// RuleTankPorts 职责：标识tank-ports检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleTankPorts RuleKind = "tank-ports"
	// RulePowerSpacing 职责：标识power-spacing检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RulePowerSpacing RuleKind = "power-spacing"
	// RuleStationSpacing 职责：标识station-spacing检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleStationSpacing RuleKind = "station-spacing"
	// RuleBeltCrossings 职责：标识belt-crossings检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleBeltCrossings RuleKind = "belt-crossings"
	// RuleBeltBuildingOccupancy 职责：标识belt-building-occupancy检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleBeltBuildingOccupancy RuleKind = "belt-building-occupancy"
	// RuleInserterGeometry 职责：标识inserter-geometry检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleInserterGeometry RuleKind = "inserter-geometry"
	// RuleSplitterClearance 职责：标识splitter-clearance检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleSplitterClearance RuleKind = "splitter-clearance"
	// RuleStationPorts 职责：标识station-ports检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleStationPorts RuleKind = "station-ports"
	// RuleStorageRoof 职责：标识storage-roof检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleStorageRoof RuleKind = "storage-roof"
	// RulePowerNetwork 职责：标识power-network检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RulePowerNetwork RuleKind = "power-network"
	// RuleSprayChannels 职责：标识spray-channels检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleSprayChannels RuleKind = "spray-channels"
	// RuleTransportTopology 职责：标识transport-topology检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleTransportTopology RuleKind = "transport-topology"
	// RuleByproductPriority 职责：标识byproduct-priority检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleByproductPriority RuleKind = "byproduct-priority"
	// RuleBuffers 职责：标识buffers检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleBuffers RuleKind = "buffers"
	// RuleDeliveries 职责：标识deliveries检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleDeliveries RuleKind = "deliveries"
	// RuleProducerSplits 职责：标识producer-splits检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleProducerSplits RuleKind = "producer-splits"
	// RuleMaterialNetworks 职责：标识material-networks检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleMaterialNetworks RuleKind = "material-networks"
	// RuleRoutes 职责：标识routes检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleRoutes RuleKind = "routes"
	// RuleRecycles 职责：标识recycles检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleRecycles RuleKind = "recycles"
	// RuleSelfSpray 职责：标识self-spray检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleSelfSpray RuleKind = "self-spray"
	// RuleLeftovers 职责：标识leftovers检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleLeftovers RuleKind = "leftovers"
	// RuleSharedCapacity 职责：标识shared-capacity检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleSharedCapacity RuleKind = "shared-capacity"
	// RuleConservation 职责：标识conservation检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleConservation RuleKind = "conservation"
	// RuleBoundaries 职责：标识boundaries检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleBoundaries RuleKind = "boundaries"
	// RuleMonitors 职责：标识monitors检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleMonitors RuleKind = "monitors"
	// RuleLayoutMetrics 职责：标识layout-metrics检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleLayoutMetrics RuleKind = "layout-metrics"
	// RuleDensityReport 职责：标识density-report检查；使用：独立规则工具；边界：不能由YAML省略后仍完成批准。
	RuleDensityReport RuleKind = "density-report"
)

// RequiredRuleKinds 职责：按依赖先后返回必需规则副本；使用：工具注册和核心回归；边界：返回切片的修改不能改变批准策略。
func RequiredRuleKinds() []RuleKind {
	return []RuleKind{RuleReferences, RuleLatitude, RulePastedSlots, RuleBeltDistance, RuleStaticColliders, RuleTankPorts, RulePowerSpacing, RuleStationSpacing, RuleBeltCrossings, RuleBeltBuildingOccupancy, RuleInserterGeometry, RuleSplitterClearance, RuleStationPorts, RuleStorageRoof, RulePowerNetwork, RuleTransportTopology, RuleByproductPriority, RuleBuffers, RuleMachines, RuleSprayChannels, RuleDeliveries, RuleProducerSplits, RuleMaterialNetworks, RuleRoutes, RuleRecycles, RuleSelfSpray, RuleLeftovers, RuleSharedCapacity, RuleConservation, RuleBoundaries, RuleMonitors, RuleLayoutMetrics, RuleDensityReport}
}

// RuleDependencies 职责：返回规则真正消费的前置证明；使用：ToolSpec requires/provides和直接核心调用；边界：未知ID不能进入检查，返回集合不共享存储。
func RuleDependencies(kind RuleKind) []RuleKind {
	switch kind {
	case RuleReferences:
		return []RuleKind{}
	case RuleLatitude:
		return []RuleKind{RuleReferences}
	case RuleMachines:
		return []RuleKind{RuleReferences, RuleBuffers}
	case RulePastedSlots:
		return []RuleKind{RuleReferences}
	case RuleBeltDistance:
		return []RuleKind{RuleReferences}
	case RuleStaticColliders:
		return []RuleKind{RuleReferences}
	case RuleTankPorts:
		return []RuleKind{RuleReferences}
	case RulePowerSpacing:
		return []RuleKind{RuleReferences}
	case RuleStationSpacing:
		return []RuleKind{RuleReferences}
	case RuleBeltCrossings:
		return []RuleKind{RuleReferences}
	case RuleBeltBuildingOccupancy:
		return []RuleKind{RuleReferences}
	case RuleInserterGeometry:
		return []RuleKind{RuleReferences}
	case RuleSplitterClearance:
		return []RuleKind{RuleReferences}
	case RuleStationPorts:
		return []RuleKind{RuleReferences}
	case RuleStorageRoof:
		return []RuleKind{RuleReferences}
	case RulePowerNetwork:
		return []RuleKind{RuleReferences, RulePowerSpacing}
	case RuleSprayChannels:
		return []RuleKind{RuleMachines}
	case RuleTransportTopology:
		return []RuleKind{RuleReferences, RuleStationPorts}
	case RuleByproductPriority:
		return []RuleKind{RuleTransportTopology}
	case RuleBuffers:
		return []RuleKind{RuleByproductPriority}
	case RuleDeliveries:
		return []RuleKind{RuleBuffers}
	case RuleProducerSplits:
		return []RuleKind{RuleDeliveries}
	case RuleMaterialNetworks:
		return []RuleKind{RuleProducerSplits}
	case RuleRoutes:
		return []RuleKind{RuleMaterialNetworks}
	case RuleRecycles:
		return []RuleKind{RuleRoutes}
	case RuleSelfSpray:
		return []RuleKind{RuleRecycles}
	case RuleLeftovers:
		return []RuleKind{RuleSelfSpray}
	case RuleSharedCapacity:
		return []RuleKind{RuleLeftovers}
	case RuleConservation:
		return []RuleKind{RuleSharedCapacity}
	case RuleBoundaries:
		return []RuleKind{RuleConservation}
	case RuleMonitors:
		return []RuleKind{RuleBoundaries, RuleMachines}
	case RuleLayoutMetrics:
		return []RuleKind{RuleMonitors}
	case RuleDensityReport:
		return []RuleKind{RuleLayoutMetrics}
	default:
		return nil
	}
}

// RuleParameters 职责：记录本次规则的有效可信参数；使用：诊断和证明绑定；边界：只读值副本，不提供放宽规则的输入开关。
type RuleParameters struct {
	// Mode 职责：记录可信设备拓扑；使用：运输规则溯源；边界：不能由规则工具改写。
	Mode int
	// BeltLevel/InserterLevel 职责：记录所选真实设备等级；使用：容量和跨度诊断；边界：不作为升档策略输入。
	BeltLevel     int
	InserterLevel int
	// DataVersion 职责：绑定量化原型版本；使用：能力证明；边界：不伪装几何版本一致。
	DataVersion string
	// Density 职责：固定物理密度；使用：规则解释；边界：来自几何快照。
	Density string
	// NeedPower 职责：固定用户供电选择；使用：覆盖报告；边界：YAML不能更改。
	NeedPower bool
	// DenseCollisions 职责：固定整图粘贴口径；使用：碰撞审计；边界：来自已冻结候选。
	DenseCollisions bool
	// SplitterModel 职责：固定分流器模型；使用：物料树核验；边界：不猜替代模型。
	SplitterModel int16
	// SurfaceMode 职责：固定坐标方式；使用：规则范围；边界：不是可变布局句柄。
	SurfaceMode string
	// LatitudeZone 职责：固定许可纬带；使用：越界检查；边界：不从报告反推授权。
	LatitudeZone string
}

// ruleProofVersion 职责：固定单项证明契约；使用：检查和汇合；边界：真实设备来源另以RuleProfile绑定。
const ruleProofVersion = "final-rule-proof-v1"

// RuleProof 职责：封闭持有某项真实检查的成功结果；使用：下一规则或CompleteValidation；边界：零值、错误、取消、panic不会形成证明。
type RuleProof struct {
	// data 职责：保存只读证据；使用：内部依赖；边界：不暴露实体或可变报告。
	data *ruleProofData
}

// ruleProofIdentity 职责：标识独立定稿候选；使用：GeometrySnapshot冻结时创建；边界：非零大小保证独立分配身份，不引用实体或请求。
type ruleProofIdentity struct{ marker byte }

// ruleProofStamp 职责：标识一次成功检查的谱系节点；使用：依赖匹配；边界：不引用父证明或其报告，避免隐式保留整条历史。
type ruleProofStamp struct{ marker byte }

// ruleProofDependency 职责：保存实际消费的前置证明身份；使用：完整批准的谱系核对；边界：只保存轻量章和规则ID，不反向持有前置报告。
type ruleProofDependency struct {
	// kind/stamp 职责：绑定前置规则及该次成功检查；使用：与完整集合逐项匹配；边界：不能拼接另一次同内容检查。
	kind  RuleKind
	stamp *ruleProofStamp
}

// ruleProofData 职责：绑定内容、规则来源和必要输出；使用：封闭规则交接；边界：不保存可变Builder或工作流实例。
type ruleProofData struct {
	// identity 职责：绑定候选轻量身份；使用：拒绝跨候选；边界：不保留整图。
	identity *ruleProofIdentity
	// stamp 职责：标识这次成功检查；使用：依赖谱系；边界：不引用其它产物。
	stamp *ruleProofStamp
	// fingerprint 职责：绑定准确内容；使用：交接失效检查；边界：报告不推进此值。
	fingerprint [32]byte
	// kind/version/profile 职责：绑定规则及来源版本；使用：汇合及编码；边界：不能用布尔完成标记替代。
	kind    RuleKind
	version string
	profile RuleProfile
	// parameters 职责：固定有效请求参数；使用：审计；边界：只读值。
	parameters RuleParameters
	// parents 职责：记录实际消费的前置证明；使用：依赖完整性；边界：只共享封闭产物。
	parents []ruleProofDependency
	// audit 职责：保存根规则初始化的审计元数据；使用：汇合；边界：不预先宣称未运行规则通过。
	audit *RuleAudit
	// logistics 职责：保存逐项物流输出；使用：下一规则及指标；边界：不得通过公开方法取得内部切片。
	logistics *ruleLogisticsState
	// checks 职责：保存当前规则对物流摘要的新增诊断；使用：汇合；边界：不修改输入报告。
	checks []LogisticsCheck
	// density 职责：保存独立密度度量；使用：最终结果；边界：编码摘要稍后填写。
	density *DensityReport
	// networkCount 职责：保存实际供电网络数；使用：审计；边界：不是供电容量证明。
	networkCount int
	// bytes 职责：保守计量产物空间；使用：工具工作区预算；边界：不是RSS。
	bytes int64
}

// Kind 职责：返回已证明规则ID；使用：静态工具交接；边界：零值为空。
func (proof RuleProof) Kind() RuleKind {
	if proof.data == nil {
		return ""
	}
	return proof.data.kind
}

// Version 职责：返回证据契约版本；使用：诊断；边界：零值为空。
func (proof RuleProof) Version() string {
	if proof.data == nil {
		return ""
	}
	return proof.data.version
}

// Fingerprint 职责：返回内容指纹；使用：实例依赖；边界：不代表任意其它规则通过。
func (proof RuleProof) Fingerprint() [32]byte {
	if proof.data == nil {
		return [32]byte{}
	}
	return proof.data.fingerprint
}

// Parameters 职责：返回有效参数值副本；使用：追踪；边界：修改副本不能更改证明。
func (proof RuleProof) Parameters() RuleParameters {
	if proof.data == nil {
		return RuleParameters{}
	}
	return proof.data.parameters
}

// EstimatedBytes 职责：返回保守证据大小；使用：工作区预算；边界：只读对象可能重复计量。
func (proof RuleProof) EstimatedBytes() int64 {
	if proof.data == nil {
		return 0
	}
	return proof.data.bytes
}

// Checks 职责：返回当前规则诊断副本；使用：报告发布；边界：写副本不改变证据内容。
func (proof RuleProof) Checks() []LogisticsCheck {
	if proof.data == nil {
		return nil
	}
	return append([]LogisticsCheck(nil), proof.data.checks...)
}

// Logistics 职责：返回本规则已有的物流报告副本；使用：诊断和独立比较；边界：不运行任何规则，修改副本不能改变证明。
func (proof RuleProof) Logistics() *LogisticsSummary {
	if proof.data == nil || proof.data.logistics == nil {
		return nil
	}
	c := newFinalizationCloner()
	return copyFinalizationPointer(c, proof.data.logistics.summary, c.buildLogisticsSummary)
}

// Metrics 职责：按值返回已存在指标；使用：独立度量证明；边界：没有度量输出时返回零，不冒充可编码批准。
func (proof RuleProof) Metrics() LayoutMetrics {
	if proof.data == nil || proof.data.logistics == nil || proof.data.logistics.summary == nil {
		return LayoutMetrics{}
	}
	return proof.data.logistics.summary.Metrics
}

// Density 职责：返回独立密度度量副本；使用：报告发布；边界：不补跑度量或共享Productions切片。
func (proof RuleProof) Density() *DensityReport {
	if proof.data == nil {
		return nil
	}
	c := newFinalizationCloner()
	return copyFinalizationPointer(c, proof.data.density, c.buildDensityReport)
}

// ruleParameters 职责：从冻结内容提取有效参数；使用：签发和核对；边界：不读取外部配置。
func ruleParameters(g GeometrySnapshot) RuleParameters {
	r := g.data.request
	p := RuleParameters{Mode: r.Mode, BeltLevel: r.Setting.BeltLv, InserterLevel: r.Setting.InserterLv, DataVersion: r.Plan.DataVersion, Density: r.Density, NeedPower: r.Setting.NeedPower == nil || *r.Setting.NeedPower, DenseCollisions: g.data.rules.DenseCollisions, SplitterModel: g.data.rules.SplitterModel}
	if r.SurfaceLayout != nil {
		p.SurfaceMode = r.SurfaceLayout.Mode
		p.LatitudeZone = r.SurfaceLayout.LatitudeZone
	}
	return p
}

// ruleProofError 职责：构造封闭证据错误；使用：依赖和批准守卫；边界：不把缺失证据当普通布局成功。
func ruleProofError(message string) error {
	return &blueprint.Error{Code: "GEN_E_RULE_PROOF", Message: message}
}

// validRuleKind 职责：识别固定必需ID；使用：签发前；边界：不接受自定义跳过项。
func validRuleKind(kind RuleKind) bool {
	for _, required := range RequiredRuleKinds() {
		if kind == required {
			return true
		}
	}
	return false
}

// verifyRuleProof 职责：核对候选、内容、版本及有效参数；使用：规则依赖和批准；边界：不代运行缺失规则。
func verifyRuleProof(g GeometrySnapshot, proof RuleProof, profile RuleProfile) error {
	p := proof.data
	if p == nil || p.stamp == nil || !validRuleKind(p.kind) || g.data.proofIdentity == nil || p.identity != g.data.proofIdentity || p.fingerprint != g.data.fingerprint || p.version != ruleProofVersion || p.profile != profile || p.parameters != ruleParameters(g) {
		return ruleProofError("规则证明缺失、跨候选、内容或版本不一致")
	}
	expected := RuleDependencies(p.kind)
	if len(p.parents) != len(expected) {
		return ruleProofError("规则证明的依赖集合不完整")
	}
	for i, parent := range p.parents {
		if parent.stamp == nil || parent.kind != expected[i] {
			return ruleProofError("规则证明依赖已经失效")
		}
	}
	return nil
}

// CheckRule 职责：只执行指定真实规则并签发成功证明；使用：具名独立工具共享固定分派；边界：不是工作流调度器，缺依赖、错误、取消及panic返回零证明。
func CheckRule(ctx context.Context, g GeometrySnapshot, kind RuleKind, deps ...RuleProof) (proof RuleProof, err error) {
	defer func() {
		if failure := recover(); failure != nil {
			proof = RuleProof{}
			err = fmt.Errorf("规则%s内部异常：%v", kind, failure)
		}
	}()
	reportProgress(ctx, "validation", ruleProgressMessage(kind), 0, 0)
	if err = checkFinalGeometry(ctx, g); err != nil {
		return RuleProof{}, err
	}
	if !validRuleKind(kind) {
		return RuleProof{}, ruleProofError("未知最终规则")
	}
	if g.data.proofIdentity == nil {
		return RuleProof{}, ruleProofError("几何定稿缺少独立证明身份")
	}
	profile, err := verifiedRuleProfile()
	if err != nil {
		return RuleProof{}, &blueprint.Error{Code: "GEN_E_COLLISION_UNVERIFIED", Message: err.Error()}
	}
	required := RuleDependencies(kind)
	if len(required) != len(deps) {
		return RuleProof{}, ruleProofError("规则前置证明数量不匹配")
	}
	parents := make([]RuleProof, len(required))
	for i, expected := range required {
		found := false
		for _, dep := range deps {
			if dep.Kind() == expected {
				if found {
					return RuleProof{}, ruleProofError("规则前置证明重复")
				}
				if err := verifyRuleProof(g, dep, profile); err != nil {
					return RuleProof{}, err
				}
				parents[i] = dep
				found = true
			}
		}
		if !found {
			return RuleProof{}, ruleProofError("缺少规则前置证明：" + string(expected))
		}
	}
	env := newRuleEnvironment(ctx, g, kind, parents)
	if err := env.execute(kind); err != nil {
		return RuleProof{}, err
	}
	if err := env.checkUnchanged(); err != nil {
		return RuleProof{}, err
	}
	// 输入已在入口核对，规则只操作独占复制视图且上面已核对该视图内容；无需第三次序列化同一冻结输入。
	if err := ctx.Err(); err != nil {
		return RuleProof{}, err
	}
	// 只复制并计量真正保留的输出；临时文档、需求和实体关系图在本调用结束后释放，不进入工作区预算。
	retained := newFinalizationCloner()
	lineage := make([]ruleProofDependency, len(parents))
	for i, parent := range parents {
		lineage[i] = ruleProofDependency{kind: parent.Kind(), stamp: parent.data.stamp}
	}
	parameters := ruleParameters(g)
	data := &ruleProofData{identity: g.data.proofIdentity, stamp: new(ruleProofStamp), fingerprint: g.data.fingerprint, kind: kind, version: ruleProofVersion, profile: profile, parameters: parameters, parents: lineage, audit: copyFinalizationPointer(retained, env.audit, retained.buildRuleAudit), logistics: cloneRuleLogisticsState(retained, env.logistics), checks: copyFinalizationSlice(retained, env.checks, retained.buildLogisticsCheck), density: copyFinalizationPointer(retained, env.density, retained.buildDensityReport), networkCount: env.networkCount}
	retained.bytes += int64(unsafe.Sizeof(*data)) + int64(len(lineage))*int64(unsafe.Sizeof(ruleProofDependency{})) + int64(unsafe.Sizeof(ruleProofIdentity{})) + int64(unsafe.Sizeof(ruleProofStamp{}))*(1+int64(len(lineage)))
	retained.bytes += int64(len(parameters.Density) + len(parameters.DataVersion) + len(parameters.SurfaceMode) + len(parameters.LatitudeZone) + len(profile.Version) + len(profile.GameVersion) + len(profile.SteamBuildID) + len(profile.AssemblySHA256) + len(profile.ResourcesSHA256) + len(kind) + len(ruleProofVersion))
	data.bytes = retained.retainedBytes(1024)
	if err := ctx.Err(); err != nil {
		return RuleProof{}, err
	}
	return RuleProof{data: data}, nil
}
