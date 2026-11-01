package build

// 文件职责：把守恒边分配到具体机器行、物料端口与可实现的单带分组。
// 使用：mode 1 几何装配前执行；边界：不改量化机器数、配方或流量，无法表示的端口明确失败。

import (
	"context"
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
	// inputStubs 职责：保留原料入口至机器带的真实链；使用：回流接入时保留上游预充口；边界：不能用零净流催化剂伪造外供边。
	inputStubs map[int][]*placed
	// recycles 职责：登记本行独立物品的优先回流结构；使用：毛流容量、路由与机器出口追踪；边界：不属于规范净流边。
	recycles []*machineRecycle
}

// transportRoute 职责：在重排期间保留规范 lane 与真实指针；使用：装配、路由和最终编号；边界：不以建筑位置反推物品。
type transportRoute struct {
	edge     quant.FlowEdge
	rate     float64
	paired   bool
	from, to *transportRow
	filters  []*placed
	path     []*placed
	// sharedPath 职责：记录同一产物各份额在分支前共用的干线；使用：聚合共享带容量；边界：不同产物不共用机器输出带，也不作为单条lane的端到端路径。
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
	return &LogisticsSummary{FlowGraphDigest: graph.Digest, BeltLevel: setting.BeltLv, BeltCapacityPerMinute: capacity, Lanes: []TransportLane{}, ExternalPorts: []BoundaryPort{}, Checks: []LogisticsCheck{{RuleID: "flow-conservation", Status: "passed", Message: "按可信量化字段重建期望净供需边，逐物品守恒已通过"}, {RuleID: "machine-rounding", Status: "passed", Message: "保留全部整数机器；规范流边表示期望净流量，设备满负荷能力另列，不把取整余量重复加入守恒边，也未对设备写入降速"}, {RuleID: "game-runtime", Status: "unverified", Message: "离线数值与协议检查不等价于 Unity Physics、地形或游戏内连续运行验收"}}}
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
		ri, exists := dataset.RecipeByID[p.RecipeID]
		if !exists || p.MachineCount <= 0 || p.MachineCount > 10000 || p.IncreaseLv < 0 || p.IncreaseLv > 3 {
			return nil, &blueprint.Error{Code: "GEN_E_PLAN_STALE", Message: fmt.Sprintf("产线 %d 的配方、机器数或喷涂等级无效", owner)}
		}
		r := dataset.Recipes[ri]
		effective, recipeErr := effectiveProductionRecipe(p, r)
		if recipeErr != nil {
			return nil, recipeErr
		}
		r = effective
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
		if b.setting.UseMixed || len(r.Items)+len(r.Results) > 6 || len(r.Results) == 0 {
			return nil, &blueprint.Error{Code: "GEN_E_PORT_UNVERIFIED", Message: fmt.Sprintf("配方 %d 需要独立物料带且原料与产物合计至多六种；循环混带端口尚无已验证逐物品过滤拓扑，请取消混带输入或更换模式", r.ID)}
		}
		if _, err := productionMaterialFlows(p, r); err != nil {
			return nil, err
		}
		counts, err := lineCounts(p, r, b.setting, dataset.ByID[p.MachineID].Speed)
		if err != nil {
			return nil, err
		}
		// 分拣器在机器行内按真实空槽并联和堆叠高度规划，不能在实体方案形成前按旧单臂布局拒绝。
		for _, count := range counts {
			row := &transportRow{owner: owner, count: count, info: p, recipe: r, inputs: map[int]*placed{}, inputStubs: map[int][]*placed{}}
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
		outputRates := map[int]float64{}
		for _, route := range b.transportRoutes {
			if route.to == row {
				inputRates[route.edge.ItemID] += route.rate
			}
			if route.from == row {
				outputRates[route.edge.ItemID] += route.rate
			}
		}
		for item, rate := range inputRates {
			if rate > capacity+1e-6 {
				return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 原料 %d 汇入 %.3f/min，超过单物品输入带容量 %.3f/min", row.owner, item, rate, capacity)}
			}
		}
		for _, item := range row.recipe.Results {
			if rate := outputRates[item]; rate > capacity+1e-6 {
				return nil, &blueprint.Error{Code: "GEN_E_PORT_CAPACITY", Message: fmt.Sprintf("机器行 %d 产物 %d 独立输出 %.3f/min，超过单带容量 %.3f/min", row.owner, item, rate, capacity)}
			}
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
	if effective, err := effectiveProductionRecipe(p, r); err != nil {
		return err
	} else {
		r = effective
	}
	inputIDs, outputIDs := map[int]bool{}, map[int]bool{}
	for _, id := range r.Items {
		inputIDs[id] = true
	}
	if spray := productionSprayItem(p); spray > 0 {
		// 量化净输入包含喷涂剂，实体由喷涂机附加供剂带接收，不占机器配方槽。
		inputIDs[spray] = true
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
