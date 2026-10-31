package build

// 文件职责：把守恒边分配到具体机器行、物料端口与可实现的单带分组。
// 使用：mode 1 几何装配前执行；边界：不改量化机器数、配方或流量，无法表示的端口明确失败。

import (
	"context"
	"errors"
	"fmt"
	"math"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// transportRow 职责：保存一组机器与其明确物料端口；使用：流量分配和装配；边界：同一量化产线可以拆行但不复制机器。
type transportRow struct {
	owner, count int
	info         quant.Production
	recipe       quant.Recipe
	line         *line
	inputs       map[int]*placed
	outputs      []*transportRoute
}

// transportRoute 职责：在重排期间保留规范 lane 与真实指针；使用：装配、路由和最终编号；边界：不以建筑位置反推物品。
type transportRoute struct {
	edge     quant.FlowEdge
	rate     float64
	paired   bool
	from, to *transportRow
	filters  []*placed
	path     []*placed
	// sharedPath 职责：记录分支拆出前被多条产物流共同使用的干线；使用：聚合共享带容量；边界：不作为单条 lane 的端到端路径。
	sharedPath []*placed
	start, end *placed
	boundary   *placed
}

// rowFlow 职责：记录某机器行尚未分配的物料；使用：稳定贪心配对；边界：只消耗当前规范节点内的供需。
type rowFlow struct {
	row       *transportRow
	remaining float64
}

// newLogisticsSummary 职责：创建所有模式共用的诊断口径；使用：规范流图建立后；边界：带等级来自归一化设置而非旧 BeltSpeed。
func newLogisticsSummary(graph quant.FlowGraph, setting Setting) *LogisticsSummary {
	capacity, _ := transport.BeltCapacityPerMinute(setting.BeltLv)
	return &LogisticsSummary{FlowGraphDigest: graph.Digest, BeltLevel: setting.BeltLv, BeltCapacityPerMinute: capacity, Lanes: []TransportLane{}, ExternalPorts: []BoundaryPort{}, Checks: []LogisticsCheck{{RuleID: "flow-conservation", Status: "passed", Message: "按可信量化字段重建规范供需边，逐物品守恒已通过"}, {RuleID: "machine-rounding", Status: "passed", Message: "保留所有取整机器；精确流量按需求节流，取整余量是设备能力余量而非凭空增加的物料"}, {RuleID: "game-runtime", Status: "unverified", Message: "离线数值与协议检查不等价于 Unity Physics、地形或游戏内连续运行验收"}}}
}

// planTransportRows 职责：按统一容量拆机器行并把每条供需边分配给实际行；使用：mode 1；边界：保留所有边、循环和副产物，不静默补供或减量。
func (b *builder) planTransportRows(ctx context.Context, req Request, dataset *quant.Dataset) ([]*transportRow, error) {
	rows := []*transportRow{}
	byOwner := map[int][]*transportRow{}
	nodeOwner := map[string]int{}
	for _, node := range b.flowGraph.Nodes {
		if node.Kind == "production" {
			nodeOwner[node.ID] = node.ProductionIndex
		}
	}
	for _, owner := range b.flowGraph.ProductionOrder {
		if ctx.Err() != nil {
			return nil, &blueprint.Error{Code: "GEN_E_TIMEOUT", Message: "物料端口规划已取消或超时"}
		}
		p := req.Plan.Productions[owner]
		if err := validateProductionCapacity(p, dataset, b.setting); err != nil {
			return nil, err
		}
		if p.ExactMachineCount > 0 && float64(p.MachineCount) > p.ExactMachineCount+1e-8 {
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "machine-throttle", Status: "unverified", Message: fmt.Sprintf("配方 %d 保留 %d 台机器但目标流量按精确需求 %.6f 台折算；生成器仅输送计划流量，不把余量当作额外产出或副产物", p.RecipeID, p.MachineCount, p.ExactMachineCount)})
		}
		ri, exists := dataset.RecipeByID[p.RecipeID]
		if !exists || p.MachineCount <= 0 || p.MachineCount > 10000 || p.IncreaseLv < 0 || p.IncreaseLv > 3 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("产线 %d 的配方、机器数或喷涂等级无效", owner)}
		}
		r := dataset.Recipes[ri]
		validFactory := false
		for _, id := range r.Factories {
			validFactory = validFactory || id == p.MachineID
		}
		if !validFactory {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("产线 %d 设施与配方不匹配", owner)}
		}
		// 取整机器只能提供不低于精确需求的能力；若旧量化快照出现下取整或不一致，生成必须关闭而不能擅改产量。
		if p.ExactMachineCount > float64(p.MachineCount)+1e-9 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_ROUNDING", Message: fmt.Sprintf("产线 %d 的取整机器数 %d 小于精确需求 %.6f，拒绝生成以避免隐形欠产", owner, p.MachineCount, p.ExactMachineCount)}
		}
		if b.setting.UseMixed || len(r.Items) > 5 {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 的循环混带端口尚无已验证逐物品过滤拓扑，请取消混带输入或更换模式", r.ID)}
		}
		for _, in := range r.Items {
			for _, out := range r.Results {
				if in == out {
					return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 物品 %d 存在设备内部循环，当前直连带未验证净流量与实体回流的转换", r.ID, in)}
				}
			}
		}
		counts, err := lineCounts(p, r, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return nil, err
		}
		if err := validateMachineInserterCapacity(p, r, b.setting); err != nil {
			// 旧版生成器把“单臂额定吞吐不足”当作整张蓝图的硬失败，
			// 导致已有工作区即使布局、带路和协议都可验证也无法导出。
			// 当前装配仍保留真实分拣器几何，并将未经游戏运行校准的吞吐
			// 降级为明确的 unverified 诊断；其它端口/连接结构错误继续关闭。
			var capacityErr *blueprint.Error
			if !errors.As(err, &capacityErr) || capacityErr.Code != "GEN_E_INSERTER_CAPACITY" {
				return nil, err
			}
			b.logistics.Checks = append(b.logistics.Checks, LogisticsCheck{RuleID: "inserter-capacity", Status: "unverified", Message: capacityErr.Message + "；已保留蓝图并要求在游戏中按实际科技/并行分拣器校准"})
			b.warnings = append(b.warnings, capacityErr.Message+"；蓝图已生成，实际吞吐需在游戏中验证")
		}
		for _, count := range counts {
			row := &transportRow{owner: owner, count: count, info: p, recipe: r, inputs: map[int]*placed{}}
			rows, byOwner[owner] = append(rows, row), append(byOwner[owner], row)
		}
	}
	supplies, demands := map[string]map[int][]*rowFlow{}, map[string]map[int][]*rowFlow{}
	for node, owner := range nodeOwner {
		supplies[node], demands[node] = map[int][]*rowFlow{}, map[int][]*rowFlow{}
		for _, row := range byOwner[owner] {
			share := float64(row.count) / float64(row.info.MachineCount)
			for _, rate := range row.info.Outputs {
				supplies[node][rate.ItemID] = append(supplies[node][rate.ItemID], &rowFlow{row: row, remaining: rate.RatePerMinute * share})
			}
			for _, rate := range row.info.Inputs {
				demands[node][rate.ItemID] = append(demands[node][rate.ItemID], &rowFlow{row: row, remaining: rate.RatePerMinute * share})
			}
		}
	}
	capacity := b.logistics.BeltCapacityPerMinute
	for _, edge := range b.flowGraph.Edges {
		left := edge.RatePerMinute
		from, hasFrom := supplies[edge.SourceNodeID]
		to, hasTo := demands[edge.TargetNodeID]
		for left > max(1e-8, edge.RatePerMinute*1e-10) {
			part := math.Min(left, capacity)
			var source, target *rowFlow
			if hasFrom {
				source = nextRowFlow(from[edge.ItemID])
				if source == nil {
					return nil, transportAllocationError(edge, left, "生产输出端口不足")
				}
				part = math.Min(part, source.remaining)
			}
			if hasTo {
				target = nextRowFlow(to[edge.ItemID])
				if target == nil {
					return nil, transportAllocationError(edge, left, "消费输入端口不足")
				}
				part = math.Min(part, target.remaining)
			}
			route := &transportRoute{edge: edge, rate: part}
			if source != nil {
				route.from = source.row
				source.row.outputs = append(source.row.outputs, route)
				source.remaining = math.Max(0, source.remaining-part)
			}
			if target != nil {
				route.to = target.row
				target.remaining = math.Max(0, target.remaining-part)
			}
			b.transportRoutes = append(b.transportRoutes, route)
			left = math.Max(0, left-part)
		}
	}
	for _, row := range rows {
		inputRates := map[int]float64{}
		outputRate := 0.
		for _, route := range b.transportRoutes {
			if route.to == row {
				inputRates[route.edge.ItemID] += route.rate
			}
			if route.from == row {
				outputRate += route.rate
			}
		}
		for item, rate := range inputRates {
			if rate > capacity+1e-6 {
				return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 原料 %d 汇入 %.3f/min，超过单物品输入带容量 %.3f/min", row.owner, item, rate, capacity)}
			}
		}
		if outputRate > capacity+1e-6 {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 多产物合计输出 %.3f/min，超过共享输出干线容量 %.3f/min", row.owner, outputRate, capacity)}
		}
	}
	return rows, nil
}

