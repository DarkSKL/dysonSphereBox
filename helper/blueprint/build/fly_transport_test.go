package build

// 文件职责：回归配送模式规范边到真实端点份额及 lane 的精确分配。
// 使用：纯内存物流回归；边界：不模拟配送机队、建筑几何可放置或真实游戏产量。

import (
	"fmt"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/transport"
)

// TestAllocateFlyEndpointsPreservesExactShares 职责：检查同物品多条边逐次消费准确端点份额；使用：端点分配回归；边界：失败分配不能提前消耗已有份额。
func TestAllocateFlyEndpointsPreservesExactShares(t *testing.T) {
	first := &flyTransportEndpoint{item: 1101, rate: 1000}
	second := &flyTransportEndpoint{item: 1101, rate: 2000}
	remaining := map[*flyTransportEndpoint]float64{first: 1000, second: 2000}
	used, err := allocateFlyEndpoints([]*flyTransportEndpoint{first, second}, remaining, 700)
	if err != nil || len(used) != 1 || used[0].endpoint != first || used[0].rate != 700 || remaining[first] != 300 || remaining[second] != 2000 {
		t.Fatalf("首条边份额错误: used=%+v remaining=%v err=%v", used, remaining, err)
	}
	used, err = allocateFlyEndpoints([]*flyTransportEndpoint{first, second}, remaining, 1800)
	if err != nil || len(used) != 2 || used[0].endpoint != first || used[0].rate != 300 || used[1].endpoint != second || used[1].rate != 1500 || remaining[first] != 0 || remaining[second] != 500 {
		t.Fatalf("第二条边未按剩余额度分配: used=%+v remaining=%v err=%v", used, remaining, err)
	}
	if _, err := allocateFlyEndpoints([]*flyTransportEndpoint{first, second}, remaining, 600); err == nil || remaining[first] != 0 || remaining[second] != 500 {
		t.Fatalf("不足失败必须原子保留端点份额: remaining=%v err=%v", remaining, err)
	}
}

// TestFlyTransportCapacityMatrixWithEntityEndpoints 职责：核对 4200/min 在三个带等级下绑定到 12/6/3 组真实端点；使用：lane 容量与外部入口回归；边界：只证明实体指针和计划分配，不替代整图生成及机队验收。
func TestFlyTransportCapacityMatrixWithEntityEndpoints(t *testing.T) {
	for _, level := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("mk%d", level), func(t *testing.T) {
			capacity, err := transport.BeltCapacityPerMinute(level)
			if err != nil {
				t.Fatal(err)
			}
			edge := quant.FlowEdge{ID: "external", ItemID: 1101, SourceNodeID: "raw:1101", TargetNodeID: "production:0", FlowKind: "external-input", RatePerMinute: 4200}
			b := newFlyTransportTestBuilder(t, level, []quant.FlowNode{{ID: "raw:1101", Kind: "raw-source", ItemID: 1101, ProductionIndex: -1}, {ID: "production:0", Kind: "production", ProductionIndex: 0}}, []quant.FlowEdge{edge})
			left := edge.RatePerMinute
			for left > 1e-8 {
				rate := math.Min(left, capacity)
				addFlyTransportTestEndpoint(b, 0, edge.ItemID, true, rate)
				left -= rate
			}
			if err := b.bindFlyTransportEdges(2); err != nil {
				t.Fatal(err)
			}
			want := map[int]int{1: 12, 2: 6, 3: 3}[level]
			if len(b.transportRoutes) != want {
				t.Fatalf("4200/min 等级 %d 实体 lane=%d，期望 %d", level, len(b.transportRoutes), want)
			}
			total := 0.0
			for index, route := range b.transportRoutes {
				endpoint := b.flyEndpoints[index]
				if route.rate <= 0 || route.rate > capacity+1e-6 || route.rate != endpoint.rate || route.boundary != endpoint.delivery || route.from != nil || route.to == nil {
					t.Fatalf("lane 份额或真实入口不匹配: %+v", route)
				}
				if len(route.path) != len(appendFlyEndpointEntities(nil, endpoint)) {
					t.Fatalf("lane %d 错误引用其它端点实体", index)
				}
				total += route.rate
			}
			if total != 4200 {
				t.Fatalf("4200/min 边总量不守恒: %g", total)
			}
			records, _, _, err := b.records()
			if err != nil {
				t.Fatal(err)
			}
			summary, err := b.finalizeLogistics(2, records, []blueprint.Area{{Width: 100, Height: 100}})
			if err != nil {
				t.Fatal(err)
			}
			if len(summary.ExternalPorts) != want || summary.Metrics.ExternalInputCount != want {
				t.Fatalf("外部端口数量不匹配: %+v", summary)
			}
			for _, lane := range summary.Lanes {
				if lane.InternalConnected || lane.RatePerMinute > lane.CapacityPerMinute+1e-6 {
					t.Fatalf("配送端点配对不得冒充连续带/机队吞吐证明: %+v", lane)
				}
			}
		})
	}
}

