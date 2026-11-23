package industryplan

import (
	"DysonSphereBox/helper/quant"
	"context"
	"errors"
	"math"
	"testing"
)

// TestSinglePlanetBeforeSpill 第一颗资源足够但无法施工时，应该先核验第二颗能够独立交付的星球。
func TestSinglePlanetBeforeSpill(t *testing.T) {
	req := Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}}}
	world := World{}
	for _, id := range []int64{101, 201} {
		world.Planets = append(world.Planets, Planet{ID: id, StarID: id / 100, Buildable: true, ResourcesKnown: true,
			Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 10}}})
	}
	ctx, metrics := WithLayoutMetrics(t.Context())
	r, err := Plan(ctx, req, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 {
			return PlanetActual{}, &FactoryPlacementError{FirstUnplaced: 0, Err: errors.New("样例本星没有完整落点")}
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !r.Verified || len(r.Planets) != 1 || r.Planets[0].PlanetID != 201 || len(r.Routes) != 0 || metrics.Rebuilds != 2 {
		t.Fatalf("单星优先不满足：错误 %v，中心 %d，使用星球 %d，航线 %d，实际重建 %d", err, r.CenterPlanetID, len(r.Planets), len(r.Routes), metrics.Rebuilds)
	}
	r.External = append(r.External, External{Kind: "resource-shortfall", ItemID: 1001, PlanetID: 201, RatePerMinute: 10})
	if singlePlanetDelivery(req, r, 201) {
		t.Fatal("未声明外供的矿物缺口通过了单星交付检查")
	}
}

// TestVerifiedLayoutSurvivesUnrelatedFailure 即使另一颗星先失败，旧星未变合同仍可复用；改变塔数必须重建。
func TestVerifiedLayoutSurvivesUnrelatedFailure(t *testing.T) {
	ctx, metrics := WithLayoutMetrics(t.Context())
	ctx = context.WithValue(ctx, verifiedLayoutsKey{}, map[int64]verifiedLayout{})
	validator := func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 103 {
			return PlanetActual{}, errors.New("本星落位失败")
		}
		return PlanetActual{Ports: site.Ports}, nil
	}
	kept := PlanetPlan{PlanetID: 102}
	var previous Result
	for i, sites := range [][]PlanetPlan{{{PlanetID: 101}, kept}, {{PlanetID: 103}}, {kept}, {{PlanetID: 102, StationCount: 1}}} {
		item := Result{Planets: sites}
		_, err := verifyCandidate(ctx, &item, previous, assignment{}, validator, func(Progress) {})
		if (err != nil) != (i == 1) {
			t.Fatalf("第 %d 轮核验结果有误：%v", i+1, err)
		}
		previous = item
	}
	if metrics.Rebuilds != 4 || metrics.Reuses != 1 {
		t.Fatalf("实际重建与复用记录有误：%+v", metrics)
	}
}

// TestVerifiedSwapRestoresConstruction 换位试摆后的原合同须拿回自己的蓝图，改过发船功率则必须重核。
func TestVerifiedSwapRestoresConstruction(t *testing.T) {
	base := PlanetPlan{PlanetID: 101, StationCount: 1}
	swapped := base
	swapped.StationCount = 2
	incumbent := Result{Verified: true, Planets: []PlanetPlan{base}, Actuals: map[int64]PlanetActual{101: {Artifact: "原图"}}}
	ctx, timing := WithLayoutMetrics(t.Context())
	ctx = context.WithValue(ctx, verifiedLayoutsKey{}, map[int64]verifiedLayout{101: {site: swapped, actual: PlanetActual{Artifact: "换位图"}}})
	changed := base
	changed.OutboundVesselMW = 3
	for _, trial := range []struct {
		site PlanetPlan
		want string
	}{{base, "原图"}, {swapped, "换位图"}, {changed, "新图"}} {
		item := Result{Planets: []PlanetPlan{trial.site}}
		_, err := verifyCandidate(ctx, &item, incumbent, assignment{}, func(context.Context, PlanetPlan) (PlanetActual, error) {
			return PlanetActual{Artifact: "新图"}, nil
		}, func(Progress) {})
		if err != nil || item.Actuals[101].Artifact != trial.want {
			t.Fatalf("星球 %d 的施工回执应为 %s，实际 %v，错误 %v", trial.site.PlanetID, trial.want, item.Actuals[101].Artifact, err)
		}
	}
	if timing.Reuses != 2 || timing.Rebuilds != 1 {
		t.Fatalf("期望复用两个已核验合同，重建一个实际改变的合同，得到 %+v", timing)
	}
}

