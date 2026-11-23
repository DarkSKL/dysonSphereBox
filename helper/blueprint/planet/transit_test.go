package planet

// 文件职责：验证跨星及玩家外供的真实双塔中转；使用：端口余量回归；边界：严格编解码证明实体与供需角色，不声称无人机数量已最优。

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"DysonSphereBox/helper/blueprint"
	"DysonSphereBox/helper/blueprint/build/placement"
	"DysonSphereBox/helper/industryplan"
	"DysonSphereBox/helper/saveanalysis"
)

// TestTransitBuildsRealPairedStations 职责：为远程和外部直通提供独立收发实体；使用：两塔与混合来源回归；边界：同向多对端共槽，反向不同塔，带口和库存不冒充吞吐。
func TestTransitBuildsRealPairedStations(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		ports []industryplan.Port
		roles map[[2]int32]int
	}{
		{"跨星多对端", []industryplan.Port{
			{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 100},
			{ItemID: 1209, Kind: "import", PeerPlanetID: 103, RatePerMinute: 88},
			{ItemID: 1209, Kind: "export", PeerPlanetID: 201, RatePerMinute: 140},
			{ItemID: 1209, Kind: "export", PeerPlanetID: 202, RatePerMinute: 48},
		}, map[[2]int32]int{{1, 2}: 1, {2, 1}: 1}},
		{"外供后出口", []industryplan.Port{
			{ItemID: 1209, Kind: "external", RatePerMinute: 60},
			{ItemID: 1209, Kind: "export", PeerPlanetID: 102, RatePerMinute: 60},
		}, map[[2]int32]int{{1, 0}: 1, {2, 1}: 1}},
		{"进口后外送", []industryplan.Port{
			{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 60},
			{ItemID: 1209, Kind: "external-export", RatePerMinute: 60},
		}, map[[2]int32]int{{1, 2}: 1, {2, 0}: 1}},
		{"混合外供与远程", []industryplan.Port{
			{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 40},
			{ItemID: 1209, Kind: "external", RatePerMinute: 20},
			{ItemID: 1209, Kind: "export", PeerPlanetID: 103, RatePerMinute: 30},
			{ItemID: 1209, Kind: "external-export", RatePerMinute: 30},
		}, map[[2]int32]int{{1, 2}: 1, {1, 0}: 1, {2, 1}: 1, {2, 0}: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
			input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Ports: tc.ports}}
			if err := appendTransit(ctx, &req, input, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual.Ports, tc.ports) {
				t.Fatalf("没有按对端准确回填中转余量：%+v", actual.Ports)
			}
			built, err := Build(ctx, req)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := blueprint.Decode(built.Blueprint, true)
			if err != nil {
				t.Fatal(err)
			}
			roles := map[[2]int32]int{}
			for _, station := range doc.Buildings {
				assertFleetTowerHasNoBelts(t, station)
				if station.Parameters[0] != 1209 || station.Parameters[3] <= 0 {
					t.Fatalf("中转塔没有真实物品槽：%v", station.Parameters[:6])
				}
				roles[[2]int32{station.Parameters[1], station.Parameters[2]}]++
			}
			if !reflect.DeepEqual(roles, tc.roles) || built.StationCount != len(tc.roles) || !slices.Contains(built.LocalDroneItems, 1209) || built.LocalModes[1209] != 3 {
				t.Fatalf("中转缺少真实双塔本地供需或远程角色错误：角色 %v，结果 %+v", roles, built)
			}
			assertFleetStationSpacing(t, req.Objects)
			if err := appendTransit(ctx, &req, input, &actual, catalog); err != nil || len(req.Objects) != len(tc.roles) || !reflect.DeepEqual(actual.Ports, tc.ports) {
				t.Fatalf("重复调用增加了塔或流量：%v，%+v", err, actual.Ports)
			}
		})
	}
}