// TestFlyTransportSplitterLanesRetainEntityShares 职责：验证四向分流端点沿用精确 lane 份额并保留分流器实体；使用：mode5 分配回归；边界：不把分流器端点存在当作运行时机队吞吐。
func TestFlyTransportSplitterLanesRetainEntityShares(t *testing.T) {
	edge := quant.FlowEdge{ID: "leftover", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "leftover:1101", FlowKind: "leftover", RatePerMinute: 4200}
	b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}, {ID: "leftover:1101", Kind: "leftover-sink", ItemID: 1101, ProductionIndex: -1}}, []quant.FlowEdge{edge})
	for _, rate := range []float64{1800, 1800, 600} {
		endpoint := addFlyTransportTestEndpoint(b, 0, edge.ItemID, false, rate)
		endpoint.splitter = b.make(2020, endpoint.box.a.x, endpoint.box.a.y, 0)
		endpoint.box.input = endpoint.splitter
	}
	if err := b.bindFlyTransportEdges(5); err != nil {
		t.Fatal(err)
	}
	if len(b.transportRoutes) != 3 {
		t.Fatalf("四向分流未保留独立端点 lane: %d", len(b.transportRoutes))
	}
	for index, route := range b.transportRoutes {
		endpoint := b.flyEndpoints[index]
		found := false
		for _, entity := range route.path {
			found = found || entity == endpoint.splitter
		}
		if !found || route.rate != endpoint.rate || route.boundary != endpoint.delivery || route.to != nil || route.from == nil {
			t.Fatalf("四向分流 lane %d 的份额/边界/分流器丢失", index)
		}
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	summary, err := b.finalizeLogistics(5, records, []blueprint.Area{{Width: 100, Height: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for index, lane := range summary.Lanes {
		if lane.InternalConnected || summary.ExternalPorts[index].Kind != "leftover" || summary.ExternalPorts[index].RatePerMinute != b.flyEndpoints[index].rate {
			t.Fatal("四向分流摘要误报连通或丢失副产物份额")
		}
	}
}

// TestFlyTransportPairsDifferentRowShares 职责：检查供需行拆分比例不同时按双方最小份额成路；使用：多行内部配送边；边界：一条 lane 仅引用其供货与需求端点，不把整条边流量重复计入所有行。
func TestFlyTransportPairsDifferentRowShares(t *testing.T) {
	edge := quant.FlowEdge{ID: "internal", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "production:1", FlowKind: "internal", RatePerMinute: 4200}
	b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}, {ID: "production:1", Kind: "production", ProductionIndex: 1}}, []quant.FlowEdge{edge})
	var sources, targets []*flyTransportEndpoint
	for _, rate := range []float64{1800, 1800, 600} {
		sources = append(sources, addFlyTransportTestEndpoint(b, 0, 1101, false, rate))
	}
	for _, rate := range []float64{1200, 1200, 1800} {
		targets = append(targets, addFlyTransportTestEndpoint(b, 1, 1101, true, rate))
	}
	if err := b.bindFlyTransportEdges(2); err != nil {
		t.Fatal(err)
	}
	wantRates := []float64{1200, 600, 600, 1200, 600}
	wantSources := []*flyTransportEndpoint{sources[0], sources[0], sources[1], sources[1], sources[2]}
	wantTargets := []*flyTransportEndpoint{targets[0], targets[1], targets[1], targets[2], targets[2]}
	if len(b.transportRoutes) != len(wantRates) {
		t.Fatalf("端点边界处未精确拆路: routes=%d", len(b.transportRoutes))
	}
	for index, route := range b.transportRoutes {
		wantPath := appendFlyEndpointEntities(nil, wantSources[index], wantTargets[index])
		if route.rate != wantRates[index] || len(route.path) != len(wantPath) || route.paired || route.boundary != nil || route.start != wantSources[index].link || route.end != wantTargets[index].link {
			t.Fatalf("lane %d 精确份额/实体错误: %+v", index, route)
		}
		for i, entity := range wantPath {
			if route.path[i] != entity {
				t.Fatalf("lane %d 路径 %d 未保留实际端点实体", index, i)
			}
		}
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	summary, err := b.finalizeLogistics(2, records, []blueprint.Area{{Width: 100, Height: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for _, lane := range summary.Lanes {
		if lane.InternalConnected {
			t.Fatal("计划机队配对被误标为已连通")
		}
	}
	if len(summary.ExternalPorts) != 0 {
		t.Fatal("内部边不能伪造外部端口")
	}
}

// TestFlyTransportSameItemEdgesConsumeDisjointShares 职责：防止同物品多条边重复支付端点能力；使用：多消费者边回归；边界：实体带的最终共享负载仍必须不超过单带容量。
func TestFlyTransportSameItemEdgesConsumeDisjointShares(t *testing.T) {
	edges := []quant.FlowEdge{{ID: "first", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "production:1", FlowKind: "internal", RatePerMinute: 1200}, {ID: "second", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "production:2", FlowKind: "internal", RatePerMinute: 600}}
	b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}, {ID: "production:1", Kind: "production", ProductionIndex: 1}, {ID: "production:2", Kind: "production", ProductionIndex: 2}}, edges)
	first := addFlyTransportTestEndpoint(b, 0, 1101, false, 1000)
	second := addFlyTransportTestEndpoint(b, 0, 1101, false, 800)
	addFlyTransportTestEndpoint(b, 1, 1101, true, 1200)
	addFlyTransportTestEndpoint(b, 2, 1101, true, 600)
	if err := b.bindFlyTransportEdges(2); err != nil {
		t.Fatal(err)
	}
	if len(b.transportRoutes) != 3 || b.transportRoutes[0].rate != 1000 || b.transportRoutes[1].rate != 200 || b.transportRoutes[2].rate != 600 || b.transportRoutes[0].start != first.link || b.transportRoutes[1].start != second.link || b.transportRoutes[2].start != second.link {
		t.Fatalf("多边消费份额错误: %+v", b.transportRoutes)
	}
	records, _, _, err := b.records()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.finalizeLogistics(2, records, []blueprint.Area{{Width: 100, Height: 100}}); err != nil {
		t.Fatalf("精确份额不应被错误累计为共享带超载: %v", err)
	}
}

// TestFlyTransportRejectsMissingOrOverloadedEndpoints 职责：检查端点缺失、错物品、错误方向和虚拟并行不能绕过容量；使用：失败关闭回归；边界：不删除规范边使测试通过。
func TestFlyTransportRejectsMissingOrOverloadedEndpoints(t *testing.T) {
	for _, failure := range []string{"missing-source", "missing-target", "wrong-item", "wrong-direction", "overloaded", "duplicate"} {
		t.Run(failure, func(t *testing.T) {
			edge := quant.FlowEdge{ID: "internal", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "production:1", FlowKind: "internal", RatePerMinute: 100}
			if failure == "overloaded" {
				edge.RatePerMinute = 4200
			}
			b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}, {ID: "production:1", Kind: "production", ProductionIndex: 1}}, []quant.FlowEdge{edge})
			if failure != "missing-source" {
				item, input := 1101, false
				if failure == "wrong-item" {
					item = 1102
				}
				if failure == "wrong-direction" {
					input = true
				}
				endpoint := addFlyTransportTestEndpoint(b, 0, item, input, edge.RatePerMinute)
				if failure == "duplicate" {
					b.flyEndpoints = append(b.flyEndpoints, endpoint)
				}
			}
			if failure != "missing-target" {
				addFlyTransportTestEndpoint(b, 1, 1101, true, edge.RatePerMinute)
			}
			if err := b.bindFlyTransportEdges(2); err == nil {
				t.Fatalf("%s 必须失败，不能伪造完整 lane", failure)
			}
		})
	}
}