// TestLogisticsPreferenceProcessesAtTheMine 验证物料收缩时比较就地加工与原矿外运，成品仍运至玩家指定的交付星。
func TestLogisticsPreferenceProcessesAtTheMine(t *testing.T) {
	request := Request{
		Quant:          quant.Request{Targets: []quant.Target{{ItemID: 1109, RatePerMinute: 300}}},
		CenterPlanetID: 201, Preference: "logistics",
	}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
			{ItemID: 1006, Kind: "solid", Count: 80, Source: "test"},
		}},
		{ID: 201, StarID: 2, Buildable: true, ResourcesKnown: true, Position: [3]float64{2400000}},
	}}
	result, err := Plan(context.Background(), request, world, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 1 || result.Routes[0].ItemID != 1109 || result.Routes[0].RatePerMinute != 300 ||
		result.CenterPlanetID != 201 || result.Candidates != 4 {
		t.Fatalf("运输偏好没有在煤矿星制造石墨并交付：状态 %s，候选 %d，运输 %+v", result.Status, result.Candidates, result.Routes)
	}
}

// TestConcentrateManufacturingWithImportedOre 验证部分矿料必须外运时，建造简单仍可选择在一个工厂完成上下游。
func TestConcentrateManufacturingWithImportedOre(t *testing.T) {
	config := Request{CenterPlanetID: 101, Preference: "construction", Quant: quant.Request{Targets: []quant.Target{{ItemID: 1301, RatePerMinute: 60}}}}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 20}}},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{30000}, Resources: []Resource{{ItemID: 1002, Kind: "solid", Count: 20}}},
	}}
	result, err := Plan(t.Context(), config, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || !result.Verified || len(result.Planets) != 2 || len(result.Routes) != 1 || result.Routes[0].ItemID != 1002 || result.Routes[0].ToPlanetID != 101 {
		t.Fatalf("未将外星铜矿送入单一制造地：%v，星球 %d，运输 %+v", err, len(result.Planets), result.Routes)
	}
	for _, site := range result.Planets {
		if (site.PlanetID == 101) != (site.Plan.MachineCount > 0) {
			t.Fatal("可在中心完成的工序分散到采矿星")
		}
	}
}

