package planet

import (
	"context"
	"math"
	"strings"
	"testing"

	"DysonSphereBox/helper/blueprint/build"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/quant"
)

func TestMineDronePowerUsesPhysicalPairAndFullLoads(t *testing.T) {
	req := Request{mineCoverage: []MiningCoverage{{ItemID: 1001, BuildingIndex: 0, RatePerMinute: 360}},
		localPorts: []build.BoundaryPort{{BuildingIndex: 2, ItemID: 1001, Kind: "external-input", RatePerMinute: 70},
			{BuildingIndex: 3, ItemID: 1001, Kind: "external-input", RatePerMinute: 70}},
		Objects: []Object{{}, stationObject(1, Position{}, 1001, 1, 0, false),
			stationObject(2, Position{Longitude: math.Pi / 2}, 1001, 2, 0, false),
			stationObject(3, Position{Longitude: math.Pi}, 1001, 2, 0, false)}}
	input := ProductionRequest{Options: ProductionOptions{DroneCargo: 70, DroneSpeed: 16},
		Plan: industryplan.PlanetPlan{Plan: quant.Result{Productions: []quant.Production{{Inputs: []quant.ItemRate{{ItemID: 1001, RatePerMinute: 140}}}}}}}
	var actual industryplan.PlanetActual
	accountLocalFreight(&req, input, &actual)
	want := (2*.8 + .04*226.2*math.Pi*1.5) / 60
	if math.Abs(req.dronePowerMW-want) > 1e-8 || req.Objects[1].Building.Parameters[326] != 100 ||
		req.Objects[1].Building.Parameters[330] != 0 || req.Objects[2].Building.Parameters[330] != 0 || len(actual.Warnings) != 1 {
		t.Fatalf("长航线没有按发货满载量预留：%.6f MW，期望 %.6f MW，%v", req.dronePowerMW, want, actual.Warnings)
	}
	input.Plan.Ports = []industryplan.Port{{Kind: "import", ItemID: 1001, RatePerMinute: 50}}
	req.Objects[2].Building.Parameters[2] = 2
	actual = industryplan.PlanetActual{}
	accountLocalFreight(&req, input, &actual)
	if math.Abs(req.dronePowerMW-want) > 1e-8 || len(actual.Warnings) != 1 || strings.Contains(actual.Warnings[0], "尚无法") {
		t.Fatal("直接从远端进入本地需求槽的 50 件不能再算成本星新货源，也不能抹去矿塔原有的配送预算")
	}
}

func TestCrossDockDronePowerCapsSharedContractAndSwitchesToReceivers(t *testing.T) {
	from := stationObject(0, Position{}, 1209, 1, 2, false)
	from.Module = "物料中转"
	a := stationObject(1, Position{Longitude: math.Pi}, 1209, 2, 1, false)
	b := stationObject(2, Position{Longitude: math.Pi}, 1209, 2, 1, false)
	req := Request{Objects: []Object{from, a, b}}
	input := ProductionRequest{Options: ProductionOptions{DroneCargo: 70, DroneSpeed: 16},
		Plan: industryplan.PlanetPlan{Ports: []industryplan.Port{{Kind: "import", ItemID: 1209, RatePerMinute: 202}, {Kind: "export", ItemID: 1209, RatePerMinute: 202}}}}
	var actual industryplan.PlanetActual
	accountLocalFreight(&req, input, &actual)
	want := 202.0 / 70 * (.8 + .04*math.Pi*226.2) / 60
	if math.Abs(req.dronePowerMW-want) > 1e-7 {
		t.Fatalf("两个本需远供塔共享一份出口，不应翻倍：%g != %g", req.dronePowerMW, want)
	}
	// 按固定带路，各占一半的两个下游各自配机能满足单塔 100 架的上限。
	req.Objects[1].Building.Parameters[2], req.Objects[2].Building.Parameters[2] = 0, 0
	req.localPorts = []build.BoundaryPort{{BuildingIndex: 1, ItemID: 1209, Kind: "external-input", RatePerMinute: 3500},
		{BuildingIndex: 2, ItemID: 1209, Kind: "external-input", RatePerMinute: 3500}}
	input.Plan.Ports = []industryplan.Port{{Kind: "import", ItemID: 1209, RatePerMinute: 7000}}
	actual = industryplan.PlanetActual{}
	accountLocalFreight(&req, input, &actual)
	if len(actual.Warnings) != 1 || !strings.Contains(actual.Warnings[0], "收货塔配机取货") || req.Objects[1].Building.Parameters[326] != 100 || req.Objects[2].Building.Parameters[326] != 100 {
		t.Fatalf("发货方机位不够时应验证收方派机并实际设置满载：%v", actual.Warnings)
	}
}