// TestFlyTransportBindingFailureDoesNotPublishPartialRoutes 职责：保证后一条边欠供时不遗留看似完整的前半份路线；使用：绑定失败关闭回归；边界：允许调用方在局部装配纠正后重试，但不接受重复成功绑定。
func TestFlyTransportBindingFailureDoesNotPublishPartialRoutes(t *testing.T) {
	edges := []quant.FlowEdge{{ID: "first", ItemID: 1101, SourceNodeID: "raw:1101", TargetNodeID: "production:0", FlowKind: "external-input", RatePerMinute: 100}, {ID: "missing", ItemID: 1102, SourceNodeID: "raw:1102", TargetNodeID: "production:0", FlowKind: "external-input", RatePerMinute: 100}}
	b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}}, edges)
	addFlyTransportTestEndpoint(b, 0, 1101, true, 100)
	if err := b.bindFlyTransportEdges(2); err == nil {
		t.Fatal("缺少第二种原料端点必须失败")
	}
	if len(b.transportRoutes) != 0 {
		t.Fatal("绑定失败不应发布前一条边的部分路线")
	}
	addFlyTransportTestEndpoint(b, 0, 1102, true, 100)
	if err := b.bindFlyTransportEdges(2); err != nil {
		t.Fatalf("补齐端点后应该能完整重算: %v", err)
	}
	if err := b.bindFlyTransportEdges(2); err == nil {
		t.Fatal("成功绑定后重复调用不能再次分配同一流量")
	}
}

