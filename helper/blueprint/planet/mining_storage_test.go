package planet

// 文件职责：验证制造与进口并存时真实矿源余料的仓储；使用：煤矿仓储回执遗漏回归；边界：不重复承诺流量或同一塔槽库存。

import (
	"context"
	"errors"
	"math"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

// TestMiningStorageWithManufacturing 职责：从真实矿机、制造和进口生成有余料库存的整星蓝图；使用：完整入口及严格解码；边界：仓储复用出矿带直连塔。
func TestMiningStorageWithManufacturing(t *testing.T) {
	input := miningStorageInput(t)
	result, err := Generate(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := blueprint.Decode(result.Blueprint, true)
	if err != nil {
		t.Fatal(err)
	}
	if document.Summary.ItemCounts[2301] != 1 || document.Summary.ItemCounts[2302] != 1 {
		t.Fatalf("缺少真实矿源或制造设备: %v", document.Summary.ItemCounts)
	}
	var storage industryplan.Port
	for _, port := range result.Actual.Ports {
		if port.ItemID == 1001 && port.Kind == "storage" {
			storage = port
		}
	}
	if storage.RatePerMinute != 60 || storage.StorageItems != 7200 {
		t.Fatalf("净余60没有真实库存回执: %+v", storage)
	}
	byIndex := map[int32]blueprint.Building{}
	var miner blueprint.Building
	for _, b := range document.Buildings {
		byIndex[b.Index] = b
		if b.ItemID == 2301 {
			miner = b
		}
	}
	var belt blueprint.Building
	for _, b := range document.Buildings {
		if b.ItemID >= 2001 && b.ItemID <= 2003 && b.InputObjIdx == miner.Index {
			belt = b
			break
		}
	}
	if belt.ItemID == 0 {
		t.Fatal("矿机没有真实出矿带")
	}
	for steps := 0; steps < len(document.Buildings); steps++ {
		next, ok := byIndex[belt.OutputObjIdx]
		if !ok {
			t.Fatal("出矿带没有连接库存塔")
		}
		if next.ItemID == 2104 {
			if next.Parameters[0] != 1001 || next.Parameters[1] != 1 || int(next.Parameters[3]) < storage.StorageItems {
				t.Fatalf("仓储不在矿源真实供给塔: %v", next.Parameters[:6])
			}
			return
		}
		if next.ItemID < 2001 || next.ItemID > 2003 {
			t.Fatalf("出矿路径连接到非带实体 %d", next.ItemID)
		}
		belt = next
	}
	t.Fatal("出矿带形成环，未证明仓储")
}

// TestMiningStorageMissingSourceRejected 职责：缺矿源或制造所需进口时不伪造余料；使用：完整入口负例；边界：最终边界缺口保持失败。
func TestMiningStorageMissingSourceRejected(t *testing.T) {
	for _, missing := range []string{"mining", "import"} {
		t.Run(missing, func(t *testing.T) {
			input := miningStorageInput(t)
			if missing == "mining" {
				input.Plan.Mines = nil
			} else {
				ports := input.Plan.Ports[:0]
				for _, port := range input.Plan.Ports {
					if port.Kind != "import" {
						ports = append(ports, port)
					}
				}
				input.Plan.Ports = ports
			}
			_, err := Generate(context.Background(), input)
			var issue *Error
			if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" || issue.Item != 1001 {
				t.Fatalf("缺少%s仍获得完整仓储或错误不明确: %v", missing, err)
			}
		})
	}
}

// TestMiningStorageCapacityNotDuplicated 职责：多个本地库存合同共同消耗真实塔槽容量；使用：重复库存负例；边界：不足部分不得用同一容量再次通过终检。
func TestMiningStorageCapacityNotDuplicated(t *testing.T) {
	input := miningPartialInput(90)
	input.Plan.Ports = []industryplan.Port{
		{ItemID: 1001, Kind: "mining", RatePerMinute: 90},
		{ItemID: 1001, Kind: "storage", RatePerMinute: 60, StorageItems: 7000},
		{ItemID: 1001, Kind: "delivery", RatePerMinute: 30, StorageItems: 7000},
	}
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
	var coverage []MiningCoverage
	if err = appendMining(context.Background(), &req, input, &actual, &coverage, catalog); err != nil {
		t.Fatal(err)
	}
	claimed := 0
	for _, port := range actual.Ports {
		claimed += port.StorageItems
	}
	if claimed > int(req.Objects[1].Building.Parameters[3]) {
		t.Fatalf("重复分配库存: %d", claimed)
	}
	_, err = Generate(context.Background(), input)
	var issue *Error
	if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" {
		t.Fatalf("不足库存通过终检: %v", err)
	}
}