// validateProductionCapacity 职责：拒绝量化取整后机器总能力不足的陈旧方案；使用：mode1 端口规划前；边界：不修改量化机器数，浮点误差按统一容差处理。
func validateProductionCapacity(p quant.Production, dataset *quant.Dataset, setting Setting) error {
	ri, ok := dataset.RecipeByID[p.RecipeID]
	if !ok || p.MachineCount <= 0 {
		return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("产线配方 %d 或机器数量无效", p.RecipeID)}
	}
	denom := p.ExactMachineCount
	if denom <= 0 {
		denom = float64(p.MachineCount)
	}
	if float64(p.MachineCount)+1e-8 < denom {
		return &blueprint.Error{Code: "GEN_E_MACHINE_CAPACITY", Message: fmt.Sprintf("配方 %d 取整机器 %d 少于精确需求 %.6f，拒绝欠能力蓝图", p.RecipeID, p.MachineCount, denom)}
	}
	r := dataset.Recipes[ri]
	inputIDs, outputIDs := map[int]bool{}, map[int]bool{}
	for _, id := range r.Items {
		inputIDs[id] = true
	}
	for _, id := range r.Results {
		outputIDs[id] = true
	}
	for _, flow := range p.Inputs {
		if !inputIDs[flow.ItemID] || !finitePositive(flow.RatePerMinute) {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的原料流 %d 不在配方或速率无效", p.RecipeID, flow.ItemID)}
		}
	}
	for _, flow := range p.Outputs {
		if !outputIDs[flow.ItemID] || !finitePositive(flow.RatePerMinute) {
			return &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("配方 %d 的产物流 %d 不在配方或速率无效", p.RecipeID, flow.ItemID)}
		}
	}
	return nil
}

