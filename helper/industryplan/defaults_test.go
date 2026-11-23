package industryplan

// 文件职责：核对规划默认值的用户优先级与真实量化效果；使用：硅直采和能耗偏好回归；边界：不代替矿点落位与资源总量核验。

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"testing"

	"DysonSphereBox/helper/quant"
)

// TestWarpDistanceMatchesBlueprintParameter 职责：保持估算与整数塔参数一致；使用：默认、原版下界和编码边界回归；边界：零保留既有缺省语义。
func TestWarpDistanceMatchesBlueprintParameter(t *testing.T) {
	for _, tc := range []struct{ input, want float64 }{{0, 120000}, {20000, 20000}, {60000.25, 60000}, {math.MaxInt32, math.MaxInt32}} {
		req, profile, err := normalize(Request{Logistics: LogisticsSettings{WarpDistance: tc.input}})
		if err != nil || req.Logistics.WarpDistance != tc.want || profile.WarpDistance != tc.want {
			t.Fatalf("曲速启用距离归一化不一致：输入%g，设置%g，船运%g，错误%v", tc.input, req.Logistics.WarpDistance, profile.WarpDistance, err)
		}
	}
	for _, input := range []float64{1, 19999, float64(math.MaxInt32) + 1, math.NaN(), math.Inf(1)} {
		if _, _, err := normalize(Request{Logistics: LogisticsSettings{WarpDistance: input}}); err == nil {
			t.Fatalf("接受了不符合原版塔参数的曲速距离：%g", input)
		}
	}
}

// TestPlanningDefaultsSiliconSources 职责：只在矿源可核验或可精算时追加硅直采；使用：默认来源边界；边界：显式设置不被覆盖，其他稀有矿不自动加入。
func TestPlanningDefaultsSiliconSources(t *testing.T) {
	disabled, exhausted := false, 0.0
	for _, tc := range []struct {
		name       string
		resource   Resource
		prepare    func(*Request, *Planet)
		canResolve bool
		want       bool
	}{
		{name: "known", resource: Resource{ItemID: 1003, Kind: "solid", Source: "saved", Count: 2}, want: true},
		{name: "candidate", resource: Resource{ItemID: 1003, Kind: "solid", Source: "candidate"}, canResolve: true, want: true},
		{name: "candidate_without_resolver", resource: Resource{ItemID: 1003, Kind: "solid", Source: "candidate"}},
		{name: "checked_unknown_candidate", resource: Resource{ItemID: 1003, Kind: "solid", Source: "candidate"}, canResolve: true, prepare: func(_ *Request, p *Planet) { p.resourcesChecked = true }},
		{name: "empty", resource: Resource{ItemID: 1003, Kind: "solid", Source: "saved"}},
		{name: "exhausted", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2, RemainingItems: &exhausted}},
		{name: "unbuildable", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2}, prepare: func(_ *Request, p *Planet) { p.Buildable = false }},
		{name: "mining_disabled", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2}, prepare: func(r *Request, _ *Planet) { r.EnableMining = &disabled }},
		{name: "manual_recipe", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2}, prepare: func(r *Request, _ *Planet) { r.Quant.Scheme.ItemRecipeChoices = map[string]int{"硅石": 1} }},
		{name: "explicit_external", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2}, prepare: func(r *Request, _ *Planet) { r.Quant.Settings.MineralizeList = map[string]bool{"硅石": true} }},
		{name: "existing_direct", resource: Resource{ItemID: 1003, Kind: "solid", Count: 2}, want: true, prepare: func(r *Request, _ *Planet) { r.Quant.Settings.DirectResourceItemIDs = []int{1003} }},
		{name: "other_rare_ore", resource: Resource{ItemID: 1117, Kind: "solid", Count: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := Request{}
			planet := Planet{ID: 101, Buildable: true, Resources: []Resource{tc.resource}}
			if tc.prepare != nil {
				tc.prepare(&req, &planet)
			}
			before, _ := json.Marshal(req)
			got := planningDefaults(req, map[int64]Planet{101: planet}, tc.canResolve)
			wantCount := 0
			if tc.want {
				wantCount = 1
			}
			if ids := got.Quant.Settings.DirectResourceItemIDs; len(ids) != wantCount || tc.want && ids[0] != 1003 {
				t.Fatalf("默认采集选择错误：%v", ids)
			}
			after, _ := json.Marshal(req)
			if string(before) != string(after) {
				t.Fatal("默认值改写了用户原请求")
			}
		})
	}
}