// TestMiningStockSurplusReservations 职责：同时保留制造净耗、全部出口、多燃料和已有交付；使用：库存余额回归；边界：来源未实际采集不能作为矿源库存。
func TestMiningStockSurplusReservations(t *testing.T) {
	plan := industryplan.PlanetPlan{Plan: quant.Result{Productions: []quant.Production{{Inputs: []quant.ItemRate{{ItemID: 1006, RatePerMinute: 100}}, Outputs: []quant.ItemRate{{ItemID: 1006, RatePerMinute: 20}}}}},
		Ports: []industryplan.Port{{ItemID: 1006, Kind: "import", RatePerMinute: 60}, {ItemID: 1006, Kind: "external", RatePerMinute: 40},
			{ItemID: 1006, Kind: "export", RatePerMinute: 40}, {ItemID: 1006, Kind: "external-export", RatePerMinute: 20}, {ItemID: 1006, Kind: "power-fuel", RatePerMinute: 30}},
		Power: &industryplan.PowerPlan{FuelItemID: 1006, FuelRatePerMinute: 99, Sources: []industryplan.PowerSource{{FuelItemID: 1006, FuelRatePerMinute: 20}, {FuelItemID: 1006, FuelRatePerMinute: 10}, {FuelItemID: 1803, FuelRatePerMinute: 100}}}}
	actual := industryplan.PlanetActual{Ports: []industryplan.Port{{ItemID: 1006, Kind: "mining", RatePerMinute: 100}, {ItemID: 1006, Kind: "storage", RatePerMinute: 5}, {ItemID: 1006, Kind: "delivery", RatePerMinute: 5}}}
	if got := miningStockSurplus(plan, actual, nil, 1006); math.Abs(got-20) > 1e-8 {
		t.Fatalf("净余%g，期望20", got)
	}
	plan.Power.Sources = nil
	plan.Power.FuelRatePerMinute = 40
	if got := miningStockSurplus(plan, actual, nil, 1006); math.Abs(got-10) > 1e-8 {
		t.Fatalf("旧供电合同净余%g，期望10", got)
	}
}

// TestMiningStockSurplusPriorExport 职责：保留前序已完成出口但不复用其已知来源；使用：增量矿区分配；边界：本次矿区的出口回执不属于前序来源。
func TestMiningStockSurplusPriorExport(t *testing.T) {
	for _, source := range []string{"independent", "mining", "import", "manufacturing"} {
		t.Run(source, func(t *testing.T) {
			plan := industryplan.PlanetPlan{Ports: []industryplan.Port{{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: 100}}}
			prior := []industryplan.Port{{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: 50}}
			switch source {
			case "mining":
				prior = append(prior, industryplan.Port{ItemID: 1001, Kind: "mining", RatePerMinute: 50})
			case "import":
				plan.Ports = append(plan.Ports, industryplan.Port{ItemID: 1001, Kind: "import", PeerPlanetID: 102, RatePerMinute: 50})
			case "manufacturing":
				plan.Plan.Productions = []quant.Production{{Outputs: []quant.ItemRate{{ItemID: 1001, RatePerMinute: 50}}}}
			}
			actual := industryplan.PlanetActual{Ports: append(append([]industryplan.Port(nil), prior...), industryplan.Port{ItemID: 1001, Kind: "mining", RatePerMinute: 70})}
			addActualPort(&actual, industryplan.Port{ItemID: 1001, Kind: "export", PeerPlanetID: 103, RatePerMinute: 100})
			if got := miningStockSurplus(plan, actual, prior, 1001); math.Abs(got-20) > 1e-8 {
				t.Fatalf("前序%s出口与本次矿源重复用量：净余%g，期望20", source, got)
			}
		})
	}
}

// miningStorageInput 职责：提供90采矿加30进口、60制造消耗和60余料的单炉计划；使用：完整仓储回归；边界：实际矿位和接驳复用原有小型矿区样例。
func miningStorageInput(t *testing.T) ProductionRequest {
	t.Helper()
	plan, err := quant.Solve(context.Background(), quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}})
	if err != nil {
		t.Fatal(err)
	}
	plan.RawInputs = []quant.ItemRate{{ItemID: 1001, RatePerMinute: 90}, {ItemID: 1001, RatePerMinute: 30}}
	plan.Leftovers = []quant.ItemRate{{ItemID: 1001, RatePerMinute: 60}}
	plan.ExternalInputItemIDs = []int{1001}
	plan.FlowGraph = nil
	graph, err := quant.BuildFlowGraphContext(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.FlowGraph = &graph
	input := miningPartialInput(90)
	input.Plan.Plan = plan
	input.Plan.Ports = []industryplan.Port{{ItemID: 1001, Kind: "mining", RatePerMinute: 90}, {ItemID: 1001, Kind: "import", PeerPlanetID: 102, RatePerMinute: 30},
		{ItemID: 1001, Kind: "storage", RatePerMinute: 60, StorageItems: 7200}, {ItemID: 1101, Kind: "delivery", RatePerMinute: 60}}
	power := false
	input.Options = ProductionOptions{Profile: "low", Setting: build.Setting{NeedPower: &power}}
	return input
}
