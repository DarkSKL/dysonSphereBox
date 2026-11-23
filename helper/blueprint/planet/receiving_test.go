package planet

// 文件职责：验证进口及玩家外供的独立交付库存；使用：原料直通和纯光子目标回归；边界：只报告实际塔槽容量，不把来源重复算成交付。

import (
	"context"
	"reflect"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
	"DysonSphereBox/helper/saveanalysis"
	"DysonSphereBox/helper/transport"
)

// TestReceivingSourceRolesAndCapacity 职责：分别配置进口和外供收货塔并核对真实库存；使用：独立交付/仓储回归；边界：同一物品混合来源仍保留各自远程角色。
func TestReceivingSourceRolesAndCapacity(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	capacity, err := transport.StationItemCapacity(2104)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		incoming  []industryplan.Port
		demand    []industryplan.Port
		wantModes map[int32]int
	}{
		{"import-delivery", []industryplan.Port{{ItemID: 1208, Kind: "import", PeerPlanetID: 102, RatePerMinute: 60}},
			[]industryplan.Port{{ItemID: 1208, Kind: "delivery", RatePerMinute: 60}}, map[int32]int{2: 1}},
		{"external-delivery", []industryplan.Port{{ItemID: 1208, Kind: "external", RatePerMinute: 60}},
			[]industryplan.Port{{ItemID: 1208, Kind: "delivery", RatePerMinute: 60}}, map[int32]int{0: 1}},
		{"import-storage", []industryplan.Port{{ItemID: 1208, Kind: "import", PeerPlanetID: 102, RatePerMinute: 60}},
			[]industryplan.Port{{ItemID: 1208, Kind: "storage", RatePerMinute: 60, StorageItems: capacity + 5000}}, map[int32]int{2: 2}},
		{"external-storage", []industryplan.Port{{ItemID: 1208, Kind: "external", RatePerMinute: 60}},
			[]industryplan.Port{{ItemID: 1208, Kind: "storage", RatePerMinute: 60, StorageItems: capacity + 5000}}, map[int32]int{0: 2}},
		{"mixed-delivery-and-storage", []industryplan.Port{{ItemID: 1208, Kind: "import", PeerPlanetID: 102, RatePerMinute: 30}, {ItemID: 1208, Kind: "external", RatePerMinute: 30}},
			[]industryplan.Port{{ItemID: 1208, Kind: "delivery", RatePerMinute: 20}, {ItemID: 1208, Kind: "storage", RatePerMinute: 40, StorageItems: capacity + 5000}}, map[int32]int{0: 1, 2: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := industryplan.PlanetPlan{PlanetID: 101, Ports: append(append([]industryplan.Port(nil), tc.incoming...), tc.demand...)}
			req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
			if err := appendReceiving(context.Background(), &req, ProductionRequest{Plan: plan}, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual.Ports, plan.Ports) {
				t.Fatalf("独立收货能力没有逐项满足合同：%+v", actual.Ports)
			}
			built, err := Build(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			modes, slots := map[int32]int{}, 0
			for _, b := range doc.Buildings {
				if b.ItemID != 2104 || b.Parameters[0] != 1208 || b.Parameters[1] != 0 {
					t.Fatal("直通交付必须具有实际且不额外向本地要货的物品库存槽")
				}
				modes[b.Parameters[2]]++
				slots += int(b.Parameters[3])
			}
			if !reflect.DeepEqual(modes, tc.wantModes) {
				t.Fatalf("来源的远程角色或塔数量不符：%v，期望 %v", modes, tc.wantModes)
			}
			stored := 0
			for _, p := range actual.Ports {
				stored += p.StorageItems
			}
			if stored > slots || slots != built.StationStorage[1208] {
				t.Fatalf("报告库存 %d，真实塔槽 %d，不能重复计算同一库存", stored, slots)
			}
			before := len(req.Objects)
			if err := appendReceiving(context.Background(), &req, ProductionRequest{Plan: plan}, &actual, catalog); err != nil || len(req.Objects) != before || !reflect.DeepEqual(actual.Ports, plan.Ports) {
				t.Fatalf("已满足的收货合同被重复建造或计数：%+v，%v", actual.Ports, err)
			}
		})
	}
}

// TestReceivingCreditsOnlyRemainingContract 职责：复用已验证的部分速率和库存；使用：制造与独立收货共同满足合同；边界：新增塔只承担尚未满足的容量。
func TestReceivingCreditsOnlyRemainingContract(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	capacity, _ := transport.StationItemCapacity(2104)
	for _, kind := range []string{"import", "external"} {
		t.Run(kind, func(t *testing.T) {
			remote := 0
			if kind == "import" {
				remote = 2
			}
			req := Request{PlanetID: 101, Objects: []Object{stationObject(0, Position{}, 1208, 0, remote, false)}}
			actual := industryplan.PlanetActual{Ports: []industryplan.Port{
				{ItemID: 1208, Kind: kind, RatePerMinute: 30},
				{ItemID: 1208, Kind: "storage", RatePerMinute: 30, StorageItems: capacity},
			}}
			plan := industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
				{ItemID: 1208, Kind: kind, RatePerMinute: 60},
				{ItemID: 1208, Kind: "storage", RatePerMinute: 60, StorageItems: capacity + 5000},
			}}
			if err := appendReceiving(context.Background(), &req, ProductionRequest{Plan: plan}, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			if len(req.Objects) != 2 || !reflect.DeepEqual(actual.Ports, plan.Ports) {
				t.Fatalf("已建库存没有抵扣，或合同被重复累加：塔 %d，端口 %+v", len(req.Objects), actual.Ports)
			}
			for _, object := range req.Objects {
				if object.Building.Parameters[1] != 0 {
					t.Fatal("纯收货库存没有本地来源，不应额外请求本地物料")
				}
			}
		})
	}
	req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1208, Kind: "external", RatePerMinute: 30}, {ItemID: 1208, Kind: "delivery", RatePerMinute: 60},
	}}}
	if err := appendReceiving(context.Background(), &req, input, &actual, catalog); err != nil || len(req.Objects) != 0 || len(actual.Ports) != 0 {
		t.Fatalf("不足的来源被错误计为完整交付：%+v，%v", actual, err)
	}
}

