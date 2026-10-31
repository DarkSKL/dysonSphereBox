package build

import (
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestStationFlowCoverageRequiresEveryProductionItem 职责：回归塔式供需端点不得漏掉副产物流；使用：站点诊断单元测试；边界：不模拟游戏内机队。
func TestStationFlowCoverageRequiresEveryProductionItem(t *testing.T) {
	graph := &quant.FlowGraph{
		Nodes: []quant.FlowNode{{ID: "production:0", Kind: "production", ProductionIndex: 0}},
		Edges: []quant.FlowEdge{
			{ID: "main", ItemID: 1, SourceNodeID: "production:0", RatePerMinute: 30},
			{ID: "byproduct", ItemID: 2, SourceNodeID: "production:0", RatePerMinute: 5},
		},
	}
	links := []stationTransportLink{{owner: 0, item: 1, output: true, rate: 30}}
	if err := validateStationFlowCoverage(graph, links); err == nil {
		t.Fatal("missing byproduct station route should fail closed")
	}
	links = append(links, stationTransportLink{owner: 0, item: 2, output: true, rate: 5})
	if err := validateStationFlowCoverage(graph, links); err != nil {
		t.Fatalf("complete item routes should pass: %v", err)
	}
	links[1].rate = 4
	if err := validateStationFlowCoverage(graph, links); err == nil {
		t.Fatal("route rate mismatch should fail")
	}
}

// TestStationEdgeAllocationSplitsAcrossRows 职责：验证拆分机器行只消费其计划份额且可组合满足单条边；使用：容量分配回归；边界：不把超额机器能力当作可用产出。
func TestStationEdgeAllocationSplitsAcrossRows(t *testing.T) {
	edge := quant.FlowEdge{ID: "split", ItemID: 1, RatePerMinute: 35}
	first := &stationTransportLink{owner: 0, item: 1, output: true, rate: 20}
	second := &stationTransportLink{owner: 0, item: 1, output: true, rate: 15}
	remaining := map[*stationTransportLink]float64{first: first.rate, second: second.rate}
	used, err := consumeStationLinks(edge, []*stationTransportLink{first, second}, remaining)
	if err != nil || len(used) != 2 {
		t.Fatalf("want both row ports to carry the edge, got used=%d err=%v", len(used), err)
	}
	if remaining[first] != 0 || remaining[second] != 0 {
		t.Fatalf("edge allocation must consume exact plan rates: %v", remaining)
	}
	if _, err := consumeStationLinks(edge, []*stationTransportLink{first, second}, remaining); err == nil {
		t.Fatal("reusing exhausted station routes should fail")
	}
}

// TestStationLinksRequireMatchingRoleAndItem 职责：防止同一产线其他物品或反方向端口错误承接流边；使用：塔路线筛选回归；边界：不按主产物猜测路线。
func TestStationLinksRequireMatchingRoleAndItem(t *testing.T) {
	links := map[string][]*stationTransportLink{
		"production:0": {
			{owner: 0, item: 1, output: true},
			{owner: 0, item: 2, output: false},
		},
	}
	edge := quant.FlowEdge{ItemID: 1, SourceNodeID: "production:0", TargetNodeID: "production:1"}
	if got := findStationLinks(edge, links, true); len(got) != 1 || got[0].item != 1 {
		t.Fatalf("unexpected supply matches: %+v", got)
	}
	if got := findStationLinks(edge, links, false); len(got) != 0 {
		t.Fatalf("wrong item or role matched demand: %+v", got)
	}
}