// TestPlanningDefaultsPreserveBacking 职责：避免默认追加覆盖调用方切片余量；使用：引用别名回归；边界：不复制或改写无关映射。
func TestPlanningDefaultsPreserveBacking(t *testing.T) {
	backing := []int{1116, 777}
	req := Request{Preference: "energy", Quant: quant.Request{
		Settings: quant.Settings{DirectResourceItemIDs: backing[:1], MineralizeList: map[string]bool{"硅石": false, "氢": true}},
		Scheme: quant.Scheme{ItemRecipeChoices: map[string]int{"硅石": 0}, CostWeight: quant.CostWeight{
			BuildingCost: map[string]float64{"电弧熔炉": 3}, ItemExtraCost: map[string]quant.ItemCost{"铁矿": {Cost: 2}},
		}},
	}}
	before, _ := json.Marshal(req)
	got := planningDefaults(req, map[int64]Planet{101: {Buildable: true, Resources: []Resource{{ItemID: 1003, Kind: "solid", Count: 1}}}}, false)
	after, _ := json.Marshal(req)
	if !slices.Equal(backing, []int{1116, 777}) || string(before) != string(after) {
		t.Fatal("默认值改写了请求映射或切片底层数组")
	}
	if !slices.Equal(got.Quant.Settings.DirectResourceItemIDs, []int{1116, 1003}) || got.Quant.Scheme.CostWeight.Power != 10 {
		t.Fatalf("默认值未正确追加：%+v", got.Quant)
	}
}

// TestPlanningDefaultsEnergyWeight 职责：仅为能耗偏好的空权重补默认值；使用：偏好优先级；边界：已有非零值仍由量化自身校验。
func TestPlanningDefaultsEnergyWeight(t *testing.T) {
	for _, preference := range []string{"", "construction", "logistics", "game-load", "energy"} {
		for _, weight := range []float64{0, 2, -1} {
			req := Request{Preference: preference, Quant: quant.Request{Scheme: quant.Scheme{CostWeight: quant.CostWeight{Power: weight}}}}
			got := planningDefaults(req, nil, false)
			want := weight
			if preference == "energy" && weight == 0 {
				want = 10
			}
			if got.Quant.Scheme.CostWeight.Power != want || req.Quant.Scheme.CostWeight.Power != weight {
				t.Fatalf("偏好 %q 权重 %v 被错误覆盖：%v", preference, weight, got.Quant.Scheme.CostWeight.Power)
			}
		}
	}
}

// TestPlanningDefaultsSiliconQuantification 职责：核实默认来源改变真实原料链而非外供分类；使用：无矿和用户选择回归；边界：不执行实体布局。
func TestPlanningDefaultsSiliconQuantification(t *testing.T) {
	for _, mode := range []string{"no_ore", "manual", "external", "mining"} {
		t.Run(mode, func(t *testing.T) {
			req := Request{Quant: quant.Request{Targets: []quant.Target{{ItemID: 1003, RatePerMinute: 30}}}}
			planets := map[int64]Planet{101: {Buildable: true, Resources: []Resource{{ItemID: 1003, Kind: "solid", Count: 1}}}}
			switch mode {
			case "no_ore":
				planets = nil
			case "manual":
				req.Quant.Scheme.ItemRecipeChoices = map[string]int{"硅石": 1}
			case "external":
				req.Quant.Settings.MineralizeList = map[string]bool{"硅石": true}
			}
			plan, err := quant.Solve(context.Background(), planningDefaults(req, planets, false).Quant)
			if err != nil {
				t.Fatal(err)
			}
			item, rate := 1005, 300.0
			if mode == "external" || mode == "mining" {
				item, rate = 1003, 30
			}
			if len(plan.RawInputs) != 1 || plan.RawInputs[0].ItemID != item || plan.RawInputs[0].RatePerMinute != rate || slices.Contains(plan.ExternalInputItemIDs, 1003) != (mode == "external") {
				t.Fatalf("硅石来源不守恒：%+v，外供 %v", plan.RawInputs, plan.ExternalInputItemIDs)
			}
		})
	}
}

