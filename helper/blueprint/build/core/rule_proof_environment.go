package build

// 文件职责：为单项规则创建独占临时读视图并保留必要输出。
// 使用：CheckRule固定分派；边界：不保留可变Builder，不调度后续规则，不向输入快照写报告。

import (
	"context"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// ruleEnvironment 职责：保存单项计算的局部视图；使用：规则执行期间；边界：执行完成后只签发独立报告及值证明。
type ruleEnvironment struct {
	// geometry/ctx 职责：固定输入及当前取消；使用：只读计算；边界：不保留到证明中。
	geometry GeometrySnapshot
	ctx      context.Context
	// cloner/document/request/surface 职责：隔离检查输入；使用：派生缓存及错误写入检测；边界：不与公开容器共享。
	cloner   *finalizationCloner
	document blueprint.Document
	request  Request
	surface  *surfaceEncoding
	// builder 职责：按需重建短时实体关系图；使用：机器和物流检查；边界：不进入最终RuleProof。
	builder *builder
	// logistics/audit/checks/density/networkCount 职责：保存当前规则必要输出；使用：封闭签发；边界：不推进实体版本。
	logistics    *ruleLogisticsState
	audit        *RuleAudit
	checks       []LogisticsCheck
	density      *DensityReport
	networkCount int
}

// newRuleEnvironment 职责：复制单项所需内容并显式消费前置结果；使用：CheckRule；边界：不运行前置规则。
func newRuleEnvironment(ctx context.Context, g GeometrySnapshot, kind RuleKind, parents []RuleProof) *ruleEnvironment {
	c := newFinalizationCloner()
	e := &ruleEnvironment{geometry: g, ctx: g.data.rules.validationContext(ctx), cloner: c, document: c.blueprintDocument(g.data.document), request: c.buildRequest(g.data.request), surface: copyRuleSurface(c, g.data.surface)}
	for _, parent := range parents {
		if e.logistics == nil && parent.data.logistics != nil {
			e.logistics = cloneRuleLogisticsState(c, parent.data.logistics)
		}
	}
	return e
}

// entityView 职责：按需创建当前检查的独占关系图；使用：真正需要指针证据的规则；边界：只应用已有证明中的报告和索引值，不补跑规则。
func (e *ruleEnvironment) entityView() *builder {
	if e.builder != nil {
		return e.builder
	}
	b := e.geometry.data.evidence.validationView(e.cloner)
	b.generationCtx = e.ctx
	b.warnings = append([]string(nil), e.geometry.data.warnings...)
	if e.logistics != nil {
		b.logistics = e.logistics.summary
		b.warnings = append([]string(nil), e.logistics.warnings...)
		if b.mode1Deliveries != nil && e.logistics.deliveryChecked {
			b.mode1Deliveries.verified = true
			b.mode1Deliveries.edges = map[[2]*placed]bool{}
			for edge, value := range e.logistics.deliveryEdges {
				b.mode1Deliveries.edges[[2]*placed{b.all[edge[0]], b.all[edge[1]]}] = value
			}
		}
	}
	e.builder = b
	return b
}

// checkUnchanged 职责：拒绝单项检查误改文档或请求；使用：签发前；边界：不为新内容重签旧指纹。
func (e *ruleEnvironment) checkUnchanged() error {
	fingerprint, err := finalGeometryFingerprint(e.document, e.request, e.geometry.data.rules)
	if err != nil {
		return err
	}
	if fingerprint != e.geometry.Fingerprint() {
		return ruleProofError("单项规则意外修改冻结实体或需求")
	}
	return nil
}

// execute 职责：把一个固定ID交给同职责真实实现；使用：CheckRule；边界：一次只运行一项，不循环补齐其它证明。
func (e *ruleEnvironment) execute(kind RuleKind) error {
	switch kind {
	case RuleReferences:
		_, err := executePlanetRule(e.ctx, kind, e.request, e.surface, e.document.Buildings)
		if err == nil && e.request.Plan.Dataset != "" {
			_, err = quant.LoadDataset(e.request.Plan.Dataset)
		}
		return err
	case RuleLatitude:
		var plan *SurfacePlan
		if e.geometry.data.surface != nil {
			plan = e.surface.plan
		}
		if err := validateDensityLatitudePlan(e.request.SurfaceLayout, plan); err != nil {
			return err
		}
		if e.geometry.data.surface != nil {
			return e.surface.validateDenseStaticLatitude(e.ctx, e.request.SurfaceLayout, e.document.Buildings)
		}
		return nil
	case RuleMachines:
		b := e.entityView()
		before := len(b.logistics.Checks)
		if err := b.checkFinalMachineIntegrity(e.ctx, e.request, e.geometry.data.dataset); err != nil {
			return err
		}
		e.checks = append([]LogisticsCheck(nil), b.logistics.Checks[before:]...)
		return nil
	case RuleSprayChannels:
		return e.checkSprayChannels()
	case RuleTransportTopology, RuleByproductPriority, RuleBuffers, RuleDeliveries, RuleProducerSplits, RuleMaterialNetworks, RuleRoutes, RuleRecycles, RuleSelfSpray, RuleLeftovers, RuleSharedCapacity, RuleConservation, RuleBoundaries, RuleMonitors:
		return e.executeLogistics(kind)
	case RuleLayoutMetrics:
		return e.measureLayout()
	case RuleDensityReport:
		var err error
		e.density, err = e.entityView().densityReport(e.request, "", e.logistics.summary.Metrics.EnvelopeArea)
		return err
	default:
		count, err := executePlanetRule(e.ctx, kind, e.request, e.surface, e.document.Buildings)
		e.networkCount = count
		return err
	}
}

// checkSprayChannels 职责：独立证明实际喷涂先后与每个供剂宿主可达；使用：机器证明后的喷涂工具；边界：不创建供剂或把喷涂参数当持续供给证明。
func (e *ruleEnvironment) checkSprayChannels() error {
	b := e.entityView()
	alive := map[*placed]bool{}
	for _, entity := range b.all {
		alive[entity] = true
	}
	graph := finalMachineMaterialGraph(b.all)
	b.appendProductionBufferEdges(graph)
	filter := func(rows [][]*placed) [][]*placed {
		out := make([][]*placed, len(rows))
		for i, row := range rows {
			for _, node := range row {
				if alive[node] {
					out[i] = append(out[i], node)
				}
			}
		}
		return out
	}
	for _, registered := range b.machineLines {
		if err := e.ctx.Err(); err != nil {
			return err
		}
		view := *registered
		view.inputs = filter(view.inputs)
		view.outputs = filter(view.outputRows())
		if len(view.outputs) > 0 {
			view.output = view.outputs[0]
		}
		if err := validateLineSprayOrder(&view); err != nil {
			return err
		}
		if err := validateFinalLineCoaters(&view, b.all, graph); err != nil {
			return err
		}
	}
	return nil
}
