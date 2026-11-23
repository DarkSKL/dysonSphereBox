package planet

// 文件职责：验证额定制造模板只回填当前合同净流；使用：制造与进口共同仓储回归；边界：不降低真实带宽、库存或来源终检。

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/industryplan/search"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
)

// TestModuleBoundaryPortsSharesNetRates 职责：核验循环相抵、多物理口和有限带宽；使用：额定边界降为当前净流；边界：保留全部物理索引，不能按端口重复兑现同一需求。
func TestModuleBoundaryPortsSharesNetRates(t *testing.T) {
	production := quant.Production{
		Inputs: []quant.ItemRate{{ItemID: 1114, RatePerMinute: 7}, {ItemID: 1114, RatePerMinute: 5},
			{ItemID: 1120, RatePerMinute: 10}, {ItemID: 1120, RatePerMinute: 20}, {ItemID: 1001, RatePerMinute: 4}},
		Outputs: []quant.ItemRate{{ItemID: 1120, RatePerMinute: 45}, {ItemID: 1120, RatePerMinute: 5},
			{ItemID: 1109, RatePerMinute: 10}, {ItemID: 1001, RatePerMinute: 4}},
	}
	ports := []build.BoundaryPort{
		{ItemID: 1114, Kind: "external-input", RatePerMinute: 10},
		{ItemID: 1114, Kind: "external-input", RatePerMinute: 10},
		{ItemID: 1120, Kind: "external-input", RatePerMinute: 5},
		{ItemID: 1120, Kind: "target-output", RatePerMinute: 12},
		{ItemID: 1120, Kind: "target-output", RatePerMinute: 30},
		{ItemID: 1120, Kind: "leftover", RatePerMinute: 3},
		{ItemID: 1001, Kind: "external-input", RatePerMinute: 4},
		{ItemID: 1001, Kind: "target-output", RatePerMinute: 4},
		{ItemID: 1109, Kind: "leftover", RatePerMinute: 9},
	}
	buildings := make([]blueprint.Building, len(ports))
	for i := range ports {
		ports[i].BuildingIndex, ports[i].AreaIndex, ports[i].X = i, 2, float64(i)
		local, remote := 1, 2
		if ports[i].Kind == "external-input" {
			local, remote = 2, 1
		}
		buildings[i] = stationObject(int32(i), Position{}, ports[i].ItemID, local, remote, false).Building
	}
	before := append([]build.BoundaryPort(nil), ports...)
	bounded := moduleBoundaryPorts(production, ports)
	wantRates := []float64{10, 2, 0, 12, 8, 0, 0, 0, 9}
	if len(bounded) != len(ports) || !reflect.DeepEqual(ports, before) {
		t.Fatal("降流改写了缓存边界或删除了零流物理端口")
	}
	for i, port := range bounded {
		want := before[i]
		want.RatePerMinute = wantRates[i]
		if port != want {
			t.Fatalf("端口%d净流或物理位置错误：%+v，期望%+v", i, port, want)
		}
	}
	plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1114, Kind: "import", PeerPlanetID: 102, RatePerMinute: 12},
		{ItemID: 1120, Kind: "export", PeerPlanetID: 103, RatePerMinute: 20},
		{ItemID: 1109, Kind: "export", PeerPlanetID: 103, RatePerMinute: 10},
	}}
	actual, err := configureProductionPorts(plan, buildings, bounded)
	want := append([]industryplan.Port(nil), plan.Ports...)
	want[2].RatePerMinute = 9
	if err != nil || !reflect.DeepEqual(actual, want) {
		t.Fatalf("循环净流、共享余额或实际带宽被扩大：%+v，%v", actual, err)
	}
	for _, index := range []int{2, 5, 6, 7} {
		if buildings[index].Parameters[2] != 0 {
			t.Fatalf("零流物理端口%d残留远程角色", index)
		}
	}
}

