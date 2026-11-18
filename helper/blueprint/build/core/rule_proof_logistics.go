package build

// 文件职责：逐项核验物流结构、路线、回流、容量、守恒及边界，并以值输出衔接。
// 使用：独立规则工具；边界：没有实体写入口，不重跑整条终检，前置图身份用最终数组下标表达。

import (
	"DysonSphereBox/helper/blueprint"
	"fmt"
)

// ruleLogisticsState 职责：保存上一物流规则的只读必要输出；使用：下一个明确依赖；边界：不保存Builder、实体指针或可写别名。
type ruleLogisticsState struct {
	// summary/warnings 职责：保留实际报告；使用：后续度量和最终组装；边界：复制后写入，不改前置证明。
	summary  *LogisticsSummary
	warnings []string
	// rates/totals 职责：保存逐最终实体毛负载和规范边净份额；使用：回流、容量及守恒；边界：不把毛流与净流抵消。
	rates  map[int]float64
	totals map[string]float64
	// deliveryChecked/deliveryEdges 职责：保留已独立检查的共享交付边；使用：生产树后续检查；边界：不是无来源的通过布尔值，绑定同内容证明及真实边索引。
	deliveryChecked bool
	deliveryEdges   map[[2]int]bool
}

// cloneRuleLogisticsState 职责：复制必要报告和数值索引；使用：单项检查交接；边界：不共享可变map或切片。
func cloneRuleLogisticsState(c *finalizationCloner, source *ruleLogisticsState) *ruleLogisticsState {
	if source == nil {
		return nil
	}
	return &ruleLogisticsState{summary: copyFinalizationPointer(c, source.summary, c.buildLogisticsSummary), warnings: copyFinalizationSlice(c, source.warnings, c.copyString), rates: copyFinalizationMap(c, source.rates, func(v int) int { return v }, func(v float64) float64 { return v }), totals: copyFinalizationMap(c, source.totals, c.copyString, func(v float64) float64 { return v }), deliveryChecked: source.deliveryChecked, deliveryEdges: copyFinalizationMap(c, source.deliveryEdges, func(v [2]int) [2]int { return v }, func(v bool) bool { return v })}
}

