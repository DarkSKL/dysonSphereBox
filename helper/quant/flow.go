package quant

// 文件职责：从可信量化净流量重建规范物料供需图；使用：Solve 和蓝图生成入口；边界：忽略输入 FlowGraph，不修改配方、机器数、目标或物流需求。
// 逻辑流：1. 校验并规范物料字段 => 2. 按物品检查守恒 => 3. 稳定贪心配对 => 4. SCC 缩并排序 => 5. 生成与数组顺序无关的摘要。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// flowEndpoint 职责：记录单物品可分配的端点；使用：稳定贪心配对；边界：只保存剩余计划流量，不产生额外供给。
type flowEndpoint struct {
	// node 职责：保存规范节点 ID；使用：边引用；边界：已在节点表登记。
	node string
	// rate 职责：保存尚待分配量；使用：配对消减；边界：允许最终浮点容差残差。
	rate float64
}

// flowProduction 职责：保存不依赖数组序的生产语义；使用：稳定排序；边界：原索引只做映射，不参与排序键。
type flowProduction struct {
	// index 职责：引用原产线；使用：装配和联动；边界：不写回原数据。
	index int
	// key 职责：保存规范生产字段；使用：确定性排序与摘要；边界：不含物料边或画板坐标。
	key string
	// inputs 职责：保存逐物品净消耗；使用：配对；边界：与 outputs 同物品已相抵。
	inputs []ItemRate
	// outputs 职责：保存逐物品净产出；使用：配对；边界：不按取整机器额定能力扩大。
	outputs []ItemRate
}

// BuildFlowGraph 职责：重建并校验规范物料图；使用：旧快照恢复和纯数据调用；边界：无副作用，需要取消的生成任务应调用 BuildFlowGraphContext。
func BuildFlowGraph(plan Result) (FlowGraph, error) {
	return BuildFlowGraphContext(context.Background(), plan)
}