func TestCrossPlanetTransportAndRealFeedback(t *testing.T) {
	request := Request{
		Quant:          quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}, {ItemID: 1104, RatePerMinute: 30}}},
		CenterPlanetID: 201,
		Profile:        "low",
		Logistics:      LogisticsSettings{WarpDistance: 60000},
	}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
			{ItemID: 1001, Kind: "solid", Count: 40, Source: "test"},
			{ItemID: 1002, Kind: "solid", Count: 40, Source: "test"},
		}},
		{ID: 201, StarID: 2, Buildable: true, ResourcesKnown: true, Position: [3]float64{2400000, 0, 0}},
	}}
	artifact := new(int)
	result, err := Plan(context.Background(), request, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.WarpDistance != 60000 {
			t.Fatalf("逐星蓝图没有收到估算所用曲速阈值：%g", site.WarpDistance)
		}
		return PlanetActual{Ports: site.Ports, Artifact: artifact}, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conditional" || result.CenterPlanetID != 201 || result.Candidates > 8 || len(result.Routes) < 2 {
		t.Fatalf("规划结果不符合预期：状态 %s，路线 %+v，外部条件 %+v，说明 %v", result.Status, result.Routes, result.External, result.Warnings)
	}
	load := 0.0
	for _, route := range result.Routes {
		if !route.Interstellar || route.Distance != 2400000 || route.RoundTripSeconds <= 0 || route.WarperRatePerMinute <= 0 {
			t.Fatalf("路线字段错误：%+v", route)
		}
		load += route.VesselLoad
	}
	vessels := 0
	for _, site := range result.Planets {
		vessels += site.VesselCount
		for _, cargo := range site.OutboundFleet {
			var highest float64
			for _, route := range result.Routes {
				if route.FromPlanetID == site.PlanetID && route.ItemID == cargo.ItemID {
					highest = max(highest, route.VesselEnergyMJ/route.RoundTripSeconds)
				}
			}
			if highest <= 0 || math.Abs(cargo.MaxVesselMW-highest) > 1e-9 {
				t.Fatalf("发货星的物品 %d 缺少逐塔充电核验所用的实际航线功率", cargo.ItemID)
			}
		}
		if result.Actuals[site.PlanetID].Artifact != artifact {
			t.Fatal("获选方案丢失复核产物")
		}
	}
	if vessels != int(math.Ceil(load)) {
		t.Fatalf("共用船队数量 %d，需求 %g", vessels, load)
	}
}

func TestResourceEvidenceStates(t *testing.T) {
	request := Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}}}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true}}}
	result, err := Plan(context.Background(), request, world, nil, nil)
	if err != nil || result.Status != "missing_data" || !hasExternalKind(result, "resource-data") {
		t.Fatalf("资源缺数据应显式返回：状态 %s，外供 %+v，错误 %v", result.Status, result.External, err)
	}
	world.Planets[0].ResourcesKnown = true
	result, err = Plan(context.Background(), request, world, nil, nil)
	if err != nil || result.Status != "infeasible" {
		t.Fatalf("已知无矿应返回不可行：状态 %s，错误 %v", result.Status, err)
	}
	disabled := false
	request.EnableMining = &disabled
	result, err = Plan(context.Background(), request, world, nil, nil)
	if err != nil || result.Status != "conditional" || !hasExternalKind(result, "mining-disabled") {
		t.Fatalf("关闭矿业应列外供：状态 %s，外供 %+v，错误 %v", result.Status, result.External, err)
	}
}

// TestAutomaticSurplusStorageUsesItsProductionPlanet 核对自动仓在各余料来源按同一缓冲时长备仓，节省只为入库的星际运输。
func TestAutomaticSurplusStorageUsesItsProductionPlanet(t *testing.T) {
	c := candidate{
		req: Request{Preference: "energy", StorageMinutes: 60}, storage: 101, ids: []int64{101, 201},
		world: map[int64]*location{
			101: {planet: Planet{ID: 101, Buildable: true}, balance: map[int]float64{1114: 25}},
			201: {planet: Planet{ID: 201, Buildable: true}, balance: map[int]float64{1114: 50}},
		},
	}
	c.allocateSurplus(1114)
	for _, loc := range c.world {
		want := loc.balance[1114]
		if len(loc.plan.Ports) != 1 || loc.plan.Ports[0].Kind != "storage" || loc.plan.Ports[0].RatePerMinute != want || loc.plan.Ports[0].StorageItems != int(60*want) {
			t.Fatalf("星球 %d 的本地仓储不符：%+v", loc.planet.ID, loc.plan.Ports)
		}
	}
	if len(c.result.Routes) != 0 || c.failure != "" {
		t.Fatalf("自动入库多出航线或缺口：%+v %s", c.result.Routes, c.failure)
	}
}