// TestTransitCreditsOnlyUncoveredRates 职责：抵扣已覆盖的制造或能源端口后再中转；使用：同物品部分消费、部分流通回归；边界：既有设备和采矿、燃料、交付、仓储回执均保留原值。
func TestTransitCreditsOnlyUncoveredRates(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	req := Request{PlanetID: 101, Objects: []Object{
		stationObject(0, Position{Longitude: -1}, 1209, 2, 2, false),
		stationObject(1, Position{Longitude: -.5}, 1209, 1, 1, false),
	}}
	oldObjects := slices.Clone(req.Objects)
	actual := industryplan.PlanetActual{Ports: []industryplan.Port{
		{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 40, StorageItems: 7},
		{ItemID: 1209, Kind: "import", PeerPlanetID: 103, RatePerMinute: 10},
		{ItemID: 1209, Kind: "export", PeerPlanetID: 201, RatePerMinute: 50, StorageItems: 5},
		{ItemID: 1001, Kind: "mining", RatePerMinute: 30},
		{ItemID: 1209, Kind: "power-fuel", RatePerMinute: 10},
		{ItemID: 1209, Kind: "delivery", RatePerMinute: 20, StorageItems: 500},
		{ItemID: 1209, Kind: "storage", RatePerMinute: 30, StorageItems: 10000},
	}}
	unrelated := slices.Clone(actual.Ports[3:])
	input := ProductionRequest{Plan: industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
		{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: 100, StorageItems: 77},
		{ItemID: 1209, Kind: "import", PeerPlanetID: 103, RatePerMinute: 88},
		{ItemID: 1209, Kind: "export", PeerPlanetID: 201, RatePerMinute: 120, StorageItems: 55},
		{ItemID: 1209, Kind: "export", PeerPlanetID: 202, RatePerMinute: 68},
		{ItemID: 1001, Kind: "mining", RatePerMinute: 50},
		{ItemID: 1209, Kind: "power-fuel", RatePerMinute: 40},
		{ItemID: 1209, Kind: "delivery", RatePerMinute: 40, StorageItems: 1000},
		{ItemID: 1209, Kind: "storage", RatePerMinute: 60, StorageItems: 20000},
	}}}
	if err := appendTransit(context.Background(), &req, input, &actual, catalog); err != nil {
		t.Fatal(err)
	}
	if len(req.Objects) != 4 || !reflect.DeepEqual(req.Objects[:2], oldObjects) || len(actual.Ports) != 8 || !reflect.DeepEqual(actual.Ports[3:7], unrelated) {
		t.Fatalf("中转改动了既有设施或无关能力：塔 %d，回执 %+v", len(req.Objects), actual.Ports)
	}
	for i, want := range input.Plan.Ports[:4] {
		got := actual.Ports[i]
		if i == 3 {
			got = actual.Ports[7]
		}
		want.StorageItems = []int{7, 0, 5, 0}[i]
		if got != want {
			t.Fatalf("部分流量被重复累计或库存被扩大：得到 %+v，期望 %+v", got, want)
		}
	}
	built, err := Build(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := blueprint.Decode(built.Blueprint, true); err != nil {
		t.Fatal(err)
	}
	assertFleetStationSpacing(t, req.Objects)
}

// TestTransitLeavesUnmatchedDemandForFinalValidation 职责：把不守恒的余量和未完成的能力留给终检拒绝；使用：避免中转掩盖生产、电厂或仓储缺口；边界：无库存承诺的塔槽不自动计为仓储回执。
func TestTransitLeavesUnmatchedDemandForFinalValidation(t *testing.T) {
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name             string
		incoming, export float64
		extra            *industryplan.Port
		transit          bool
	}{
		{"制造消耗未接通", 100, 60, nil, false},
		{"制造产出未接通", 60, 100, nil, false},
		{"电厂仍缺带口", 60, 60, &industryplan.Port{ItemID: 1209, Kind: "power-fuel", RatePerMinute: 40}, true},
		{"仓储仍缺库存", 60, 60, &industryplan.Port{ItemID: 1209, Kind: "storage", StorageItems: 20000}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := ProductionRequest{
				Plan: industryplan.PlanetPlan{PlanetID: 101, Ports: []industryplan.Port{
					{ItemID: 1209, Kind: "import", PeerPlanetID: 102, RatePerMinute: tc.incoming},
					{ItemID: 1209, Kind: "export", PeerPlanetID: 103, RatePerMinute: tc.export},
				}},
				Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
			}
			if tc.extra != nil {
				input.Plan.Ports = append(input.Plan.Ports, *tc.extra)
			}
			req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
			if err := appendTransit(context.Background(), &req, input, &actual, catalog); err != nil {
				t.Fatal(err)
			}
			wantCount := 0
			if tc.transit {
				wantCount = 2
			}
			if len(req.Objects) != wantCount || len(actual.Ports) != wantCount {
				t.Fatalf("错误填补不守恒或其他类型端口：塔 %d，回执 %+v", len(req.Objects), actual.Ports)
			}
			for _, port := range actual.Ports {
				if port.Kind != "import" && port.Kind != "export" || port.StorageItems != 0 {
					t.Fatalf("中转冒充未建造能力：%+v", port)
				}
			}
			generated, err := Generate(context.Background(), input)
			var issue *Error
			if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" || issue.Item != 1209 || generated.Blueprint != "" {
				t.Fatalf("最终验证未拒绝缺口：%v，蓝图 %q", err, generated.Blueprint)
			}
		})
	}
}