// BuildFlowGraphContext 职责：可取消地重建守恒流图；使用：量化和所有蓝图模式；边界：无效速率、物品或真实供需缺口硬失败，不接受请求提供的任意边。
func BuildFlowGraphContext(ctx context.Context, plan Result) (FlowGraph, error) {
	out := FlowGraph{Nodes: []FlowNode{}, Edges: []FlowEdge{}, Components: []FlowComponent{}, ProductionOrder: []int{}, Balances: []FlowBalance{}}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	d, err := LoadDataset(plan.Dataset)
	if err != nil {
		return out, err
	}
	if len(plan.Productions) > 10000 {
		return out, qerror("Q_E_FLOW", "生产节点超过 10000 上限，请拆分目标")
	}
	productions := make([]flowProduction, 0, len(plan.Productions))
	for i, p := range plan.Productions {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if _, ok := d.ByID[p.MainItemID]; !ok {
			return out, qerror("Q_E_FLOW", fmt.Sprintf("产线 %d 引用无效主产物 %d", i, p.MainItemID))
		}
		inputs, err := normalizeFlowRates(d, p.Inputs, fmt.Sprintf("产线 %d 输入", i))
		if err != nil {
			return out, err
		}
		outputs, err := normalizeFlowRates(d, p.Outputs, fmt.Sprintf("产线 %d 输出", i))
		if err != nil {
			return out, err
		}
		inputs, outputs = netFlowRates(inputs, outputs)
		canonical := p
		// 条件能力不是期望净流图的一部分，旧快照与新元数据必须保持同一规范摘要。
		canonical.Capacity = nil
		canonical.Inputs, canonical.Outputs = inputs, outputs
		canonical.TargetItemIDs = append([]int{}, p.TargetItemIDs...)
		sort.Ints(canonical.TargetItemIDs)
		key, err := json.Marshal(canonical)
		if err != nil {
			return out, qerror("Q_E_FLOW", fmt.Sprintf("产线 %d 包含非有限配置", i))
		}
		productions = append(productions, flowProduction{index: i, key: string(key), inputs: inputs, outputs: outputs})
	}
	sort.SliceStable(productions, func(i, j int) bool { return productions[i].key < productions[j].key })
	offers, needs := map[int][]flowEndpoint{}, map[int][]flowEndpoint{}
	for ordinal, p := range productions {
		id := fmt.Sprintf("production:%d", ordinal)
		out.Nodes = append(out.Nodes, FlowNode{ID: id, Kind: "production", ItemID: plan.Productions[p.index].MainItemID, ProductionIndex: p.index})
		for _, rate := range p.inputs {
			needs[rate.ItemID] = append(needs[rate.ItemID], flowEndpoint{node: id, rate: rate.RatePerMinute})
		}
		for _, rate := range p.outputs {
			offers[rate.ItemID] = append(offers[rate.ItemID], flowEndpoint{node: id, rate: rate.RatePerMinute})
		}
	}
	raw, err := normalizeFlowRates(d, plan.RawInputs, "外部输入")
	if err != nil {
		return out, err
	}
	external := map[int]bool{}
	for _, id := range plan.ExternalInputItemIDs {
		if _, ok := d.ByID[id]; !ok || external[id] {
			return out, qerror("Q_E_FLOW", fmt.Sprintf("显式外供标记 %d 无效或重复", id))
		}
		external[id] = true
	}
	for _, rate := range raw {
		id, kind := fmt.Sprintf("source:%d", rate.ItemID), "raw-source"
		if external[rate.ItemID] {
			id, kind = fmt.Sprintf("external:%d", rate.ItemID), "external-source"
			delete(external, rate.ItemID)
		}
		out.Nodes = append(out.Nodes, FlowNode{ID: id, Kind: kind, ItemID: rate.ItemID, ProductionIndex: -1})
		offers[rate.ItemID] = append(offers[rate.ItemID], flowEndpoint{node: id, rate: rate.RatePerMinute})
	}
	if len(external) > 0 {
		return out, qerror("Q_E_FLOW", "显式外供标记必须对应非零 RawInputs")
	}
	targetRates := make([]ItemRate, 0, len(plan.Targets))
	for _, target := range plan.Targets {
		targetRates = append(targetRates, ItemRate{ItemID: target.ItemID, RatePerMinute: target.RatePerMinute})
	}
	targets, err := normalizeFlowRates(d, targetRates, "目标交付")
	if err != nil {
		return out, err
	}
	leftovers, err := normalizeFlowRates(d, plan.Leftovers, "剩余副产物")
	if err != nil {
		return out, err
	}
	for _, group := range []struct {
		kind   string
		prefix string
		rates  []ItemRate
	}{{"target-sink", "target", targets}, {"leftover-sink", "leftover", leftovers}} {
		for _, rate := range group.rates {
			id := fmt.Sprintf("%s:%d", group.prefix, rate.ItemID)
			out.Nodes = append(out.Nodes, FlowNode{ID: id, Kind: group.kind, ItemID: rate.ItemID, ProductionIndex: -1})
			needs[rate.ItemID] = append(needs[rate.ItemID], flowEndpoint{node: id, rate: rate.RatePerMinute})
		}
	}
	sort.Slice(out.Nodes, func(i, j int) bool { return out.Nodes[i].ID < out.Nodes[j].ID })
	nodes := make(map[string]FlowNode, len(out.Nodes))
	for _, n := range out.Nodes {
		nodes[n.ID] = n
	}
	itemSet := map[int]bool{}
	for id := range offers {
		itemSet[id] = true
	}
	for id := range needs {
		itemSet[id] = true
	}
	items := make([]int, 0, len(itemSet))
	for id := range itemSet {
		items = append(items, id)
	}
	sort.Ints(items)
	for _, itemID := range items {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		supply, demand := 0., 0.
		for _, v := range offers[itemID] {
			supply += v.rate
		}
		for _, v := range needs[itemID] {
			demand += v.rate
		}
		tolerance := FlowTolerance(supply, demand)
		if math.Abs(supply-demand) > tolerance {
			return out, qerror("Q_E_FLOW", fmt.Sprintf("物品 %d %s 供需不守恒：供给 %.9g/min，需求 %.9g/min，差额 %+.9g/min；供给节点 %v，消费节点 %v", itemID, d.ByID[itemID].Name, supply, demand, supply-demand, flowEndpointIDs(offers[itemID]), flowEndpointIDs(needs[itemID])))
		}
		out.Balances = append(out.Balances, FlowBalance{ItemID: itemID, SupplyPerMinute: supply, DemandPerMinute: demand, DifferencePerMinute: supply - demand})
		// 生产端点先于边界，单物品按稳定节点配对；每次至少耗尽一端，避免 all-to-all 碎边。
		os, ns := offers[itemID], needs[itemID]
		oi, ni := 0, 0
		for oi < len(os) && ni < len(ns) {
			rate := math.Min(os[oi].rate, ns[ni].rate)
			if rate > 0 {
				kind := "internal"
				if nodes[os[oi].node].Kind != "production" {
					kind = "external-input"
				} else if nodes[ns[ni].node].Kind == "target-sink" {
					kind = "target-output"
				} else if nodes[ns[ni].node].Kind == "leftover-sink" {
					kind = "leftover"
				}
				out.Edges = append(out.Edges, FlowEdge{ID: fmt.Sprintf("flow:%d:%s>%s", itemID, os[oi].node, ns[ni].node), ItemID: itemID, SourceNodeID: os[oi].node, TargetNodeID: ns[ni].node, RatePerMinute: rate, FlowKind: kind})
			}
			os[oi].rate -= rate
			ns[ni].rate -= rate
			if os[oi].rate <= 0 {
				oi++
			}
			if ns[ni].rate <= 0 {
				ni++
			}
		}
	}
	sort.Slice(out.Edges, func(i, j int) bool { return out.Edges[i].ID < out.Edges[j].ID })
	if err := orderFlowComponents(ctx, &out); err != nil {
		return out, err
	}
	semanticKeys := make([]string, len(productions))
	for i, p := range productions {
		semanticKeys[i] = p.key
	}
	canonicalNodes := append([]FlowNode{}, out.Nodes...)
	for i := range canonicalNodes {
		canonicalNodes[i].ProductionIndex = -1
	}
	canonical, err := json.Marshal(struct {
		Dataset     string
		DataVersion string
		Productions []string
		Nodes       []FlowNode
		Edges       []FlowEdge
		Balances    []FlowBalance
	}{d.Info.Key, plan.DataVersion, semanticKeys, canonicalNodes, out.Edges, out.Balances})
	if err != nil {
		return out, qerror("Q_E_FLOW", "规范流图包含非有限流量")
	}
	digest := sha256.Sum256(canonical)
	out.Digest = hex.EncodeToString(digest[:])
	return out, nil
}