// TestManufacturingRatedBoundaryWithReceiving 职责：避免额定余量吞掉真实进口对应的仓储需求；使用：完整生成和严格解码；边界：75能力只报告60制造，余下10由进口证明。
func TestManufacturingRatedBoundaryWithReceiving(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = WithTemplateCache(search.WithControl(ctx, search.New(2*time.Minute, nil)))
	plan := moduleRegionTestPlan(t, ctx, 60)
	plan.RawInputs = append(plan.RawInputs, quant.ItemRate{ItemID: 1101, RatePerMinute: 10})
	plan.ExternalInputItemIDs = append(plan.ExternalInputItemIDs, 1101)
	plan.Targets = []quant.Target{{ItemID: 1101, RatePerMinute: 20}}
	plan.Leftovers = []quant.ItemRate{{ItemID: 1101, RatePerMinute: 50}}
	plan.FlowGraph = nil
	graph, err := quant.BuildFlowGraphContext(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.FlowGraph = &graph
	needPower := false
	input := ProductionRequest{
		Plan:      industryplan.PlanetPlan{PlanetID: 101, Plan: plan},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
		Options:   ProductionOptions{Name: "实际净流仓储", Profile: "low", Setting: build.Setting{NeedPower: &needPower, BeltLv: 3, InserterLv: 3}},
	}
	for _, raw := range plan.RawInputs {
		port := industryplan.Port{ItemID: raw.ItemID, Kind: "external", RatePerMinute: raw.RatePerMinute}
		if raw.ItemID == 1101 {
			port.Kind, port.PeerPlanetID = "import", 102
		}
		input.Plan.Ports = append(input.Plan.Ports, port)
	}
	input.Plan.Ports = append(input.Plan.Ports,
		industryplan.Port{ItemID: 1101, Kind: "export", PeerPlanetID: 103, RatePerMinute: 20},
		industryplan.Port{ItemID: 1101, Kind: "storage", RatePerMinute: 50, StorageItems: 20000})
	before, _ := json.Marshal(input)
	result, err := Generate(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := blueprint.Decode(result.Blueprint, true)
	if err != nil || doc.Summary.ItemCounts[2302] != 1 {
		t.Fatalf("真实单炉蓝图未通过严格解码：%v，设施%v", err, doc.Summary.ItemCounts)
	}
	for _, want := range input.Plan.Ports {
		found := false
		for _, got := range result.Actual.Ports {
			if got.ItemID == want.ItemID && got.Kind == want.Kind && got.PeerPlanetID == want.PeerPlanetID {
				found = math.Abs(got.RatePerMinute-want.RatePerMinute) < 1e-8 && got.StorageItems == want.StorageItems
			}
		}
		if !found {
			t.Fatalf("制造额定量改变了合同：期望%+v，实际%+v", want, result.Actual.Ports)
		}
	}
	stock, receiving := 0, false
	for _, b := range doc.Buildings {
		if b.ItemID != 2104 {
			continue
		}
		if slot := stationSlot(&b, 1101); slot >= 0 {
			stock += int(b.Parameters[slot*6+3])
			receiving = receiving || b.Parameters[slot*6+1] == 2 && b.Parameters[slot*6+2] == 2
		}
	}
	if !receiving || stock < 20000 {
		t.Fatalf("本地余料与进口10没有共享收货库存实体：收货塔%v，库存%d", receiving, stock)
	}
	after, _ := json.Marshal(input)
	if !bytes.Equal(before, after) {
		t.Fatal("边界分配改写了原生产合同或额定能力")
	}

	// 同一硬件还可生产75，但缺少的1件进口不能由额定余量替合同补齐。
	input.Plan.Ports = append([]industryplan.Port(nil), input.Plan.Ports...)
	for i := range input.Plan.Ports {
		if input.Plan.Ports[i].ItemID == 1101 && input.Plan.Ports[i].Kind == "import" {
			input.Plan.Ports[i].RatePerMinute = 9
		}
	}
	_, err = Generate(ctx, input)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" || issue.Item != 1101 {
		t.Fatalf("来源不足仍通过，或丢失真实缺口：%v", err)
	}
}