// TestTransitAfterPartialArtificialStarFuel 职责：将一次进口拆为本地恒星燃料与继续出口；使用：真实电厂装配及完整生成回归；边界：进口只记一次，接通燃料带不会凭空提供合同库存。
func TestTransitAfterPartialArtificialStarFuel(t *testing.T) {
	ctx := context.Background()
	catalog, err := placement.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	input := ProductionRequest{
		Plan: industryplan.PlanetPlan{PlanetID: 101,
			Power: &industryplan.PowerPlan{FacilityID: 2210, FuelItemID: 1803, Count: 1, PowerMW: 72, FuelRatePerMinute: .6},
			Ports: []industryplan.Port{
				{ItemID: 1803, Kind: "import", PeerPlanetID: 102, RatePerMinute: 10.6, StorageItems: 1000},
				{ItemID: 1803, Kind: "power-fuel", RatePerMinute: .6},
				{ItemID: 1803, Kind: "export", PeerPlanetID: 103, RatePerMinute: 10},
			}},
		Resources: saveanalysis.PlanetResourceDetail{Planet: saveanalysis.PlanetResourceSummary{PlanetID: 101}, Terrain: dryTerrain()},
	}
	check := func(built Result, actual industryplan.PlanetActual) {
		t.Helper()
		want := map[string]industryplan.Port{}
		for _, port := range input.Plan.Ports {
			port.StorageItems = 0
			want[port.Kind] = port
		}
		if len(actual.Ports) != len(want) {
			t.Fatalf("部分燃料进口漏记或重复记账：%+v", actual.Ports)
		}
		for _, port := range actual.Ports {
			if port != want[port.Kind] {
				t.Fatalf("进口应为 10.6，其中本地燃料 0.6、中转出口 10，且不承诺库存：%+v", actual.Ports)
			}
		}
		if actual.Power == nil || actual.Power.Count != 1 || actual.Power.PowerMW != 72 || actual.Power.FuelRatePerMinute != .6 {
			t.Fatalf("实际人造恒星供电合同错误：%+v", actual.Power)
		}
		doc, err := blueprint.Decode(built.Blueprint, true)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Summary.ItemCounts[2210] != 1 || doc.Summary.ItemCounts[2104] != 3 || doc.Summary.ItemCounts[2001] != 8 || doc.Summary.ItemCounts[2013] != 1 || built.LocalModes[1803] != 3 || !slices.Contains(built.LocalDroneItems, 1803) {
			t.Fatalf("恒星、燃料输入带或双塔中转没有真实落位：设施 %+v，本地物流 %v", doc.Summary.ItemCounts, built.LocalModes)
		}
		roles := map[[2]int32]int{}
		for _, building := range doc.Buildings {
			if building.ItemID == 2104 {
				if building.Parameters[0] != 1803 {
					t.Fatalf("燃料塔配置了错误物品：%v", building.Parameters[:6])
				}
				roles[[2]int32{building.Parameters[1], building.Parameters[2]}]++
			}
		}
		if !reflect.DeepEqual(roles, map[[2]int32]int{{2, 2}: 1, {1, 2}: 1, {2, 1}: 1}) {
			t.Fatalf("本地燃料和中转的塔角色错误：%v", roles)
		}
	}

	req, actual := Request{PlanetID: 101}, industryplan.PlanetActual{}
	if err := appendPower(ctx, &req, input, &actual, catalog); err != nil {
		t.Fatal(err)
	}
	built, err := Build(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	check(built, actual)
	assertFleetStationSpacing(t, req.Objects)

	// 完整入口不得把燃料/中转塔的槽位自动算作已承诺库存。
	if _, err := Generate(ctx, input); err == nil {
		t.Fatal("没有独立库存回执仍通过了额外库存合同")
	} else {
		var issue *Error
		if !errors.As(err, &issue) || issue.Code != "PLANET_PORT" || issue.Item != 1803 {
			t.Fatalf("库存合同因非端口原因失败：%v", err)
		}
	}
	input.Plan.Ports[0].StorageItems = 0
	generated, err := Generate(ctx, input)
	if err != nil {
		t.Fatalf("部分燃料进口与中转未通过完整生成：%v", err)
	}
	check(generated.Result, generated.Actual)
}