// TestPlanningDefaultsUniverseMatrix 职责：核对每分钟两千白矩阵的默认改善；使用：真实配方 LP 回归；边界：只衡量制造及原料需求，不宣称蓝图已落位。
func TestPlanningDefaultsUniverseMatrix(t *testing.T) {
	dataset, err := quant.LoadDataset("Vanilla")
	if err != nil {
		t.Fatal(err)
	}
	req := Request{Preference: "energy", Quant: quant.Request{Dataset: "Vanilla", Targets: []quant.Target{{ItemID: 6006, RatePerMinute: 2000}}}}
	for _, recipe := range dataset.Recipes {
		choice := quant.RecipeScheme{}
		if recipe.Proliferator&2 != 0 {
			choice.ProductionMode, choice.IncreasePoints = 2, 4
		}
		req.Quant.Scheme.SchemeForRecipe = append(req.Quant.Scheme.SchemeForRecipe, choice)
	}
	base, err := quant.Solve(context.Background(), req.Quant)
	if err != nil {
		t.Fatal(err)
	}
	planets := map[int64]Planet{101: {Buildable: true, Resources: []Resource{{ItemID: 1003, Kind: "solid", Count: 1}}}}
	optimized, err := quant.Solve(context.Background(), planningDefaults(req, planets, false).Quant)
	if err != nil {
		t.Fatal(err)
	}
	photonRate := func(plan quant.Result) float64 {
		for _, raw := range plan.RawInputs {
			if raw.ItemID == 1208 {
				return raw.RatePerMinute
			}
		}
		return 0
	}
	if optimized.MachineCount >= base.MachineCount || optimized.PowerMW >= base.PowerMW || photonRate(base) <= 10000 || math.Abs(photonRate(optimized)-1600) > quant.FlowTolerance(1600, photonRate(optimized)) || slices.Contains(optimized.ExternalInputItemIDs, 1003) {
		t.Fatalf("白矩阵默认优化未生效：机器 %d→%d，功率 %.3f→%.3f MW，光子 %.3f→%.3f/min", base.MachineCount, optimized.MachineCount, base.PowerMW, optimized.PowerMW, photonRate(base), photonRate(optimized))
	}
	energy := planningDefaults(req, planets, false)
	unlocked := World{RecipeUnlocked: func(id int) bool { return id == 58 }}
	candidate, cracked, use := compareOilCracking(t.Context(), energy, unlocked, optimized)
	if !use || candidate.Quant.Scheme.ItemRecipeChoices["氢"] == 0 || energy.Quant.Scheme.ItemRecipeChoices["氢"] != 0 ||
		len(cracked.Leftovers) != 0 || cracked.PowerMW >= optimized.PowerMW {
		t.Fatal("已解锁的裂解候选须改善功耗和余油，并保留输入的自动选项")
	}
	if _, _, use := compareOilCracking(t.Context(), energy, World{}, optimized); use {
		t.Fatal("科技未知时不能自动选择裂解")
	}
	energy.Quant.Scheme.ItemRecipeChoices = map[string]int{"氢": 1}
	if _, _, use := compareOilCracking(t.Context(), energy, unlocked, optimized); use {
		t.Fatal("自动配方覆盖了已指定的氢来源")
	}
	t.Logf("机器 %d→%d，功率 %.3f→%.3f MW，光子 %.3f→%.3f/min", base.MachineCount, optimized.MachineCount, base.PowerMW, optimized.PowerMW, photonRate(base), photonRate(optimized))

	// 单星、矿料外供，隔离戴森球额度和配方选择。建造偏好在选到符合功率的自动配方后仍然保留。
	req.Preference = "construction"
	mining := false
	req.EnableMining = &mining
	req.Dyson = DysonSettings{Mode: "sphere", Lenses: true, Supplies: []DysonSupply{{StarID: 1, PlanetIDs: []int64{101}, AvailableMW: 75000, Efficiency: .8, Uptime: 1}}}
	world := World{Planets: []Planet{{ID: 101, StarID: 1, Buildable: true, ResourcesKnown: true}}}
	result, err := Plan(t.Context(), req, world, nil, nil)
	if err != nil || result.Status != "conditional" || result.Preference != "construction" || math.Abs(photonRate(result.Plan)-1600) > 1e-5 {
		t.Fatalf("可用的自动配方未通过光子初筛：%v %s %.3f，%v", err, result.Status, photonRate(result.Plan), result.Warnings)
	}
	req.Quant.Targets = []quant.Target{{ItemID: 1208, RatePerMinute: 3001}}
	result, err = Plan(t.Context(), req, world, nil, nil)
	if err != nil || result.Status != "infeasible" {
		t.Fatalf("交付本身已超出戴森球 3000 光子/分的额度：%v %s", err, result.Status)
	}
}