// FlowTolerance 职责：统一供需与路线账浮点容差；使用：纯流量校验；边界：不能用于放行真实欠供、负数或带速超载。
func FlowTolerance(supply, demand float64) float64 {
	return math.Max(1e-6, math.Max(math.Abs(supply), math.Abs(demand))*1e-8)
}

// normalizeFlowRates 职责：校验、合并并按物品排序速率；使用：规范图构造；边界：允许同一目标重复累计，负值和未知物品直接失败。
func normalizeFlowRates(d *Dataset, input []ItemRate, label string) ([]ItemRate, error) {
	values := map[int]float64{}
	for _, rate := range input {
		item, ok := d.ByID[rate.ItemID]
		if !ok || rate.RatePerMinute < 0 || math.IsNaN(rate.RatePerMinute) || math.IsInf(rate.RatePerMinute, 0) {
			return nil, qerror("Q_E_FLOW", fmt.Sprintf("%s 的物品 %d %s 或流量 %.9g 无效", label, rate.ItemID, item.Name, rate.RatePerMinute))
		}
		values[rate.ItemID] += rate.RatePerMinute
		if math.IsInf(values[rate.ItemID], 0) {
			return nil, qerror("Q_E_FLOW", label+"累计流量溢出")
		}
	}
	ids := make([]int, 0, len(values))
	for id, value := range values {
		if value > 0 {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	out := make([]ItemRate, 0, len(ids))
	for _, id := range ids {
		out = append(out, ItemRate{ItemID: id, Name: d.ByID[id].Name, RatePerMinute: values[id]})
	}
	return out, nil
}

// netFlowRates 职责：抵消同一设备内部同物品流量；使用：旧快照兼容；边界：仅在一个生产实例内部抵消，不吞掉不同节点间循环边。
func netFlowRates(inputs, outputs []ItemRate) ([]ItemRate, []ItemRate) {
	in, out := make([]ItemRate, 0, len(inputs)), make([]ItemRate, 0, len(outputs))
	values := map[int]float64{}
	for _, rate := range outputs {
		values[rate.ItemID] = rate.RatePerMinute
	}
	for _, rate := range inputs {
		cancel := math.Min(rate.RatePerMinute, values[rate.ItemID])
		rate.RatePerMinute -= cancel
		values[rate.ItemID] -= cancel
		if rate.RatePerMinute > 0 {
			in = append(in, rate)
		}
	}
	for _, rate := range outputs {
		rate.RatePerMinute = values[rate.ItemID]
		if rate.RatePerMinute > 0 {
			out = append(out, rate)
		}
	}
	return in, out
}

// flowEndpointIDs 职责：生成稳定错误端点列表；使用：守恒诊断；边界：只返回节点键，不访问用户路径。
func flowEndpointIDs(endpoints []flowEndpoint) []string {
	out := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, endpoint.node)
	}
	return out
}