func TestCoproductDeliveryBeforeStorage(t *testing.T) {
	disabled := false
	request := Request{
		Quant:           quant.Request{Targets: []quant.Target{{ItemID: 1114, RatePerMinute: 60}, {ItemID: 1120, RatePerMinute: 10}}},
		CenterPlanetID:  101,
		StoragePlanetID: 102,
		EnableMining:    &disabled,
	}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{10000, 0, 0}},
	}}
	result, err := Plan(context.Background(), request, world, nil, nil)
	if err != nil || result.Status != "conditional" {
		t.Fatalf("联产规划失败：状态 %s，说明 %v，错误 %v", result.Status, result.Warnings, err)
	}
	for _, site := range result.Planets {
		for _, port := range site.Ports {
			if port.Kind == "storage" && port.ItemID == 1120 {
				if site.PlanetID != 102 || port.RatePerMinute != 20 || port.StorageItems != 1200 {
					t.Fatalf("联产应优先扣除氢目标再入库：星球 %d，端口 %+v", site.PlanetID, port)
				}
				return
			}
		}
	}
	t.Fatalf("未规划氢副产物仓储：全局剩余 %+v", result.Plan.Leftovers)
}

func TestFeedbackReuseAndCancellation(t *testing.T) {
	kept := PlanetPlan{PlanetID: 101, Ports: []Port{{ItemID: 1001, Kind: "external", RatePerMinute: 30}}}
	failed := PlanetPlan{PlanetID: 102, Ports: []Port{{ItemID: 1101, Kind: "delivery", RatePerMinute: 30}}}
	artifact := new(int)
	previous := Result{Planets: []PlanetPlan{kept}, Actuals: map[int64]PlanetActual{101: {Ports: kept.Ports, Artifact: artifact}}}
	current := Result{Planets: []PlanetPlan{kept, failed}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	_, err := verifyCandidate(ctx, &current, previous, assignment{}, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		calls++
		if site.PlanetID != 102 {
			t.Fatalf("未变化星球重复生成：%d", site.PlanetID)
		}
		cancel()
		return PlanetActual{}, ctx.Err()
	}, func(Progress) {})
	if calls != 1 || !errors.Is(err, context.Canceled) || current.Actuals[101].Artifact != artifact {
		t.Fatalf("实际结果复用或取消有误：调用 %d，错误 %v", calls, err)
	}
}

func TestLocalRepairMovesManufacturingKeepsDelivery(t *testing.T) {
	request := Request{
		Quant:          quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}},
		CenterPlanetID: 101,
		Profile:        "low",
	}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 10}}},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{5000, 0, 0}},
	}}
	result, err := Plan(context.Background(), request, world, func(_ context.Context, site PlanetPlan) (PlanetActual, error) {
		if site.PlanetID == 101 && len(site.Plan.Productions) > 0 {
			return PlanetActual{}, errors.New("该星球放不下制造线")
		}
		return PlanetActual{Ports: site.Ports}, nil
	}, nil)
	if err != nil || result.Status != "conditional" || result.Repairs != 1 || result.CenterPlanetID != 101 {
		t.Fatalf("修正后应保留中心，状态 %s、修正次数 %d、错误 %v、说明 %v", result.Status, result.Repairs, err, result.Warnings)
	}
	if len(result.Planets) != 2 || len(result.Planets[0].Plan.Productions) != 0 || len(result.Planets[1].Plan.Productions) == 0 {
		t.Fatal("修正未将生产移到邻星")
	}
	for _, port := range result.Planets[0].Ports {
		if port.ItemID == 1101 && port.Kind == "delivery" && port.RatePerMinute == 60 {
			return
		}
	}
	t.Fatal("定点修正丢失中心净交付")
}

func hasExternalKind(result Result, kind string) bool {
	for _, external := range result.External {
		if external.Kind == kind {
			return true
		}
	}
	return false
}

