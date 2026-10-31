package quant

// 文件职责：验证规范流图守恒、稳定配对、循环保边及真实量化样本；使用：go test 回归；边界：不宣称编码、几何或游戏放置通过。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
)

// flowRate 职责：构造纯数据速率样本；使用：流图单测；边界：名称由加载的数据集规范化。
func flowRate(id int, rate float64) ItemRate {
	return ItemRate{ItemID: id, RatePerMinute: rate}
}

// TestFlowGraphConservationAndTopology 职责：证明净流量、剩余出口及上游顺序一致；使用：核心流图回归；边界：不包含实体路线。
func TestFlowGraphConservationAndTopology(t *testing.T) {
	plan := Result{Dataset: "Vanilla", RawInputs: []ItemRate{flowRate(1001, 600)}, Targets: []Target{{ItemID: 1201, RatePerMinute: 300}}, Leftovers: []ItemRate{flowRate(1101, 300)}, Productions: []Production{
		{MainItemID: 1201, RecipeID: 3, Inputs: []ItemRate{flowRate(1101, 300)}, Outputs: []ItemRate{flowRate(1201, 300)}},
		{MainItemID: 1101, RecipeID: 1, Inputs: []ItemRate{flowRate(1001, 600)}, Outputs: []ItemRate{flowRate(1101, 600)}},
	}}
	graph, err := BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(graph.ProductionOrder, []int{1, 0}) {
		t.Fatalf("upstream order = %v", graph.ProductionOrder)
	}
	if len(graph.Edges) != 4 {
		t.Fatalf("want four precise edges, got %+v", graph.Edges)
	}
	for _, balance := range graph.Balances {
		if math.Abs(balance.DifferencePerMinute) > 1e-9 {
			t.Fatalf("unbalanced %+v", balance)
		}
	}
	plan.Productions[0], plan.Productions[1] = plan.Productions[1], plan.Productions[0]
	permuted, err := BuildFlowGraph(plan)
	if err != nil || graph.Digest != permuted.Digest {
		t.Fatalf("array order changed canonical graph: %s / %s: %v", graph.Digest, permuted.Digest, err)
	}
	if !reflect.DeepEqual(permuted.ProductionOrder, []int{0, 1}) {
		t.Fatalf("original index mapping was not updated: %v", permuted.ProductionOrder)
	}
}

// TestFlowGraphSparseMatchingAndDuplicateMainItem 职责：避免多来源全连接及主产物 ID 节点冲突；使用：多目标共享上游回归；边界：不同产线实例不合并。
func TestFlowGraphSparseMatchingAndDuplicateMainItem(t *testing.T) {
	plan := Result{Dataset: "Vanilla", Targets: []Target{{ItemID: 1201, RatePerMinute: 300}, {ItemID: 1202, RatePerMinute: 600}}, Productions: []Production{
		{MainItemID: 1101, RecipeID: 1, Outputs: []ItemRate{flowRate(1101, 300)}},
		{MainItemID: 1101, RecipeID: 2, Outputs: []ItemRate{flowRate(1101, 600)}},
		{MainItemID: 1201, Inputs: []ItemRate{flowRate(1101, 300)}, Outputs: []ItemRate{flowRate(1201, 300)}},
		{MainItemID: 1202, Inputs: []ItemRate{flowRate(1101, 600)}, Outputs: []ItemRate{flowRate(1202, 600)}},
	}}
	graph, err := BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	seen, internal := map[string]bool{}, 0
	for _, node := range graph.Nodes {
		if seen[node.ID] {
			t.Fatalf("duplicate node %s", node.ID)
		}
		seen[node.ID] = true
	}
	for _, edge := range graph.Edges {
		if edge.FlowKind == "internal" {
			internal++
		}
	}
	if internal > 3 {
		t.Fatalf("all-to-all fragmentation: %+v", graph.Edges)
	}
	plan.FlowGraph = &FlowGraph{Edges: []FlowEdge{{ID: "forged", RatePerMinute: -1}}}
	rebuilt, err := BuildFlowGraph(plan)
	if err != nil || rebuilt.Digest != graph.Digest {
		t.Fatalf("trusted fields were not rebuilt: %v", err)
	}
}

// TestFlowGraphSCCKeepsCycleEdges 职责：检查循环组缩并不吞掉物流；使用：循环配方回归；边界：不将纯循环供需可行性冒充设备运行可行性。
func TestFlowGraphSCCKeepsCycleEdges(t *testing.T) {
	plan := Result{Dataset: "Vanilla", Targets: []Target{{ItemID: 1201, RatePerMinute: 10}}, Leftovers: []ItemRate{flowRate(1101, 20)}, Productions: []Production{
		{MainItemID: 1101, Inputs: []ItemRate{flowRate(1201, 30)}, Outputs: []ItemRate{flowRate(1101, 100)}},
		{MainItemID: 1201, Inputs: []ItemRate{flowRate(1101, 80)}, Outputs: []ItemRate{flowRate(1201, 40)}},
	}}
	graph, err := BuildFlowGraph(plan)
	if err != nil {
		t.Fatal(err)
	}
	cycles, internal := 0, 0
	for _, group := range graph.Components {
		if group.Cyclic {
			cycles++
			if len(group.NodeIDs) != 2 {
				t.Fatalf("bad cyclic group %+v", group)
			}
		}
	}
	for _, edge := range graph.Edges {
		if edge.FlowKind == "internal" {
			internal++
		}
	}
	if cycles != 1 || internal != 2 {
		t.Fatalf("cycle lost: groups %+v edges %+v", graph.Components, graph.Edges)
	}
}