// executeLogistics 职责：执行一个具名物流规则；使用：固定分派；边界：只消费前置数值和报告，不执行未声明的其它规则。
func (e *ruleEnvironment) executeLogistics(kind RuleKind) error {
	b := e.entityView()
	if b.policy == nil {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: "生成收尾缺少装配策略"}
	}
	if b.logistics == nil || b.flowGraph == nil {
		return &blueprint.Error{Code: "GEN_E_FLOW", Message: "缺少统一物流规划"}
	}
	if e.logistics == nil {
		e.logistics = &ruleLogisticsState{summary: b.logistics, warnings: append([]string(nil), b.warnings...), rates: map[int]float64{}, totals: map[string]float64{}, deliveryEdges: map[[2]int]bool{}}
	}
	indices := map[*placed]int{}
	for i, p := range b.all {
		indices[p] = i
	}
	rates := map[*placed]float64{}
	for index, rate := range e.logistics.rates {
		if index < 0 || index >= len(b.all) {
			return ruleProofError("负载证明引用未知实体")
		}
		rates[b.all[index]] = rate
	}
	policy := b.policy.logistics
	var err error
	switch kind {
	case RuleTransportTopology:
		err = e.checkTransportTopology(b)
	case RuleByproductPriority:
		err = b.validateByproductPriorityInputs()
	case RuleBuffers:
		err = b.validateProductionBuffers()
	case RuleDeliveries:
		err = b.validateMode1Deliveries()
		if err == nil && b.mode1Deliveries != nil {
			e.logistics.deliveryChecked = true
			e.logistics.deliveryEdges = map[[2]int]bool{}
			for edge, value := range b.mode1Deliveries.edges {
				left, lok := indices[edge[0]]
				right, rok := indices[edge[1]]
				if !lok || !rok {
					return ruleProofError("共享交付证明引用未知实体")
				}
				e.logistics.deliveryEdges[[2]int{left, right}] = value
			}
		}
	case RuleProducerSplits:
		err = b.validateDenseProducerSplits()
	case RuleMaterialNetworks:
		err = b.validateDenseMaterialNetworks()
	case RuleRoutes:
		rates, e.logistics.totals, err = checkLogisticsRoutes(b, policy)
	case RuleRecycles:
		err = b.validateMachineRecycles(indices, rates)
	case RuleSelfSpray:
		if policy.validateSelfSprays {
			err = b.validateConnectedSelfSprays(indices, rates)
		}
	case RuleLeftovers:
		if policy.validateLeftovers {
			err = b.validateLeftoverBoundaries()
		}
		if err == nil && b.policy.records == recordFly {
			var check *LogisticsCheck
			var warning string
			check, warning, err = b.flyLeftoverBoundaryCheck(b.policy.mode, b.transportRoutes)
			if err == nil && check != nil {
				appendUniqueRuleChecks(b, *check)
				appendUniqueRuleWarnings(b, warning)
			}
		}
	case RuleSharedCapacity:
		err = checkLogisticsSharedCapacity(b, rates)
		if err == nil && b.policy.records == recordFly {
			var checks []LogisticsCheck
			var warnings []string
			checks, warnings, err = b.flyLocalInserterChecks(b.policy.mode)
			if err == nil {
				appendUniqueRuleChecks(b, checks...)
				appendUniqueRuleWarnings(b, warnings...)
			}
		}
	case RuleConservation:
		err = checkLogisticsConservation(b, policy, e.logistics.totals)
	case RuleBoundaries:
		err = finalizeBoundaryCoordinates(b.logistics, e.document.Buildings, e.document.Areas)
	case RuleMonitors:
		err = b.validateFinalMonitorIntegrity(e.ctx, e.request, e.document.Buildings)
	default:
		return ruleProofError("不是已登记物流规则")
	}
	if err != nil {
		return err
	}
	e.logistics.summary = b.logistics
	e.logistics.warnings = append([]string(nil), b.warnings...)
	e.logistics.rates = map[int]float64{}
	for entity, rate := range rates {
		index, exists := indices[entity]
		if !exists {
			return ruleProofError("共享带证明引用蓝图外实体")
		}
		e.logistics.rates[index] = rate
	}
	return nil
}

// checkTransportTopology 职责：检查当前真实运输拓扑和绑定份额；使用：独立拓扑规则；边界：纯绑定由几何完成，重算只用于比对冻结证据，不替换实际路线。
func (e *ruleEnvironment) checkTransportTopology(b *builder) error {
	if b.policy.mode != e.request.Mode {
		return &blueprint.Error{Code: "GEN_E_MODE", Message: "冻结运输策略与可信请求模式不一致"}
	}
	switch b.policy.records {
	case recordConnected:
		if e.request.Mode != 1 && e.request.Mode != 6 {
			return &blueprint.Error{Code: "GEN_E_MODE", Message: "冻结运输策略丢失塔或配送的真实拓扑规则"}
		}
		return nil
	case recordStation:
		if e.request.Mode != 3 && e.request.Mode != 4 || b.policy.station == nil {
			return &blueprint.Error{Code: "GEN_E_MODE", Message: "塔场拓扑证明缺少真实设备策略"}
		}
		return b.validateStationTransport(e.ctx, e.document.Buildings)
	case recordFlyBlackbox:
		return finalizeDenseFlyBlackbox(e.ctx, b, e.document.Buildings)
	case recordStationBlackbox:
		if e.request.Mode != 3 && e.request.Mode != 4 || b.policy.station == nil {
			return &blueprint.Error{Code: "GEN_E_MODE", Message: "塔黑盒拓扑证明缺少真实设备策略"}
		}
		return finalizeDenseStationBlackbox(e.ctx, b, e.document.Buildings)
	case recordFly:
		if err := b.validateFlyEndpointTopology(b.policy.mode); err != nil {
			return err
		}
		actual := b.transportRoutes
		b.transportRoutes = nil
		if err := b.bindFlyTransportRecords(b.policy.mode); err != nil {
			return err
		}
		expected := b.transportRoutes
		b.transportRoutes = actual
		if len(actual) != len(expected) {
			return &blueprint.Error{Code: "GEN_E_FLOW", Message: "配送实际绑定路线与规范份额数量不一致"}
		}
		owner := func(row *transportRow) int {
			if row == nil {
				return -1
			}
			return row.owner
		}
		for i, route := range actual {
			want := expected[i]
			if route == nil || route.edge != want.edge || route.rate != want.rate || owner(route.from) != owner(want.from) || owner(route.to) != owner(want.to) || route.start != want.start || route.end != want.end || route.boundary != want.boundary || !sameDenseMaterialNodes(route.path, want.path) {
				return transportAllocationError(want.edge, want.rate, "冻结的实际配送路线与规范份额或真实端点不一致")
			}
		}
		appendFlyBindingReport(b, b.policy.mode)
		return nil
	default:
		return &blueprint.Error{Code: "GEN_E_MODE", Message: "未知实体记录证明策略"}
	}
}

