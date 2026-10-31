package build

// 文件职责：回归 mode6 总线 FlowGraph 到实体路线的逐边绑定、端口输出和能力失败。
// 使用：go test ./helper/blueprint/build；边界：测试仅验证数值和指针拓扑，不模拟游戏机队或 Unity 运行时。

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/quant"
)

// TestBusTransportBindsExternalFlowToPhysicalRoute 职责：确认原料边生成带有真实建筑索引的 lane 和开放入口；使用：mode6 实体覆盖回归；边界：只验证规划流，不宣称外界持续供货。
func TestBusTransportBindsExternalFlowToPhysicalRoute(t *testing.T) {
	b, edge := testBusExternalInputBuilder()
	if err := b.validateBusTransport(); err != nil {
		t.Fatalf("valid external route rejected: %v", err)
	}
	if len(b.logistics.Lanes) != 2 || b.logistics.Lanes[0].EdgeID != edge.ID || len(b.logistics.Lanes[0].BuildingIndices) < 4 {
		t.Fatalf("expected one lane tied to real route buildings, got %+v", b.logistics.Lanes)
	}
	if len(b.logistics.ExternalPorts) != 2 || b.logistics.ExternalPorts[0].Kind != "external-input" || b.logistics.ExternalPorts[0].ItemID != edge.ItemID || b.logistics.ExternalPorts[1].Kind != "target-output" || b.logistics.ExternalPorts[1].ItemID != 1102 || b.logistics.Metrics.ExternalInputCount != 1 {
		t.Fatalf("expected one marked external port, got %+v metrics=%+v", b.logistics.ExternalPorts, b.logistics.Metrics)
	}
}

// TestBusTransportRejectsUncoveredExternalFlow 职责：防止 mode6 没有入口实体仍返回成功；使用：fail-closed 回归；边界：只断言边界错误，不比较错误文案。
func TestBusTransportRejectsUncoveredExternalFlow(t *testing.T) {
	b, _ := testBusExternalInputBuilder()
	b.busLinks = b.busLinks[1:]
	if err := b.validateBusTransport(); err == nil {
		t.Fatal("an uncovered external-input edge must fail")
	}
}

// TestBusTransportRejectsMissingTargetBoundary 职责：确保目标交付边有明确开放端口；使用：边界覆盖回归；边界：不猜测缺失的出带。
func TestBusTransportRejectsMissingTargetBoundary(t *testing.T) {
	b, _ := testBusExternalInputBuilder()
	b.busLinks = b.busLinks[:2]
	if err := b.validateBusTransport(); err == nil {
		t.Fatal("an uncovered target-output edge must fail")
	}
}

// TestBusTransportKeepsTargetAndLeftoverPortsSeparate 职责：确保同物品目标交付与剩余副产物不会被绑定到彼此端口；使用：边界语义回归；边界：只核验计划路线标识。
func TestBusTransportKeepsTargetAndLeftoverPortsSeparate(t *testing.T) {
	b, _ := testBusExternalInputBuilder()
	b.flowGraph.Nodes = append(b.flowGraph.Nodes, quant.FlowNode{ID: "leftover:1102", Kind: "leftover-sink", ItemID: 1102, ProductionIndex: -1})
	b.flowGraph.Edges = append(b.flowGraph.Edges, quant.FlowEdge{ID: "flow:1102:production:0>leftover:1102", ItemID: 1102, SourceNodeID: "production:0", TargetNodeID: "leftover:1102", RatePerMinute: 5, FlowKind: "leftover"})
	first := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{2, 3, 0}, owner: 0}
	last := &placed{record: blueprintBuildingForBus(2001, 1102), a: vector{3, 3, 0}, owner: 0}
	connect(first, last)
	leftoverArm := testBusArm(2013, first, last, 1102)
	b.all = append(b.all, first, last)
	b.all = append(b.all, leftoverArm)
	b.busLinks = append(b.busLinks, busTransportLink{kind: "leftover", owner: 0, item: 1102, layer: -1, rate: 5, endpoint: last, parts: []*placed{leftoverArm}, row: []*placed{first, last}})

	if err := b.validateBusTransport(); err != nil {
		t.Fatalf("valid target and leftover routes rejected: %v", err)
	}
	ports := map[string]BoundaryPort{}
	for _, port := range b.logistics.ExternalPorts {
		ports[port.Kind] = port
	}
	indices := map[*placed]int{}
	for index, placed := range b.all {
		indices[placed] = index
	}
	if ports["target-output"].BuildingIndex != indices[b.busLinks[2].endpoint] || ports["leftover"].BuildingIndex != indices[b.busLinks[3].endpoint] {
		t.Fatalf("target and leftover edges must retain distinct output entities: %+v", ports)
	}
}