// finitePositive 职责：检查量化流量为有限正数；使用：生成前快照校验；边界：不改变量化值或补造流量。
func finitePositive(v float64) bool { return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }

// nextRowFlow 职责：按机器行稳定顺序选择仍有容量的端口；使用：供需贪心；边界：只忽略浮点舍入残差。
func nextRowFlow(flows []*rowFlow) *rowFlow {
	for _, flow := range flows {
		if flow.remaining > 1e-8 {
			return flow
		}
	}
	return nil
}

// transportAllocationError 职责：为端口分配失败补充物料和边信息；使用：流量规划失败；边界：不得返回已装配的局部结果。
func transportAllocationError(edge quant.FlowEdge, rate float64, reason string) error {
	return &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("物品 %d、边 %s、%.6f/min：%s", edge.ItemID, edge.ID, rate, reason)}
}

// inserterCapacityPerMinute 职责：提供已核对普通分拣器的单臂保守吞吐；使用：端口与过滤分支；边界：白爪仅取蓝爪已知下界，不假定未知集装科技。
func inserterCapacityPerMinute(level, reach int) float64 {
	capacity, err := transport.InserterCapacityPerMinute(level, reach)
	if err != nil {
		return 0
	}
	return capacity
}

// validateMachineInserterCapacity 职责：独立检查机器端单臂吞吐；使用：直连带装配前；边界：超能力明确拒绝，不用带容量冒充设备端口能力。
func validateMachineInserterCapacity(p quant.Production, recipe quant.Recipe, setting Setting) error {
	denominator := p.ExactMachineCount
	if denominator <= 0 {
		denominator = float64(p.MachineCount)
	}
	for i, item := range recipe.Items {
		reach := i + 1
		if i >= 3 {
			reach = i - 2
		}
		for _, in := range p.Inputs {
			if in.ItemID != item {
				continue
			}
			rate := in.RatePerMinute / denominator
			if p.MachineType == 6 {
				rate *= float64(min(p.MachineCount, setting.MaxLays))
			}
			capacity := inserterCapacityPerMinute(setting.InserterLv, reach)
			if rate > capacity+1e-6 {
				return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 原料 %d 单端口 %.3f/min 超过 Mk.%d 跨 %d 格分拣器已验证能力 %.3f/min，请提高分拣器等级或降低设备工作倍率", recipe.ID, item, rate, setting.InserterLv, reach, capacity)}
			}
		}
	}
	output := 0.
	for _, out := range p.Outputs {
		output += out.RatePerMinute / denominator
	}
	if p.MachineType == 6 {
		output *= float64(min(p.MachineCount, setting.MaxLays))
	}
	reach := 1 + max(0, len(recipe.Items)-3)
	capacity := inserterCapacityPerMinute(setting.InserterLv, reach)
	if output > capacity+1e-6 {
		return &blueprint.Error{Code: "GEN_E_INSERTER_CAPACITY", Message: fmt.Sprintf("配方 %d 混合输出单臂 %.3f/min 超过 Mk.%d 跨 %d 格能力 %.3f/min", recipe.ID, output, setting.InserterLv, reach, capacity)}
	}
	return nil
}