func TestThermalPowerAndFuelSwitches(t *testing.T) {
	req := Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 60}}}}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true, Resources: []Resource{
		{ItemID: 1001, Kind: "solid", Count: 100}, {ItemID: 1006, Kind: "solid", Count: 100},
	}}}}
	grid, err := Plan(context.Background(), req, world, nil, nil)
	if err != nil || !hasExternalKind(grid, "grid-power") || grid.Planets[0].Power != nil {
		t.Fatalf("默认应明列外接电网，错误 %v，方案 %+v", err, grid.External)
	}
	req.EnablePower = true
	external, err := Plan(context.Background(), req, world, nil, nil)
	if err != nil || !hasExternalKind(external, "fuel-input") || external.Planets[0].Power == nil {
		t.Fatalf("火电外供燃料计算错误：%v，%+v", err, external.External)
	}
	power := external.Planets[0].Power
	if power.FacilityID != 2204 || power.FuelItemID != 1006 || power.PowerMW < power.RequiredMW || power.FuelRatePerMinute != float64(power.Count*60) {
		t.Fatalf("火电煤热值和额定功率不一致：%+v", power)
	}
	req.EnableFuel = true
	local, err := Plan(context.Background(), req, world, nil, nil)
	if err != nil || hasExternalKind(local, "fuel-input") || local.Planets[0].Power == nil {
		t.Fatalf("自采火电燃料不应重复算外供：%v，%+v", err, local.External)
	}
	for _, mine := range local.Planets[0].Mines {
		if mine.ItemID == 1006 && mine.RatePerMinute == power.FuelRatePerMinute {
			return
		}
	}
	t.Fatal("煤矿外采缺少新的火电负荷")
}

func TestFinalVerificationRejectsMissingPowerOrStorage(t *testing.T) {
	power := &PowerPlan{FacilityID: 2204, FuelItemID: 1006, Count: 2, FuelRatePerMinute: 120, PowerMW: 4.32, RequiredMW: 3}
	storage := Port{ItemID: 1120, Kind: "storage", RatePerMinute: 20, StorageItems: 1200}
	site := PlanetPlan{PlanetID: 101, Power: power, Ports: []Port{storage}}
	if err := verifyPorts(site, PlanetActual{Ports: site.Ports}); err == nil {
		t.Fatal("没有真实电站不能通过")
	}
	limited := storage
	limited.StorageItems--
	if err := verifyPorts(site, PlanetActual{Power: power, Ports: []Port{limited}}); err == nil {
		t.Fatal("库存少一件时不能通过")
	}
	if err := verifyPorts(site, PlanetActual{Power: power, Ports: site.Ports}); err != nil {
		t.Fatal(err)
	}
}

func TestSharedShipLimitAndDisabledTransport(t *testing.T) {
	req := Request{
		Quant:             quant.Request{Targets: []quant.Target{{ItemID: 1101, RatePerMinute: 30000}}},
		CenterPlanetID:    101,
		DeliveryPlanetIDs: map[int]int64{1101: 101},
		Logistics:         LogisticsSettings{VesselsPerStation: 1, MaxStationsPerPlanet: 1},
	}
	world := World{Planets: []Planet{
		{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true},
		{ID: 102, StarID: 1, Buildable: true, ResourcesKnown: true, Position: [3]float64{50000, 0, 0},
			Resources: []Resource{{ItemID: 1001, Kind: "solid", Count: 2000}}},
	}}
	result, err := Plan(context.Background(), req, world, nil, nil)
	if err != nil || result.Status != "infeasible" {
		t.Fatalf("共享船队达到上限后应报不可行，状态 %s，错误 %v", result.Status, err)
	}
	disabled := false
	req.EnableLogistics = &disabled
	result, err = Plan(context.Background(), req, world, nil, nil)
	if err != nil || result.Status != "conditional" || len(result.Routes) == 0 {
		t.Fatalf("关闭自动运输应保留外运条件，状态 %s，错误 %v", result.Status, err)
	}
	for _, external := range result.External {
		if external.Kind == "transport" && external.PlanetID == 102 && external.PeerPlanetID == 101 {
			return
		}
	}
	t.Fatalf("未记录手动运输的两端：%+v", result.External)
}