// TestFlowGraphRejectsInvalidData 职责：保证守恒和输入错误失败关闭；使用：纯数据负样本；边界：浮点容差仅接受数值残差。
func TestFlowGraphRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name string
		raw  ItemRate
		want string
	}{{"shortage", flowRate(1101, 59), "不守恒"}, {"surplus", flowRate(1101, 61), "不守恒"}, {"negative", flowRate(1101, -1), "无效"}, {"nan", flowRate(1101, math.NaN()), "无效"}, {"unknown", flowRate(999999, 60), "无效"}} {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildFlowGraph(Result{RawInputs: []ItemRate{test.raw}, Targets: []Target{{ItemID: 1101, RatePerMinute: 60}}})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %s error, got %v", test.want, err)
			}
		})
	}
	_, err := BuildFlowGraph(Result{RawInputs: []ItemRate{flowRate(1101, 60+1e-8)}, Targets: []Target{{ItemID: 1101, RatePerMinute: 60}}})
	if err != nil {
		t.Fatalf("floating residual rejected: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BuildFlowGraphContext(ctx, Result{}); err != context.Canceled {
		t.Fatalf("cancel error = %v", err)
	}
}

// TestFlowGraphNettingAndExternalSource 职责：校验设备内抵消及显式外供语义；使用：旧数据重建；边界：不跨生产节点抵消。
func TestFlowGraphNettingAndExternalSource(t *testing.T) {
	graph, err := BuildFlowGraph(Result{RawInputs: []ItemRate{flowRate(1101, 30)}, ExternalInputItemIDs: []int{1101}, Targets: []Target{{ItemID: 1201, RatePerMinute: 30}}, Productions: []Production{{MainItemID: 1201, Inputs: []ItemRate{flowRate(1101, 40)}, Outputs: []ItemRate{flowRate(1101, 10), flowRate(1201, 30)}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Edges) != 2 || graph.Edges[0].SourceNodeID != "external:1101" {
		t.Fatalf("net/external graph %+v", graph)
	}
}

// TestVanillaAcceptanceFlowFixtures 职责：用真实 Solve 验证四个单目标和综合目标的规范物料账；使用：固定数据集回归；边界：数据指纹变化必须审核 fixture，不能覆盖预期绕过失败。
func TestVanillaAcceptanceFlowFixtures(t *testing.T) {
	var fixture struct {
		Metadata struct {
			DatasetSHA256 string `json:"datasetSHA256"`
			DataVersion   string `json:"dataVersion"`
			GameVersion   string `json:"gameVersion"`
		} `json:"metadata"`
		Cases []struct {
			Name     string  `json:"name"`
			Request  Request `json:"request"`
			Baseline struct {
				MachineCount int `json:"machineCount"`
			} `json:"baseline"`
		} `json:"cases"`
	}
	raw, err := os.ReadFile("testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	dataset, err := os.ReadFile("data/Vanilla.json")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(dataset)
	if hex.EncodeToString(sum[:]) != fixture.Metadata.DatasetSHA256 {
		t.Fatal("Vanilla dataset changed: review fixture metadata and baseline before accepting")
	}
	if len(fixture.Cases) != 5 {
		t.Fatalf("want four single and one combined fixture, got %d", len(fixture.Cases))
	}
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			plan, err := Solve(context.Background(), test.Request)
			if err != nil {
				t.Fatal(err)
			}
			if plan.DataVersion != fixture.Metadata.DataVersion || plan.GameVersion != fixture.Metadata.GameVersion || plan.MachineCount != test.Baseline.MachineCount {
				t.Fatalf("quantification changed: version %s / %s machines %d want %d", plan.DataVersion, plan.GameVersion, plan.MachineCount, test.Baseline.MachineCount)
			}
			if !reflect.DeepEqual(plan.Targets, test.Request.Targets) || plan.FlowGraph == nil || len(plan.FlowGraph.Edges) == 0 {
				t.Fatal("target account or canonical graph missing")
			}
			for _, target := range plan.Targets {
				if target.RatePerMinute != 120 {
					t.Fatalf("target changed %+v", target)
				}
			}
			for _, balance := range plan.FlowGraph.Balances {
				if math.Abs(balance.DifferencePerMinute) > FlowTolerance(balance.SupplyPerMinute, balance.DemandPerMinute) {
					t.Fatalf("flow imbalance %+v", balance)
				}
			}
			assertRoundedProductionFlow(t, plan)
			digest := plan.FlowGraph.Digest
			for i, j := 0, len(plan.Productions)-1; i < j; i, j = i+1, j-1 {
				plan.Productions[i], plan.Productions[j] = plan.Productions[j], plan.Productions[i]
			}
			rebuilt, err := BuildFlowGraph(plan)
			if err != nil || rebuilt.Digest != digest {
				t.Fatalf("canonical fixture changed on permutation: %v", err)
			}
			t.Logf("machines=%d nodes=%d edges=%d SCCs=%d digest=%s", plan.MachineCount, len(rebuilt.Nodes), len(rebuilt.Edges), len(rebuilt.Components), rebuilt.Digest)
		})
	}
}

// assertRoundedProductionFlow 职责：验证机器取整只增加设备能力余量而不改变计划物料流；使用：真实 Vanilla fixture；边界：不模拟取整机器持续满负荷后的游戏库存，实际计划仍以 Solve 的精确流量为准。
func assertRoundedProductionFlow(t *testing.T, plan Result) {
	t.Helper()
	productionNodes := map[int]string{}
	for _, node := range plan.FlowGraph.Nodes {
		if node.Kind == "production" {
			productionNodes[node.ProductionIndex] = node.ID
		}
	}
	for index, production := range plan.Productions {
		if production.ExactMachineCount <= 0 || production.MachineCount <= 0 {
			t.Fatalf("production %d has invalid machine counts: %+v", index, production)
		}
		if float64(production.MachineCount)+1e-9 < production.ExactMachineCount {
			t.Fatalf("production %d rounds below exact capacity: %+v", index, production)
		}
		if math.Abs(float64(production.MachineCount)-production.ExactMachineCount-production.RoundingMargin) > 1e-8 {
			t.Fatalf("production %d rounding margin mismatch: %+v", index, production)
		}
		nodeID, ok := productionNodes[index]
		if !ok {
			t.Fatalf("production %d has no flow node", index)
		}
		for _, expected := range production.Outputs {
			actual := 0.
			for _, edge := range plan.FlowGraph.Edges {
				if edge.SourceNodeID == nodeID && edge.ItemID == expected.ItemID {
					actual += edge.RatePerMinute
				}
			}
			if math.Abs(actual-expected.RatePerMinute) > FlowTolerance(actual, expected.RatePerMinute) {
				t.Fatalf("production %d output %d changed by machine rounding: got %.9f want %.9f", index, expected.ItemID, actual, expected.RatePerMinute)
			}
		}
		for _, expected := range production.Inputs {
			actual := 0.
			for _, edge := range plan.FlowGraph.Edges {
				if edge.TargetNodeID == nodeID && edge.ItemID == expected.ItemID {
					actual += edge.RatePerMinute
				}
			}
			if math.Abs(actual-expected.RatePerMinute) > FlowTolerance(actual, expected.RatePerMinute) {
				t.Fatalf("production %d input %d changed by machine rounding: got %.9f want %.9f", index, expected.ItemID, actual, expected.RatePerMinute)
			}
		}
	}
}

// TestSolvePreservesLegacyNearIntegerRounding 职责：记录 Solve 在接近整数精确台数时的既有取整和计划速率语义；使用：量化器兼容回归及生成器欠能力拒绝边界；边界：测试刻意记录历史行为，不表示一台设备物理上能达到 60.1/min。
func TestSolvePreservesLegacyNearIntegerRounding(t *testing.T) {
	plan, err := Solve(context.Background(), Request{
		Dataset: "Vanilla",
		Targets: []Target{{ItemID: 1101, RatePerMinute: 60.1}},
		Scheme:  Scheme{ItemRecipeChoices: map[string]int{"铁块": 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Productions) != 1 {
		t.Fatalf("want one iron-smelting production, got %d", len(plan.Productions))
	}
	production := plan.Productions[0]
	if production.RecipeID != 1 || production.MachineCount != 1 || math.Abs(production.ExactMachineCount-1.0016666666666667) > 1e-12 || production.RoundingMargin >= 0 {
		t.Fatalf("legacy quant rounding changed: %+v", production)
	}
	if production.OutputCount != 60.1 || plan.Targets[0].RatePerMinute != 60.1 || plan.FlowGraph == nil {
		t.Fatalf("quant flow changed to rounded machine capability: %+v", plan)
	}
	flowOutput := 0.
	for _, edge := range plan.FlowGraph.Edges {
		if edge.SourceNodeID == productionNodeID(plan.FlowGraph, 0) && edge.ItemID == 1101 {
			flowOutput += edge.RatePerMinute
		}
	}
	if math.Abs(flowOutput-60.1) > 1e-8 {
		t.Fatalf("canonical flow must preserve planned output 60.1/min, got %.9f", flowOutput)
	}
}

// productionNodeID 职责：从规范流图取回原量化索引对应节点；使用：取整边界回归；边界：未登记产线返回空键供断言失败。
func productionNodeID(graph *FlowGraph, productionIndex int) string {
	for _, node := range graph.Nodes {
		if node.Kind == "production" && node.ProductionIndex == productionIndex {
			return node.ID
		}
	}
	return ""
}