func TestFactoryDronePowerUsesNetTowerEdgesAndMixedDispatcher(t *testing.T) {
	first := stationObject(0, Position{}, 1104, 1, 0, false)
	mixed := stationObject(1, Position{Longitude: .5}, 1104, 2, 0, false)
	mixed.Building.Parameters[6], mixed.Building.Parameters[7], mixed.Building.Parameters[9] = 1202, 1, 1000
	last := stationObject(2, Position{Longitude: 1}, 1202, 2, 0, false)
	req := Request{Objects: []Object{first, mixed, last}, localPorts: []build.BoundaryPort{
		{BuildingIndex: 0, ItemID: 1104, Kind: "target-output", RatePerMinute: 140},
		{BuildingIndex: 1, ItemID: 1104, Kind: "external-input", RatePerMinute: 140},
		{BuildingIndex: 1, ItemID: 1202, Kind: "target-output", RatePerMinute: 70},
		{BuildingIndex: 2, ItemID: 1202, Kind: "external-input", RatePerMinute: 70},
	}}
	input := ProductionRequest{Options: ProductionOptions{DroneCargo: 70, DroneSpeed: 16}}
	var actual industryplan.PlanetActual
	accountLocalFreight(&req, input, &actual)
	want := 3 * (.8 + .04*226.2*.5) / 60
	if math.Abs(req.dronePowerMW-want) > 1e-8 || len(actual.Warnings) != 1 || !strings.Contains(actual.Warnings[0], "磁线圈 70.0") {
		t.Fatalf("两个制造边界流的耗电不符：%.6f MW，期望 %.6f，%v", req.dronePowerMW, want, actual.Warnings)
	}
	for i, o := range req.Objects {
		if o.Building.Parameters[330] != 0 || (i < 2 && o.Building.Parameters[326] != 100) {
			t.Fatalf("供货及兼收塔均需满载发机，所有端均须关闭自动补充；塔 %d 参数 %v", i, o.Building.Parameters[326:331])
		}
	}
}

func TestLocalDroneFleetMixedItemsAndHardMinimum(t *testing.T) {
	station := func(id int) []localDroneEndpoint { return []localDroneEndpoint{{tower: id, rate: 10}} }
	slots := map[int]*localCargoSlots{1001: {active: true, sources: station(1), sinks: station(2)}, 1101: {active: true, sources: station(2), sinks: station(3)}}
	active := mixedLocalDispatch(slots, map[int]float64{1: 150, 2: 37, 3: 253})
	if len(active) != 1 || !active[2] {
		t.Fatalf("两条货运均由中间可用塔负责时不必给两个端塔派机：%v", active)
	}
	from := stationObject(0, Position{}, 1202, 1, 0, false)
	to := stationObject(1, Position{Longitude: math.Pi}, 1202, 2, 0, false)
	req := Request{PlanetID: 204, Objects: []Object{from, to}, localPorts: []build.BoundaryPort{
		{BuildingIndex: 0, ItemID: 1202, Kind: "target-output", RatePerMinute: 20000},
		{BuildingIndex: 1, ItemID: 1202, Kind: "external-input", RatePerMinute: 20000}}}
	input := ProductionRequest{Options: ProductionOptions{DroneCargo: 70, DroneSpeed: 16}}
	var actual industryplan.PlanetActual
	err := accountLocalFreight(&req, input, &actual)
	if e, ok := err.(*Error); !ok || e.Code != "PLANET_LOCAL_FLEET" {
		t.Fatalf("两塔各放满100架仍达不到的本地带口，应阻止整星验收：%v", err)
	}
	req.Objects[1].Building.Parameters[2] = 2
	if err := accountLocalFreight(&req, input, &actual); err != nil {
		t.Fatalf("本需槽仍可直接远程补库时，不能把同样的本地预算当作硬下界：%v", err)
	}
}

func TestUnpoweredStationRequiresRealGridDraw(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 204, Objects: []Object{stationObject(0, Position{}, 6001, 2, 0, false)}}
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 204}}
	if err := appendPower(context.Background(), &req, input, &industryplan.PlanetActual{}, catalog); err != nil {
		t.Fatal(err)
	}
	if math.Abs(req.requiredGridMW-.06) > 1e-9 {
		t.Fatalf("独立收货塔应持续补充0.060 MW，实际%.6f MW", req.requiredGridMW)
	}
	input.Plan.OutboundVesselMW = 5
	if err := appendPower(context.Background(), &req, input, &industryplan.PlanetActual{}, catalog); err != nil {
		t.Fatal(err)
	}
	if math.Abs(req.requiredGridMW-5.06) > 1e-9 {
		t.Fatalf("远供发船和待机功耗应各计一次，实际%.6f MW", req.requiredGridMW)
	}
}