// appendFlyBindingReport 职责：在真实绑定检查后保留既有诊断；使用：拓扑规则和测试组合；边界：纯几何绑定不写通过报告。
func appendFlyBindingReport(b *builder, mode int) {
	appendUniqueRuleChecks(b, LogisticsCheck{RuleID: "fly-edge-binding", Status: "passed", Message: fmt.Sprintf("模式 %d 已将 %d 条规范边按端点实际份额拆成 %d 条计划 lane；每路只引用对应的真实本地配送器/箱体接驳链", mode, len(b.flowGraph.Edges), len(b.transportRoutes))}, LogisticsCheck{RuleID: "fly-runtime-delivery", Status: "unverified", Message: "配送机队、箱体库存、远程供给和游戏科技状态未在离线蓝图中模拟；逐边速率是按计划供料/取货契约记录"})
}

// appendUniqueRuleChecks 职责：保留既有规则诊断而不重复写入；使用：兼容几何与逐项规则迁移；边界：只去重报告，不跳过实际校验。
func appendUniqueRuleChecks(b *builder, checks ...LogisticsCheck) {
	for _, check := range checks {
		found := false
		for _, known := range b.logistics.Checks {
			if known.RuleID == check.RuleID {
				found = true
				break
			}
		}
		if !found {
			b.logistics.Checks = append(b.logistics.Checks, check)
		}
	}
}

// appendUniqueRuleWarnings 职责：保留规则产生的非阻断提示；使用：物流报告；边界：不降低错误或改变实体。
func appendUniqueRuleWarnings(b *builder, warnings ...string) {
	for _, warning := range warnings {
		found := false
		for _, known := range b.warnings {
			if known == warning {
				found = true
				break
			}
		}
		if !found {
			b.warnings = append(b.warnings, warning)
		}
	}
}

// measureLayout 职责：只度量最终记录和已经证明的物流报告；使用：独立度量工具；边界：不再次运行物流、碰撞、供电或机器链。
func (e *ruleEnvironment) measureLayout() error {
	b := e.entityView()
	b.logistics.Metrics.acceptedAspect = e.request.shape.MaxAspect
	b.logistics.Metrics.preferredAspect = e.request.shape.PreferredAspect
	if err := measureLogisticsLayout(b, e.document.Buildings, e.document.Areas); err != nil {
		return err
	}
	var surface *surfaceEncoding
	if e.geometry.data.surface != nil {
		surface = e.surface
	}
	if err := updateTotalBeltMetrics(e.ctx, surface, e.document.Buildings, &b.logistics.Metrics); err != nil {
		return err
	}
	if err := updateDenseRootShapeMetrics(e.ctx, surface, e.document.Buildings, &b.logistics.Metrics); err != nil {
		return err
	}
	e.logistics.summary = b.logistics
	return nil
}