// TestBusTransportRejectsOpenRing 职责：确保混带闭环不是仅有一串孤立带；使用：有向连通回归；边界：不推演带内物品时序。
func TestBusTransportRejectsOpenRing(t *testing.T) {
	b, _ := testBusExternalInputBuilder()
	b.busLayerBelts[0][0].output = nil
	if err := b.validateBusTransport(); err == nil {
		t.Fatal("an open bus ring must fail")
	}
}

// TestBusTransportRejectsInserterOverCapacity 职责：确保总线分拣器使用版本锁定额定吞吐；使用：单臂容量回归；边界：额定容量不是游戏内机队仿真。
func TestBusTransportRejectsInserterOverCapacity(t *testing.T) {
	from := &placed{a: vector{0, 0, 0}}
	to := &placed{a: vector{0, 1, 0}}
	arm := testBusArm(2013, from, to, 0)
	if err := validateBusInserter(arm, 1); err != nil {
		t.Fatalf("small verified load rejected: %v", err)
	}
	if err := validateBusInserter(arm, 10000); err == nil {
		t.Fatal("over-capacity inserter must fail")
	}
}

// TestBusBoundaryCreatesFilteredEndpoints 职责：确认多产物边界分别落到独立尾端和过滤臂；使用：mode6 装配回归；边界：只验证离线实体拓扑，不代替游戏连续运行。
func TestBusBoundaryCreatesFilteredEndpoints(t *testing.T) {
	first := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{0, 0, 0}, owner: 0}
	graph := &quant.FlowGraph{
		Nodes: []quant.FlowNode{
			{ID: "production:0", Kind: "production", ProductionIndex: 0},
			{ID: "target:1102", Kind: "target-sink", ItemID: 1102, ProductionIndex: -1},
			{ID: "leftover:1114", Kind: "leftover-sink", ItemID: 1114, ProductionIndex: -1},
		},
		Edges: []quant.FlowEdge{
			{ID: "flow:1102:production:0>target:1102", ItemID: 1102, SourceNodeID: "production:0", TargetNodeID: "target:1102", RatePerMinute: 10, FlowKind: "target-output"},
			{ID: "flow:1114:production:0>leftover:1114", ItemID: 1114, SourceNodeID: "production:0", TargetNodeID: "leftover:1114", RatePerMinute: 5, FlowKind: "leftover"},
		},
	}
	b := &builder{all: []*placed{first}, flowGraph: graph, setting: Setting{BeltLv: 1, InserterLv: 3}}
	l := &line{output: []*placed{first}}
	if err := b.addBusBoundaryLinks(l, 1, nil); err != nil {
		t.Fatalf("boundary branch construction failed: %v", err)
	}
	if len(b.busLinks) != 2 || b.busLinks[0].endpoint == b.busLinks[1].endpoint {
		t.Fatalf("expected distinct boundary endpoints, got %+v", b.busLinks)
	}
	for _, link := range b.busLinks {
		if len(link.parts) != 1 || link.parts[0].input == nil || link.parts[0].output != link.endpoint || int(link.parts[0].record.FilterID) != link.item {
			t.Fatalf("boundary link is not item-filtered: %+v", link)
		}
	}
}

// TestVanillaMode6RejectsUnsupportedSingleArmLoad 职责：锁定真实 Vanilla 量化夹具在当前单臂总线下的硬拒绝；使用：端到端容量回归；边界：此测试不是游戏内运行或物流验收。
func TestVanillaMode6RejectsUnsupportedSingleArmLoad(t *testing.T) {
	var fixture struct {
		Cases []struct {
			Name    string        `json:"name"`
			Request quant.Request `json:"request"`
			Setting Setting       `json:"setting"`
		} `json:"cases"`
	}
	data, err := os.ReadFile("../../quant/testdata/acceptance-fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("Vanilla acceptance fixture is empty")
	}
	plan, err := quant.Solve(context.Background(), fixture.Cases[0].Request)
	if err != nil {
		t.Fatal(err)
	}
	power := false
	fixture.Cases[0].Setting.NeedPower = &power
	_, err = Generate(context.Background(), Request{Plan: plan, Mode: 6, Setting: fixture.Cases[0].Setting})
	var generated *blueprint.Error
	if !errors.As(err, &generated) || generated.Code != "GEN_E_PORT_CAPACITY" {
		t.Fatalf("want explicit entity capacity refusal, got %v", err)
	}
}