// TestReceivingSharesStorageWithManufacturing 职责：验证本地制造和进口共同填入真实库存；使用：整星生成与严格解码；边界：六百分钟缓冲不能仅靠关闭本地物流的远程塔凑容量。
func TestReceivingSharesStorageWithManufacturing(t *testing.T) {
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	plan.Targets = nil
	plan.RawInputs = []quant.ItemRate{{ItemID: 1001, RatePerMinute: 60}, {ItemID: 1101, RatePerMinute: 10}}
	plan.Leftovers = []quant.ItemRate{{ItemID: 1101, RatePerMinute: 70}}
	plan.ExternalInputItemIDs = []int{1001, 1101}
	plan.FlowGraph = nil
	graph, err := quant.BuildFlowGraphContext(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.FlowGraph = &graph
	ports := []industryplan.Port{
		{ItemID: 1001, Kind: "external", RatePerMinute: 60},
		{ItemID: 1101, Kind: "import", PeerPlanetID: 102, RatePerMinute: 10},
		{ItemID: 1101, Kind: "storage", RatePerMinute: 70, StorageItems: 42000},
	}
	power := false
	result, err := Generate(context.Background(), ProductionRequest{
		Plan:      industryplan.PlanetPlan{PlanetID: 101, Plan: plan, Ports: ports},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
		Options:   ProductionOptions{Profile: "low", Setting: build.Setting{NeedPower: &power}},
	})
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if document.Summary.ItemCounts[2302] != 1 {
		t.Fatal("缺少真实本地制造设备")
	}
	supplyStorage, demandStorage := 0, 0
	for _, building := range document.Buildings {
		if building.ItemID != 2104 {
			continue
		}
		for slot := range 5 {
			base := slot * 6
			if building.Parameters[base] != 1101 {
				continue
			}
			switch building.Parameters[base+1] {
			case 1:
				supplyStorage += int(building.Parameters[base+3])
			case 2:
				if building.Parameters[base+2] != 2 {
					t.Fatal("共用库存丢失进口角色")
				}
				demandStorage += int(building.Parameters[base+3])
			}
		}
	}
	if supplyStorage == 0 || demandStorage == 0 || supplyStorage+demandStorage < 70*600 {
		t.Fatalf("六百分钟本地产物与进口没有共同可达的真实库存：供给槽%d，需求槽%d", supplyStorage, demandStorage)
	}
	paired := false
	for _, item := range result.LocalDroneItems {
		paired = paired || item == 1101
	}
	if !paired {
		t.Fatal("制造供给与新增库存没有不同实体之间的本地无人机供需")
	}
	for _, wanted := range ports {
		found := false
		for _, actual := range result.Actual.Ports {
			found = found || actual == wanted
		}
		if !found {
			t.Fatalf("缺少真实库存或进口回执：%+v", wanted)
		}
	}
}

// TestGenerateDirectPhotonDelivery 职责：让玩家外供光子及跨星进口都产生可交付整星蓝图；使用：纯原料目标完整入口回归；边界：外供光子不自动建造射线接收器。
func TestGenerateDirectPhotonDelivery(t *testing.T) {
	for _, kind := range []string{"external", "import"} {
		t.Run(kind, func(t *testing.T) {
			plan := quant.Result{PlanID: "photon-delivery", Targets: []quant.Target{{ItemID: 1208, RatePerMinute: 60}},
				RawInputs: []quant.ItemRate{{ItemID: 1208, RatePerMinute: 60}}, ExternalInputItemIDs: []int{1208}}
			graph, err := quant.BuildFlowGraphContext(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.FlowGraph = &graph
			ports := []industryplan.Port{{ItemID: 1208, Kind: kind, RatePerMinute: 60}, {ItemID: 1208, Kind: "delivery", RatePerMinute: 60}}
			remote := int32(0)
			if kind == "import" {
				remote, ports[0].PeerPlanetID = 2, 102
			}
			result, err := Generate(context.Background(), ProductionRequest{
				Plan:      industryplan.PlanetPlan{PlanetID: 101, Plan: plan, Ports: ports, WarpDistance: 60000},
				Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
			})
			if err != nil {
				t.Fatalf("纯光子交付没有生成独立收货蓝图：%v", err)
			}
			doc, err := blueprint.Decode(result.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Buildings) != 1 || doc.Buildings[0].ItemID != 2104 || doc.Buildings[0].Parameters[0] != 1208 || doc.Buildings[0].Parameters[2] != remote {
				t.Fatal("纯光子交付缺少正确来源的独立收货塔，或夹带了未经声明的生产设施")
			}
			if doc.Buildings[0].Parameters[324] != 60000 {
				t.Fatal("蓝图塔未采用与规划一致的曲速启用距离")
			}
			if !reflect.DeepEqual(result.Actual.Ports, ports) || result.StationStorage[1208] != int(doc.Buildings[0].Parameters[3]) {
				t.Fatalf("最终交付合同或实际库存不符：%+v", result.Actual.Ports)
			}
		})
	}
}