// TestFlyTransportKeepsTinyPositiveFlow 职责：防止容差把合法正流量直接丢成零条 lane；使用：低速副产物和小目标回归；边界：允许尾数舍入容差，但第一份正流量必须有真实端点证据。
func TestFlyTransportKeepsTinyPositiveFlow(t *testing.T) {
	edge := quant.FlowEdge{ID: "tiny", ItemID: 1101, SourceNodeID: "production:0", TargetNodeID: "leftover:1101", FlowKind: "leftover", RatePerMinute: 1e-10}
	b := newFlyTransportTestBuilder(t, 3, []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}}, []quant.FlowEdge{edge})
	endpoint := addFlyTransportTestEndpoint(b, 0, 1101, false, edge.RatePerMinute)
	if err := b.bindFlyTransportEdges(2); err != nil {
		t.Fatal(err)
	}
	if len(b.transportRoutes) != 1 || b.transportRoutes[0].rate != edge.RatePerMinute || b.transportRoutes[0].boundary != endpoint.delivery {
		t.Fatal("合法微小正流量被容差删除")
	}
}

// newFlyTransportTestBuilder 职责：构造仅含规范图的测试装配器；使用：端点分配测试；边界：不生成机器、不校验几何放置。
func newFlyTransportTestBuilder(t *testing.T, level int, nodes []quant.FlowNode, edges []quant.FlowEdge) *builder {
	t.Helper()
	b, err := newBuilder(Setting{BeltLv: level, InserterLv: 3})
	if err != nil {
		t.Fatal(err)
	}
	b.flowGraph = &quant.FlowGraph{Nodes: nodes, Edges: edges}
	b.logistics = newLogisticsSummary(*b.flowGraph, b.setting)
	return b
}

// addFlyTransportTestEndpoint 职责：创建独立且可统一编号的实体配送端点；使用：分配和最终摘要测试；边界：几何为合成坐标，不当作真实可建证明。
func addFlyTransportTestEndpoint(b *builder, owner, item int, input bool, rate float64) *flyTransportEndpoint {
	b.owner = owner
	x := float64(len(b.flyEndpoints) * 12)
	link := b.make(2000+b.setting.BeltLv, x, 0, 0)
	belts := b.byPath([]vector{{x + 1, 0, 0}, {x + 2, 0, 0}})
	box := b.make(2101, x+5, 0, 0)
	delivery := b.make(2107, x+5, 0, 1.875)
	delivery.input = box
	delivery.record.FilterID = int16(item)
	arm := b.make(2013, x+2, 0, 0)
	arm.record.FilterID = int16(item)
	if input {
		reverse(belts)
		connect(belts[0], link)
		arm.input, arm.output = box, belts[len(belts)-1]
	} else {
		connect(link, belts[0])
		arm.input, arm.output = belts[len(belts)-1], box
	}
	endpoint := &flyTransportEndpoint{owner: owner, item: item, input: input, rate: rate, link: link, belts: belts, box: box, delivery: delivery, arm: arm}
	b.flyEndpoints = append(b.flyEndpoints, endpoint)
	return endpoint
}