// testBusExternalInputBuilder 职责：构造最小真实外部输入到生产行拓扑；使用：mode6 验证器测试；边界：不构建蓝图编码或依赖量化数据库。
func testBusExternalInputBuilder() (*builder, quant.FlowEdge) {
	item := 1101
	graph := &quant.FlowGraph{
		Nodes: []quant.FlowNode{
			{ID: "source:1101", Kind: "raw-source", ItemID: item, ProductionIndex: -1},
			{ID: "production:0", Kind: "production", ItemID: item, ProductionIndex: 0},
			{ID: "target:1102", Kind: "target-sink", ItemID: 1102, ProductionIndex: -1},
		},
		Edges: []quant.FlowEdge{
			{ID: "flow:1101:source:1101>production:0", ItemID: item, SourceNodeID: "source:1101", TargetNodeID: "production:0", RatePerMinute: 30, FlowKind: "external-input"},
			{ID: "flow:1102:production:0>target:1102", ItemID: 1102, SourceNodeID: "production:0", TargetNodeID: "target:1102", RatePerMinute: 15, FlowKind: "target-output"},
		},
	}
	cycle := make([]*placed, 4)
	for i, point := range []vector{{0, 1, 0}, {1, 1, 0}, {1, 0, 0}, {0, 0, 0}} {
		cycle[i] = &placed{record: blueprintBuildingForBus(2001, 0), a: point}
	}
	for i := range cycle {
		cycle[i].output = cycle[(i+1)%len(cycle)]
	}
	external := &placed{record: blueprintBuildingForBus(2001, item), a: vector{-2, 0, 0}}
	center := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{-1, 0, 0}}
	feed := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{0, 0, 0}}
	connect(external, center)
	connect(center, feed)
	rawArm := testBusArm(2013, feed, cycle[0], 0)
	returnBelt := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{-1, 1, 0}, owner: -1}
	connect(returnBelt, center)
	returnArm := testBusArm(2013, cycle[0], returnBelt, item)
	rowFirst := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{2, 1, 0}, owner: 0}
	rowLast := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{3, 1, 0}, owner: 0}
	connect(rowFirst, rowLast)
	productionArm := testBusArm(2013, cycle[1], rowFirst, item)
	outputFirst := &placed{record: blueprintBuildingForBus(2001, 0), a: vector{2, 2, 0}, owner: 0}
	outputLast := &placed{record: blueprintBuildingForBus(2001, 1102), a: vector{3, 2, 0}, owner: 0}
	connect(outputFirst, outputLast)
	outputArm := testBusArm(2013, outputFirst, outputLast, 1102)
	links := []busTransportLink{
		{kind: "raw-input", owner: -1, item: item, layer: 0, rate: 30, endpoint: external, ring: cycle[0], ringPath: cycle, parts: []*placed{rawArm, returnArm}, row: []*placed{external, center, feed}},
		{kind: "production-input", owner: 0, item: item, layer: 0, rate: 30, endpoint: rowFirst, ring: cycle[1], ringPath: cycle, parts: []*placed{productionArm}, row: []*placed{rowFirst, rowLast}},
		{kind: "target-output", owner: 0, item: 1102, layer: -1, rate: 15, endpoint: outputLast, parts: []*placed{outputArm}, row: []*placed{outputFirst, outputLast}},
	}
	all := append([]*placed{}, cycle...)
	all = append(all, external, center, feed, rawArm, returnBelt, returnArm, rowFirst, rowLast, productionArm, outputFirst, outputLast, outputArm)
	return &builder{all: all, flowGraph: graph, busLinks: links, busLayers: map[int]int{item: 0}, busLayerBelts: map[int][]*placed{0: cycle}, logistics: &LogisticsSummary{BeltLevel: 1, BeltCapacityPerMinute: 360}}, graph.Edges[0]
}

// testBusArm 职责：构造与坐标跨度一致的分拣器指针；使用：实体拓扑回归；边界：容量来源仍通过 helper/transport 读取。
func testBusArm(itemID int, from, to *placed, filter int) *placed {
	record := blueprintBuildingForBus(itemID, 0)
	record.FilterID = int16(filter)
	record.Parameters = []int32{1}
	return &placed{record: record, a: from.a, b: to.a, input: from, output: to}
}

// blueprintBuildingForBus 职责：创建带稳定原型和物品标记的最小建筑记录；使用：纯校验测试；边界：不参与编码校验。
func blueprintBuildingForBus(itemID, markedItem int) blueprint.Building {
	params := []int32{}
	if markedItem != 0 {
		params = []int32{int32(markedItem), 30}
	}
	return blueprint.Building{ItemID: int16(itemID), Parameters: params, OutputObjIdx: -1, InputObjIdx: -1}
}
